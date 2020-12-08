// Package gcssources implements Sources.
package gcssources

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/ingest/parser"
	"go.skia.org/infra/perf/go/tracestore/mocks"
	"go.skia.org/infra/perf/go/types"
)

// sourceFileBody is a source file that contains the following keys:
//
//    ,config=8888,name=mytest,sub_result=min_ms,
//    ,config=565,name=mytest,sub_result=min_ms,
const sourceFileBody = `{
	"gitHash": "fe4a4029a080bc955e9588d05a6cd9eb490845d4",
	"issue": "327697",
	"patchset": "1",
	"results": {
	  "mytest": {
		"8888": {
		  "min_ms": 2.223606
		},
		"565": {
		  "min_ms": 2.215988,
		},
	  }
	},
	"key": {
	},
  }
`

func TestNew(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()
	gcsclient := &test_gcsclient.GCSClient{}

	// The two trace ids found in the sourceFileBody.
	const traceID1 = ",config=8888,name=mytest,sub_result=min_ms,"
	const traceID2 = ",config=565,name=mytest,sub_result=min_ms,"
	const e = vec32.MissingDataSentinel
	const tileSize int32 = 50

	traceStore := &mocks.TraceStore{}

	traceStore.On("GetLatestTile", testutils.AnyContext).Return(types.TileNumber(1), nil)
	traceStore.On("TileSize").Return(tileSize, nil)

	// Return a traceSet that has commits of good data which map to CommitNumbers
	// {41, 44, 46, 48, 49}.
	v := vec32.New(int(tileSize))
	v[tileSize-1] = 1
	v[tileSize-2] = 2
	v[tileSize-4] = 3
	traceSet := types.TraceSet{traceID1: v}
	traceStore.On("ReadTracesForCommitRange", testutils.AnyContext, []string{traceID1}, types.CommitNumber(0), types.CommitNumber(49)).Return(traceSet, nil)

	const sourceFilePath = "/path/file.json"
	const sourceFileName = "gs://skia-perf" + sourceFilePath
	traceStore.On("GetSource", testutils.AnyContext, types.CommitNumber(49), traceID1).Return(sourceFileName, nil)

	r := bytes.NewBufferString(sourceFileBody)
	gcsclient.On("FileReader", testutils.AnyContext, sourceFilePath).Return(ioutil.NopCloser(r), nil)

	instanceConfig := &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			Branches: nil,
		},
	}
	p := parser.New(instanceConfig)
	sourceLoader := New(traceStore, gcsclient, p)
	sourceFileNames, err := sourceLoader.Load(ctx, []string{traceID1}, 1)
	require.NoError(t, err)
	assert.Equal(t, []string{sourceFileName}, sourceFileNames)
}
