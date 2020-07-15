package bundle

import (
	"context"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/hashicorp/go-hclog"
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/common/bundleutil"
	"github.com/spiffe/spire/pkg/server/plugin/datastore"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message/registrationentry"
	"github.com/spiffe/spire/proto/spire/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type handler struct {
	log           hclog.Logger
	store         *protokv.Store
	regEntryStore *protokv.Store
}

func New(kv protokv.KV, log hclog.Logger) Operations {
	return &handler{
		log:           log,
		store:         protokv.NewStore(kv, &Message),
		regEntryStore: protokv.NewStore(kv, &registrationentry.Message),
	}
}

func (h *handler) Append(ctx context.Context, req *datastore.AppendBundleRequest) (*datastore.AppendBundleResponse, error) {
	if req.Bundle == nil {
		return nil, status.Error(codes.InvalidArgument, "bundle must be non-nil")
	}

	in := &common.Bundle{
		TrustDomainId: req.Bundle.TrustDomainId,
	}

	out := &common.Bundle{}
	if err := h.store.ReadProto(ctx, in, out); err != nil {
		if protokv.NotFound.Has(err) {
			return h.appendToNonExistentBundle(ctx, req.Bundle)
		}

		return nil, err
	}

	bundle, changed := bundleutil.MergeBundles(out, req.Bundle)
	if changed {
		if err := h.updateBundle(ctx, bundle); err != nil {
			return nil, err
		}
	}

	return &datastore.AppendBundleResponse{
		Bundle: bundle,
	}, nil
}

func (h *handler) Create(ctx context.Context, req *datastore.CreateBundleRequest) (*datastore.CreateBundleResponse, error) {
	if req.Bundle == nil {
		return nil, status.Error(codes.InvalidArgument, "bundle must be non-nil")
	}

	if err := h.createBundle(ctx, req.Bundle); err != nil {
		return nil, err
	}
	return &datastore.CreateBundleResponse{
		Bundle: req.Bundle,
	}, nil
}

func (h *handler) Delete(ctx context.Context, req *datastore.DeleteBundleRequest) (*datastore.DeleteBundleResponse, error) {
	switch req.Mode {
	case datastore.DeleteBundleRequest_RESTRICT:
		return h.deleteRestrictMode(ctx, req)
	case datastore.DeleteBundleRequest_DELETE:
		return h.deleteDeleteMode(ctx, req)
	case datastore.DeleteBundleRequest_DISSOCIATE:
		return h.deleteDissociateMode(ctx, req)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unrecognized delete mode %v", req.Mode)
	}
}

func (h *handler) Fetch(ctx context.Context, req *datastore.FetchBundleRequest) (*datastore.FetchBundleResponse, error) {
	in := &common.Bundle{
		TrustDomainId: req.TrustDomainId,
	}
	out := new(common.Bundle)
	if err := h.store.ReadProto(ctx, in, out); err != nil {
		if protokv.NotFound.Has(err) {
			return &datastore.FetchBundleResponse{}, nil
		}
		return nil, err
	}

	return &datastore.FetchBundleResponse{Bundle: out}, nil
}

func (h *handler) List(ctx context.Context, req *datastore.ListBundlesRequest) (*datastore.ListBundlesResponse, error) {
	var token []byte
	var limit int
	var err error
	if req.Pagination != nil {
		if len(req.Pagination.Token) > 0 {
			token, err = decodePaginationToken(req.Pagination.Token)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid pagination token: %s", req.Pagination.Token)
			}
		}

		limit = int(req.Pagination.PageSize)
	}

	values, token, err := h.store.Page(ctx, token, limit)
	if err != nil {
		return nil, err
	}

	bundles := make([]*common.Bundle, len(values))
	for i := 0; i < len(values); i++ {
		bundles[i] = &common.Bundle{}
		if err = proto.Unmarshal(values[i], bundles[i]); err != nil {
			return nil, err
		}
	}

	var pagination *datastore.Pagination
	if req.Pagination != nil {
		pagination = &datastore.Pagination{
			Token: encodePaginationToken(token),
		}
	}

	return &datastore.ListBundlesResponse{
		Bundles:    bundles,
		Pagination: pagination,
	}, nil
}

