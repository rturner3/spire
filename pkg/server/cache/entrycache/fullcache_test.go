package entrycache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/spire/pkg/server/plugin/datastore"
	sqlds "github.com/spiffe/spire/pkg/server/plugin/datastore/sql"
	"github.com/spiffe/spire/proto/spire/common"
	spi "github.com/spiffe/spire/proto/spire/common/plugin"
	"github.com/spiffe/spire/test/fakes/fakedatastore"
	"github.com/spiffe/spire/test/spiretest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	spiffeScheme = "spiffe"
	trustDomain  = "example.org"
)

var (
	_ EntryIterator = (*entryIterator)(nil)
	_ AgentIterator = (*agentIterator)(nil)
	_ EntryIterator = (*errorEntryIterator)(nil)
	_ AgentIterator = (*errorAgentIterator)(nil)
	// The following are set by the linker during integration tests to
	// run these unit tests against various SQL backends.
	TestDialect      string
	TestConnString   string
	TestROConnString string
)

func TestCache(t *testing.T) {
	ds := fakedatastore.New(t)
	ctx := context.Background()

	const rootID = "spiffe://example.org/root"
	const serverID = "spiffe://example.org/spire/server"
	const numEntries = 5
	entryIDs := make([]string, numEntries)
	for i := 0; i < numEntries; i++ {
		entryIDURI := url.URL{
			Scheme: spiffeScheme,
			Host:   trustDomain,
			Path:   strconv.Itoa(i),
		}

		entryIDs[i] = entryIDURI.String()
	}

	a1 := &common.Selector{Type: "a", Value: "1"}
	b2 := &common.Selector{Type: "b", Value: "2"}

	irrelevantSelectors := []*common.Selector{
		{Type: "not", Value: "relevant"},
	}

	//
	//        root             3(a1,b2)
	//        /   \           /
	//       0     1         4
	//            /
	//           2
	//
	// node resolvers map from 1 to 3

	entriesToCreate := []*common.RegistrationEntry{
		{
			ParentId:  rootID,
			SpiffeId:  entryIDs[0],
			Selectors: irrelevantSelectors,
		},
		{
			ParentId:  rootID,
			SpiffeId:  entryIDs[1],
			Selectors: irrelevantSelectors,
		},
		{
			ParentId:  entryIDs[1],
			SpiffeId:  entryIDs[2],
			Selectors: irrelevantSelectors,
		},
		{
			ParentId:  serverID,
			SpiffeId:  entryIDs[3],
			Selectors: []*common.Selector{a1, b2},
		},
		{

			ParentId:  entryIDs[3],
			SpiffeId:  entryIDs[4],
			Selectors: irrelevantSelectors,
		},
	}

	entries := make([]*common.RegistrationEntry, len(entriesToCreate))
	for i, e := range entriesToCreate {
		entries[i] = createRegistrationEntry(ctx, t, ds, e)
	}

	setNodeSelectors(ctx, t, ds, entryIDs[1], a1, b2)

	cache, err := BuildFromDataStore(context.Background(), ds)
	assert.NoError(t, err)

	actual := cache.GetAuthorizedEntries(rootID)

	assert.Equal(t, entries, actual)
}

