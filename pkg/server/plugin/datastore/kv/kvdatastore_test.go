package kv

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/spiffe/spire/pkg/server/plugin/datastore"
	spi "github.com/spiffe/spire/proto/spire/common/plugin"
	"github.com/spiffe/spire/test/spiretest"
)

type PluginSuite struct {
	spiretest.Suite

	dir    string
	ds     datastore.Plugin
	nextID int
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
	_, err := s.ds.Configure(context.Background(), &spi.ConfigureRequest{
		Configuration: `
		database_type = "wrong"
		connection_string = "bad"
		`,
	})
	s.RequireErrorContains(err, "unsupported database_type: wrong")
}
