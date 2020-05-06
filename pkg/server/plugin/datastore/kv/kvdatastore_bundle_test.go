package kv

import (
	"crypto/x509"
	"errors"
	"time"

	"github.com/spiffe/spire/pkg/common/bundleutil"
	"github.com/spiffe/spire/pkg/server/plugin/datastore"
	"github.com/spiffe/spire/proto/spire/common"
	"google.golang.org/grpc/codes"
)

func (s *PluginSuite) TestAppendBundle() {
	trustDomain := "spiffe://foo"
	bundle1 := bundleutil.BundleProtoFromRootCA(trustDomain, s.caCert)
	bundle2 := bundleutil.BundleProtoFromRootCA(trustDomain, s.cert)
	certs := []*x509.Certificate{s.caCert, s.cert}
	appendedBundle := bundleutil.BundleProtoFromRootCAs(trustDomain, certs)

	// Appending to non-existent bundle should create bundle
	aReq1 := &datastore.AppendBundleRequest{
		Bundle: bundle1,
	}
	aResp1, err := s.ds.AppendBundle(ctx, aReq1)
	s.Require().NoError(err)
	s.Require().NotNil(aResp1)
	s.Assert().NotNil(aResp1.Bundle)

	// Appending to existing bundle for trust domain ID should append to rootCAs
	aReq2 := &datastore.AppendBundleRequest{
		Bundle: bundle2,
	}
	aResp2, err := s.ds.AppendBundle(ctx, aReq2)
	s.Require().NoError(err)
	s.Require().NotNil(aResp2)
	s.Require().NotNil(aResp2.Bundle)
	s.AssertProtoEqual(appendedBundle, aResp2.Bundle)

	// Appending to existing bundle with contents already in bundle does nothing
	aResp3, err := s.ds.AppendBundle(ctx, aReq2)
	s.Require().NoError(err)
	s.Require().NotNil(aResp3)
	s.Require().NotNil(aResp3.Bundle)
	s.AssertProtoEqual(appendedBundle, aResp3.Bundle)
}

func (s *PluginSuite) TestAppendNilBundle() {
	nilBundleReq := &datastore.AppendBundleRequest{}
	_, err := s.ds.AppendBundle(ctx, nilBundleReq)
	s.AssertGRPCStatus(err, codes.InvalidArgument, "bundle must be non-nil")
}

func (s *PluginSuite) TestCreateBundle() {
	trustDomain := "spiffe://foo"
	bundle := bundleutil.BundleProtoFromRootCA(trustDomain, s.caCert)

	cReq := &datastore.CreateBundleRequest{
		Bundle: bundle,
	}
	cResp, err := s.ds.CreateBundle(ctx, cReq)
	s.Require().NoError(err)
	s.Require().NotNil(cResp)
	s.Assert().NotNil(cResp.Bundle)

	fReq := &datastore.FetchBundleRequest{
		TrustDomainId: trustDomain,
	}
	fResp, err := s.ds.FetchBundle(ctx, fReq)
	s.Require().NoError(err)
	s.Require().NotNil(fResp)
	s.Require().NotNil(fResp.Bundle)
	s.Assert().Equal(trustDomain, fResp.Bundle.TrustDomainId)

	lReq := &datastore.ListBundlesRequest{}
	lResp, err := s.ds.ListBundles(ctx, lReq)
	s.Require().NoError(err)
	s.Require().NotNil(lResp)
	s.Require().NotEmpty(lResp.Bundles)
	expectedBundles := []*common.Bundle{cResp.Bundle}
	s.AssertProtoListEqual(expectedBundles, lResp.Bundles)
}

func (s *PluginSuite) TestCreateNilBundle() {
	createNilBundleReq := &datastore.CreateBundleRequest{}
	_, err := s.ds.CreateBundle(ctx, createNilBundleReq)
	s.AssertGRPCStatus(err, codes.InvalidArgument, "bundle must be non-nil")
}

