package kv

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/spiffe/spire/pkg/common/util"
	"github.com/spiffe/spire/pkg/server/plugin/datastore"
	"github.com/spiffe/spire/proto/spire/common"
	spi "github.com/spiffe/spire/proto/spire/common/plugin"
	"github.com/spiffe/spire/test/spiretest"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

var (
	ctx = context.Background()
)

type PluginSuite struct {
	spiretest.Suite

	dir    string
	ds     datastore.Plugin
	nextID int
}

type ListRegistrationsReq struct {
	name               string
	pagination         *datastore.Pagination
	selectors          []*common.Selector
	expectedList       []*common.RegistrationEntry
	expectedPagination *datastore.Pagination
	err                string
}

func TestPlugin(t *testing.T) {
	spiretest.Run(t, new(PluginSuite))
}

func (s *PluginSuite) SetupTest() {
	s.dir = s.TempDir()
	s.ds = s.newPlugin()
}

func (s *PluginSuite) newPlugin() datastore.Plugin {
	var ds datastore.Plugin
	s.LoadPlugin(BuiltIn(), &ds)

	// TODO: Support mysql and postgres backends in integration tests, so far just supporting sqlite3 for unit tests
	s.nextID++
	dbPath := filepath.Join(s.dir, fmt.Sprintf("db%d.sqlite3", s.nextID))
	cfgHclTemplate := `
database_type = "sqlite3"
connection_string = "%s"
`

	cfgHcl := fmt.Sprintf(cfgHclTemplate, dbPath)
	cfgReq := &spi.ConfigureRequest{
		Configuration: cfgHcl,
	}

	_, err := ds.Configure(context.Background(), cfgReq)
	s.Require().NoError(err)

	return ds
}

func (s *PluginSuite) TestInvalidPluginConfiguration() {
	var err error
	tests := []struct {
		name     string
		cfg      string
		errorMsg string
	}{
		{
			name: "malformed config",
			cfg: `
yaml: isNotSupported
`,
			errorMsg: "unable to parse config",
		},
		{
			name: "missing database_type",
			cfg: `
connection_string = "bad"
`,
			errorMsg: "database_type must be set",
		},
		{
			name: "missing connection_string",
			cfg: `
database_type = "sqlite3"
`,
			errorMsg: "connection_string must be set",
		},
		{
			name: "unrecognized database_type",
			cfg: `
		database_type = "wrong"
		connection_string = "bad"
`,
			errorMsg: "unsupported database_type: wrong",
		},
		{
			name: "invalid MySQL connection_string",
			cfg: `
		database_type = "mysql"
		connection_string = "bad"
`,
			errorMsg: "invalid connection_string",
		},
	}

	for _, test := range tests {
		s.T().Run(test.name, func(t *testing.T) {
			_, err = s.ds.Configure(context.Background(), &spi.ConfigureRequest{
				Configuration: test.cfg,
			})
			s.RequireGRPCStatusContains(err, codes.InvalidArgument, kvErrorString(test.errorMsg))
		})
	}
}

func (s *PluginSuite) TestCreateRegistrationEntry() {
	var validRegistrationEntries []*common.RegistrationEntry
	s.getTestDataFromJSONFile(filepath.Join("testdata", "valid_registration_entries.json"), &validRegistrationEntries)

	for _, validRegistrationEntry := range validRegistrationEntries {
		createdEntry := s.createRegistrationEntry(validRegistrationEntry)
		s.NotEmpty(createdEntry.EntryId)
		createdEntry.EntryId = ""
		s.RequireProtoEqual(validRegistrationEntry, createdEntry)
	}
}

