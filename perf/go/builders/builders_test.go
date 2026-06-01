package builders

import (
	"context"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
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

	store, err := NewShortcutStoreFromConfig(ctx, false, instanceConfig)
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

func TestNewTraceStoreFromConfig_ShowOnlyPublicTraces_Success(t *testing.T) {
	ctx, instanceConfig := newDBConfigForTest(t)

	// Configure the instance as ShowOnlyPublicTraces inside VisibilityConfig:
	instanceConfig.VisibilityConfig = &config.VisibilityConfig{
		ShowOnlyPublicTraces: true,
	}

	// Get a raw pool connection to populate our test database first!
	db, err := NewDBPoolFromConfig(ctx, instanceConfig, false)
	require.NoError(t, err)

	// Write public and private trace parameters into DB:
	publicTraceName := ",arch=x86,config=8888,"
	privateTraceName := ",arch=x86,config=565,"

	// Populate TraceParams table manually:
	insertTraceParams := `
	INSERT INTO TraceParams (trace_id, params, is_public) VALUES
		($1, CAST($2 AS JSONB), TRUE),
		($3, CAST($4 AS JSONB), FALSE)
	`
	publicBytes := types.TraceIDForSQLInBytesFromTraceName(publicTraceName)
	privateBytes := types.TraceIDForSQLInBytesFromTraceName(privateTraceName)

	_, err = db.Exec(ctx, insertTraceParams, publicBytes[:], `{"arch": "x86", "config": "8888"}`, privateBytes[:], `{"arch": "x86", "config": "565"}`)
	require.NoError(t, err)

	// Insert paramsets to cover index fields:
	insertIntoParamSets := `
	INSERT INTO ParamSets (tile_number, param_key, param_value) VALUES
		(0, 'arch', 'x86'),
		(0, 'config', '8888'),
		(0, 'config', '565')
	`
	_, err = db.Exec(ctx, insertIntoParamSets)
	require.NoError(t, err)

	// Now build the real SQLTraceStore through our config-builder API!
	store, err := NewTraceStoreFromConfig(ctx, instanceConfig)
	require.NoError(t, err)

	// 1. Verify that the initial GetParamSet options contain ONLY the public trace settings!
	ps, err := store.GetParamSet(ctx, 0)
	require.NoError(t, err)
	expectedParamSet := paramtools.ReadOnlyParamSet{
		"arch":   []string{"x86"},
		"config": []string{"8888"},
	}
	assert.Equal(t, expectedParamSet, ps)

	// 2. Verify that general QueryTracesIDOnly queries strictly return only public trace items!
	u, err := url.ParseQuery("arch=x86")
	require.NoError(t, err)
	q, err := query.New(u)
	require.NoError(t, err)

	outParams, err := store.QueryTracesIDOnly(ctx, 0, q)
	require.NoError(t, err)
	var results []paramtools.Params
	for p := range outParams {
		results = append(results, p)
	}
	assert.Equal(t, 1, len(results))
	assert.Equal(t, "8888", results[0]["config"])
}
