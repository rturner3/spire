package registrationentry

import (
	"context"

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
		store: protokv.NewStore(kv, &registrationEntryMessage),
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

func (h *handler) Fetch(ctx context.Context, req *datastore.FetchRegistrationEntryRequest) (*datastore.FetchRegistrationEntryResponse, error) {
	in := &common.RegistrationEntry{
		EntryId: req.EntryId,
	}
	out := new(common.RegistrationEntry)
	if err := h.store.ReadProto(ctx, in, out); err != nil {
		if protokv.NotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
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

func (h *handler) listRegistrationEntriesOnce(ctx context.Context,
	req *datastore.ListRegistrationEntriesRequest) (*datastore.ListRegistrationEntriesResponse, error) {

	msg := new(common.RegistrationEntry)

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

	resp := new(datastore.ListRegistrationEntriesResponse)
	for _, value := range values {
		entry := new(common.RegistrationEntry)
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
