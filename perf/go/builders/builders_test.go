package builders

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/alerts/alertstest"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/file/dirsource"
	"go.skia.org/infra/perf/go/git/gittest"
	"go.skia.org/infra/perf/go/regression/regressiontest"
	"go.skia.org/infra/perf/go/shortcut/shortcuttest"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/types"
)

func TestNewSourceFromConfig_DirSource_Success(t *testing.T) {
	unittest.SmallTest(t)
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
	unittest.SmallTest(t)
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
	ctx := context.Background()

	const databaseName = "builders"

	connectionString := fmt.Sprintf("postgresql://root@%s/%s?sslmode=disable", perfsql.GetCockroachDBEmulatorHost(), databaseName)

	_, cleanup := sqltest.NewCockroachDBForTests(t, databaseName, sqltest.DoNotApplyMigrations)

	instanceConfig := &config.InstanceConfig{
		DataStoreConfig: config.DataStoreConfig{
			DataStoreType:    config.CockroachDBDataStoreType,
			ConnectionString: connectionString,
		},
	}
	return ctx, instanceConfig, cleanup
}

func newGCPDatastoreConfigForTest(t *testing.T, kind ds.Kind) (context.Context, *config.InstanceConfig, util.CleanupFunc) {
	ctx := context.Background()
	cleanup := testutil.InitDatastore(t, kind)
	instanceConfig := &config.InstanceConfig{
		DataStoreConfig: config.DataStoreConfig{
			DataStoreType: config.GCPDataStoreType,
		},
	}
	return ctx, instanceConfig, cleanup
}

func TestNewTraceStoreFromConfig_CockroachDB_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, instanceConfig, cleanup := newCockroachDBConfigForTest(t)
	defer cleanup()

	store, err := NewTraceStoreFromConfig(ctx, true, instanceConfig)
	require.NoError(t, err)
	err = store.WriteTraces(types.CommitNumber(0), []paramtools.Params{{"config": "8888"}}, []float32{1.2}, nil, "gs://foobar", time.Now())
	assert.NoError(t, err)
}

func TestNewTraceStoreFromConfig_InvalidDatastoreTypeIsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx, instanceConfig, cleanup := newCockroachDBConfigForTest(t)
	defer cleanup()

	const invalidDataStoreType = config.DataStoreType("not-a-valid-datastore-type")
	instanceConfig.DataStoreConfig.DataStoreType = invalidDataStoreType

	_, err := NewTraceStoreFromConfig(ctx, true, instanceConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), invalidDataStoreType)
}

func TestNewAlertStoreFromConfig_GCPDatastore_Success(t *testing.T) {
	unittest.ManualTest(t)
	ctx, instanceConfig, cleanup := newGCPDatastoreConfigForTest(t, ds.ALERT)
	defer cleanup()

	store, err := NewAlertStoreFromConfig(ctx, false, instanceConfig)
	require.NoError(t, err)

	alertstest.Store_SaveListDelete(t, store)
}

func TestNewAlertStoreFromConfig_CockroachDB_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, instanceConfig, cleanup := newCockroachDBConfigForTest(t)
	defer cleanup()

	store, err := NewAlertStoreFromConfig(ctx, false, instanceConfig)
	require.NoError(t, err)

	alertstest.Store_SaveListDelete(t, store)
}

func TestNewAlertStoreFromConfig_InvalidDatastoreTypeIsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx, instanceConfig, cleanup := newCockroachDBConfigForTest(t)
	defer cleanup()

	const invalidDataStoreType = config.DataStoreType("not-a-valid-datastore-type")
	instanceConfig.DataStoreConfig.DataStoreType = invalidDataStoreType

	_, err := NewAlertStoreFromConfig(ctx, true, instanceConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), invalidDataStoreType)
}

func TestNewRegressionStoreFromConfig_CochroachDB_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, instanceConfig, cleanup := newCockroachDBConfigForTest(t)
	defer cleanup()

	store, err := NewRegressionStoreFromConfig(ctx, false, nil, instanceConfig)
	require.NoError(t, err)

	regressiontest.SetLowAndTriage(t, store)
}

