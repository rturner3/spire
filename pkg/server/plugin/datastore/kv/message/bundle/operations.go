package bundle

import (
	"context"

	"github.com/spiffe/spire/pkg/server/plugin/datastore"
)

type Operations interface {
	Append(ctx context.Context, req *datastore.AppendBundleRequest) (*datastore.AppendBundleResponse, error)
	Create(ctx context.Context, req *datastore.CreateBundleRequest) (*datastore.CreateBundleResponse, error)
	Delete(ctx context.Context, req *datastore.DeleteBundleRequest) (*datastore.DeleteBundleResponse, error)
	Fetch(ctx context.Context, req *datastore.FetchBundleRequest) (*datastore.FetchBundleResponse, error)
	List(ctx context.Context, req *datastore.ListBundlesRequest) (*datastore.ListBundlesResponse, error)
	Prune(ctx context.Context, req *datastore.PruneBundleRequest) (*datastore.PruneBundleResponse, error)
	Set(ctx context.Context, req *datastore.SetBundleRequest) (*datastore.SetBundleResponse, error)
	Update(ctx context.Context, req *datastore.UpdateBundleRequest) (*datastore.UpdateBundleResponse, error)
}