func (s *PluginSuite) TestFetchRegistrationEntry() {
	registeredEntry := &common.RegistrationEntry{
		Selectors: []*common.Selector{
			{Type: "Type1", Value: "Value1"},
			{Type: "Type2", Value: "Value2"},
			{Type: "Type3", Value: "Value3"},
		},
		SpiffeId: "SpiffeId",
		ParentId: "ParentId",
		Ttl:      1,
		DnsNames: []string{
			"abcd.efg",
			"somehost",
		},
	}

	createdEntry := s.createRegistrationEntry(registeredEntry)

	fetchRegistrationEntryResponse, err := s.ds.FetchRegistrationEntry(ctx, &datastore.FetchRegistrationEntryRequest{EntryId: createdEntry.EntryId})
	s.Require().NoError(err)
	s.Require().NotNil(fetchRegistrationEntryResponse)
	s.RequireProtoEqual(createdEntry, fetchRegistrationEntryResponse.Entry)
}

func (s *PluginSuite) TestFetchNonExistentRegistrationEntry() {
	fetchRegistrationEntryResponse, err := s.ds.FetchRegistrationEntry(ctx, &datastore.FetchRegistrationEntryRequest{EntryId: "foobar"})
	s.RequireGRPCStatusContains(err, codes.NotFound, "")
	s.Require().Nil(fetchRegistrationEntryResponse)
}

func (s *PluginSuite) TestListRegistrationEntries() {
	entriesToCreate := []*common.RegistrationEntry{
		{
			Selectors: []*common.Selector{
				{Type: "Type1", Value: "Value1"},
				{Type: "Type2", Value: "Value2"},
				{Type: "Type3", Value: "Value3"},
			},
			SpiffeId: "spiffe://example.org/foo",
			ParentId: "spiffe://example.org/bar",
			Ttl:      1,
			Admin:    true,
		},
		{
			Selectors: []*common.Selector{
				{Type: "Type3", Value: "Value3"},
				{Type: "Type4", Value: "Value4"},
				{Type: "Type5", Value: "Value5"},
			},
			SpiffeId:   "spiffe://example.org/baz",
			ParentId:   "spiffe://example.org/bat",
			Ttl:        2,
			Downstream: true,
		},
	}

	createdEntries := s.createRegistrationEntries(entriesToCreate)
	resp, err := s.ds.ListRegistrationEntries(ctx, &datastore.ListRegistrationEntriesRequest{})
	s.Require().NoError(err)
	s.Require().NotNil(resp)

	expectedResponse := &datastore.ListRegistrationEntriesResponse{
		Entries: createdEntries,
	}
	util.SortRegistrationEntries(expectedResponse.Entries)
	util.SortRegistrationEntries(resp.Entries)
	s.Equal(expectedResponse, resp)
}

func (s *PluginSuite) TestListParentIDEntries() {
	allEntries := make([]*common.RegistrationEntry, 0)
	s.getTestDataFromJSONFile(filepath.Join("testdata", "entries.json"), &allEntries)
	tests := []struct {
		name                string
		registrationEntries []*common.RegistrationEntry
		parentID            string
		expectedList        []*common.RegistrationEntry
	}{
		{

			name:                "test_parentID_found",
			registrationEntries: allEntries,
			parentID:            "spiffe://parent",
			expectedList:        allEntries[:2],
		},
		{
			name:                "test_parentID_notfound",
			registrationEntries: allEntries,
			parentID:            "spiffe://imnoparent",
			expectedList:        nil,
		},
	}
	for _, test := range tests {
		test := test
		s.T().Run(test.name, func(t *testing.T) {
			ds := s.newPlugin()
			for _, entry := range test.registrationEntries {
				createdEntry := s.createRegistrationEntry(entry)
				entry.EntryId = createdEntry.EntryId
			}
			result, err := ds.ListRegistrationEntries(ctx, &datastore.ListRegistrationEntriesRequest{
				ByParentId: &wrappers.StringValue{
					Value: test.parentID,
				},
			})
			require.NoError(t, err)
			spiretest.RequireProtoListElementsMatch(t, test.expectedList, result.Entries)
		})
	}
}

