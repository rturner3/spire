package registrationentry

import (
	"context"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore"
	"github.com/spiffe/spire/proto/spire/common"
	"github.com/zeebo/errs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type handler struct {
	store *protokv.Store
}

func New(kv protokv.KV) Operations {
	return &handler{
		store: protokv.NewStore(kv, &RegistrationEntryMessage),
	}
}

func (h *handler) Create(ctx context.Context, req *datastore.CreateRegistrationEntryRequest) (*datastore.CreateRegistrationEntryResponse, error) {
	var err error
	req.Entry.EntryId, err = newEntryID()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	value, err := proto.Marshal(req.Entry)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	if err := h.store.Create(ctx, value); err != nil {
		return nil, errs.Wrap(err)
	}

	return &datastore.CreateRegistrationEntryResponse{
		Entry: req.Entry,
	}, nil
}

func (h *handler) Delete(ctx context.Context, req *datastore.DeleteRegistrationEntryRequest) (*datastore.DeleteRegistrationEntryResponse, error) {
	in := &common.RegistrationEntry{
		EntryId: req.EntryId,
	}

	out := &common.RegistrationEntry{}
	if err := h.store.ReadProto(ctx, in, out); err != nil {
		if protokv.NotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, "registration entry not found for entry id %s", req.EntryId)
		}
		return nil, err
	}

	outBytes, err := proto.Marshal(out)
	if err != nil {
		return nil, err
	}

	if err := h.store.Delete(ctx, outBytes); err != nil {
		return nil, err
	}

	return &datastore.DeleteRegistrationEntryResponse{
		Entry: out,
	}, nil
}

func (h *handler) Fetch(ctx context.Context, req *datastore.FetchRegistrationEntryRequest) (*datastore.FetchRegistrationEntryResponse, error) {
	in := &common.RegistrationEntry{
		EntryId: req.EntryId,
	}
	out := &common.RegistrationEntry{}
	if err := h.store.ReadProto(ctx, in, out); err != nil {
		if protokv.NotFound.Has(err) {
			return &datastore.FetchRegistrationEntryResponse{}, nil
		}
		return nil, err
	}

	return &datastore.FetchRegistrationEntryResponse{Entry: out}, nil
}

func (h *handler) List(ctx context.Context, req *datastore.ListRegistrationEntriesRequest) (*datastore.ListRegistrationEntriesResponse, error) {
	if req.Pagination != nil && req.Pagination.PageSize <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "cannot paginate with pagesize = %d", req.Pagination.PageSize)
	}
	if req.BySelectors != nil && len(req.BySelectors.Selectors) == 0 {
		return nil, status.Error(codes.InvalidArgument, "cannot list by empty selector set")
	}

	type selectorKey struct {
		Type  string
		Value string
	}
	var selectorSet map[selectorKey]struct{}
	if req.BySelectors != nil {
		selectorSet = make(map[selectorKey]struct{})
		for _, s := range req.BySelectors.Selectors {
			selectorSet[selectorKey{Type: s.Type, Value: s.Value}] = struct{}{}
		}
	}

	for {
		resp, err := h.listRegistrationEntriesOnce(ctx, req)
		if err != nil {
			return nil, err
		}

		// Not filtering by selectors? return what we've got
		if req.BySelectors == nil ||
			len(req.BySelectors.Selectors) == 0 {
			return resp, nil
		}

		matching := make([]*common.RegistrationEntry, 0, len(resp.Entries))
		for _, entry := range resp.Entries {
			matches := true
			switch req.BySelectors.Match {
			case datastore.BySelectors_MATCH_SUBSET:
				for _, s := range entry.Selectors {
					if _, ok := selectorSet[selectorKey{Type: s.Type, Value: s.Value}]; !ok {
						matches = false
						break
					}
				}
			case datastore.BySelectors_MATCH_EXACT:
				// The listing currently contains all entries that have AT LEAST
				// the provided selectors. We only want those that match exactly.
				matches = len(entry.Selectors) == len(selectorSet)
			}
			if matches {
				matching = append(matching, entry)
			}
		}
		resp.Entries = matching

		if len(resp.Entries) > 0 || resp.Pagination == nil || len(resp.Pagination.Token) == 0 {
			return resp, nil
		}

		req.Pagination = resp.Pagination
	}
}

