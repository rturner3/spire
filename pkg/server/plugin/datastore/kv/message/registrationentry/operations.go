package registrationentry

import (
	"context"

	"github.com/spiffe/spire/pkg/server/plugin/datastore"
)

type Operations interface {
	Create(ctx context.Context, req *datastore.CreateRegistrationEntryRequest) (*datastore.CreateRegistrationEntryResponse, error)
	Fetch(ctx context.Context, req *datastore.FetchRegistrationEntryRequest) (*datastore.FetchRegistrationEntryResponse, error)
	List(ctx context.Context, req *datastore.ListRegistrationEntriesRequest) (*datastore.ListRegistrationEntriesResponse, error)
}