func (s *PluginSuite) TestDeleteBundleRestrictedByRegistrationEntries() {
	// create the bundle and associated entry
	trustDomain := "spiffe://otherdomain.org"
	s.createBundle(trustDomain)

	federatedEntry := &common.RegistrationEntry{
		Selectors: []*common.Selector{
			{
				Type:  "Type1",
				Value: "Value1",
			},
		},
		SpiffeId:      "spiffe://example.org/foo",
		FederatesWith: []string{trustDomain},
	}

	s.createRegistrationEntry(federatedEntry, s.ds)

	dReq := &datastore.DeleteBundleRequest{
		Mode:          datastore.DeleteBundleRequest_RESTRICT,
		TrustDomainId: "spiffe://otherdomain.org",
	}

	_, err := s.ds.DeleteBundle(ctx, dReq)
	s.RequireErrorContains(err, "cannot delete bundle; federated with 1 registration entries")
}

func (s *PluginSuite) TestDeleteBundleDeleteRegistrationEntries() {
	// create an unrelated registration entry to make sure the delete
	// operation only deletes associated registration entries.
	unrelatedEntryToCreate := &common.RegistrationEntry{
		Selectors: []*common.Selector{
			{
				Type:  "TYPE",
				Value: "VALUE",
			},
		},
		SpiffeId: "spiffe://example.org/foo",
	}

	trustDomain := "spiffe://otherdomain.org"
	unrelated := s.createRegistrationEntry(unrelatedEntryToCreate, s.ds)

	federatedEntry := &common.RegistrationEntry{
		FederatesWith: []string{trustDomain},
		Selectors: []*common.Selector{
			{
				Type:  "Type1",
				Value: "Value1",
			},
		},
		SpiffeId: "spiffe://example.org/foo",
	}

	// create the bundle and associated entry
	s.createBundle(trustDomain)
	entry := s.createRegistrationEntry(federatedEntry, s.ds)

	dReq := &datastore.DeleteBundleRequest{
		TrustDomainId: trustDomain,
		Mode:          datastore.DeleteBundleRequest_DELETE,
	}

	// delete the bundle in DELETE mode
	_, err := s.ds.DeleteBundle(ctx, dReq)
	s.Require().NoError(err)

	fReq := &datastore.FetchRegistrationEntryRequest{
		EntryId: entry.EntryId,
	}
	// verify that the registration entry has been deleted
	resp, err := s.ds.FetchRegistrationEntry(ctx, fReq)
	s.Require().NoError(err)
	s.Require().Nil(resp.Entry)

	// make sure the unrelated entry still exists
	s.fetchRegistrationEntry(unrelated.EntryId)
}

func (s *PluginSuite) TestDeleteBundleDissociateRegistrationEntries() {
	trustDomain := "spiffe://otherdomain.org"
	// create the bundle and associated entry
	s.createBundle(trustDomain)

	federatedEntry := &common.RegistrationEntry{
		FederatesWith: []string{trustDomain},
		Selectors: []*common.Selector{
			{
				Type:  "Type1",
				Value: "Value1",
			},
		},
		SpiffeId: "spiffe://example.org/foo",
	}

	entry := s.createRegistrationEntry(federatedEntry, s.ds)

	// delete the bundle in DISSOCIATE mode
	dReq := &datastore.DeleteBundleRequest{
		TrustDomainId: trustDomain,
		Mode:          datastore.DeleteBundleRequest_DISSOCIATE,
	}

	_, err := s.ds.DeleteBundle(ctx, dReq)
	s.Require().NoError(err)

	// make sure the entry still exists, albeit without an associated bundle
	entry = s.fetchRegistrationEntry(entry.EntryId)
	s.Require().Empty(entry.FederatesWith)
}

func (s *PluginSuite) TestDeleteBundleNonexistentBundle() {
	dReq := &datastore.DeleteBundleRequest{
		TrustDomainId: "spiffe://foo.domain",
	}

	_, err := s.ds.DeleteBundle(ctx, dReq)
	s.AssertGRPCStatusContains(err, codes.NotFound, "record not found")
}

