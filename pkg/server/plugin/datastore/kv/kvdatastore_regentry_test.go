package kv

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/spiffe/spire/pkg/common/util"
	"github.com/spiffe/spire/pkg/server/plugin/datastore"
	"github.com/spiffe/spire/proto/spire/common"
	"github.com/spiffe/spire/test/spiretest"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

type ListRegistrationEntryPaginationTest struct {
	s               *PluginSuite
	name            string
	req             *datastore.ListRegistrationEntriesRequest
	expectedEntries []*common.RegistrationEntry
	pageSize        int
}

func (s *PluginSuite) TestCreateRegistrationEntry() {
	var validRegistrationEntries []*common.RegistrationEntry
	s.getTestDataFromJSONFile(filepath.Join("testdata", "valid_registration_entries.json"), &validRegistrationEntries)

	for _, validRegistrationEntry := range validRegistrationEntries {
		createdEntry := s.createRegistrationEntry(validRegistrationEntry, s.ds)
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

	createdEntry := s.createRegistrationEntry(registeredEntry, s.ds)

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

	createdEntries := s.createRegistrationEntries(entriesToCreate, s.ds)
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
				createdEntry := s.createRegistrationEntry(entry, ds)
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

	createdEntries := s.createRegistrationEntries(entriesToCreate, s.ds)
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
				createdEntry := s.createRegistrationEntry(entry, ds)
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
				createdEntry := s.createRegistrationEntry(entry, ds)
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
	testTemplates := s.generatePaginationTestTemplates()
	var tests []ListRegistrationEntryPaginationTest
	for _, testTempl := range testTemplates {
		for pageSize := 1; pageSize <= len(testTempl.expectedEntries); pageSize++ {
			tests = append(tests, s.generatePaginationTest(pageSize, testTempl))
		}
	}

	for _, test := range tests {
		s.T().Run(test.name, func(t *testing.T) {
			test.execute()
		})
	}
}

func (s *PluginSuite) TestListRegistrationEntriesWithInvalidPageSize() {
	tests := []struct {
		name     string
		pageSize int32
	}{
		{
			name:     "zero page size",
			pageSize: 0,
		},
		{
			name:     "negative page size",
			pageSize: -1,
		},
	}

	for _, test := range tests {
		s.T().Run(test.name, func(t *testing.T) {
			req := &datastore.ListRegistrationEntriesRequest{
				Pagination: &datastore.Pagination{
					PageSize: test.pageSize,
				},
			}

			_, err := s.ds.ListRegistrationEntries(ctx, req)
			s.AssertGRPCStatusContains(err, codes.InvalidArgument, "cannot paginate with pagesize")
		})
	}
}

func (s *PluginSuite) createRegistrationEntries(entriesToCreate []*common.RegistrationEntry, ds datastore.Plugin) []*common.RegistrationEntry {
	createdEntries := make([]*common.RegistrationEntry, len(entriesToCreate))
	for i, entryToCreate := range entriesToCreate {
		createdEntries[i] = s.createRegistrationEntry(entryToCreate, ds)
	}

	return createdEntries
}

func (s *PluginSuite) createRegistrationEntry(entry *common.RegistrationEntry, ds datastore.Plugin) *common.RegistrationEntry {
	resp, err := ds.CreateRegistrationEntry(ctx, &datastore.CreateRegistrationEntryRequest{
		Entry: entry,
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Require().NotNil(resp.Entry)
	return resp.Entry
}

func (s *PluginSuite) generatePaginationTestTemplates() []ListRegistrationEntryPaginationTest {
	selectors, entries := s.generatePaginationTestData()
	return []ListRegistrationEntryPaginationTest{
		{
			name:            "all",
			req:             &datastore.ListRegistrationEntriesRequest{},
			expectedEntries: entries,
		},
		{
			name: "by parent id",
			req: &datastore.ListRegistrationEntriesRequest{
				ByParentId: &wrappers.StringValue{
					Value: "spiffe://example.org/bat",
				},
			},
			expectedEntries: entries[3:],
		},
		{
			name: "with selector subset match",
			req: &datastore.ListRegistrationEntriesRequest{
				BySelectors: &datastore.BySelectors{
					Selectors: selectors[:3],
					Match:     datastore.BySelectors_MATCH_SUBSET,
				},
			},
			expectedEntries: entries[:6],
		},
		{
			name: "with selector set exact match",
			req: &datastore.ListRegistrationEntriesRequest{
				BySelectors: &datastore.BySelectors{
					Selectors: selectors[:3],
					Match:     datastore.BySelectors_MATCH_EXACT,
				},
			},
			expectedEntries: entries[:4],
		},
	}
}

func (s *PluginSuite) generatePaginationTestData() ([]*common.Selector, []*common.RegistrationEntry) {
	numSelectors := 5
	selectors := make([]*common.Selector, numSelectors)
	for i := 1; i <= numSelectors; i++ {
		selectors[i-1] = &common.Selector{
			Type:  fmt.Sprintf("Type%d", i),
			Value: fmt.Sprintf("Value%d", i),
		}
	}

	entries := []*common.RegistrationEntry{
		{
			Selectors: selectors[:3],
			SpiffeId:  "spiffe://example.org/foo",
			ParentId:  "spiffe://example.org/bar",
		},
		{
			Selectors: selectors[:3],
			SpiffeId:  "spiffe://example.org/tez",
			ParentId:  "spiffe://example.org/taz",
		},
		{
			Selectors: selectors[:3],
			SpiffeId:  "spiffe://example.org/foot",
			ParentId:  "spiffe://example.org/bam",
		},
		{
			Selectors: selectors[:3],
			SpiffeId:  "spiffe://example.org/fool",
			ParentId:  "spiffe://example.org/bat",
		},
		{
			Selectors: selectors[:2],
			SpiffeId:  "spiffe://example.org/food",
			ParentId:  "spiffe://example.org/bat",
		},
		{
			Selectors: selectors[:1],
			SpiffeId:  "spiffe://example.org/foos",
			ParentId:  "spiffe://example.org/bat",
		},
		{
			Selectors: selectors[2:],
			SpiffeId:  "spiffe://example.org/baz",
			ParentId:  "spiffe://example.org/bat",
		},
	}

	return selectors, entries
}

func (s *PluginSuite) generatePaginationTest(pageSize int, testTemplate ListRegistrationEntryPaginationTest) ListRegistrationEntryPaginationTest {
	return ListRegistrationEntryPaginationTest{
		s:               s,
		name:            fmt.Sprintf("%s with page size %d", testTemplate.name, pageSize),
		req:             testTemplate.req,
		expectedEntries: testTemplate.expectedEntries,
		pageSize:        pageSize,
	}
}

func (test *ListRegistrationEntryPaginationTest) execute() {
	s := test.s
	ds := s.newPlugin()
	for _, entry := range test.expectedEntries {
		createdEntry := s.createRegistrationEntry(entry, ds)
		entry.EntryId = createdEntry.EntryId
	}

	numExpectedEntries := len(test.expectedEntries)
	resultsByEntryId := make(map[string]*common.RegistrationEntry, numExpectedEntries)
	test.req.Pagination = &datastore.Pagination{
		PageSize: int32(test.pageSize),
	}

	var numExpectedRequests int
	lastEmptyRequest := false
	if numExpectedEntries < test.pageSize {
		numExpectedEntries = 1
	} else {
		if numExpectedEntries%test.pageSize == 0 {
			// When the last page with results has exactly PageSize number of results,
			// we will still get a pagination token in the response.
			// We need to make one more request, which should have no results.
			lastEmptyRequest = true
		}

		numExpectedRequests = (numExpectedEntries / test.pageSize) + 1
	}

	for reqNum := 1; reqNum <= numExpectedRequests; reqNum++ {
		resp, err := ds.ListRegistrationEntries(ctx, test.req)
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		if reqNum == numExpectedRequests {
			if lastEmptyRequest {
				s.Assert().Empty(resp.Entries)
				break
			}
		} else {
			s.Require().NotNil(resp.Pagination)
			s.Require().NotEqual("", resp.Pagination.Token, "didn't receive pagination token for list request #%d out of %d expected requests", reqNum, numExpectedRequests)
			test.req.Pagination.Token = resp.Pagination.Token
		}

		s.Require().NotNil(resp.Entries, "received nil entries in response #%d of %d expected requests", reqNum, numExpectedRequests)
		s.Require().True(len(resp.Entries) > 0, "received empty entries in response #%d of %d expected requests", reqNum, numExpectedRequests)

		for _, entry := range resp.Entries {
			_, ok := resultsByEntryId[entry.EntryId]
			s.Assert().False(ok, "received same entry in multiple pages for entry id: %v", entry.EntryId)
			resultsByEntryId[entry.EntryId] = entry
		}
	}

	s.Assert().Equal(len(test.expectedEntries), len(resultsByEntryId))
	var resultEntryIds []string
	for entryId := range resultsByEntryId {
		resultEntryIds = append(resultEntryIds, entryId)
	}

	var expectedEntryIds []string
	for _, entry := range test.expectedEntries {
		expectedEntryIds = append(expectedEntryIds, entry.EntryId)
	}

	s.Assert().ElementsMatch(expectedEntryIds, resultEntryIds)
	for _, expectedEntry := range test.expectedEntries {
		actualEntry, ok := resultsByEntryId[expectedEntry.EntryId]
		s.Require().True(ok) // duplicate check of entry id from above, to do all we can to avoid panics
		s.AssertProtoEqual(expectedEntry, actualEntry)
	}
}