func TestFullCacheNodeAliasing(t *testing.T) {
	ds := fakedatastore.New(t)
	ctx := context.Background()

	const serverID = "spiffe://example.org/spire/server"
	agentIDs := []spiffeid.ID{
		spiffeid.RequireFromString("spiffe://example.org/spire/agent/agent1"),
		spiffeid.RequireFromString("spiffe://example.org/spire/agent/agent2"),
		spiffeid.RequireFromString("spiffe://example.org/spire/agent/agent3"),
	}

	s1 := &common.Selector{Type: "s", Value: "1"}
	s2 := &common.Selector{Type: "s", Value: "2"}
	s3 := &common.Selector{Type: "s", Value: "3"}

	irrelevantSelectors := []*common.Selector{
		{Type: "not", Value: "relevant"},
	}

	nodeAliasEntriesToCreate := []*common.RegistrationEntry{
		{
			ParentId:  serverID,
			SpiffeId:  "spiffe://example.org/agent1",
			Selectors: []*common.Selector{s1, s2},
		},
		{
			ParentId:  serverID,
			SpiffeId:  "spiffe://example.org/agent2",
			Selectors: []*common.Selector{s1},
		},
	}

	nodeAliasEntries := make([]*common.RegistrationEntry, len(nodeAliasEntriesToCreate))
	for i, e := range nodeAliasEntriesToCreate {
		nodeAliasEntries[i] = createRegistrationEntry(ctx, t, ds, e)
	}

	workloadEntriesToCreate := []*common.RegistrationEntry{
		{
			ParentId:  nodeAliasEntries[0].SpiffeId,
			SpiffeId:  "spiffe://example.org/workload1",
			Selectors: irrelevantSelectors,
		},
		{
			ParentId:  nodeAliasEntries[1].SpiffeId,
			SpiffeId:  "spiffe://example.org/workload2",
			Selectors: irrelevantSelectors,
		},
		{
			ParentId:  agentIDs[2].String(),
			SpiffeId:  "spiffe://example.org/workload3",
			Selectors: irrelevantSelectors,
		},
	}

	workloadEntries := make([]*common.RegistrationEntry, len(workloadEntriesToCreate))
	for i, e := range workloadEntriesToCreate {
		workloadEntries[i] = createRegistrationEntry(ctx, t, ds, e)
	}

	setNodeSelectors(ctx, t, ds, agentIDs[0].String(), s1, s2)
	setNodeSelectors(ctx, t, ds, agentIDs[1].String(), s1, s3)

	cache, err := BuildFromDataStore(context.Background(), ds)
	assert.NoError(t, err)

	assertAuthorizedEntries := func(agentID spiffeid.ID, entries ...*common.RegistrationEntry) {
		require.NoError(t, err)
		assert.ElementsMatch(t, entries, cache.GetAuthorizedEntries(agentID.String()))
	}

	assertAuthorizedEntries(agentIDs[0], append(nodeAliasEntries, workloadEntries[:2]...)...)
	assertAuthorizedEntries(agentIDs[1], nodeAliasEntries[1], workloadEntries[1])
	assertAuthorizedEntries(agentIDs[2], workloadEntries[2])
}