func (s *PluginSuite) TestListRegistrationEntriesAgainstMultipleCriteria() {
	entriesToCreate := []*common.RegistrationEntry{
		{
			ParentId: "spiffe://example.org/P1",
			SpiffeId: "spiffe://example.org/S1",
			Selectors: []*common.Selector{
				{Type: "T1", Value: "V1"},
			},
		},
		// shares a parent ID with first entry
		{
			ParentId: "spiffe://example.org/P1",
			SpiffeId: "spiffe://example.org/S2",
			Selectors: []*common.Selector{
				{Type: "T2", Value: "V2"},
			},
		},
		// shares a spiffe ID with first entry
		{
			ParentId: "spiffe://example.org/P3",
			SpiffeId: "spiffe://example.org/S1",
			Selectors: []*common.Selector{
				{Type: "T3", Value: "V3"},
			},
		},
		// shares selectors with first entry
		{
			ParentId: "spiffe://example.org/P4",
			SpiffeId: "spiffe://example.org/S4",
			Selectors: []*common.Selector{
				{Type: "T1", Value: "V1"},
			},
		},
	}

	createdEntries := s.createRegistrationEntries(entriesToCreate)
	resp, err := s.ds.ListRegistrationEntries(ctx, &datastore.ListRegistrationEntriesRequest{
		ByParentId: &wrappers.StringValue{
			Value: "spiffe://example.org/P1",
		},
		BySpiffeId: &wrappers.StringValue{
			Value: "spiffe://example.org/S1",
		},
		BySelectors: &datastore.BySelectors{
			Selectors: []*common.Selector{
				{Type: "T1", Value: "V1"},
			},
			Match: datastore.BySelectors_MATCH_EXACT,
		},
	})

	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.RequireProtoListEqual([]*common.RegistrationEntry{createdEntries[0]}, resp.Entries)
}

func (s *PluginSuite) TestListEntriesBySelectorsExactMatch() {
	allEntries := make([]*common.RegistrationEntry, 0)
	s.getTestDataFromJSONFile(filepath.Join("testdata", "entries.json"), &allEntries)
	tests := []struct {
		name                string
		registrationEntries []*common.RegistrationEntry
		selectors           []*common.Selector
		expectedList        []*common.RegistrationEntry
	}{
		{
			name:                "entries_by_selector_found",
			registrationEntries: allEntries,
			selectors: []*common.Selector{
				{Type: "a", Value: "1"},
				{Type: "b", Value: "2"},
				{Type: "c", Value: "3"},
			},
			expectedList: []*common.RegistrationEntry{allEntries[0]},
		},
		{
			name:                "entries_by_selector_not_found",
			registrationEntries: allEntries,
			selectors: []*common.Selector{
				{Type: "e", Value: "0"},
			},
			expectedList: nil,
		},
	}
	for _, test := range tests {
		test := test
		s.T().Run(test.name, func(t *testing.T) {
			ds := s.newPlugin()
			for _, entry := range test.registrationEntries {
				createdEntry := s.createRegistrationEntry(entry)
				entry.EntryId = createdEntry.EntryId
			}
			result, err := ds.ListRegistrationEntries(ctx, &datastore.ListRegistrationEntriesRequest{
				BySelectors: &datastore.BySelectors{
					Selectors: test.selectors,
					Match:     datastore.BySelectors_MATCH_EXACT,
				},
			})
			require.NoError(t, err)
			spiretest.RequireProtoListEqual(t, test.expectedList, result.Entries)
		})
	}
}

func (s *PluginSuite) TestListEntriesBySelectorSubset() {
	allEntries := make([]*common.RegistrationEntry, 0)
	s.getTestDataFromJSONFile(filepath.Join("testdata", "entries.json"), &allEntries)
	tests := []struct {
		name                string
		registrationEntries []*common.RegistrationEntry
		selectors           []*common.Selector
		expectedList        []*common.RegistrationEntry
	}{
		{
			name:                "test1",
			registrationEntries: allEntries,
			selectors: []*common.Selector{
				{Type: "a", Value: "1"},
				{Type: "b", Value: "2"},
				{Type: "c", Value: "3"},
			},
			expectedList: []*common.RegistrationEntry{
				allEntries[0],
				allEntries[1],
				allEntries[2],
			},
		},
		{
			name:                "test2",
			registrationEntries: allEntries,
			selectors: []*common.Selector{
				{Type: "d", Value: "4"},
			},
			expectedList: nil,
		},
	}
	for _, test := range tests {
		test := test
		s.T().Run(test.name, func(t *testing.T) {
			ds := s.newPlugin()
			for _, entry := range test.registrationEntries {
				createdEntry := s.createRegistrationEntry(entry)
				entry.EntryId = createdEntry.EntryId
			}
			result, err := ds.ListRegistrationEntries(ctx, &datastore.ListRegistrationEntriesRequest{
				BySelectors: &datastore.BySelectors{
					Selectors: test.selectors,
					Match:     datastore.BySelectors_MATCH_SUBSET,
				},
			})
			require.NoError(t, err)
			util.SortRegistrationEntries(test.expectedList)
			util.SortRegistrationEntries(result.Entries)
			s.RequireProtoListEqual(test.expectedList, result.Entries)
		})
	}
}