func TestNewRegressionStoreFromConfig_InvalidDatastoreTypeIsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx, instanceConfig, cleanup := newCockroachDBConfigForTest(t)
	defer cleanup()

	const invalidDataStoreType = config.DataStoreType("not-a-valid-datastore-type")
	instanceConfig.DataStoreConfig.DataStoreType = invalidDataStoreType

	_, err := NewRegressionStoreFromConfig(ctx, false, nil, instanceConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), invalidDataStoreType)
}

func TestNewShortcutStoreFromConfig_GCPDatastore_Success(t *testing.T) {
	unittest.ManualTest(t)
	ctx, instanceConfig, cleanup := newGCPDatastoreConfigForTest(t, ds.SHORTCUT)
	defer cleanup()

	store, err := NewShortcutStoreFromConfig(ctx, false, instanceConfig)
	require.NoError(t, err)

	shortcuttest.InsertGet(t, store)
}

func TestNewShortcutStoreFromConfig_CockroachDB_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, instanceConfig, cleanup := newCockroachDBConfigForTest(t)
	defer cleanup()

	store, err := NewShortcutStoreFromConfig(ctx, false, instanceConfig)
	require.NoError(t, err)

	shortcuttest.InsertGet(t, store)
}

func TestNewShortcutStoreFromConfig_CockroachDB_InvalidDatastoreTypeIsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx, instanceConfig, cleanup := newCockroachDBConfigForTest(t)
	defer cleanup()

	const invalidDataStoreType = config.DataStoreType("not-a-valid-datastore-type")
	instanceConfig.DataStoreConfig.DataStoreType = invalidDataStoreType

	_, err := NewShortcutStoreFromConfig(ctx, false, instanceConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), invalidDataStoreType)
}

func TestNewPerfGitFromConfig_ErrIfConnectionStringNotSet(t *testing.T) {
	unittest.LargeTest(t)
	ctx, _, _, _, _, instanceConfig, cleanup := gittest.NewForTest(t, perfsql.CockroachDBDialect)
	defer cleanup()

	instanceConfig.DataStoreConfig.DataStoreType = config.GCPDataStoreType

	_, err := NewPerfGitFromConfig(ctx, false, instanceConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection_string")
}

func TestNewPerfGitFromConfig_GCP_ErrorsOnNonPostgresConnectionString(t *testing.T) {
	unittest.LargeTest(t)
	ctx, _, _, _, _, instanceConfig, cleanup := gittest.NewForTest(t, perfsql.CockroachDBDialect)
	defer cleanup()

	instanceConfig.DataStoreConfig.DataStoreType = config.GCPDataStoreType

	// Create temp file to use for connection string.
	tmpfile, err := ioutil.TempFile("", "newperfgit")
	require.NoError(t, err)
	require.NoError(t, tmpfile.Close())
	defer func() {
		assert.NoError(t, os.Remove(tmpfile.Name()))
	}()
	instanceConfig.DataStoreConfig.ConnectionString = tmpfile.Name()

	_, err = NewPerfGitFromConfig(ctx, false, instanceConfig)
	require.Error(t, err)
}

func TestNewPerfGitFromConfig_GCP_SuccessIfConnectionStringIsCockroachDB(t *testing.T) {
	unittest.LargeTest(t)
	ctx, _, _, hashes, _, instanceConfig, cleanup := gittest.NewForTest(t, perfsql.CockroachDBDialect)
	defer cleanup()

	instanceConfig.DataStoreConfig.DataStoreType = config.GCPDataStoreType
	instanceConfig.DataStoreConfig.ConnectionString = fmt.Sprintf("postgresql://root@%s/%s?sslmode=disable", perfsql.GetCockroachDBEmulatorHost(), gittest.CockroachDatabaseName)

	g, err := NewPerfGitFromConfig(ctx, false, instanceConfig)
	require.NoError(t, err)

	gitHash, err := g.GitHashFromCommitNumber(ctx, types.CommitNumber(2))
	require.NoError(t, err)
	assert.Equal(t, hashes[2], gitHash)
}

func TestNewPerfGitFromConfig_CockroachDB_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, _, _, hashes, _, instanceConfig, cleanup := gittest.NewForTest(t, perfsql.CockroachDBDialect)
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