func (s *PluginSuite) TestDeleteBundleUnrecognizedDeleteMode() {
	trustDomain := "spiffe://foo.domain"
	s.createBundle(trustDomain)

	dReq := &datastore.DeleteBundleRequest{
		Mode:          -1,
		TrustDomainId: trustDomain,
	}

	_, err := s.ds.DeleteBundle(ctx, dReq)
	s.RequireGRPCStatusContains(err, codes.InvalidArgument, "unrecognized delete mode")
}

func (s *PluginSuite) TestPruneBundle() {
	trustDomain := "spiffe://foo"
	// Setup
	// Create new bundle with two cert (one valid and one expired)
	bundle := bundleutil.BundleProtoFromRootCAs(trustDomain, []*x509.Certificate{s.cert, s.caCert})

	// Add two JWT signing keys (one valid and one expired)
	expiredKeyTime, err := time.Parse(time.RFC3339, expiredNotAfterString)
	s.Require().NoError(err)

	nonExpiredKeyTime, err := time.Parse(time.RFC3339, validNotAfterString)
	s.Require().NoError(err)

	// middleTime is a point between the two certs expiration time
	middleTime, err := time.Parse(time.RFC3339, middleTimeString)
	s.Require().NoError(err)

	bundle.JwtSigningKeys = []*common.PublicKey{
		{
			NotAfter: expiredKeyTime.Unix(),
		},
		{
			NotAfter: nonExpiredKeyTime.Unix(),
		},
	}

	// Store bundle in datastore
	cReq := &datastore.CreateBundleRequest{Bundle: bundle}
	_, err = s.ds.CreateBundle(ctx, cReq)
	s.Require().NoError(err)

	// Prune
	// prune non existent bundle should not return error, no bundle to prune
	expiration := time.Now().Unix()
	pReqNonExistent := &datastore.PruneBundleRequest{
		TrustDomainId: "spiffe://notexistent",
		ExpiresBefore: expiration,
	}
	pResp, err := s.ds.PruneBundle(ctx, pReqNonExistent)
	emptyPResp := &datastore.PruneBundleResponse{}
	s.NoError(err)
	s.AssertProtoEqual(emptyPResp, pResp)

	// prune fails if internal prune bundle fails. For instance, if all certs are expired
	expiration = time.Now().Unix()
	pReqAllExpired := &datastore.PruneBundleRequest{
		TrustDomainId: bundle.TrustDomainId,
		ExpiresBefore: expiration,
	}
	pResp, err = s.ds.PruneBundle(ctx, pReqAllExpired)
	expectedError := errors.New("prune failed: would prune all certificates")
	s.Error(err, expectedError.Error())
	s.Nil(pResp)

	// prune should remove expired certs
	pReqSomeExpired := &datastore.PruneBundleRequest{
		TrustDomainId: bundle.TrustDomainId,
		ExpiresBefore: middleTime.Unix(),
	}
	pResp, err = s.ds.PruneBundle(ctx, pReqSomeExpired)
	s.NoError(err)
	s.NotNil(pResp)
	s.True(pResp.BundleChanged)

	// Fetch and verify pruned bundle is the expected
	expectedPrunedBundle := bundleutil.BundleProtoFromRootCAs(trustDomain, []*x509.Certificate{s.cert})
	expectedPrunedBundle.JwtSigningKeys = []*common.PublicKey{
		{
			NotAfter: nonExpiredKeyTime.Unix(),
		},
	}

	fReq := &datastore.FetchBundleRequest{
		TrustDomainId: trustDomain,
	}

	fResp, err := s.ds.FetchBundle(ctx, fReq)
	s.Require().NoError(err)
	s.AssertProtoEqual(expectedPrunedBundle, fResp.Bundle)
}

