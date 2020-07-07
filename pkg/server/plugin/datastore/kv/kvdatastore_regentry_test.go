package kv

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

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

func (s *PluginSuite) TestDeleteRegistrationEntry() {
	// delete non-existing
	badDeleteReq := &datastore.DeleteRegistrationEntryRequest{
		EntryId: "badid",
	}

	_, err := s.ds.DeleteRegistrationEntry(ctx, badDeleteReq)
	s.RequireGRPCStatusContains(err, codes.NotFound, "registration entry not found for entry id")

	selectors := []*common.Selector{
		{
			Type:  "Type1",
			Value: "Value1",
		},
		{
			Type:  "Type2",
			Value: "Value2",
		},
		{
			Type:  "Type3",
			Value: "Value3",
		},
		{
			Type:  "Type4",
			Value: "Value4",
		},
		{
			Type:  "Type5",
			Value: "Value5",
		},
	}

	entriesToCreate := []*common.RegistrationEntry{
		{
			Selectors: selectors[:3],
			SpiffeId:  "spiffe://example.org/foo",
			ParentId:  "spiffe://example.org/bar",
			Ttl:       1,
		},
		{
			Selectors: selectors[2:],
			SpiffeId:  "spiffe://example.org/baz",
			ParentId:  "spiffe://example.org/bat",
			Ttl:       2,
		},
	}

	createdEntries := make([]*common.RegistrationEntry, len(entriesToCreate))
	for i := range entriesToCreate {
		createdEntries[i] = s.createRegistrationEntry(entriesToCreate[i], s.ds)
	}

	// We have two registration entries
	lReq := &datastore.ListRegistrationEntriesRequest{}
	entriesResp, err := s.ds.ListRegistrationEntries(ctx, lReq)
	s.Require().NoError(err)
	s.Require().Len(entriesResp.Entries, 2)

	// Make sure we deleted the right one
	dReq := &datastore.DeleteRegistrationEntryRequest{
		EntryId: createdEntries[0].EntryId,
	}

	delRes, err := s.ds.DeleteRegistrationEntry(ctx, dReq)
	s.Require().NoError(err)
	s.Require().Equal(createdEntries[0], delRes.Entry)

	// Make sure we have now only one registration entry
	entriesResp, err = s.ds.ListRegistrationEntries(ctx, lReq)
	s.Require().NoError(err)
	s.Require().Len(entriesResp.Entries, 1)
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
	s.Require().NoError(err)
	s.Require().NotNil(fetchRegistrationEntryResponse)
	s.Require().Nil(fetchRegistrationEntryResponse.Entry)
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

func (s *PluginSuite) TestListRegistrationEntriesBySelectorsWithNoSelectors() {
	tests := []struct {
		name        string
		bySelectors *datastore.BySelectors
	}{
		{
			name: "nil selectors",
			bySelectors: &datastore.BySelectors{
				Match: datastore.BySelectors_MATCH_EXACT,
			},
		},
		{
			name: "empty selectors",
			bySelectors: &datastore.BySelectors{
				Match:     datastore.BySelectors_MATCH_EXACT,
				Selectors: []*common.Selector{},
			},
		},
	}

	allEntries := make([]*common.RegistrationEntry, 0)
	s.getTestDataFromJSONFile(filepath.Join("testdata", "entries.json"), &allEntries)

	for _, test := range tests {
		bySelectors := test.bySelectors
		s.T().Run(test.name, func(t *testing.T) {
			ds := s.newPlugin()
			s.createRegistrationEntries(allEntries, ds)
			req := &datastore.ListRegistrationEntriesRequest{
				BySelectors: bySelectors,
			}

			_, err := ds.ListRegistrationEntries(ctx, req)
			s.AssertGRPCStatusContains(err, codes.InvalidArgument, "cannot list by empty selector set")
		})
	}
}

func (s *PluginSuite) TestListRegistrationEntriesWithInvalidSelectorMatch() {
	allEntries := make([]*common.RegistrationEntry, 0)
	s.getTestDataFromJSONFile(filepath.Join("testdata", "entries.json"), &allEntries)
	s.createRegistrationEntries(allEntries, s.ds)
	req := &datastore.ListRegistrationEntriesRequest{
		BySelectors: &datastore.BySelectors{
			Match: datastore.BySelectors_MatchBehavior(-1),
			Selectors: []*common.Selector{
				{
					Type:  "a",
					Value: "1",
				},
			},
		},
	}

	_, err := s.ds.ListRegistrationEntries(ctx, req)
	s.AssertGRPCStatusContains(err, codes.InvalidArgument, "unhandled match behavior")
}

func (s *PluginSuite) TestListRegistrationEntriesByParentID() {
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

func (s *PluginSuite) TestListRegistrationEntriesBySelectorsExactMatch() {
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

func (s *PluginSuite) TestListRegistrationEntriesBySelectorSubset() {
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
	testTemplates := s.generateListRegistrationPaginationTestTemplates()
	var tests []ListRegistrationEntryPaginationTest
	for _, testTempl := range testTemplates {
		for pageSize := 1; pageSize <= len(testTempl.expectedEntries)+1; pageSize++ {
			tests = append(tests, s.generateRegistrationEntryPaginationTest(pageSize, testTempl))
		}
	}

	for _, test := range tests {
		execute := test.execute
		s.T().Run(test.name, func(t *testing.T) {
			execute()
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
		pageSize := test.pageSize
		s.T().Run(test.name, func(t *testing.T) {
			req := &datastore.ListRegistrationEntriesRequest{
				Pagination: &datastore.Pagination{
					PageSize: pageSize,
				},
			}

			_, err := s.ds.ListRegistrationEntries(ctx, req)
			s.AssertGRPCStatusContains(err, codes.InvalidArgument, "cannot paginate with pagesize")
		})
	}
}

func (s *PluginSuite) TestPruneRegistrationEntries() {
	now := time.Now().Unix()
	registeredEntry := &common.RegistrationEntry{
		Selectors: []*common.Selector{
			{Type: "Type1", Value: "Value1"},
			{Type: "Type2", Value: "Value2"},
			{Type: "Type3", Value: "Value3"},
		},
		SpiffeId:    "SpiffeId",
		ParentId:    "ParentId",
		Ttl:         1,
		EntryExpiry: now,
	}

	createdEntry := s.createRegistrationEntry(registeredEntry, s.ds)

	// Ensure we don't prune valid entries, wind clock back 10s
	_, err := s.ds.PruneRegistrationEntries(ctx, &datastore.PruneRegistrationEntriesRequest{
		ExpiresBefore: now - 10,
	})
	s.Require().NoError(err)

	fReq := &datastore.FetchRegistrationEntryRequest{
		EntryId: createdEntry.EntryId,
	}

	fetchRegistrationEntryResponse, err := s.ds.FetchRegistrationEntry(ctx, fReq)
	s.Require().NoError(err)
	s.Require().NotNil(fetchRegistrationEntryResponse)
	s.Equal(createdEntry, fetchRegistrationEntryResponse.Entry)

	// Ensure we don't prune on the exact ExpiresBefore
	pruneNowReq := &datastore.PruneRegistrationEntriesRequest{
		ExpiresBefore: now,
	}

	_, err = s.ds.PruneRegistrationEntries(ctx, pruneNowReq)
	s.Require().NoError(err)

	fetchRegistrationEntryResponse, err = s.ds.FetchRegistrationEntry(ctx, fReq)
	s.Require().NoError(err)
	s.Require().NotNil(fetchRegistrationEntryResponse)
	s.Equal(createdEntry, fetchRegistrationEntryResponse.Entry)

	// Ensure we prune old entries
	pruneOldReq := &datastore.PruneRegistrationEntriesRequest{
		ExpiresBefore: now + 10,
	}

	_, err = s.ds.PruneRegistrationEntries(ctx, pruneOldReq)
	s.Require().NoError(err)

	fetchRegistrationEntryResponse, err = s.ds.FetchRegistrationEntry(ctx, fReq)
	s.Require().NoError(err)
	s.Nil(fetchRegistrationEntryResponse.Entry)
}

func (s *PluginSuite) TestUpdateRegistrationEntry() {
	entryToCreate := &common.RegistrationEntry{
		Selectors: []*common.Selector{
			{
				Type:  "Type1",
				Value: "Value1",
			},
			{
				Type:  "Type2",
				Value: "Value2",
			},
			{
				Type:  "Type3",
				Value: "Value3",
			},
		},
		SpiffeId: "spiffe://example.org/foo",
		ParentId: "spiffe://example.org/bar",
		Ttl:      1,
	}

	entry := s.createRegistrationEntry(entryToCreate, s.ds)

	entry.Ttl = 2
	entry.Admin = true
	entry.Downstream = true

	uReq := &datastore.UpdateRegistrationEntryRequest{
		Entry: entry,
	}

	updateRegistrationEntryResponse, err := s.ds.UpdateRegistrationEntry(ctx, uReq)

	s.Require().NoError(err)
	s.Require().NotNil(updateRegistrationEntryResponse)

	fReq := &datastore.FetchRegistrationEntryRequest{
		EntryId: entry.EntryId,
	}

	fetchRegistrationEntryResponse, err := s.ds.FetchRegistrationEntry(ctx, fReq)
	s.Require().NoError(err)
	s.Require().NotNil(fetchRegistrationEntryResponse)
	s.Require().NotNil(fetchRegistrationEntryResponse.Entry)
	s.RequireProtoEqual(entry, fetchRegistrationEntryResponse.Entry)

	entry.EntryId = "badid"
	_, err = s.ds.UpdateRegistrationEntry(ctx, uReq)

	s.AssertGRPCStatusContains(err, codes.NotFound, "registration entry not found for entry id")
}

func (s *PluginSuite) TestUpdateNilEntry() {
	uReq := &datastore.UpdateRegistrationEntryRequest{}
	_, err := s.ds.UpdateRegistrationEntry(ctx, uReq)
	s.AssertGRPCStatusContains(err, codes.InvalidArgument, "entry cannot be nil")
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

func (s *PluginSuite) generateListRegistrationPaginationTestTemplates() []ListRegistrationEntryPaginationTest {
	selectors, entries := s.generateRegistrationEntryPaginationTestData()
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

func (s *PluginSuite) generateRegistrationEntryPaginationTestData() ([]*common.Selector, []*common.RegistrationEntry) {
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

func (s *PluginSuite) generateRegistrationEntryPaginationTest(pageSize int, testTemplate ListRegistrationEntryPaginationTest) ListRegistrationEntryPaginationTest {
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
	test.req.Pagination = &datastore.Pagination{
		PageSize: int32(test.pageSize),
	}

	resultsByEntryID := s.executePaginatedListRegistrationEntriesRequests(test.pageSize, numExpectedEntries, test.req, ds)
	s.assertSameRegistrationEntries(test.expectedEntries, resultsByEntryID)
}

func (s *PluginSuite) calculateNumExpectedPagedRequests(numExpectedEntries, pageSize int) (int, bool) {
	var numExpectedRequests int
	var lastEmptyRequest bool
	if numExpectedEntries >= pageSize {
		// When the last page with results has exactly PageSize number of results,
		// we will still get a pagination token in the response.
		// We need to make one more request, which should have no results.
		lastEmptyRequest = numExpectedEntries%pageSize == 0
		numExpectedRequests = (numExpectedEntries / pageSize) + 1
	} else {
		numExpectedRequests = 1
	}

	return numExpectedRequests, lastEmptyRequest
}

func (s *PluginSuite) executePaginatedListRegistrationEntriesRequests(
	pageSize,
	numExpectedEntries int,
	req *datastore.ListRegistrationEntriesRequest,
	ds datastore.Plugin) map[string]*common.RegistrationEntry {
	resultsByEntryID := make(map[string]*common.RegistrationEntry, numExpectedEntries)
	numExpectedRequests, lastEmptyRequest := s.calculateNumExpectedPagedRequests(numExpectedEntries, pageSize)

	for reqNum := 1; reqNum <= numExpectedRequests; reqNum++ {
		s.executePaginatedListRegistrationEntriesRequest(reqNum, numExpectedRequests, lastEmptyRequest, req, ds, resultsByEntryID)
	}

	return resultsByEntryID
}

func (s *PluginSuite) executePaginatedListRegistrationEntriesRequest(
	reqNum,
	numExpectedRequests int,
	lastEmptyRequest bool,
	req *datastore.ListRegistrationEntriesRequest,
	ds datastore.Plugin,
	resultsByEntryID map[string]*common.RegistrationEntry) {
	resp, err := ds.ListRegistrationEntries(ctx, req)
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	if reqNum == numExpectedRequests {
		if lastEmptyRequest {
			s.Assert().Empty(resp.Entries)
			return
		}
	} else {
		s.Require().NotNil(resp.Pagination)
		s.Require().NotEqual("", resp.Pagination.Token, "didn't receive pagination token for list request #%d out of %d expected requests", reqNum, numExpectedRequests)
		req.Pagination.Token = resp.Pagination.Token
	}

	s.Require().NotNil(resp.Entries, "received nil entries in response #%d of %d expected requests", reqNum, numExpectedRequests)
	s.Require().True(len(resp.Entries) > 0, "received empty entries in response #%d of %d expected requests", reqNum, numExpectedRequests)

	for _, entry := range resp.Entries {
		_, ok := resultsByEntryID[entry.EntryId]
		s.Assert().False(ok, "received same entry in multiple pages for entry id: %v", entry.EntryId)
		resultsByEntryID[entry.EntryId] = entry
	}
}

func (s *PluginSuite) assertSameRegistrationEntries(expectedEntries []*common.RegistrationEntry, actualEntriesByEntryID map[string]*common.RegistrationEntry) {
	s.Assert().Equal(len(expectedEntries), len(actualEntriesByEntryID))
	var resultEntryIDs []string
	for entryID := range actualEntriesByEntryID {
		resultEntryIDs = append(resultEntryIDs, entryID)
	}

	var expectedEntryIDs []string
	for _, entry := range expectedEntries {
		expectedEntryIDs = append(expectedEntryIDs, entry.EntryId)
	}

	s.Assert().ElementsMatch(expectedEntryIDs, resultEntryIDs)
	for _, expectedEntry := range expectedEntries {
		actualEntry, ok := actualEntriesByEntryID[expectedEntry.EntryId]
		s.Require().True(ok) // duplicate check of entry id from above, to do all we can to avoid panics
		s.AssertProtoEqual(expectedEntry, actualEntry)
	}
}

func (s *PluginSuite) fetchRegistrationEntry(entryID string) *common.RegistrationEntry {
	resp, err := s.ds.FetchRegistrationEntry(ctx, &datastore.FetchRegistrationEntryRequest{
		EntryId: entryID,
	})
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Require().NotNil(resp.Entry)
	return resp.Entry
}
