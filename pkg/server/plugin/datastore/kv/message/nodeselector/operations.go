package nodeselector

import (
	"context"

	"github.com/spiffe/spire/pkg/server/plugin/datastore"
)

type Operations interface {
	Get(ctx context.Context, req *datastore.GetNodeSelectorsRequest) (*datastore.GetNodeSelectorsResponse, error)
	Set(ctx context.Context, req *datastore.SetNodeSelectorsRequest) (*datastore.SetNodeSelectorsResponse, error)
}
