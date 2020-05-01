package kv

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/spiffe/spire/pkg/server/plugin/datastore"
	"github.com/spiffe/spire/proto/spire/common"
	"google.golang.org/grpc/codes"
)

type ListAttestedNodeRequestPaginationTest struct {
	s             *PluginSuite
	name          string
	req           *datastore.ListAttestedNodesRequest
	expectedNodes []*common.AttestedNode
	pageSize      int
}

func (s *PluginSuite) TestCreateAttestedNode() {
	beginTestTime := time.Now()
	node := &common.AttestedNode{
		SpiffeId:            "foo",
		AttestationDataType: "aws-tag",
		CertSerialNumber:    "badcafe",
		CertNotAfter:        beginTestTime.Add(time.Hour).Unix(),
	}

	creq := &datastore.CreateAttestedNodeRequest{
		Node: node,
	}

	cresp, err := s.ds.CreateAttestedNode(ctx, creq)
	s.Require().NoError(err)
	s.AssertProtoEqual(node, cresp.Node)

	freq := &datastore.FetchAttestedNodeRequest{
		SpiffeId: node.SpiffeId,
	}

	fresp, err := s.ds.FetchAttestedNode(ctx, freq)
	s.Require().NoError(err)
	s.AssertProtoEqual(node, fresp.Node)

	listAllReq := &datastore.ListAttestedNodesRequest{}
	listAllResp, err := s.ds.ListAttestedNodes(ctx, listAllReq)
	s.Require().NoError(err)
	expectedNodes := []*common.AttestedNode{node}
	s.AssertProtoListEqual(expectedNodes, listAllResp.Nodes)
}

func (s *PluginSuite) TestDeleteAttestedNode() {
	entry := &common.AttestedNode{
		SpiffeId:            "foo",
		AttestationDataType: "aws-tag",
		CertSerialNumber:    "badcafe",
		CertNotAfter:        time.Now().Add(time.Hour).Unix(),
	}

	// delete it before it exists
	dReq := &datastore.DeleteAttestedNodeRequest{
		SpiffeId: entry.SpiffeId,
	}

	_, err := s.ds.DeleteAttestedNode(ctx, dReq)
	s.RequireGRPCStatusContains(err, codes.NotFound, "attested node was not found with spiffe id")

	cReq := &datastore.CreateAttestedNodeRequest{
		Node: entry,
	}

	_, err = s.ds.CreateAttestedNode(ctx, cReq)
	s.Require().NoError(err)

	dResp, err := s.ds.DeleteAttestedNode(ctx, dReq)
	s.Require().NoError(err)
	s.AssertProtoEqual(entry, dResp.Node)

	fReq := &datastore.FetchAttestedNodeRequest{
		SpiffeId: entry.SpiffeId,
	}

	fResp, err := s.ds.FetchAttestedNode(ctx, fReq)
	s.Require().NoError(err)
	s.Nil(fResp.Node)
}

func (s *PluginSuite) TestFetchAttestedNodeMissing() {
	fresp, err := s.ds.FetchAttestedNode(ctx, &datastore.FetchAttestedNodeRequest{SpiffeId: "missing"})
	s.Require().NoError(err)
	s.Require().Nil(fresp.Node)
}

