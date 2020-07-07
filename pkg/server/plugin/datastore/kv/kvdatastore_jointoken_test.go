package kv

import (
	"testing"
	"time"

	"github.com/spiffe/spire/pkg/server/plugin/datastore"
)

func (s *PluginSuite) TestCreateAndFetchJoinToken() {
	now := time.Now().Unix()
	joinToken := &datastore.JoinToken{
		Token:  "foobar",
		Expiry: now,
	}

	cReq := &datastore.CreateJoinTokenRequest{
		JoinToken: joinToken,
	}

	_, err := s.ds.CreateJoinToken(ctx, cReq)
	s.Require().NoError(err)

	fReq := &datastore.FetchJoinTokenRequest{
		Token: joinToken.Token,
	}
	res, err := s.ds.FetchJoinToken(ctx, fReq)
	s.Require().NoError(err)
	s.Equal("foobar", res.JoinToken.Token)
	s.Equal(now, res.JoinToken.Expiry)
}

func (s *PluginSuite) TestCreateInvalidJoinToken() {
	tests := []struct {
		name        string
		token       *datastore.JoinToken
		expectedErr string
	}{
		{
			name:        "nil join token",
			expectedErr: "token is required",
		},
		{
			name: "empty join token",
			token: &datastore.JoinToken{
				Expiry: 1000,
			},
			expectedErr: "token is required",
		},
		{
			name: "expiry of zero",
			token: &datastore.JoinToken{
				Token:  "foo",
				Expiry: 0,
			},
			expectedErr: "expiry is required",
		},
		{
			name: "negative expiry",
			token: &datastore.JoinToken{
				Token:  "foo",
				Expiry: -1,
			},
			expectedErr: "expiry is required",
		},
	}

	for _, test := range tests {
		expectedErr := test.expectedErr
		token := test.token
		s.T().Run(test.name, func(t *testing.T) {
			ds := s.newPlugin()
			cReq := &datastore.CreateJoinTokenRequest{
				JoinToken: token,
			}

			_, err := ds.CreateJoinToken(ctx, cReq)
			s.Assert().Error(err, expectedErr)
		})
	}
}

func (s *PluginSuite) TestDeleteJoinToken() {
	now := time.Now().Unix()
	joinToken1 := &datastore.JoinToken{
		Token:  "foobar",
		Expiry: now,
	}

	cReq1 := &datastore.CreateJoinTokenRequest{
		JoinToken: joinToken1,
	}

	_, err := s.ds.CreateJoinToken(ctx, cReq1)
	s.Require().NoError(err)

	joinToken2 := &datastore.JoinToken{
		Token:  "batbaz",
		Expiry: now,
	}

	cReq2 := &datastore.CreateJoinTokenRequest{
		JoinToken: joinToken2,
	}

	_, err = s.ds.CreateJoinToken(ctx, cReq2)
	s.Require().NoError(err)

	dReq := &datastore.DeleteJoinTokenRequest{
		Token: joinToken1.Token,
	}

	_, err = s.ds.DeleteJoinToken(ctx, dReq)
	s.Require().NoError(err)

	fReq1 := &datastore.FetchJoinTokenRequest{
		Token: joinToken1.Token,
	}
	// Should not be able to fetch after delete
	resp, err := s.ds.FetchJoinToken(ctx, fReq1)
	s.Require().NoError(err)
	s.Nil(resp.JoinToken)

	fReq2 := &datastore.FetchJoinTokenRequest{
		Token: joinToken2.Token,
	}

	// Second token should still be present
	resp, err = s.ds.FetchJoinToken(ctx, fReq2)
	s.Require().NoError(err)
	s.AssertProtoEqual(joinToken2, resp.JoinToken)
}

func (s *PluginSuite) TestPruneJoinTokens() {
	now := time.Now().Unix()
	joinToken := &datastore.JoinToken{
		Token:  "foobar",
		Expiry: now,
	}

	cReq := &datastore.CreateJoinTokenRequest{
		JoinToken: joinToken,
	}

	_, err := s.ds.CreateJoinToken(ctx, cReq)
	s.Require().NoError(err)

	pReq1 := &datastore.PruneJoinTokensRequest{
		ExpiresBefore: now - 10,
	}

	// Ensure we don't prune valid tokens, wind clock back 10s
	_, err = s.ds.PruneJoinTokens(ctx, pReq1)
	s.Require().NoError(err)

	fReq := &datastore.FetchJoinTokenRequest{
		Token: joinToken.Token,
	}

	resp, err := s.ds.FetchJoinToken(ctx, fReq)
	s.Require().NoError(err)
	s.Equal("foobar", resp.JoinToken.Token)

	pReq2 := &datastore.PruneJoinTokensRequest{
		ExpiresBefore: now,
	}
	// Ensure we don't prune on the exact ExpiresBefore
	_, err = s.ds.PruneJoinTokens(ctx, pReq2)
	s.Require().NoError(err)

	resp, err = s.ds.FetchJoinToken(ctx, fReq)
	s.Require().NoError(err)
	s.Equal("foobar", resp.JoinToken.Token)

	// Ensure we prune old tokens
	joinToken.Expiry = now + 10
	pReq3 := &datastore.PruneJoinTokensRequest{
		ExpiresBefore: joinToken.Expiry,
	}
	_, err = s.ds.PruneJoinTokens(ctx, pReq3)
	s.Require().NoError(err)

	resp, err = s.ds.FetchJoinToken(ctx, fReq)
	s.Require().NoError(err)
	s.Nil(resp.JoinToken)
}
