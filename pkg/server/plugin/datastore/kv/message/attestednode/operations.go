package attestednode

import (
	"context"

	"github.com/spiffe/spire/pkg/server/plugin/datastore"
)

type Operations interface {
	Create(ctx context.Context, req *datastore.CreateAttestedNodeRequest) (*datastore.CreateAttestedNodeResponse, error)
	Delete(ctx context.Context, req *datastore.DeleteAttestedNodeRequest) (*datastore.DeleteAttestedNodeResponse, error)
	Fetch(ctx context.Context, req *datastore.FetchAttestedNodeRequest) (*datastore.FetchAttestedNodeResponse, error)
	List(ctx context.Context, req *datastore.ListAttestedNodesRequest) (*datastore.ListAttestedNodesResponse, error)
	Update(ctx context.Context, req *datastore.UpdateAttestedNodeRequest) (*datastore.UpdateAttestedNodeResponse, error)
}