func (h *handler) Prune(ctx context.Context, req *datastore.PruneRegistrationEntriesRequest) (*datastore.PruneRegistrationEntriesResponse, error) {
	var token []byte
	var limit int
	values, _, err := h.store.Page(ctx, token, limit)
	if err != nil {
		return nil, err
	}

	entries := make([]*common.RegistrationEntry, len(values))
	for i, entryBytes := range values {
		entry := &common.RegistrationEntry{}
		if err := proto.Unmarshal(entryBytes, entry); err != nil {
			return nil, err
		}
		entries[i] = entry
	}

	// TODO: Consider creating a "batch" Delete() API in protokv to minimize query load
	entriesToPrune := h.entriesToPrune(entries, req.ExpiresBefore)

	var errors []error
	for _, entryToPrune := range entriesToPrune {
		entryBytes, err := proto.Marshal(entryToPrune)
		if err != nil {
			return nil, err
		}

		if err := h.store.Delete(ctx, entryBytes); err != nil {
			// Don't block entire operation on one delete failure
			errors = append(errors, err)
		}
	}

	numErrs := len(errors)
	if numErrs > 0 {
		return nil, fmt.Errorf("failed to delete %v registration entries", numErrs)
	}

	return &datastore.PruneRegistrationEntriesResponse{}, nil
}

func (h *handler) Update(ctx context.Context, req *datastore.UpdateRegistrationEntryRequest) (*datastore.UpdateRegistrationEntryResponse, error) {
	if req.Entry == nil {
		return nil, status.Errorf(codes.InvalidArgument, "entry cannot be nil")
	}

	reqEntry := req.Entry

	in := &common.RegistrationEntry{
		EntryId: reqEntry.EntryId,
	}

	out := &common.RegistrationEntry{}
	if err := h.store.ReadProto(ctx, in, out); err != nil {
		if protokv.NotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, "registration entry not found for entry id %v", reqEntry.EntryId)
		}
		return nil, err
	}

	out.Admin = reqEntry.Admin
	out.DnsNames = reqEntry.DnsNames
	out.Downstream = reqEntry.Downstream
	out.EntryExpiry = reqEntry.EntryExpiry
	out.EntryId = reqEntry.EntryId
	out.FederatesWith = reqEntry.FederatesWith
	out.ParentId = reqEntry.ParentId
	out.Selectors = reqEntry.Selectors
	out.Ttl = reqEntry.Ttl

	outBytes, err := proto.Marshal(out)
	if err != nil {
		return nil, err
	}

	if err := h.store.Update(ctx, outBytes); err != nil {
		return nil, err
	}

	return &datastore.UpdateRegistrationEntryResponse{
		Entry: out,
	}, nil
}

func (h *handler) listRegistrationEntriesOnce(ctx context.Context,
	req *datastore.ListRegistrationEntriesRequest) (*datastore.ListRegistrationEntriesResponse, error) {

	msg := &common.RegistrationEntry{}

	var fields []protokv.Field
	var setOps []protokv.SetOp
	if req.BySelectors != nil {
		msg.Selectors = req.BySelectors.Selectors
		switch req.BySelectors.Match {
		case datastore.BySelectors_MATCH_SUBSET:
			fields = append(fields, selectorsField)
			setOps = append(setOps, protokv.SetUnion)
		case datastore.BySelectors_MATCH_EXACT:
			fields = append(fields, selectorsField)
			setOps = append(setOps, protokv.SetIntersect)
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unhandled match behavior %q", req.BySelectors.Match)
		}
	}
	if req.ByParentId != nil {
		msg.ParentId = req.ByParentId.Value
		fields = append(fields, parentIdField)
		setOps = append(setOps, protokv.SetDefault)
	}
	if req.BySpiffeId != nil {
		msg.SpiffeId = req.BySpiffeId.Value
		fields = append(fields, spiffeIdField)
		setOps = append(setOps, protokv.SetDefault)
	}

	var token []byte
	var limit int
	var err error
	if req.Pagination != nil {
		if len(req.Pagination.Token) > 0 {
			token, err = decodePaginationToken(req.Pagination.Token)
			if err != nil {
				return nil, err
			}
		}
		limit = int(req.Pagination.PageSize)
	}

	var values [][]byte
	if len(fields) == 0 {
		values, token, err = h.store.Page(ctx, token, limit)
	} else {
		msgBytes, err := proto.Marshal(msg)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		values, token, err = h.store.PageIndex(ctx, msgBytes, token, limit, fields, setOps)
	}
	if err != nil {
		return nil, errs.Wrap(err)
	}

	resp := &datastore.ListRegistrationEntriesResponse{}
	for _, value := range values {
		entry := &common.RegistrationEntry{}
		if err := proto.Unmarshal(value, entry); err != nil {
			return nil, errs.Wrap(err)
		}
		resp.Entries = append(resp.Entries, entry)
	}
	if req.Pagination != nil {
		resp.Pagination = &datastore.Pagination{
			Token: encodePaginationToken(token),
		}
	}
	return resp, nil
}

func (h *handler) entriesToPrune(entries []*common.RegistrationEntry, expiresBeforeSeconds int64) []*common.RegistrationEntry {
	var entriesToPrune []*common.RegistrationEntry
	for _, entry := range entries {
		if entry.EntryExpiry < expiresBeforeSeconds {
			entriesToPrune = append(entriesToPrune, entry)
		}
	}

	return entriesToPrune
}
