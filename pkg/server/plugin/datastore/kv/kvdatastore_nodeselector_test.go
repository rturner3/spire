package kv

import (
	"fmt"

	"github.com/spiffe/spire/pkg/server/plugin/datastore"
	"github.com/spiffe/spire/proto/spire/common"
)

func (s *PluginSuite) TestNodeSelectors() {
	foo1 := []*common.Selector{
		{Type: "FOO1", Value: "1"},
	}
	foo2 := []*common.Selector{
		{Type: "FOO2", Value: "1"},
	}
	bar := []*common.Selector{
		{Type: "BAR", Value: "FIGHT"},
	}

	fooSpiffeID := "foo"
	barSpiffeID := "bar"
	emptySelectors := []*common.Selector{}

	// assert there are no selectors for foo
	s.assertGetNodeSelectorsEqual(fooSpiffeID, emptySelectors)

	// set selectors on foo and bar
	s.setNodeSelectors(fooSpiffeID, foo1)
	s.setNodeSelectors(barSpiffeID, bar)

	// get foo selectors
	s.assertGetNodeSelectorsEqual(fooSpiffeID, foo1)

	// replace foo selectors
	s.setNodeSelectors(fooSpiffeID, foo2)
	s.assertGetNodeSelectorsEqual(fooSpiffeID, foo2)

	// delete foo selectors
	s.setNodeSelectors(fooSpiffeID, nil)
	s.assertGetNodeSelectorsEqual(fooSpiffeID, emptySelectors)

	// get bar selectors (make sure they weren't impacted by deleting foo)
	s.assertGetNodeSelectorsEqual(barSpiffeID, bar)
}

func (s *PluginSuite) TestSetNodeSelectorsUnderLoad() {
	selectors := []*common.Selector{
		{Type: "TYPE", Value: "VALUE"},
	}

	const numWorkers = 20
	const numRequestsPerWorker = 10
	inputChans := make([]chan int, numWorkers)
	resultChans := make([]chan error, numWorkers)
	for i := 0; i < numWorkers; i++ {
		inputChans[i] = make(chan int)
		resultChans[i] = make(chan error, numRequestsPerWorker)
	}

	for i := 0; i < numWorkers; i++ {
		inCh := inputChans[i]
		resultCh := resultChans[i]
		go func() {
			id := <-inCh
			spiffeID := fmt.Sprintf("ID%d", id)
			for j := 0; j < numRequestsPerWorker; j++ {
				_, err := s.ds.SetNodeSelectors(ctx, &datastore.SetNodeSelectorsRequest{
					Selectors: &datastore.NodeSelectors{
						SpiffeId:  spiffeID,
						Selectors: selectors,
					},
				})
				if err != nil {
					resultCh <- err
				}
			}
			resultCh <- nil
		}()
	}

	for i, ch := range inputChans {
		ch <- i
	}

	for _, resultCh := range resultChans {
		s.Require().NoError(<-resultCh)
	}
}

func (s *PluginSuite) getNodeSelectors(spiffeID string, tolerateStale bool) []*common.Selector {
	resp, err := s.ds.GetNodeSelectors(ctx, &datastore.GetNodeSelectorsRequest{
		SpiffeId:      spiffeID,
		TolerateStale: tolerateStale,
	})

	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Require().NotNil(resp.Selectors)
	s.Require().Equal(spiffeID, resp.Selectors.SpiffeId)
	return resp.Selectors.Selectors
}

func (s *PluginSuite) setNodeSelectors(spiffeID string, selectors []*common.Selector) {
	resp, err := s.ds.SetNodeSelectors(ctx, &datastore.SetNodeSelectorsRequest{
		Selectors: &datastore.NodeSelectors{
			SpiffeId:  spiffeID,
			Selectors: selectors,
		},
	})

	s.Require().NoError(err)
	s.RequireProtoEqual(&datastore.SetNodeSelectorsResponse{}, resp)
}

func (s *PluginSuite) assertGetNodeSelectorsEqual(spiffeID string, expectedSelectors []*common.Selector) {
	for _, tolerateStale := range []bool{true, false} {
		selectors := s.getNodeSelectors(spiffeID, tolerateStale)
		s.RequireProtoListEqual(expectedSelectors, selectors)
	}
}
