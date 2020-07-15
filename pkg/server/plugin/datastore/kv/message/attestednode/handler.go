package attestednode

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"github.com/hashicorp/go-hclog"
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/pkg/server/plugin/datastore"
	"github.com/spiffe/spire/proto/spire/common"
	"github.com/zeebo/errs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func (h *handler) Create(ctx context.Context, req *datastore.CreateAttestedNodeRequest) (*datastore.CreateAttestedNodeResponse, error) {
	value, err := proto.Marshal(req.Node)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	if err := h.store.Create(ctx, value); err != nil {
		return nil, errs.Wrap(err)
	}

	return &datastore.CreateAttestedNodeResponse{
		Node: req.Node,
	}, nil
}

func (h *handler) Delete(ctx context.Context, req *datastore.DeleteAttestedNodeRequest) (*datastore.DeleteAttestedNodeResponse, error) {
	in := &common.AttestedNode{SpiffeId: req.SpiffeId}
	out := &common.AttestedNode{}
	var err error
	if err = h.store.ReadProto(ctx, in, out); err != nil {
		if protokv.NotFound.Has(err) {
			return nil, status.Errorf(codes.NotFound, "attested node was not found with spiffe id %s", req.SpiffeId)
		}

		return nil, err
	}

	outProto, err := proto.Marshal(out)
	if err != nil {
		return nil, err
	}

	if err = h.store.Delete(ctx, outProto); err != nil {
		return nil, err
	}

	return &datastore.DeleteAttestedNodeResponse{
		Node: out,
	}, nil
}

func (h *handler) Fetch(ctx context.Context, req *datastore.FetchAttestedNodeRequest) (*datastore.FetchAttestedNodeResponse, error) {
	in := &common.AttestedNode{SpiffeId: req.SpiffeId}
	out := &common.AttestedNode{}
	if err := h.store.ReadProto(ctx, in, out); err != nil {
		if protokv.NotFound.Has(err) {
			// Forcibly suppressing this error to keep the plugin compliant with the existing behavior of the sql plugin.
			// TODO: Evaluate whether we can return a NotFound gRPC error here and in the sql plugin.
			return &datastore.FetchAttestedNodeResponse{}, nil
		}
		return nil, err
	}

	return &datastore.FetchAttestedNodeResponse{
		Node: out,
	}, nil
}

func (h *handler) List(ctx context.Context, req *datastore.ListAttestedNodesRequest) (*datastore.ListAttestedNodesResponse, error) {
	// TODO: protokv does not have support yet for subsets of indices, i.e., what
	// would be needed to implement ByExpiresFor
	if req.ByExpiresBefore != nil {
		return nil, status.Error(codes.Unimplemented, "by-expires-before support not implemented")
	}

	if req.Pagination != nil && req.Pagination.PageSize <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "cannot paginate with pagesize = %d", req.Pagination.PageSize)
	}

	var token []byte
	var limit int
	var err error
	if req.Pagination != nil {
		if len(req.Pagination.Token) > 0 {
			token, err = decodePaginationToken(req.Pagination.Token)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid pagination token: %s", req.Pagination.Token)
			}
		}

		limit = int(req.Pagination.PageSize)
	}

	values, token, err := h.store.Page(ctx, token, limit)

	if err != nil {
		return nil, errs.Wrap(err)
	}

	var nodes []*common.AttestedNode
	for _, value := range values {
		attNode := &common.AttestedNode{}
		if err := proto.Unmarshal(value, attNode); err != nil {
			return nil, errs.Wrap(err)
		}
		nodes = append(nodes, attNode)
	}

	var pagination *datastore.Pagination
	if req.Pagination != nil {
		pagination = &datastore.Pagination{
			Token: encodePaginationToken(token),
		}
	}

	return &datastore.ListAttestedNodesResponse{
		Nodes:      nodes,
		Pagination: pagination,
	}, nil
}

func (h *handler) Update(ctx context.Context, req *datastore.UpdateAttestedNodeRequest) (*datastore.UpdateAttestedNodeResponse, error) {
	in := &common.AttestedNode{SpiffeId: req.SpiffeId}
	out := &common.AttestedNode{}
	if err := h.store.ReadProto(ctx, in, out); err != nil {
		return nil, status.Errorf(codes.NotFound, "attested node not found for spiffe id %s", req.SpiffeId)
	}

	out.CertNotAfter = req.CertNotAfter
	out.CertSerialNumber = req.CertSerialNumber
	out.NewCertNotAfter = req.NewCertNotAfter
	out.NewCertSerialNumber = req.NewCertSerialNumber
	out.SpiffeId = req.SpiffeId

	outBytes, err := proto.Marshal(out)
	if err != nil {
		return nil, err
	}

	if err := h.store.Update(ctx, outBytes); err != nil {
		return nil, err
	}

	return &datastore.UpdateAttestedNodeResponse{
		Node: out,
	}, nil
}