func (h *handler) Prune(ctx context.Context, req *datastore.PruneBundleRequest) (*datastore.PruneBundleResponse, error) {
	in := &common.Bundle{
		TrustDomainId: req.TrustDomainId,
	}

	current := &common.Bundle{}
	if err := h.store.ReadProto(ctx, in, current); err != nil {
		if protokv.NotFound.Has(err) {
			return &datastore.PruneBundleResponse{
				BundleChanged: false,
			}, nil
		}
		return nil, err
	}

	newBundle, changed, err := bundleutil.PruneBundle(current, time.Unix(req.ExpiresBefore, 0), h.log)
	if err != nil {
		return nil, fmt.Errorf("prune failed: %v", err)
	}

	if changed {
		if err := h.updateBundle(ctx, newBundle); err != nil {
			return nil, err
		}
	}

	return &datastore.PruneBundleResponse{
		BundleChanged: changed,
	}, nil
}

func (h *handler) Set(ctx context.Context, req *datastore.SetBundleRequest) (*datastore.SetBundleResponse, error) {
	if req.Bundle == nil {
		return nil, status.Error(codes.InvalidArgument, "bundle must be non-nil")
	}

	value, err := proto.Marshal(req.Bundle)
	if err != nil {
		return nil, err
	}

	if err := h.store.Upsert(ctx, value); err != nil {
		return nil, err
	}

	return &datastore.SetBundleResponse{
		Bundle: req.Bundle,
	}, nil
}

func (h *handler) Update(ctx context.Context, req *datastore.UpdateBundleRequest) (*datastore.UpdateBundleResponse, error) {
	if req.Bundle == nil {
		return nil, status.Error(codes.InvalidArgument, "bundle must be non-nil")
	}

	if err := h.updateBundle(ctx, req.Bundle); err != nil {
		return nil, err
	}

	return &datastore.UpdateBundleResponse{
		Bundle: req.Bundle,
	}, nil
}

func (h *handler) appendToNonExistentBundle(ctx context.Context, bundle *common.Bundle) (*datastore.AppendBundleResponse, error) {
	if err := h.createBundle(ctx, bundle); err != nil {
		return nil, err
	}

	return &datastore.AppendBundleResponse{
		Bundle: bundle,
	}, nil
}

func (h *handler) createBundle(ctx context.Context, bundle *common.Bundle) error {
	value, err := proto.Marshal(bundle)
	if err != nil {
		return err
	}

	if err := h.store.Create(ctx, value); err != nil {
		return err
	}

	return nil
}

func (h *handler) deleteRestrictMode(ctx context.Context, req *datastore.DeleteBundleRequest) (*datastore.DeleteBundleResponse, error) {
	federatedEntries, err := h.fetchFederatedEntries(ctx, req.TrustDomainId)
	if err != nil {
		return nil, err
	}

	if len(federatedEntries) > 0 {
		return nil, fmt.Errorf("cannot delete bundle; federated with %d registration entries", len(federatedEntries))
	}

	return h.deleteBundle(ctx, req)
}

func (h *handler) deleteDeleteMode(ctx context.Context, req *datastore.DeleteBundleRequest) (*datastore.DeleteBundleResponse, error) {
	// TODO: DELETE mode is currently subject to various race conditions because it needs to be executed as a transaction.
	//  This requires protokv.Store to support transactions.
	federatedEntries, err := h.fetchFederatedEntries(ctx, req.TrustDomainId)
	if err != nil {
		return nil, err
	}

	if err := h.deleteEntries(ctx, federatedEntries); err != nil {
		return nil, err
	}

	return h.deleteBundle(ctx, req)
}

