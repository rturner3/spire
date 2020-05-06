package kv

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/spiffe/spire/pkg/server/plugin/datastore"
	spi "github.com/spiffe/spire/proto/spire/common/plugin"
	"github.com/spiffe/spire/test/clock"
	"github.com/spiffe/spire/test/spiretest"
	testutil "github.com/spiffe/spire/test/util"
	"google.golang.org/grpc/codes"
)

const (
	ttl                   = time.Hour
	expiredNotAfterString = "2018-01-10T01:34:00+00:00"
	validNotAfterString   = "2018-01-10T01:36:00+00:00"
	middleTimeString      = "2018-01-10T01:35:00+00:00"
)

var (
	ctx = context.Background()
)

type PluginSuite struct {
	spiretest.Suite

	cert   *x509.Certificate
	caCert *x509.Certificate

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

func (s *PluginSuite) SetupSuite() {
	clk := clock.NewMock(s.T())

	expiredNotAfterTime, err := time.Parse(time.RFC3339, expiredNotAfterString)
	s.Require().NoError(err)
	validNotAfterTime, err := time.Parse(time.RFC3339, validNotAfterString)
	s.Require().NoError(err)

	caTemplate, err := testutil.NewCATemplate(clk, "foo")
	s.Require().NoError(err)

	caTemplate.NotAfter = expiredNotAfterTime
	caTemplate.NotBefore = expiredNotAfterTime.Add(-ttl)

	cacert, cakey, err := testutil.SelfSign(caTemplate)
	s.Require().NoError(err)

	svidTemplate, err := testutil.NewSVIDTemplate(clk, "spiffe://foo/id1")
	s.Require().NoError(err)

	svidTemplate.NotAfter = validNotAfterTime
	svidTemplate.NotBefore = validNotAfterTime.Add(-ttl)

	cert, _, err := testutil.Sign(svidTemplate, cacert, cakey)
	s.Require().NoError(err)

	s.caCert = cacert
	s.cert = cert
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

	_, err := ds.Configure(ctx, cfgReq)
	s.Require().NoError(err)

	return ds
}

func (s *PluginSuite) TestInvalidPluginConfiguration() {
	var err error
	tests := []struct {
		name     string
		cfg      string
		errorMsg string
	}{
		{
			name: "malformed config",
			cfg: `
yaml: isNotSupported
`,
			errorMsg: "unable to parse config",
		},
		{
			name: "missing database_type",
			cfg: `
connection_string = "bad"
`,
			errorMsg: "database_type must be set",
		},
		{
			name: "missing connection_string",
			cfg: `
database_type = "sqlite3"
`,
			errorMsg: "connection_string must be set",
		},
		{
			name: "unrecognized database_type",
			cfg: `
		database_type = "wrong"
		connection_string = "bad"
`,
			errorMsg: "unsupported database_type: wrong",
		},
		{
			name: "invalid MySQL connection_string",
			cfg: `
		database_type = "mysql"
		connection_string = "bad"
`,
			errorMsg: "invalid connection_string",
		},
	}

	for _, test := range tests {
		s.T().Run(test.name, func(t *testing.T) {
			_, err = s.ds.Configure(context.Background(), &spi.ConfigureRequest{
				Configuration: test.cfg,
			})
			s.RequireGRPCStatusContains(err, codes.InvalidArgument, kvErrorString(test.errorMsg))
		})
	}
}

func (s *PluginSuite) TestGetPluginInfo() {
	resp, err := s.ds.GetPluginInfo(ctx, &spi.GetPluginInfoRequest{})
	s.Require().NoError(err)
	s.Require().NotNil(resp)
	s.Require().NotEqual("", resp.Category)
	s.Require().NotEqual("", resp.Type)
	s.Require().NotEqual("", resp.Name)
	s.Require().NotEqual("", resp.Description)
}

func (s *PluginSuite) getTestDataFromJSONFile(filePath string, jsonValue interface{}) {
	jsonBytes, err := ioutil.ReadFile(filePath)
	s.Require().NoError(err)

	err = json.Unmarshal(jsonBytes, &jsonValue)
	s.Require().NoError(err)
}

func kvErrorString(errorStr string) string {
	return fmt.Sprintf("datastore-kv: %s", errorStr)
}