func (s *PluginSuite) TestListAttestedNodesWithPagination() {
	now := time.Now()
	validTime := now.Add(time.Hour).Unix()
	expTime := now.Add(-time.Hour).Unix()
	expiredNodes := []*common.AttestedNode{
		{
			SpiffeId:            "expiredNode1",
			AttestationDataType: "aws-tag",
			CertSerialNumber:    "badcafe",
			CertNotAfter:        expTime,
		},
		{
			SpiffeId:            "expiredNode2",
			AttestationDataType: "aws-tag",
			CertSerialNumber:    "badcafe",
			CertNotAfter:        expTime,
		},
		{
			SpiffeId:            "expiredNode3",
			AttestationDataType: "aws-tag",
			CertSerialNumber:    "badcafe",
			CertNotAfter:        expTime,
		},
	}

	validNodes := []*common.AttestedNode{
		{
			SpiffeId:            "validNode1",
			AttestationDataType: "aws-tag",
			CertSerialNumber:    "deadbeef",
			CertNotAfter:        validTime,
		},
		{
			SpiffeId:            "validNode2",
			AttestationDataType: "aws-tag",
			CertSerialNumber:    "foobar",
			CertNotAfter:        validTime,
		},
	}

	nodes := append(expiredNodes, validNodes...)

	testTemplates := []struct {
		name          string
		req           *datastore.ListAttestedNodesRequest
		expectedNodes []*common.AttestedNode
	}{
		{
			name:          "all",
			req:           &datastore.ListAttestedNodesRequest{},
			expectedNodes: nodes,
		},
	}

	for _, testTemplate := range testTemplates {
		for pageSize := 1; pageSize <= len(nodes)+1; pageSize++ {
			test := ListAttestedNodeRequestPaginationTest{
				s:             s,
				name:          fmt.Sprintf("%s with page size %v", testTemplate.name, pageSize),
				req:           testTemplate.req,
				expectedNodes: testTemplate.expectedNodes,
				pageSize:      pageSize,
			}

			s.T().Run(test.name, func(t *testing.T) {
				ds := s.newPlugin()
				for _, node := range nodes {
					cReq := &datastore.CreateAttestedNodeRequest{
						Node: node,
					}

					_, err := ds.CreateAttestedNode(ctx, cReq)
					s.Require().NoError(err)
				}

				numExpectedNodes := len(test.expectedNodes)
				test.req.Pagination = &datastore.Pagination{
					PageSize: int32(test.pageSize),
				}

				resultsBySpiffeId := s.executePaginatedListAttestedNodesRequests(test.pageSize, numExpectedNodes, test.req, ds)
				s.assertSameAttestedNodes(test.expectedNodes, resultsBySpiffeId)
			})
		}
	}
}

func (s *PluginSuite) TestListAttestedNodesWithInvalidPagination() {
	tests := []struct {
		name        string
		pagination  *datastore.Pagination
		expectedErr string
	}{
		{
			name: "invalid token",
			pagination: &datastore.Pagination{
				Token:    ";.",
				PageSize: 1,
			},
			expectedErr: "invalid pagination token",
		},
		{
			name: "invalid page size",
			pagination: &datastore.Pagination{
				PageSize: -1,
			},
			expectedErr: "cannot paginate with pagesize",
		},
	}

	for _, test := range tests {
		s.T().Run(test.name, func(t *testing.T) {
			req := &datastore.ListAttestedNodesRequest{
				Pagination: test.pagination,
			}

			_, err := s.ds.ListAttestedNodes(ctx, req)
			s.AssertGRPCStatusContains(err, codes.InvalidArgument, test.expectedErr)
		})
	}
}

func (s *PluginSuite) TestListAttestedNodesWithByExpiresBefore() {
	req := &datastore.ListAttestedNodesRequest{
		ByExpiresBefore: &wrappers.Int64Value{
			Value: 12345,
		},
	}

	_, err := s.ds.ListAttestedNodes(ctx, req)
	s.AssertGRPCStatus(err, codes.Unimplemented, "by-expires-before support not implemented")
}

