package bundle

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
		store: protokv.NewStore(kv, &bundleMessage),
	}
}

func (h *handler) Append(ctx context.Context, req *datastore.AppendBundleRequest) (*datastore.AppendBundleResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *handler) Create(ctx context.Context, req *datastore.CreateBundleRequest) (*datastore.CreateBundleResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *handler) Delete(ctx context.Context, req *datastore.DeleteBundleRequest) (*datastore.DeleteBundleResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *handler) Fetch(ctx context.Context, req *datastore.FetchBundleRequest) (*datastore.FetchBundleResponse, error) {
	in := &common.Bundle{
		TrustDomainId: req.TrustDomainId,
	}
	out := new(common.Bundle)
	if err := h.store.ReadProto(ctx, in, out); err != nil {
		if protokv.NotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		return nil, err
	}

	return &datastore.FetchBundleResponse{Bundle: out}, nil
}

func (h *handler) List(ctx context.Context, req *datastore.ListBundlesRequest) (*datastore.ListBundlesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *handler) Prune(ctx context.Context, req *datastore.PruneBundleRequest) (*datastore.PruneBundleResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *handler) Set(ctx context.Context, req *datastore.SetBundleRequest) (*datastore.SetBundleResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (h *handler) Update(ctx context.Context, req *datastore.UpdateBundleRequest) (*datastore.UpdateBundleResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