func (s *PluginSuite) TestSetBundle() {
	trustDomain := "spiffe://foo"
	// create a couple of bundles for tests. the contents don't really matter
	// as long as they are for the same trust domain but have different contents.
	bundle := bundleutil.BundleProtoFromRootCA(trustDomain, s.cert)
	bundle2 := bundleutil.BundleProtoFromRootCA(trustDomain, s.caCert)

	// ensure the bundle does not exist (it shouldn't)
	fetchBundleResp := s.fetchBundle(trustDomain)
	s.Require().Nil(fetchBundleResp)

	// set the bundle and make sure it is created
	sReq1 := &datastore.SetBundleRequest{
		Bundle: bundle,
	}
	_, err := s.ds.SetBundle(ctx, sReq1)
	s.Require().NoError(err)

	fetchBundleResp = s.fetchBundle(trustDomain)
	s.RequireProtoEqual(bundle, fetchBundleResp)

	// set the bundle and make sure it is updated
	sReq2 := &datastore.SetBundleRequest{
		Bundle: bundle2,
	}
	_, err = s.ds.SetBundle(ctx, sReq2)
	s.Require().NoError(err)

	fetchBundleResp = s.fetchBundle(trustDomain)
	s.RequireProtoEqual(bundle2, fetchBundleResp)
}

func (s *PluginSuite) TestSetNilBundle() {
	setNilBundleReq := &datastore.SetBundleRequest{}
	_, err := s.ds.SetBundle(ctx, setNilBundleReq)
	s.AssertGRPCStatus(err, codes.InvalidArgument, "bundle must be non-nil")
}

/*func (s *PluginSuite) TestBundleCRUD() {
	bundle := bundleutil.BundleProtoFromRootCA("spiffe://foo", s.cert)

	// fetch non-existent
	fresp, err := s.ds.FetchBundle(ctx, &datastore.FetchBundleRequest{TrustDomainId: "spiffe://foo"})
	s.Require().NoError(err)
	s.Require().NotNil(fresp)
	s.Require().Nil(fresp.Bundle)

	// update non-existent
	_, err = s.ds.UpdateBundle(ctx, &datastore.UpdateBundleRequest{Bundle: bundle})
	s.RequireGRPCStatus(err, codes.NotFound, _notFoundErrMsg)

	// delete non-existent
	_, err = s.ds.DeleteBundle(ctx, &datastore.DeleteBundleRequest{TrustDomainId: "spiffe://foo"})
	s.RequireGRPCStatus(err, codes.NotFound, _notFoundErrMsg)

	// create
	_, err = s.ds.CreateBundle(ctx, &datastore.CreateBundleRequest{
		Bundle: bundle,
	})
	s.Require().NoError(err)

	// fetch
	fresp, err = s.ds.FetchBundle(ctx, &datastore.FetchBundleRequest{TrustDomainId: "spiffe://foo"})
	s.Require().NoError(err)
	s.AssertProtoEqual(bundle, fresp.Bundle)

	// fetch (with denormalized id)
	fresp, err = s.ds.FetchBundle(ctx, &datastore.FetchBundleRequest{TrustDomainId: "spiffe://fOO"})
	s.Require().NoError(err)
	s.AssertProtoEqual(bundle, fresp.Bundle)

	// list
	lresp, err := s.ds.ListBundles(ctx, &datastore.ListBundlesRequest{})
	s.Require().NoError(err)
	s.Equal(1, len(lresp.Bundles))
	s.AssertProtoEqual(bundle, lresp.Bundles[0])

	bundle2 := bundleutil.BundleProtoFromRootCA(bundle.TrustDomainId, s.caCert)
	appendedBundle := bundleutil.BundleProtoFromRootCAs(bundle.TrustDomainId,
		[]*x509.Certificate{s.cert, s.caCert})

	// append
	aresp, err := s.ds.AppendBundle(ctx, &datastore.AppendBundleRequest{
		Bundle: bundle2,
	})
	s.Require().NoError(err)
	s.Require().NotNil(aresp.Bundle)
	s.AssertProtoEqual(appendedBundle, aresp.Bundle)

	// append identical
	aresp, err = s.ds.AppendBundle(ctx, &datastore.AppendBundleRequest{
		Bundle: bundle2,
	})
	s.Require().NoError(err)
	s.Require().NotNil(aresp.Bundle)
	s.AssertProtoEqual(appendedBundle, aresp.Bundle)

	// append on a new bundle
	bundle3 := bundleutil.BundleProtoFromRootCA("spiffe://bar", s.caCert)
	anresp, err := s.ds.AppendBundle(ctx, &datastore.AppendBundleRequest{
		Bundle: bundle3,
	})
	s.Require().NoError(err)
	s.AssertProtoEqual(bundle3, anresp.Bundle)

	// update
	uresp, err := s.ds.UpdateBundle(ctx, &datastore.UpdateBundleRequest{
		Bundle: bundle2,
	})
	s.Require().NoError(err)
	s.AssertProtoEqual(bundle2, uresp.Bundle)

	lresp, err = s.ds.ListBundles(ctx, &datastore.ListBundlesRequest{})
	s.Require().NoError(err)
	assertBundlesEqual(s.T(), []*common.Bundle{bundle2, bundle3}, lresp.Bundles)

	// delete
	dresp, err := s.ds.DeleteBundle(ctx, &datastore.DeleteBundleRequest{
		TrustDomainId: bundle.TrustDomainId,
	})
	s.Require().NoError(err)
	s.AssertProtoEqual(bundle2, dresp.Bundle)

	lresp, err = s.ds.ListBundles(ctx, &datastore.ListBundlesRequest{})
	s.Require().NoError(err)
	s.Equal(1, len(lresp.Bundles))
	s.AssertProtoEqual(bundle3, lresp.Bundles[0])

	// delete (with denormalized id)
	dresp, err = s.ds.DeleteBundle(ctx, &datastore.DeleteBundleRequest{
		TrustDomainId: "spiffe://bAR",
	})
	s.Require().NoError(err)
	s.AssertProtoEqual(bundle3, dresp.Bundle)

	lresp, err = s.ds.ListBundles(ctx, &datastore.ListBundlesRequest{})
	s.Require().NoError(err)
	s.Empty(lresp.Bundles)
}*/