func (s *PluginSuite) TestListRegistrationEntriesWithPagination() {
	entriesToCreate := []*common.RegistrationEntry{
		{
			Selectors: []*common.Selector{
				{Type: "Type1", Value: "Value1"},
				{Type: "Type2", Value: "Value2"},
				{Type: "Type3", Value: "Value3"},
			},
			SpiffeId: "spiffe://example.org/foo",
			ParentId: "spiffe://example.org/bar",
			Ttl:      1,
		},
		{
			Selectors: []*common.Selector{
				{Type: "Type3", Value: "Value3"},
				{Type: "Type4", Value: "Value4"},
				{Type: "Type5", Value: "Value5"},
			},
			SpiffeId: "spiffe://example.org/baz",
			ParentId: "spiffe://example.org/bat",
			Ttl:      2,
		},
		{
			Selectors: []*common.Selector{
				{Type: "Type1", Value: "Value1"},
				{Type: "Type2", Value: "Value2"},
				{Type: "Type3", Value: "Value3"},
			},
			SpiffeId: "spiffe://example.org/tez",
			ParentId: "spiffe://example.org/taz",
			Ttl:      2,
		},
	}

	createdEntries := s.createRegistrationEntries(entriesToCreate)
	selectors := []*common.Selector{
		{Type: "Type1", Value: "Value1"},
		{Type: "Type2", Value: "Value2"},
		{Type: "Type3", Value: "Value3"},
	}

	tests := []ListRegistrationsReq{
		{
			name: "pagination_without_token",
			pagination: &datastore.Pagination{
				PageSize: 2,
			},
			expectedList: []*common.RegistrationEntry{createdEntries[1], createdEntries[0]},
			expectedPagination: &datastore.Pagination{
				Token:    "2",
				PageSize: 2,
			},
		},
		{
			name: "pagination_not_null_but_page_size_is_zero",
			pagination: &datastore.Pagination{
				Token:    "0",
				PageSize: 0,
			},
			err: "rpc error: code = InvalidArgument desc = cannot paginate with pagesize = 0",
		},
		{
			name: "get_all_entries_first_page",
			pagination: &datastore.Pagination{
				Token:    "0",
				PageSize: 2,
			},
			expectedList: []*common.RegistrationEntry{createdEntries[1], createdEntries[0]},
			expectedPagination: &datastore.Pagination{
				Token:    "2",
				PageSize: 2,
			},
		},
		{
			name: "get_all_entries_second_page",
			pagination: &datastore.Pagination{
				Token:    "2",
				PageSize: 2,
			},
			expectedList: []*common.RegistrationEntry{createdEntries[2]},
			expectedPagination: &datastore.Pagination{
				Token:    "3",
				PageSize: 2,
			},
		},
		{
			name: "get_all_entries_third_page_no_results",
			pagination: &datastore.Pagination{
				Token:    "3",
				PageSize: 2,
			},
			expectedPagination: &datastore.Pagination{
				PageSize: 2,
			},
		},
		{
			name: "get_entries_by_selector_get_only_page_first_page",
			pagination: &datastore.Pagination{
				Token:    "0",
				PageSize: 2,
			},
			selectors:    selectors,
			expectedList: []*common.RegistrationEntry{createdEntries[0], createdEntries[2]},
			expectedPagination: &datastore.Pagination{
				Token:    "3",
				PageSize: 2,
			},
		},
		{
			name: "get_entries_by_selector_get_only_page_second_page_no_results",
			pagination: &datastore.Pagination{
				Token:    "3",
				PageSize: 2,
			},
			selectors: selectors,
			expectedPagination: &datastore.Pagination{
				PageSize: 2,
			},
		},
		{
			name: "get_entries_by_selector_first_page",
			pagination: &datastore.Pagination{
				Token:    "0",
				PageSize: 1,
			},
			selectors:    selectors,
			expectedList: []*common.RegistrationEntry{createdEntries[0]},
			expectedPagination: &datastore.Pagination{
				Token:    "1",
				PageSize: 1,
			},
		},
		{
			name: "get_entries_by_selector_second_page",
			pagination: &datastore.Pagination{
				Token:    "1",
				PageSize: 1,
			},
			selectors:    selectors,
			expectedList: []*common.RegistrationEntry{createdEntries[2]},
			expectedPagination: &datastore.Pagination{
				Token:    "3",
				PageSize: 1,
			},
		},
		{
			name: "get_entries_by_selector_third_page_no_results",
			pagination: &datastore.Pagination{
				Token:    "3",
				PageSize: 1,
			},
			selectors: selectors,
			expectedPagination: &datastore.Pagination{
				PageSize: 1,
			},
		},
	}

	s.listRegistrationEntries(tests, true)
	s.listRegistrationEntries(tests, false)

	// with invalid token
	resp, err := s.ds.ListRegistrationEntries(ctx, &datastore.ListRegistrationEntriesRequest{
		Pagination: &datastore.Pagination{
			Token:    "invalid int",
			PageSize: 10,
		},
	})
	s.Require().Nil(resp)
	s.Require().Error(err, "could not parse token 'invalid int'")
}

