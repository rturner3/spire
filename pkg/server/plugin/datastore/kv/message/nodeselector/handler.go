package nodeselector

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore"
	"github.com/zeebo/errs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type handler struct {
	store *protokv.Store
}

func New(kv protokv.KV) Operations {
	return &handler{
		store: protokv.NewStore(kv, &nodeSelectorMessage),
	}
}

func (h *handler) Get(ctx context.Context, req *datastore.GetNodeSelectorsRequest) (*datastore.GetNodeSelectorsResponse, error) {
	in := &datastore.NodeSelectors{SpiffeId: req.SpiffeId}
	out := new(datastore.NodeSelectors)
	if err := h.store.ReadProto(ctx, in, out); err != nil {
		if protokv.NotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		return nil, err
	}

	return &datastore.GetNodeSelectorsResponse{Selectors: out}, nil
}

func (h *handler) Set(ctx context.Context, req *datastore.SetNodeSelectorsRequest) (*datastore.SetNodeSelectorsResponse, error) {
	value, err := proto.Marshal(req.Selectors)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	if err := h.store.Upsert(ctx, value); err != nil {
		return nil, errs.Wrap(err)
	}
	return &datastore.SetNodeSelectorsResponse{}, nil
}
