// Package process does the whole process of ingesting files into a trace store.
package process

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/config"
)

func TestStart_IngestDemoRepoWithSQLite3TraceStore_Success(t *testing.T) {
	unittest.LargeTest(t)

	// Get a temp file to use as an sqlite3 database.
	tmpfile, err := ioutil.TempFile("", "ingest-process")
	require.NoError(t, err)
	require.NoError(t, tmpfile.Close())

	// Get tmp dir to use for repo checkout.
	tmpDir, err := ioutil.TempDir("", "ingest-process")
	require.NoError(t, err)

	defer func() {
		err = os.Remove(tmpfile.Name())
		assert.NoError(t, err)
		err = os.RemoveAll(tmpDir)
		assert.NoError(t, err)
	}()

	instanceConfig := config.InstanceConfig{
		DataStoreConfig: config.DataStoreConfig{
			DataStoreType:    config.SQLite3DataStoreType,
			TileSize:         256,
			ConnectionString: tmpfile.Name(),
		},
		IngestionConfig: config.IngestionConfig{
			SourceConfig: config.SourceConfig{
				SourceType: config.DirSourceType,
				Sources:    []string{"../../../integration/data"},
			},
		},
		GitRepoConfig: config.GitRepoConfig{
			URL: "https://github.com/skia-dev/perf-demo-repo.git",
			Dir: tmpDir,
		},
	}

	err = Start(context.Background(), true, &instanceConfig)
	require.NoError(t, err)
	// The integration data set has 9 good files, 1 file with a bad commit, and
	// 1 malformed JSON file.
	assert.Equal(t, int64(11), metrics2.GetCounter("perfserver_ingest_files_received").Get())
	assert.Equal(t, int64(1), metrics2.GetCounter("perfserver_ingest_bad_githash").Get())
	assert.Equal(t, int64(1), metrics2.GetCounter("perfserver_ingest_failed_to_parse").Get())
}