func (s *PluginSuite) TestUpdateBundle() {
	trustDomain := "spiffe://foo"
	bundle := bundleutil.BundleProtoFromRootCA(trustDomain, s.cert)
	cReq := &datastore.CreateBundleRequest{
		Bundle: bundle,
	}

	_, err := s.ds.CreateBundle(ctx, cReq)
	s.Require().NoError(err)

	bundle2 := bundleutil.BundleProtoFromRootCAs(trustDomain, []*x509.Certificate{s.cert, s.caCert})

	uReq := &datastore.UpdateBundleRequest{
		Bundle: bundle2,
	}

	uResp, err := s.ds.UpdateBundle(ctx, uReq)
	s.Require().NoError(err)
	s.AssertProtoEqual(bundle2, uResp.Bundle)
}

func (s *PluginSuite) TestUpdateNilBundle() {
	updateNilBundleReq := &datastore.UpdateBundleRequest{}
	_, err := s.ds.UpdateBundle(ctx, updateNilBundleReq)
	s.AssertGRPCStatus(err, codes.InvalidArgument, "bundle must be non-nil")
}

func (s *PluginSuite) createBundle(trustDomainID string) {
	bundle := bundleutil.BundleProtoFromRootCA(trustDomainID, s.cert)
	cReq := &datastore.CreateBundleRequest{
		Bundle: bundle,
	}

	_, err := s.ds.CreateBundle(ctx, cReq)
	s.Require().NoError(err)
}

func (s *PluginSuite) fetchBundle(trustDomainID string) *common.Bundle {
	fReq := &datastore.FetchBundleRequest{
		TrustDomainId: trustDomainID,
	}
	resp, err := s.ds.FetchBundle(ctx, fReq)
	s.Require().NoError(err)
	return resp.Bundle
}
