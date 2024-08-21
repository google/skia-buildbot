package coverage

import (
	"testing"

	"go.skia.org/infra/go/coverage/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coverage_store "go.skia.org/infra/go/coverage/coveragestore/sqlcoveragestore"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

const (
	configFile = "test.json"
)

func setupTestApp(t *testing.T) *Coverage {
	db := sqltest.NewCockroachDBForTests(t, "coveragemapping")
	coverageStore, _ := coverage_store.New(db)

	var coverageConfig config.CoverageConfig
	config, err := coverageConfig.LoadCoverageConfig(configFile)

	c, err := New(config, coverageStore)
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
	//TODO(seawardt: Hook up Test to DB emulator)
}
