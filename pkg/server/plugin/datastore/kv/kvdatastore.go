package kv

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/hcl"
	"github.com/spiffe/spire/internal/protokv"
	"github.com/spiffe/spire/internal/protokv/mysqlkv"
	"github.com/spiffe/spire/internal/protokv/sqlite3kv"
	"github.com/spiffe/spire/pkg/common/catalog"
	"github.com/spiffe/spire/pkg/server/plugin/datastore"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message/attestednode"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message/bundle"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message/jointoken"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message/nodeselector"
	"github.com/spiffe/spire/pkg/server/plugin/datastore/kv/message/registrationentry"
	spi "github.com/spiffe/spire/proto/spire/common/plugin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	PluginName = "kv"
)

var (
	pluginInfo = spi.GetPluginInfoResponse{
		Name:        "kv",
		Category:    "datastore",
		Type:        "kv",
		Description: "Key-value, indexed protocol buffer data store",
		DateCreated: "",
		Version:     "",
		Author:      "",
		Company:     "",
	}
)

func BuiltIn() catalog.Plugin {
	return builtin(New())
}

func builtin(p *Plugin) catalog.Plugin {
	return catalog.MakePlugin(PluginName,
		datastore.PluginServer(p),
	)
}

type Config struct {
	DatabaseType       string  `hcl:"database_type" json:"database_type"`
	ConnectionString   string  `hcl:"connection_string" json:"connection_string"`
	RoConnectionString string  `hcl:"ro_connection_string" json:"ro_connection_string"`
	ConnMaxLifetime    *string `hcl:"conn_max_lifetime" json:"conn_max_lifetime"`
	MaxOpenConns       *int    `hcl:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns       *int    `hcl:"max_idle_conns" json:"max_idle_conns"`
}

type Plugin struct {
	datastore.Plugin

	log hclog.Logger
	kv  protokv.KV

	bundles             bundle.Operations
	attestedNodes       attestednode.Operations
	joinTokens          jointoken.Operations
	registrationEntries registrationentry.Operations
	nodeSelectors       nodeselector.Operations
}

func New() *Plugin {
	return &Plugin{}
}

func (c *Config) Validate() error {
	if c.DatabaseType == "" {
		return errors.New("database_type must be set")
	}

	if c.ConnectionString == "" {
		return errors.New("connection_string must be set")
	}

	switch c.DatabaseType {
	case sqlite, sqlite3:
	case mySQL:
		return c.validateMySQLConfig()
	default:
		return fmt.Errorf("unsupported database_type: %s", c.DatabaseType)
	}

	return nil
}

func (p *Plugin) CreateBundle(ctx context.Context, req *datastore.CreateBundleRequest) (*datastore.CreateBundleResponse, error) {
	return p.bundles.Create(ctx, req)
}

func (p *Plugin) FetchBundle(ctx context.Context, req *datastore.FetchBundleRequest) (*datastore.FetchBundleResponse, error) {
	return p.bundles.Fetch(ctx, req)
}

func (p *Plugin) ListBundles(ctx context.Context, req *datastore.ListBundlesRequest) (*datastore.ListBundlesResponse, error) {
	return p.bundles.List(ctx, req)
}

func (p *Plugin) UpdateBundle(ctx context.Context, req *datastore.UpdateBundleRequest) (*datastore.UpdateBundleResponse, error) {
	return p.bundles.Update(ctx, req)
}

func (p *Plugin) SetBundle(ctx context.Context, req *datastore.SetBundleRequest) (*datastore.SetBundleResponse, error) {
	return p.bundles.Set(ctx, req)
}

func (p *Plugin) AppendBundle(ctx context.Context, req *datastore.AppendBundleRequest) (*datastore.AppendBundleResponse, error) {
	return p.bundles.Append(ctx, req)
}

func (p *Plugin) DeleteBundle(ctx context.Context, req *datastore.DeleteBundleRequest) (*datastore.DeleteBundleResponse, error) {
	return p.bundles.Delete(ctx, req)
}

func (p *Plugin) PruneBundle(ctx context.Context, req *datastore.PruneBundleRequest) (*datastore.PruneBundleResponse, error) {
	return p.bundles.Prune(ctx, req)
}

func (p *Plugin) CreateAttestedNode(ctx context.Context, req *datastore.CreateAttestedNodeRequest) (*datastore.CreateAttestedNodeResponse, error) {
	return p.attestedNodes.Create(ctx, req)
}

func (p *Plugin) FetchAttestedNode(ctx context.Context, req *datastore.FetchAttestedNodeRequest) (*datastore.FetchAttestedNodeResponse, error) {
	return p.attestedNodes.Fetch(ctx, req)
}

func (p *Plugin) ListAttestedNodes(ctx context.Context, req *datastore.ListAttestedNodesRequest) (*datastore.ListAttestedNodesResponse, error) {
	return p.attestedNodes.List(ctx, req)
}

func (p *Plugin) UpdateAttestedNode(ctx context.Context, req *datastore.UpdateAttestedNodeRequest) (*datastore.UpdateAttestedNodeResponse, error) {
	return p.attestedNodes.Update(ctx, req)
}

func (p *Plugin) DeleteAttestedNode(ctx context.Context, req *datastore.DeleteAttestedNodeRequest) (*datastore.DeleteAttestedNodeResponse, error) {
	return p.attestedNodes.Delete(ctx, req)
}

func (p *Plugin) CreateJoinToken(ctx context.Context, req *datastore.CreateJoinTokenRequest) (*datastore.CreateJoinTokenResponse, error) {
	return p.joinTokens.Create(ctx, req)
}

func (p *Plugin) FetchJoinToken(ctx context.Context, req *datastore.FetchJoinTokenRequest) (*datastore.FetchJoinTokenResponse, error) {
	return p.joinTokens.Fetch(ctx, req)
}

func (p *Plugin) DeleteJoinToken(ctx context.Context, req *datastore.DeleteJoinTokenRequest) (*datastore.DeleteJoinTokenResponse, error) {
	return p.joinTokens.Delete(ctx, req)
}

func (p *Plugin) PruneJoinTokens(ctx context.Context, req *datastore.PruneJoinTokensRequest) (*datastore.PruneJoinTokensResponse, error) {
	return p.joinTokens.Prune(ctx, req)
}

func (p *Plugin) GetNodeSelectors(ctx context.Context, req *datastore.GetNodeSelectorsRequest) (*datastore.GetNodeSelectorsResponse, error) {
	return p.nodeSelectors.Get(ctx, req)
}

func (p *Plugin) SetNodeSelectors(ctx context.Context, req *datastore.SetNodeSelectorsRequest) (*datastore.SetNodeSelectorsResponse, error) {
	return p.nodeSelectors.Set(ctx, req)
}

func (p *Plugin) CreateRegistrationEntry(ctx context.Context, req *datastore.CreateRegistrationEntryRequest) (*datastore.CreateRegistrationEntryResponse, error) {
	return p.registrationEntries.Create(ctx, req)
}

func (p *Plugin) FetchRegistrationEntry(ctx context.Context, req *datastore.FetchRegistrationEntryRequest) (*datastore.FetchRegistrationEntryResponse, error) {
	return p.registrationEntries.Fetch(ctx, req)
}

func (p *Plugin) ListRegistrationEntries(ctx context.Context, req *datastore.ListRegistrationEntriesRequest) (*datastore.ListRegistrationEntriesResponse, error) {
	return p.registrationEntries.List(ctx, req)
}

func (p *Plugin) UpdateRegistrationEntry(ctx context.Context, req *datastore.UpdateRegistrationEntryRequest) (*datastore.UpdateRegistrationEntryResponse, error) {
	return p.registrationEntries.Update(ctx, req)
}

func (p *Plugin) DeleteRegistrationEntry(ctx context.Context, req *datastore.DeleteRegistrationEntryRequest) (*datastore.DeleteRegistrationEntryResponse, error) {
	return p.registrationEntries.Delete(ctx, req)
}

func (p *Plugin) PruneRegistrationEntries(ctx context.Context, req *datastore.PruneRegistrationEntriesRequest) (*datastore.PruneRegistrationEntriesResponse, error) {
	return p.registrationEntries.Prune(ctx, req)
}

func (p *Plugin) Configure(ctx context.Context, req *spi.ConfigureRequest) (*spi.ConfigureResponse, error) {
	var err error
	if p.log == nil {
		p.log = hclog.NewNullLogger()
	}

	config := new(Config)
	if err = hcl.Decode(config, req.Configuration); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "datastore-kv: unable to parse config: %v", err)
	}
	if err = config.Validate(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "datastore-kv: %v", err)
	}

	var connMaxLifetime *time.Duration
	if config.ConnMaxLifetime != nil {
		cml, err := time.ParseDuration(*config.ConnMaxLifetime)
		if err != nil {
			return nil, fmt.Errorf("failed to parse conn_max_lifetime %q: %v", *config.ConnMaxLifetime, err)
		}

		connMaxLifetime = &cml
	}

	kvCfg := protokv.Configuration{
		ConnectionString: config.ConnectionString,
		ConnMaxLifetime:  connMaxLifetime,
		MaxOpenConns:     config.MaxOpenConns,
		MaxIdleConns:     config.MaxIdleConns,
	}

	var kv protokv.KV
	switch strings.ToLower(config.DatabaseType) {
	case sqlite, sqlite3:
		kv, err = sqlite3kv.Open(kvCfg)
	case mySQL:
		kv, err = mysqlkv.Open(kvCfg)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "datastore-kv: unsupported database_type: %s", config.DatabaseType)
	}
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "datastore-kv: %v", err)
	}

	// TODO: reconfiguration
	p.kv = kv
	p.bundles = bundle.New(kv, p.log)
	p.attestedNodes = attestednode.New(kv, p.log)
	p.joinTokens = jointoken.New(kv, p.log)
	p.registrationEntries = registrationentry.New(kv, p.log)
	p.nodeSelectors = nodeselector.New(kv, p.log)

	return &spi.ConfigureResponse{}, nil
}

func (p *Plugin) SetLogger(log hclog.Logger) {
	p.log = log
}

func (p *Plugin) GetPluginInfo(context.Context, *spi.GetPluginInfoRequest) (*spi.GetPluginInfoResponse, error) {
	return &pluginInfo, nil
}

func (p *Plugin) closeDB() error {
	return p.kv.Close()
}

func (c *Config) validateMySQLConfig() error {
	_, err := mysql.ParseDSN(c.ConnectionString)
	if err != nil {
		return fmt.Errorf("invalid connection_string: %w", err)
	}

	return nil
}
