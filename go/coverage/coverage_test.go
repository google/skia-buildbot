package coverage

import (
	"testing"

	"go.skia.org/infra/go/coverage/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coverage_store "go.skia.org/infra/go/coverage/coveragestore/sqlcoveragestore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	rpcHost    = "localhost"
	rpcPort    = 8007
	dbPort     = 26257
	configFile = "test.json"
	dbName     = "coveragemapping"
)

func setupTestApp(t *testing.T) *Coverage {
	db := sqltest.NewCockroachDBForTests(t, "coverage")
	coverageConfig := &config.CoverageConfig{
		DatabaseName: db.Config().ConnConfig.Database,
		DatabaseHost: db.Config().ConnConfig.Host,
	}
	sklog.Infof("Config: %s", coverageConfig)
	coverageStore, _ := coverage_store.New(db)
	sklog.Infof("Config file: %s", configFile)
	c, err := New(coverageConfig, coverageStore)
	require.NoError(t, err)
	ch := make(chan interface{})
	go func() {
		err := c.ServeGRPC()
		assert.NoError(t, err)
		ch <- nil
	}()

	t.Cleanup(func() {
		c.Cleanup()
		<-ch
	})

	return c
}

func TestAppSetup(t *testing.T) {
	c := setupTestApp(t)
	assert.NotNil(t, c)

	_, err := grpc.Dial(string(rune(rpcPort)), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
}
