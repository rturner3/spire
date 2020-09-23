package entrycache

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/spiffe/spire/pkg/server/plugin/datastore"

	"github.com/spiffe/go-spiffe/v2/spiffeid"

	"github.com/spiffe/spire/proto/spire/common"
	"github.com/spiffe/spire/test/fakes/fakedatastore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Maps selector type -> selector value
type selectorMap map[string][]string

func TestEntryIteratorDS(t *testing.T) {
	ds := fakedatastore.New(t)
	ctx := context.Background()

	t.Run("no entries", func(t *testing.T) {
		it := makeEntryIteratorDS(ds)
		assert.False(t, it.Next(ctx))
		assert.NoError(t, it.Err())
	})

	const numEntries = entryPageSize + 1
	const parentID = "spiffe://example.org/parent"
	const spiffeIDPrefix = "spiffe://example.org/entry"
	selectors := []*common.Selector{
		{Type: "doesn't", Value: "matter"},
	}
	entriesToCreate := make([]*common.RegistrationEntry, numEntries)
	for i := 0; i < numEntries; i++ {
		entriesToCreate[i] = &common.RegistrationEntry{
			ParentId:  parentID,
			SpiffeId:  spiffeIDPrefix + strconv.Itoa(i),
			Selectors: selectors,
		}
	}

	expectedEntries := make([]*common.RegistrationEntry, len(entriesToCreate))
	for i, e := range entriesToCreate {
		expectedEntries[i] = createRegistrationEntry(ctx, t, ds, e)
	}

	t.Run("multiple pages", func(t *testing.T) {
		it := makeEntryIteratorDS(ds)
		var entries []*common.RegistrationEntry
		for i := 0; i < numEntries; i++ {
			assert.True(t, it.Next(ctx))
			require.NoError(t, it.Err())

			entry := it.Entry()
			require.NotNil(t, entry)
			entries = append(entries, entry)
		}

		assert.False(t, it.Next(ctx))
		assert.NoError(t, it.Err())
		assert.ElementsMatch(t, expectedEntries, entries)
	})

	t.Run("multiple pages with datastore error in-between pages", func(t *testing.T) {
		it := makeEntryIteratorDS(ds)
		for i := 0; i < entryPageSize; i++ {
			assert.True(t, it.Next(ctx))
			require.NoError(t, it.Err())
		}

		dsErr := errors.New("some datastore error")
		ds.SetNextError(dsErr)
		assert.False(t, it.Next(ctx))
		assert.Error(t, it.Err())
		// it.Next() returns false after encountering an error on previous call to Next()
		assert.False(t, it.Next(ctx))
	})
}

func TestAgentIteratorDS(t *testing.T) {
	ds := fakedatastore.New(t)
	ctx := context.Background()

	t.Run("no entries", func(t *testing.T) {
		it := makeAgentIteratorDS(ds)
		assert.False(t, it.Next(ctx))
		assert.NoError(t, it.Err())
	})

	const numAgents = agentPageSize + 1
	selectors := []*common.Selector{
		{Type: "a", Value: "1"},
		{Type: "b", Value: "2"},
		{Type: "c", Value: "3"},
	}

	selMap := make(selectorMap)
	for _, selector := range selectors {
		selMap[selector.Type] = []string{selector.Value}
	}

	// Map containing Agent SPIFFE ID -> {Selector type -> Selector value}
	expectedAgents := make(map[spiffeid.ID]map[string][]string, numAgents)
	for i := 0; i < numAgents; i++ {
		iterStr := strconv.Itoa(i)
		agentID, err := spiffeid.FromString("spiffe://example.org/spire/agent/agent" + iterStr)
		require.NoError(t, err)
		agentIDStr := agentID.String()
		node := &common.AttestedNode{
			SpiffeId:            agentIDStr,
			AttestationDataType: testNodeAttestor,
			CertSerialNumber:    iterStr,
			CertNotAfter:        time.Now().Add(24 * time.Hour).Unix(),
		}

		createAttestedNode(t, ds, node)
		setNodeSelectors(ctx, t, ds, agentIDStr, selectors...)
		expectedAgents[agentID] = selMap
	}

	t.Run("multiple pages", func(t *testing.T) {
		it := makeAgentIteratorDS(ds)
		agents := make([]Agent, numAgents)
		for i := 0; i < numAgents; i++ {
			assert.True(t, it.Next(ctx))
			assert.NoError(t, it.Err())
			agents[i] = it.Agent()
		}

		assert.False(t, it.Next(ctx))
		require.NoError(t, it.Err())
		require.Len(t, agents, len(expectedAgents))
		for _, agent := range agents {
			expectedAgentSelectorMap, ok := expectedAgents[agent.ID]
			require.True(t, ok)
			require.Len(t, agent.Selectors, len(expectedAgentSelectorMap))
			for _, selector := range agent.Selectors {
				expectedSelectorValues, ok := expectedAgentSelectorMap[selector.Type]
				require.True(t, ok)
				assert.Contains(t, expectedSelectorValues, selector.Value)
			}
		}
	})

	t.Run("datastore error", func(t *testing.T) {
		it := makeAgentIteratorDS(ds)
		ds.SetNextError(errors.New("some datastore error"))
		assert.False(t, it.Next(ctx))
		assert.Error(t, it.Err())
		// it.Next() returns false after encountering an error on previous call to Next()
		assert.False(t, it.Next(ctx))
	})
}

func createAttestedNode(t testing.TB, ds datastore.DataStore, node *common.AttestedNode) {
	req := &datastore.CreateAttestedNodeRequest{
		Node: node,
	}

	_, err := ds.CreateAttestedNode(context.Background(), req)
	require.NoError(t, err)
}
