package builders

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
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
	dir, err := os.MkdirTemp("", "perf-builders")
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
	source, err := NewSourceFromConfig(ctx, instanceConfig)
	require.NoError(t, err)
	assert.IsType(t, &dirsource.DirSource{}, source)
}

func TestNewSourceFromConfig_MissingSourceForDirSourceIsError(t *testing.T) {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "perf-builders")
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
	_, err = NewSourceFromConfig(ctx, instanceConfig)
	assert.Error(t, err)
}

func newDBConfigForTest(t *testing.T) (context.Context, *config.InstanceConfig) {
	ctx := context.Background()

	// This creates a new database with a different random suffix on each call
	// (e.g. "builders_<random number>").
	conn := sqltest.NewSpannerDBForTests(t, "builders")

	// If we don't clear the singleton pool, newDBFromConfig will reuse the DB connection
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
			DataStoreType:    config.SpannerDataStoreType,
			ConnectionString: conn.Config().ConnString(),
		},
	}
	return ctx, instanceConfig
}

func TestNewTraceStoreFromConfig_Success(t *testing.T) {
	ctx, instanceConfig := newDBConfigForTest(t)

	store, err := NewTraceStoreFromConfig(ctx, instanceConfig)
	require.NoError(t, err)
	err = store.WriteTraces(ctx, types.CommitNumber(0), []paramtools.Params{{"config": "8888"}}, []float32{1.2}, nil, "gs://foobar", time.Now())
	assert.NoError(t, err)
}

func TestNewRegressionStoreFromConfig_Success(t *testing.T) {
	ctx, instanceConfig := newDBConfigForTest(t)

	store, err := NewRegressionStoreFromConfig(ctx, instanceConfig, nil)
	require.NoError(t, err)

	regressiontest.SetLowAndTriage(t, store)
}

func TestNewShortcutStoreFromConfig_Success(t *testing.T) {
	ctx, instanceConfig := newDBConfigForTest(t)

	store, err := NewShortcutStoreFromConfig(ctx, instanceConfig)
	require.NoError(t, err)

	shortcuttest.InsertGet(t, store)
}

func TestNewPerfGitFromConfig_Success(t *testing.T) {
	ctx, _, _, hashes, _, instanceConfig := gittest.NewForTest(t)

	instanceConfig.DataStoreConfig.DataStoreType = config.SpannerDataStoreType

	// Get db for its connection string.
	_, dbInstanceConfig := newDBConfigForTest(t)

	instanceConfig.DataStoreConfig.ConnectionString = dbInstanceConfig.DataStoreConfig.ConnectionString

	g, err := NewPerfGitFromConfig(ctx, false, instanceConfig)
	require.NoError(t, err)

	gitHash, err := g.GitHashFromCommitNumber(ctx, types.CommitNumber(2))
	require.NoError(t, err)
	assert.Equal(t, hashes[2], gitHash)
}
