package builders

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/alerts/alertstest"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/file/dirsource"
	"go.skia.org/infra/perf/go/git/gittest"
	"go.skia.org/infra/perf/go/regression/regressiontest"
	"go.skia.org/infra/perf/go/shortcut/shortcuttest"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/types"
)

func TestNewSourceFromConfig_DirSource_Success(t *testing.T) {
	ctx := context.Background()
	dir, err := ioutil.TempDir("", "perf-builders")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(dir))
	}()
	instanceConfig := &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			SourceConfig: config.SourceConfig{
				SourceType: config.DirSourceType,
				Sources:    []string{dir},
			},
		},
	}
	local := true
	source, err := NewSourceFromConfig(ctx, instanceConfig, local)
	require.NoError(t, err)
	assert.IsType(t, &dirsource.DirSource{}, source)
}

func TestNewSourceFromConfig_MissingSourceForDirSourceIsError(t *testing.T) {
	ctx := context.Background()
	dir, err := ioutil.TempDir("", "perf-builders")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(dir))
	}()
	instanceConfig := &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			SourceConfig: config.SourceConfig{
				SourceType: config.DirSourceType,
				Sources:    []string{},
			},
		},
	}
	local := true
	_, err = NewSourceFromConfig(ctx, instanceConfig, local)
	assert.Error(t, err)
}

type cleanupFunc func()

func newCockroachDBConfigForTest(t *testing.T) (context.Context, *config.InstanceConfig, sqltest.Cleanup) {
	unittest.RequiresCockroachDB(t)

	ctx := context.Background()

	// This creates a new database with a different random suffix on each call
	// (e.g. "builders_<random number>").
	conn, cleanup := sqltest.NewCockroachDBForTests(t, "builders")

	// If we don't clear the singleton pool, newCockroachDBFromConfig will reuse the DB connection
	// established by the last executed test case, which points to a different database than the one
	// we just created (see previous step).
	singletonPoolMutex.Lock()
	defer singletonPoolMutex.Unlock()
	if singletonPool != nil {
		singletonPool.Close()
		singletonPool = nil
	}

	instanceConfig := &config.InstanceConfig{
		DataStoreConfig: config.DataStoreConfig{
			DataStoreType:    config.CockroachDBDataStoreType,
			ConnectionString: conn.Config().ConnString(),
		},
	}
	return ctx, instanceConfig, cleanup
}

func TestNewTraceStoreFromConfig_CockroachDB_Success(t *testing.T) {
	ctx, instanceConfig, cleanup := newCockroachDBConfigForTest(t)
	defer cleanup()

	store, err := NewTraceStoreFromConfig(ctx, true, instanceConfig)
	require.NoError(t, err)
	err = store.WriteTraces(ctx, types.CommitNumber(0), []paramtools.Params{{"config": "8888"}}, []float32{1.2}, nil, "gs://foobar", time.Now())
	assert.NoError(t, err)
}

func TestNewTraceStoreFromConfig_InvalidDatastoreTypeIsError(t *testing.T) {
	ctx, instanceConfig, cleanup := newCockroachDBConfigForTest(t)
	defer cleanup()

	const invalidDataStoreType = config.DataStoreType("not-a-valid-datastore-type")
	instanceConfig.DataStoreConfig.DataStoreType = invalidDataStoreType

	_, err := NewTraceStoreFromConfig(ctx, true, instanceConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), invalidDataStoreType)
}

func TestNewAlertStoreFromConfig_CockroachDB_Success(t *testing.T) {
	ctx, instanceConfig, cleanup := newCockroachDBConfigForTest(t)
	defer cleanup()

	store, err := NewAlertStoreFromConfig(ctx, false, instanceConfig)
	require.NoError(t, err)

	alertstest.Store_SaveListDelete(t, store)
}

func TestNewAlertStoreFromConfig_InvalidDatastoreTypeIsError(t *testing.T) {
	ctx, instanceConfig, cleanup := newCockroachDBConfigForTest(t)
	defer cleanup()

	const invalidDataStoreType = config.DataStoreType("not-a-valid-datastore-type")
	instanceConfig.DataStoreConfig.DataStoreType = invalidDataStoreType

	_, err := NewAlertStoreFromConfig(ctx, true, instanceConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), invalidDataStoreType)
}

func TestNewRegressionStoreFromConfig_CochroachDB_Success(t *testing.T) {
	ctx, instanceConfig, cleanup := newCockroachDBConfigForTest(t)
	defer cleanup()

	store, err := NewRegressionStoreFromConfig(ctx, false, instanceConfig)
	require.NoError(t, err)

	regressiontest.SetLowAndTriage(t, store)
}

func TestNewRegressionStoreFromConfig_InvalidDatastoreTypeIsError(t *testing.T) {
	ctx, instanceConfig, cleanup := newCockroachDBConfigForTest(t)
	defer cleanup()

	const invalidDataStoreType = config.DataStoreType("not-a-valid-datastore-type")
	instanceConfig.DataStoreConfig.DataStoreType = invalidDataStoreType

	_, err := NewRegressionStoreFromConfig(ctx, false, instanceConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), invalidDataStoreType)
}

func TestNewShortcutStoreFromConfig_CockroachDB_Success(t *testing.T) {
	ctx, instanceConfig, cleanup := newCockroachDBConfigForTest(t)
	defer cleanup()

	store, err := NewShortcutStoreFromConfig(ctx, false, instanceConfig)
	require.NoError(t, err)

	shortcuttest.InsertGet(t, store)
}

func TestNewShortcutStoreFromConfig_CockroachDB_InvalidDatastoreTypeIsError(t *testing.T) {
	ctx, instanceConfig, cleanup := newCockroachDBConfigForTest(t)
	defer cleanup()

	const invalidDataStoreType = config.DataStoreType("not-a-valid-datastore-type")
	instanceConfig.DataStoreConfig.DataStoreType = invalidDataStoreType

	_, err := NewShortcutStoreFromConfig(ctx, false, instanceConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), invalidDataStoreType)
}

func TestNewPerfGitFromConfig_CockroachDB_Success(t *testing.T) {
	ctx, _, _, hashes, instanceConfig, cleanup := gittest.NewForTest(t)
	defer cleanup()

	instanceConfig.DataStoreConfig.DataStoreType = config.CockroachDBDataStoreType

	// Get cockroachdb for its connection string.
	_, cockroackdbInstanceConfig, cockroackDBCleanup := newCockroachDBConfigForTest(t)
	defer cockroackDBCleanup()

	instanceConfig.DataStoreConfig.ConnectionString = cockroackdbInstanceConfig.DataStoreConfig.ConnectionString

	g, err := NewPerfGitFromConfig(ctx, false, instanceConfig)
	require.NoError(t, err)

	gitHash, err := g.GitHashFromCommitNumber(ctx, types.CommitNumber(2))
	require.NoError(t, err)
	assert.Equal(t, hashes[2], gitHash)
}
