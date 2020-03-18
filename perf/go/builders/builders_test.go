package builders

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/file/dirsource"
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

func TestNewTraceStoreFromConfig_SQLite3_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	dir, err := ioutil.TempDir("", "perf-builders")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(dir))
	}()

	instanceConfig := &config.InstanceConfig{
		DataStoreConfig: config.DataStoreConfig{
			DataStoreType:    config.SQLite3DataStoreType,
			ConnectionString: filepath.Join(dir, "test.db"),
		},
	}

	store, err := NewTraceStoreFromConfig(ctx, true, instanceConfig)
	require.NoError(t, err)
	err = store.WriteTraces(types.CommitNumber(0), []paramtools.Params{{"config": "8888"}}, []float32{1.2}, nil, "gs://foobar", time.Now())
	assert.NoError(t, err)
}

func TestNewTraceStoreFromConfig_CockroachDB_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()

	const databaseName = "builders"

	connectionString := fmt.Sprintf("postgresql://root@%s/%s?sslmode=disable", perfsql.GetCockroachDBEmulatorHost(), databaseName)

	_, cleanup := sqltest.NewCockroachDBForTests(t, databaseName, sqltest.DoNotApplyMigrations)
	defer cleanup()

	instanceConfig := &config.InstanceConfig{
		DataStoreConfig: config.DataStoreConfig{
			DataStoreType:    config.CockroachDBDataStoreType,
			ConnectionString: connectionString,
		},
	}

	store, err := NewTraceStoreFromConfig(ctx, true, instanceConfig)
	require.NoError(t, err)
	err = store.WriteTraces(types.CommitNumber(0), []paramtools.Params{{"config": "8888"}}, []float32{1.2}, nil, "gs://foobar", time.Now())
	assert.NoError(t, err)
}

func TestNewTraceStoreFromConfig_InvalidDatastoreTypeIsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	dir, err := ioutil.TempDir("", "perf-builders")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(dir))
	}()

	const invalidDataStoreType = config.DataStoreType("not-a-valid-datastore-type")
	instanceConfig := &config.InstanceConfig{
		DataStoreConfig: config.DataStoreConfig{
			DataStoreType:    invalidDataStoreType,
			ConnectionString: filepath.Join(dir, "test.db"),
		},
	}

	_, err = NewTraceStoreFromConfig(ctx, true, instanceConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), invalidDataStoreType)
}
