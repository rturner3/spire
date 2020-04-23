package attestednode

import (
	"context"

	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore"
	"github.com/spiffe/spire/proto/spire/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type handler struct {
	store *protokv.Store
}

func New(kv protokv.KV) Operations {
	return &handler{
		store: protokv.NewStore(kv, &attestedNodeMessage),
	}
}

func (h *handler) Create(ctx context.Context, req *datastore.CreateAttestedNodeRequest) (*datastore.CreateAttestedNodeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *handler) Delete(ctx context.Context, req *datastore.DeleteAttestedNodeRequest) (*datastore.DeleteAttestedNodeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *handler) Fetch(ctx context.Context, req *datastore.FetchAttestedNodeRequest) (*datastore.FetchAttestedNodeResponse, error) {
	in := &common.AttestedNode{SpiffeId: req.SpiffeId}
	out := new(common.AttestedNode)
	if err := h.store.ReadProto(ctx, in, out); err != nil {
		if protokv.NotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		return nil, err
	}

	return &datastore.FetchAttestedNodeResponse{Node: out}, nil
}

func (h *handler) List(ctx context.Context, req *datastore.ListAttestedNodesRequest) (*datastore.ListAttestedNodesResponse, error) {
	// TODO: protokv does not have support yet for subsets of indices, i.e., what
	// would be needed to implement ByExpiresFor
	if req.ByExpiresBefore != nil {
		return nil, status.Error(codes.Unimplemented, "by-expires-before support not implemented")
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *handler) Update(ctx context.Context, req *datastore.UpdateAttestedNodeRequest) (*datastore.UpdateAttestedNodeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