func (s *PluginSuite) createRegistrationEntries(entriesToCreate []*common.RegistrationEntry) []*common.RegistrationEntry {
	createdEntries := make([]*common.RegistrationEntry, len(entriesToCreate))
	for i, entryToCreate := range entriesToCreate {
		createdEntries[i] = s.createRegistrationEntry(entryToCreate)
	}

	return createdEntries
}

func (s *PluginSuite) createRegistrationEntry(entry *common.RegistrationEntry) *common.RegistrationEntry {
	resp, err := s.ds.CreateRegistrationEntry(ctx, &datastore.CreateRegistrationEntryRequest{
		Entry: entry,
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Require().NotNil(resp.Entry)
	return resp.Entry
}

func (s *PluginSuite) getTestDataFromJSONFile(filePath string, jsonValue interface{}) {
	jsonBytes, err := ioutil.ReadFile(filePath)
	s.Require().NoError(err)

	err = json.Unmarshal(jsonBytes, &jsonValue)
	s.Require().NoError(err)
}

func (s *PluginSuite) listRegistrationEntries(tests []ListRegistrationsReq, tolerateStale bool) {
	for _, test := range tests {
		s.T().Run(test.name, func(t *testing.T) {
			var bySelectors *datastore.BySelectors
			if test.selectors != nil {
				bySelectors = &datastore.BySelectors{
					Selectors: test.selectors,
					Match:     datastore.BySelectors_MATCH_EXACT,
				}
			}
			resp, err := s.ds.ListRegistrationEntries(ctx, &datastore.ListRegistrationEntriesRequest{
				BySelectors:   bySelectors,
				Pagination:    test.pagination,
				TolerateStale: tolerateStale,
			})
			if test.err != "" {
				require.EqualError(t, err, test.err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, resp)

			expectedResponse := &datastore.ListRegistrationEntriesResponse{
				Entries:    test.expectedList,
				Pagination: test.expectedPagination,
			}
			util.SortRegistrationEntries(expectedResponse.Entries)
			util.SortRegistrationEntries(resp.Entries)
			require.Equal(t, expectedResponse, resp)
		})
	}
}

func kvErrorString(errorStr string) string {
	return fmt.Sprintf("datastore-kv: %s", errorStr)
}
