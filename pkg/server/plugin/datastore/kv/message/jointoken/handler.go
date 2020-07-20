package jointoken

import (
	"context"
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/hashicorp/go-hclog"
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore"
)

type handler struct {
	log   hclog.Logger
	store *protokv.Store
}

func New(kv protokv.KV, log hclog.Logger) Operations {
	return &handler{
		log:   log,
		store: protokv.NewStore(kv, &Message),
	}
}

func (h *handler) Create(ctx context.Context, req *datastore.CreateJoinTokenRequest) (*datastore.CreateJoinTokenResponse, error) {
	// TODO: Prefer to send InvalidArgument code here, but keeping it this way for now to match the sql plugin behavior
	if req.JoinToken == nil || req.JoinToken.Token == "" {
		return nil, errors.New("token is required")
	}

	// TODO: Prefer to send InvalidArgument code here, but keeping it this way for now to match the sql plugin behavior
	if req.JoinToken.Expiry <= 0 {
		return nil, errors.New("expiry is required")
	}

	value, err := proto.Marshal(req.JoinToken)
	if err != nil {
		return nil, err
	}

	if err := h.store.Create(ctx, value); err != nil {
		return nil, err
	}

	return &datastore.CreateJoinTokenResponse{
		JoinToken: req.JoinToken,
	}, nil
}

func (h *handler) Delete(ctx context.Context, req *datastore.DeleteJoinTokenRequest) (*datastore.DeleteJoinTokenResponse, error) {
	in := &datastore.JoinToken{
		Token: req.Token,
	}

	out := &datastore.JoinToken{}
	if err := h.store.ReadProto(ctx, in, out, false); err != nil {
		return nil, err
	}

	outBytes, err := proto.Marshal(out)
	if err != nil {
		return nil, err
	}

	if err := h.store.Delete(ctx, outBytes); err != nil {
		return nil, err
	}

	return &datastore.DeleteJoinTokenResponse{
		JoinToken: out,
	}, nil
}

func (h *handler) Fetch(ctx context.Context, req *datastore.FetchJoinTokenRequest) (*datastore.FetchJoinTokenResponse, error) {
	in := &datastore.JoinToken{
		Token: req.Token,
	}

	out := &datastore.JoinToken{}
	if err := h.store.ReadProto(ctx, in, out, true); err != nil {
		// TODO: Prefer to send NotFound here, but keeping it this way for now to match the sql plugin behavior
		if protokv.NotFound.Has(err) {
			return &datastore.FetchJoinTokenResponse{}, nil
		}
		return nil, err
	}

	return &datastore.FetchJoinTokenResponse{JoinToken: out}, nil
}

func (h *handler) Prune(ctx context.Context, req *datastore.PruneJoinTokensRequest) (*datastore.PruneJoinTokensResponse, error) {
	var token []byte
	var limit int
	values, _, err := h.store.Page(ctx, token, limit, false)
	if err != nil {
		return nil, err
	}

	joinTokens := make([]*datastore.JoinToken, len(values))
	for i, joinTokenBytes := range values {
		joinToken := &datastore.JoinToken{}
		if err := proto.Unmarshal(joinTokenBytes, joinToken); err != nil {
			return nil, err
		}
		joinTokens[i] = joinToken
	}

	joinTokensToPrune := h.joinTokensToPrune(joinTokens, req.ExpiresBefore)
	var errs []error
	for _, joinToken := range joinTokensToPrune {
		// TODO: Consider creating a "batch" Delete() API in protokv to minimize query load
		joinTokenBytes, err := proto.Marshal(joinToken)
		if err != nil {
			return nil, err
		}

		if err := h.store.Delete(ctx, joinTokenBytes); err != nil {
			errs = append(errs, err)
		}
	}

	numErrs := len(errs)
	if numErrs > 0 {
		return nil, fmt.Errorf("failed to prune %v join tokens", numErrs)
	}

	return &datastore.PruneJoinTokensResponse{}, nil
}

func (h *handler) joinTokensToPrune(joinTokens []*datastore.JoinToken, expiresBeforeSeconds int64) []*datastore.JoinToken {
	var joinTokensToPrune []*datastore.JoinToken
	for _, joinToken := range joinTokens {
		if joinToken.Expiry < expiresBeforeSeconds {
			joinTokensToPrune = append(joinTokensToPrune, joinToken)
		}
	}

	return joinTokensToPrune
}
