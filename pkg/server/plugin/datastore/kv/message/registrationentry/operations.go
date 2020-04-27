package registrationentry

import (
	"context"

	"github.com/spiffe/spire/pkg/server/plugin/datastore"
)

type Operations interface {
	Create(ctx context.Context, req *datastore.CreateRegistrationEntryRequest) (*datastore.CreateRegistrationEntryResponse, error)
	Delete(ctx context.Context, req *datastore.DeleteRegistrationEntryRequest) (*datastore.DeleteRegistrationEntryResponse, error)
	Fetch(ctx context.Context, req *datastore.FetchRegistrationEntryRequest) (*datastore.FetchRegistrationEntryResponse, error)
	List(ctx context.Context, req *datastore.ListRegistrationEntriesRequest) (*datastore.ListRegistrationEntriesResponse, error)
	Prune(ctx context.Context, req *datastore.PruneRegistrationEntriesRequest) (*datastore.PruneRegistrationEntriesResponse, error)
	Update(ctx context.Context, req *datastore.UpdateRegistrationEntryRequest) (*datastore.UpdateRegistrationEntryResponse, error)
}
