package entrycache

import (
	"context"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/spire/pkg/server/plugin/datastore"
	"github.com/spiffe/spire/proto/spire/common"
)

func BuildFromDataStore(ctx context.Context, ds datastore.DataStore) (*Cache, error) {
	return Build(ctx, makeEntryIteratorDS(ds), makeAgentIteratorDS(ds))
}

type entryIteratorDS struct {
	ds      datastore.DataStore
	entries []*common.RegistrationEntry
	next    int
	err     error
}

func makeEntryIteratorDS(ds datastore.DataStore) EntryIterator {
	return &entryIteratorDS{
		ds: ds,
	}
}

func (it *entryIteratorDS) Next(ctx context.Context) bool {
	if it.err != nil {
		return false
	}
	if it.entries == nil {
		resp, err := it.ds.ListRegistrationEntries(ctx, &datastore.ListRegistrationEntriesRequest{
			TolerateStale: true,
		})
		if err != nil {
			it.err = err
			return false
		}
		it.entries = resp.Entries
	}
	if it.next >= len(it.entries) {
		return false
	}
	it.next++
	return true
}

func (it *entryIteratorDS) Entry() *common.RegistrationEntry {
	return it.entries[it.next-1]
}

func (it *entryIteratorDS) Err() error {
	return it.err
}

type agentIteratorDS struct {
	ds     datastore.DataStore
	agents []Agent
	next   int
	err    error
}

func makeAgentIteratorDS(ds datastore.DataStore) AgentIterator {
	return &agentIteratorDS{
		ds: ds,
	}
}

func (it *agentIteratorDS) Next(ctx context.Context) bool {
	if it.err != nil {
		return false
	}
	if it.agents == nil {
		resp, err := it.ds.ListNodeSelectors(ctx, &datastore.ListNodeSelectorsRequest{
			TolerateStale: true,
		})
		if err != nil {
			it.err = err
			return false
		}
		agents := make([]Agent, 0, len(resp.Selectors))
		for _, selector := range resp.Selectors {
			agent := Agent{
				ID:        spiffeid.RequireFromString(selector.SpiffeId),
				Selectors: selector.Selectors,
			}
			agents = append(agents, agent)
		}
		it.agents = agents
	}
	if it.next >= len(it.agents) {
		return false
	}
	it.next++
	return true
}

func (it *agentIteratorDS) Agent() Agent {
	return it.agents[it.next-1]
}

func (it *agentIteratorDS) Err() error {
	return it.err
}