func (s *PluginSuite) TestUpdateAttestedNode() {
	node := &common.AttestedNode{
		SpiffeId:            "foo",
		AttestationDataType: "aws-tag",
		CertSerialNumber:    "badcafe",
		CertNotAfter:        time.Now().Add(time.Hour).Unix(),
	}

	uSerial := "deadbeef"
	uExpires := time.Now().Add(time.Hour * 2).Unix()

	// update non-existing attested node
	uReq := &datastore.UpdateAttestedNodeRequest{
		SpiffeId:         node.SpiffeId,
		CertSerialNumber: uSerial,
		CertNotAfter:     uExpires,
	}

	_, err := s.ds.UpdateAttestedNode(ctx, uReq)
	s.RequireGRPCStatusContains(err, codes.NotFound, "attested node not found for spiffe id")

	cReq := &datastore.CreateAttestedNodeRequest{
		Node: node,
	}

	_, err = s.ds.CreateAttestedNode(ctx, cReq)
	s.Require().NoError(err)

	uResp, err := s.ds.UpdateAttestedNode(ctx, uReq)
	s.Require().NoError(err)

	uNode := uResp.Node
	s.Require().NotNil(uNode)

	s.Equal(node.SpiffeId, uNode.SpiffeId)
	s.Equal(node.AttestationDataType, uNode.AttestationDataType)
	s.Equal(uSerial, uNode.CertSerialNumber)
	s.Equal(uExpires, uNode.CertNotAfter)

	fReq := &datastore.FetchAttestedNodeRequest{
		SpiffeId: node.SpiffeId,
	}

	fResp, err := s.ds.FetchAttestedNode(ctx, fReq)
	s.Require().NoError(err)

	fNode := fResp.Node
	s.Require().NotNil(fNode)

	s.Equal(node.SpiffeId, fNode.SpiffeId)
	s.Equal(node.AttestationDataType, fNode.AttestationDataType)
	s.Equal(uSerial, fNode.CertSerialNumber)
	s.Equal(uExpires, fNode.CertNotAfter)
}

func (s *PluginSuite) executePaginatedListAttestedNodesRequests(
	pageSize,
	numExpectedNodes int,
	req *datastore.ListAttestedNodesRequest,
	ds datastore.Plugin) map[string]*common.AttestedNode {

	numExpectedRequests, lastEmptyRequest := s.calculateNumExpectedPagedRequests(numExpectedNodes, pageSize)
	resultsBySpiffeId := make(map[string]*common.AttestedNode, numExpectedNodes)
	for reqNum := 1; reqNum <= numExpectedRequests; reqNum++ {
		s.executePaginatedListAttestedNodesRequest(reqNum, numExpectedRequests, lastEmptyRequest, req, ds, resultsBySpiffeId)
	}

	return resultsBySpiffeId
}

func (s *PluginSuite) executePaginatedListAttestedNodesRequest(
	reqNum,
	numExpectedRequests int,
	lastEmptyRequest bool,
	req *datastore.ListAttestedNodesRequest,
	ds datastore.Plugin,
	resultsBySpiffeId map[string]*common.AttestedNode) {

	resp, err := ds.ListAttestedNodes(ctx, req)
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	if reqNum == numExpectedRequests {
		if lastEmptyRequest {
			s.Assert().Empty(resp.Nodes)
			return
		}
	} else {
		s.Require().NotNil(resp.Pagination)
		s.Require().NotEqual("", resp.Pagination.Token, "didn't receive pagination token for list request #%d out of %d expected requests", reqNum, numExpectedRequests)
		req.Pagination.Token = resp.Pagination.Token
	}

	s.Require().NotNil(resp.Nodes, "received nil nodes in response #%d of %d expected requests", reqNum, numExpectedRequests)
	s.Require().True(len(resp.Nodes) > 0, "received empty nodes in response #%d of %d expected requests", reqNum, numExpectedRequests)

	for _, node := range resp.Nodes {
		_, ok := resultsBySpiffeId[node.SpiffeId]
		s.Assert().False(ok, "received same node in multiple pages for spiffe id: %v", node.SpiffeId)
		resultsBySpiffeId[node.SpiffeId] = node
	}
}

func (s *PluginSuite) assertSameAttestedNodes(expectedNodes []*common.AttestedNode, actualNodesBySpiffeId map[string]*common.AttestedNode) {
	s.Assert().Equal(len(expectedNodes), len(actualNodesBySpiffeId))
	var resultSpiffeIds []string
	for spiffeId := range actualNodesBySpiffeId {
		resultSpiffeIds = append(resultSpiffeIds, spiffeId)
	}

	var expectedSpiffeIds []string
	for _, node := range expectedNodes {
		expectedSpiffeIds = append(expectedSpiffeIds, node.SpiffeId)
	}

	s.Assert().ElementsMatch(expectedSpiffeIds, resultSpiffeIds)
	for _, expectedNode := range expectedNodes {
		actualNode, ok := actualNodesBySpiffeId[expectedNode.SpiffeId]
		s.Require().True(ok)
		s.AssertProtoEqual(expectedNode, actualNode)
	}
}
