package jointoken

import (
	"context"

	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type handler struct {
	store *protokv.Store
}

func New(kv protokv.KV) Operations {
	return &handler{
		store: protokv.NewStore(kv, &JoinTokenMessage),
	}
}

func (h *handler) Create(ctx context.Context, req *datastore.CreateJoinTokenRequest) (*datastore.CreateJoinTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *handler) Delete(ctx context.Context, req *datastore.DeleteJoinTokenRequest) (*datastore.DeleteJoinTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *handler) Fetch(ctx context.Context, req *datastore.FetchJoinTokenRequest) (*datastore.FetchJoinTokenResponse, error) {
	in := &datastore.JoinToken{Token: req.Token}
	out := new(datastore.JoinToken)
	if err := h.store.ReadProto(ctx, in, out); err != nil {
		if protokv.NotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		return nil, err
	}

	return &datastore.FetchJoinTokenResponse{JoinToken: out}, nil
}

func (h *handler) Prune(ctx context.Context, req *datastore.PruneJoinTokensRequest) (*datastore.PruneJoinTokensResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