func (h *handler) deleteDissociateMode(ctx context.Context, req *datastore.DeleteBundleRequest) (*datastore.DeleteBundleResponse, error) {
	// TODO: DISSOCIATE mode is currently subject to various race conditions because it needs to be executed as a transaction.
	//  This requires protokv.Store to support transactions.
	federatedEntries, err := h.fetchFederatedEntries(ctx, req.TrustDomainId)
	if err != nil {
		return nil, err
	}

	if err := h.dissociateEntries(ctx, req.TrustDomainId, federatedEntries); err != nil {
		return nil, err
	}

	return h.deleteBundle(ctx, req)
}

func (h *handler) deleteBundle(ctx context.Context, req *datastore.DeleteBundleRequest) (*datastore.DeleteBundleResponse, error) {
	in := &common.Bundle{
		TrustDomainId: req.TrustDomainId,
	}

	value, err := proto.Marshal(in)
	if err != nil {
		return nil, err
	}

	if err := h.store.Delete(ctx, value); err != nil {
		if protokv.NotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, "record not found")
		}
		return nil, err
	}

	return &datastore.DeleteBundleResponse{
		Bundle: in,
	}, nil
}

func (h *handler) fetchFederatedEntries(ctx context.Context, trustDomainID string) ([]*common.RegistrationEntry, error) {
	regEntryIn := &common.RegistrationEntry{
		FederatesWith: []string{trustDomainID},
	}

	regEntryValue, err := proto.Marshal(regEntryIn)
	if err != nil {
		return nil, err
	}

	var token []byte
	var limit int
	fields := []protokv.Field{registrationentry.FederatesWithField}
	setOps := []protokv.SetOp{protokv.SetUnion}
	values, _, err := h.regEntryStore.PageIndex(ctx, regEntryValue, token, limit, fields, setOps)
	if err != nil {
		return nil, err
	}

	entries := make([]*common.RegistrationEntry, len(values))
	for i, value := range values {
		entries[i] = &common.RegistrationEntry{}
		if err := proto.Unmarshal(value, entries[i]); err != nil {
			return nil, err
		}
	}

	return entries, nil
}

func (h *handler) deleteEntries(ctx context.Context, entries []*common.RegistrationEntry) error {
	for _, entry := range entries {
		if err := h.deleteEntry(ctx, entry); err != nil {
			return err
		}
	}

	return nil
}

func (h *handler) deleteEntry(ctx context.Context, entry *common.RegistrationEntry) error {
	value, err := proto.Marshal(entry)
	if err != nil {
		return err
	}

	return h.regEntryStore.Delete(ctx, value)
}

func (h *handler) dissociateEntries(ctx context.Context, trustDomainID string, entries []*common.RegistrationEntry) error {
	for _, entry := range entries {
		if err := h.dissociateEntry(ctx, trustDomainID, entry); err != nil {
			return err
		}
	}

	return nil
}

func (h *handler) dissociateEntry(ctx context.Context, trustDomainID string, entry *common.RegistrationEntry) error {
	trustDomainIndex := -1
	for i, trustDomain := range entry.FederatesWith {
		if trustDomain == trustDomainID {
			trustDomainIndex = i
			break
		}
	}

	if trustDomainIndex < 0 {
		// This should never happen if the query for the entries is working properly
		return fmt.Errorf("no federated trust domain for bundle in registration entry with entry ID %s", entry.EntryId)
	}

	entry.FederatesWith = deleteFromIndex(trustDomainIndex, entry.FederatesWith)

	in, err := proto.Marshal(entry)
	if err != nil {
		return err
	}

	return h.regEntryStore.Update(ctx, in)
}

func (h *handler) updateBundle(ctx context.Context, bundle *common.Bundle) error {
	value, err := proto.Marshal(bundle)
	if err != nil {
		return err
	}

	if err := h.store.Update(ctx, value); err != nil {
		return err
	}

	return nil
}

func deleteFromIndex(idx int, arr []string) []string {
	return append(arr[:idx], arr[idx+1:]...)
}
