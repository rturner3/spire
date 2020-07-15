package nodeselector

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"github.com/hashicorp/go-hclog"
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore"
	"github.com/zeebo/errs"
)

type handler struct {
	log   hclog.Logger
	store *protokv.Store
}

func New(kv protokv.KV, log hclog.Logger) Operations {
	return &handler{
		log:   log,
		store: protokv.NewStore(kv, &Message),
	}
}

func (h *handler) Get(ctx context.Context, req *datastore.GetNodeSelectorsRequest) (*datastore.GetNodeSelectorsResponse, error) {
	in := &datastore.NodeSelectors{SpiffeId: req.SpiffeId}
	out := new(datastore.NodeSelectors)
	if err := h.store.ReadProto(ctx, in, out); err != nil {
		if protokv.NotFound.Has(err) {
			// Forcibly suppressing this error to keep the plugin compliant with the existing behavior of the sql plugin.
			// TODO: Evaluate whether we can return a NotFound gRPC error here and in the sql plugin.
			return &datastore.GetNodeSelectorsResponse{
				Selectors: &datastore.NodeSelectors{
					SpiffeId: req.SpiffeId,
				},
			}, nil
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