func TestBuildIteratorError(t *testing.T) {
	tests := []struct {
		desc    string
		entryIt EntryIterator
		agentIt AgentIterator
	}{
		{
			desc:    "entry iterator error",
			entryIt: &errorEntryIterator{},
			agentIt: makeAgentIterator(nil),
		},
		{
			desc:    "agent iterator error",
			entryIt: makeEntryIterator(nil),
			agentIt: &errorAgentIterator{},
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		entryIt := tt.entryIt
		agentIt := tt.agentIt
		t.Run(tt.desc, func(t *testing.T) {
			cache, err := Build(ctx, entryIt, agentIt)
			assert.Error(t, err)
			assert.Nil(t, cache)
		})
	}
}

func BenchmarkBuildInMemory(b *testing.B) {
	allEntries, agents := buildBenchmarkData()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Build(context.Background(), makeEntryIterator(allEntries), makeAgentIterator(agents))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetAuthorizedEntriesInMemory(b *testing.B) {
	allEntries, agents := buildBenchmarkData()
	cache, err := Build(context.Background(), makeEntryIterator(allEntries), makeAgentIterator(agents))
	require.NoError(b, err)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.GetAuthorizedEntries(agents[i%len(agents)].ID.String())
	}
}

// To run this benchmark against a real MySQL or Postgres database, set the following flags in your test run,
// substituting in the required connection string parameters for each of the ldflags:
// -bench 'BenchmarkBuildSQL' -benchtime <some-reasonable-time-limit> -ldflags "-X github.com/spiffe/spire/pkg/server/cache/entrycache.TestDialect=<mysql|postgres> -X github.com/spiffe/spire/pkg/server/cache/entrycache.TestConnString=<CONNECTION_STRING_HERE> -X github.com/spiffe/spire/pkg/server/cache/entrycache.TestROConnString=<CONNECTION_STRING_HERE>"
func BenchmarkBuildSQL(b *testing.B) {
	allEntries, agents := buildBenchmarkData()
	ctx := context.Background()
	ds := newSQLPlugin(ctx, b)

	for _, entry := range allEntries {
		createRegistrationEntry(ctx, b, ds, entry)
	}

	for _, agent := range agents {
		setNodeSelectors(ctx, b, ds, agent.ID.String(), agent.Selectors...)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := BuildFromDataStore(ctx, ds)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func makeAgentID(i int) spiffeid.ID {
	return spiffeid.RequireFromString(fmt.Sprintf("spiffe://domain.test/spire/agent/%04d", i))
}

type entryIterator struct {
	entries []*common.RegistrationEntry
	next    int
}

func makeEntryIterator(entries []*common.RegistrationEntry) *entryIterator {
	return &entryIterator{
		entries: entries,
	}
}

func (it *entryIterator) Next(context.Context) bool {
	if it.next >= len(it.entries) {
		return false
	}
	it.next++
	return true
}

func (it *entryIterator) Entry() *common.RegistrationEntry {
	return it.entries[it.next-1]
}

func (it *entryIterator) Err() error {
	return nil
}

type agentIterator struct {
	agents []Agent
	next   int
}

func makeAgentIterator(agents []Agent) *agentIterator {
	return &agentIterator{
		agents: agents,
	}
}

func (it *agentIterator) Next(context.Context) bool {
	if it.next >= len(it.agents) {
		return false
	}
	it.next++
	return true
}

func (it *agentIterator) Agent() Agent {
	return it.agents[it.next-1]
}

func (it *agentIterator) Err() error {
	return nil
}

type errorEntryIterator struct{}

func (e *errorEntryIterator) Next(context.Context) bool {
	return false
}

func (e *errorEntryIterator) Err() error {
	return errors.New("some entry iterator error")
}

func (e *errorEntryIterator) Entry() *common.RegistrationEntry {
	return nil
}

type errorAgentIterator struct{}

func (e *errorAgentIterator) Next(context.Context) bool {
	return false
}

func (e *errorAgentIterator) Err() error {
	return errors.New("some agent iterator error")
}

func (e *errorAgentIterator) Agent() Agent {
	return Agent{}
}

func wipePostgres(tb testing.TB, connString string) {
	db, err := sql.Open("postgres", connString)
	require.NoError(tb, err)
	defer db.Close()

	rows, err := db.Query(`SELECT tablename FROM pg_tables WHERE schemaname = 'public';`)
	require.NoError(tb, err)
	defer rows.Close()

	dropTablesInRows(tb, db, rows)
}

func wipeMySQL(tb testing.TB, connString string) {
	db, err := sql.Open("mysql", connString)
	require.NoError(tb, err)
	defer db.Close()

	rows, err := db.Query(`SELECT table_name FROM information_schema.tables WHERE table_schema = 'spire';`)
	require.NoError(tb, err)
	defer rows.Close()

	dropTablesInRows(tb, db, rows)
}

func dropTablesInRows(tb testing.TB, db *sql.DB, rows *sql.Rows) {
	for rows.Next() {
		var q string
		err := rows.Scan(&q)
		require.NoError(tb, err)
		_, err = db.Exec("DROP TABLE IF EXISTS " + q + " CASCADE")
		require.NoError(tb, err)
	}
	require.NoError(tb, rows.Err())
}

func createRegistrationEntry(ctx context.Context, tb testing.TB, ds datastore.DataStore, entry *common.RegistrationEntry) *common.RegistrationEntry {
	resp, err := ds.CreateRegistrationEntry(ctx, &datastore.CreateRegistrationEntryRequest{
		Entry: entry,
	})
	require.NoError(tb, err)
	return resp.Entry
}

func setNodeSelectors(ctx context.Context, tb testing.TB, ds datastore.DataStore, spiffeID string, selectors ...*common.Selector) {
	_, err := ds.SetNodeSelectors(ctx, &datastore.SetNodeSelectorsRequest{
		Selectors: &datastore.NodeSelectors{
			SpiffeId:  spiffeID,
			Selectors: selectors,
		},
	})
	require.NoError(tb, err)
}

func buildBenchmarkData() ([]*common.RegistrationEntry, []Agent) {
	staticSelector1 := &common.Selector{
		Type:  "static",
		Value: "static-1",
	}
	staticSelector2 := &common.Selector{
		Type:  "static",
		Value: "static-1",
	}

	aliasID1 := "spiffe://domain.test/alias1"
	aliasID2 := "spiffe://domain.test/alias2"
	serverID := "spiffe://domain.test/spire/server"

	const numAgents = 50000
	agents := make([]Agent, 0, numAgents)
	for i := 0; i < numAgents; i++ {
		agents = append(agents, Agent{
			ID: makeAgentID(i),
			Selectors: []*common.Selector{
				staticSelector1,
			},
		})
	}

	var allEntries = []*common.RegistrationEntry{
		// Alias
		{
			SpiffeId: aliasID1,
			ParentId: serverID,
			Selectors: []*common.Selector{
				staticSelector1,
			},
		},
		// False alias
		{
			SpiffeId: aliasID2,
			ParentId: serverID,
			Selectors: []*common.Selector{
				staticSelector2,
			},
		},
	}

	var workloadEntries1 []*common.RegistrationEntry
	for i := 0; i < 300; i++ {
		workloadEntries1 = append(workloadEntries1, &common.RegistrationEntry{
			SpiffeId: fmt.Sprintf("spiffe://domain.test/workload%d", i),
			ParentId: aliasID1,
			Selectors: []*common.Selector{
				{Type: "unix", Value: fmt.Sprintf("uid:%d", i)},
			},
		})
	}

	var workloadEntries2 []*common.RegistrationEntry
	for i := 0; i < 300; i++ {
		workloadEntries2 = append(workloadEntries2, &common.RegistrationEntry{
			SpiffeId: fmt.Sprintf("spiffe://domain.test/workload%d", i),
			ParentId: aliasID2,
			Selectors: []*common.Selector{
				{Type: "unix", Value: fmt.Sprintf("uid:%d", i)},
			},
		})
	}

	allEntries = append(allEntries, workloadEntries1...)
	allEntries = append(allEntries, workloadEntries2...)
	return allEntries, agents
}

func newSQLPlugin(ctx context.Context, tb testing.TB) datastore.Plugin {
	p := sqlds.BuiltIn()
	var ds datastore.Plugin
	spiretest.LoadPlugin(tb, p, &ds)

	// When the test suite is executed normally, we test against sqlite3 since
	// it requires no external dependencies. The integration test framework
	// builds the test harness for a specific dialect and connection string
	var cfg string
	switch TestDialect {
	case "":
		d, err := ioutil.TempDir("", "spire-fullcache-test")
		require.NoError(tb, err)
		dbPath := filepath.Join(d, "db.sqlite3")
		cfg = fmt.Sprintf(`
				database_type = "sqlite3"
				log_sql = true
				connection_string = "%s"
				`, dbPath)
	case "mysql":
		require.NotEmpty(tb, TestConnString, "connection string must be set")
		wipeMySQL(tb, TestConnString)
		cfg = fmt.Sprintf(`
				database_type = "mysql"
				log_sql = true
				connection_string = "%s"
				ro_connection_string = "%s"
				`, TestConnString, TestROConnString)
	case "postgres":
		require.NotEmpty(tb, TestConnString, "connection string must be set")
		wipePostgres(tb, TestConnString)
		cfg = fmt.Sprintf(`
				database_type = "postgres"
				log_sql = true
				connection_string = "%s"
				ro_connection_string = "%s"
				`, TestConnString, TestROConnString)
	default:
		require.FailNowf(tb, "Unsupported external test dialect %q", TestDialect)
	}

	cfgReq := &spi.ConfigureRequest{
		Configuration: cfg,
	}
	_, err := ds.Configure(ctx, cfgReq)
	require.NoError(tb, err)

	return ds
}
