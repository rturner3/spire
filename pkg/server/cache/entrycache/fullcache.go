package entrycache

import (
	"context"
	"sync"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/spire/proto/spire/common"
)

var (
	selectorSetPool = sync.Pool{
		New: func() interface{} {
			return make(selectorSet)
		},
	}

	seenSetPool = sync.Pool{
		New: func() interface{} {
			return make(seenSet)
		},
	}
)

type selectorSet map[Selector]struct{}
type seenSet map[string]struct{}

type Selector struct {
	Type  string
	Value string
}

type EntryIterator interface {
	Next(ctx context.Context) bool
	Entry() *common.RegistrationEntry
	Err() error
}

type AgentIterator interface {
	Next(ctx context.Context) bool
	Agent() Agent
	Err() error
}

type Agent struct {
	ID        spiffeid.ID
	Selectors []*common.Selector
}

type aliasEntry struct {
	id    string
	entry *common.RegistrationEntry
}

type Cache struct {
	aliases map[string][]aliasEntry
	entries map[string][]*common.RegistrationEntry
}

func Build(ctx context.Context, entryIter EntryIterator, agentIter AgentIterator) (*Cache, error) {
	type aliasInfo struct {
		aliasEntry
		selectors selectorSet
	}
	bysel := make(map[Selector][]aliasInfo)

	entries := make(map[string][]*common.RegistrationEntry)
	for entryIter.Next(ctx) {
		entry := entryIter.Entry()
		parentID := spiffeIDFromProto(entry.ParentId)
		if isNodeEntry(entry) {
			alias := aliasInfo{
				aliasEntry: aliasEntry{
					id:    entry.SpiffeId,
					entry: entry,
				},
				selectors: selectorSetFromProto(entry.Selectors),
			}
			for selector := range alias.selectors {
				bysel[selector] = append(bysel[selector], alias)
			}
			continue
		}
		entries[parentID] = append(entries[parentID], entry)
	}
	if err := entryIter.Err(); err != nil {
		return nil, err
	}

	aliasSeen := allocSeenSet()
	defer freeSeenSet(aliasSeen)

	aliases := make(map[string][]aliasEntry)
	for agentIter.Next(ctx) {
		agent := agentIter.Agent()
		agentID := spiffeIDFromID(agent.ID)
		agentSelectors := selectorSetFromProto(agent.Selectors)
		// track which aliases we've evaluated so far to make sure we don't
		// add one twice.
		clearSeenSet(aliasSeen)
		for s := range agentSelectors {
			for _, alias := range bysel[s] {
				if _, ok := aliasSeen[alias.entry.EntryId]; ok {
					continue
				}
				aliasSeen[alias.entry.EntryId] = struct{}{}
				if isSubset(alias.selectors, agentSelectors) {
					aliases[agentID] = append(aliases[agentID], alias.aliasEntry)
				}
			}
		}
	}
	if err := agentIter.Err(); err != nil {
		return nil, err
	}

	return &Cache{
		aliases: aliases,
		entries: entries,
	}, nil
}

func (c *Cache) GetAuthorizedEntries(agentID string) []*common.RegistrationEntry {
	seen := allocSeenSet()
	defer freeSeenSet(seen)

	return c.getAuthorizedEntries(agentID, seen)
}

func (c *Cache) getAuthorizedEntries(id string, seen map[string]struct{}) []*common.RegistrationEntry {
	entries := c.crawl(id, seen)
	for _, descendant := range entries {
		entries = append(entries, c.getAuthorizedEntries(spiffeIDFromProto(descendant.SpiffeId), seen)...)
	}

	for _, alias := range c.aliases[id] {
		entries = append(entries, alias.entry)
		entries = append(entries, c.getAuthorizedEntries(alias.id, seen)...)
	}
	return entries
}

func (c *Cache) crawl(parentID string, seen map[string]struct{}) []*common.RegistrationEntry {
	if _, ok := seen[parentID]; ok {
		return nil
	}
	seen[parentID] = struct{}{}

	// Make a copy so that the entries aren't aliasing the backing array
	entries := append([]*common.RegistrationEntry(nil), c.entries[parentID]...)
	for _, entry := range entries {
		entries = append(entries, c.crawl(spiffeIDFromProto(entry.SpiffeId), seen)...)
	}
	return entries
}

func spiffeIDFromID(id spiffeid.ID) string {
	return id.String()
}

func spiffeIDFromProto(id string) string {
	return id
}

func selectorSetFromProto(selectors []*common.Selector) selectorSet {
	set := make(selectorSet, len(selectors))
	for _, selector := range selectors {
		set[Selector{Type: selector.Type, Value: selector.Value}] = struct{}{}
	}
	return set
}

func allocSelectorSet() selectorSet {
	return selectorSetPool.Get().(selectorSet)

}

func freeSelectorSet(set selectorSet) {
	clearSelectorSet(set)
	selectorSetPool.Put(set)
}

func clearSelectorSet(set selectorSet) {
	for k := range set {
		delete(set, k)
	}
}

func allocSeenSet() seenSet {
	return seenSetPool.Get().(seenSet)

}

func freeSeenSet(set seenSet) {
	clearSeenSet(set)
	seenSetPool.Put(set)
}

func clearSeenSet(set seenSet) {
	for k := range set {
		delete(set, k)
	}
}

func isSubset(sub, whole selectorSet) bool {
	if len(sub) > len(whole) {
		return false
	}
	for s := range sub {
		if _, ok := whole[s]; !ok {
			return false
		}
	}
	return true
}

func isNodeEntry(e *common.RegistrationEntry) bool {
	id, err := spiffeid.FromString(e.ParentId)
	return err == nil && id.Path() == "/spire/server"
}
