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
	"go.skia.org/infra/go/skerr"
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
		  "min_ms": 2.215988
		}
	  }
	}
  }
`

// sourceFileBody2 is a source file that contains the following keys:
//
//    ,config=8888,name=mytest,sub_result=min_ms,
//    ,config=565,name=mytest,sub_result=min_ms,
//    ,config=gles,name=mytest,sub_result=min_ms,
const sourceFileBody2 = `{
	"gitHash": "fe4a4029a080bc955e9588d05a6cd9eb490845d4",
	"issue": "327697",
	"patchset": "1",
	"results": {
	  "mytest": {
		"8888": {
		  "min_ms": 2.223606
		},
		"565": {
		  "min_ms": 2.215988
		},
		"gles": {
			"min_ms": 1.15123
		}
	  }
	}
  }
`

var expectedError = skerr.Fmt("an error")

// Returns ingestion parser.
func ingestParser() *parser.Parser {
	instanceConfig := &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			Branches: nil,
		},
	}
	return parser.New(instanceConfig)
}

func TestLoad_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	// The two trace ids found in the sourceFileBody.
	const traceID1 = ",config=8888,name=mytest,sub_result=min_ms,"
	const traceID2 = ",config=565,name=mytest,sub_result=min_ms,"
	const tileSize int32 = 50
	const sourceFilePath = "/path/file.json"
	const sourceFileName = "gs://skia-perf" + sourceFilePath

	// Configure the traceStore mock.
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

	// Return our good source file name for the right commit number.
	traceStore.On("GetSource", testutils.AnyContext, types.CommitNumber(49), traceID1).Return(sourceFileName, nil)

	// Configure the gcsclient mock.
	gcsclient := &test_gcsclient.GCSClient{}
	r := bytes.NewBufferString(sourceFileBody)
	gcsclient.On("FileReader", testutils.AnyContext, sourceFilePath).Return(ioutil.NopCloser(r), nil)

	s := New(traceStore, gcsclient, ingestParser())
	sourceFileNames, err := s.Load(ctx, []string{traceID1}, 1)
	require.NoError(t, err)
	assert.Equal(t, []string{sourceFileName}, sourceFileNames)
}

func TestLoad_HappyPathLoadingTwoCommits_Success(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	// The two trace ids found in the sourceFileBody.
	const traceID1 = ",config=8888,name=mytest,sub_result=min_ms,"
	const traceID2 = ",config=565,name=mytest,sub_result=min_ms,"
	const traceID3 = ",config=gles,name=mytest,sub_result=min_ms,"
	const tileSize int32 = 50
	const sourceFilePath = "/path/sourceFileBody.json"   // corresponds to sourceFileBody
	const sourceFilePath2 = "/path/sourceFileBody2.json" // corresponds to sourceFileBody2
	const sourceFileName = "gs://skia-perf" + sourceFilePath
	const sourceFileName2 = "gs://skia-perf" + sourceFilePath2

	// Configure the traceStore mock.
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

	// Return our good source file name for the right commit number.
	traceStore.On("GetSource", testutils.AnyContext, types.CommitNumber(49), traceID1).Return(sourceFileName, nil)
	traceStore.On("GetSource", testutils.AnyContext, types.CommitNumber(48), traceID1).Return(sourceFileName2, nil)

	// Configure the gcsclient mock.
	gcsclient := &test_gcsclient.GCSClient{}
	r := bytes.NewBufferString(sourceFileBody)
	gcsclient.On("FileReader", testutils.AnyContext, sourceFilePath).Return(ioutil.NopCloser(r), nil)
	r2 := bytes.NewBufferString(sourceFileBody2)
	gcsclient.On("FileReader", testutils.AnyContext, sourceFilePath2).Return(ioutil.NopCloser(r2), nil)

	s := New(traceStore, gcsclient, ingestParser())
	sourceFileNames, err := s.Load(ctx, []string{traceID1}, 2)
	require.NoError(t, err)
	assert.Equal(t, []string{sourceFileName, sourceFileName2}, sourceFileNames)
}

func TestLoad_HappyPathLoadingTwoCommitsSplitAcrossTwoIters_Success(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	// The two trace ids found in the sourceFileBody.
	const traceID1 = ",config=8888,name=mytest,sub_result=min_ms,"
	const traceID2 = ",config=565,name=mytest,sub_result=min_ms,"
	const traceID3 = ",config=gles,name=mytest,sub_result=min_ms,"
	const tileSize int32 = 2
	const sourceFilePath = "/path/sourceFileBody.json"   // corresponds to sourceFileBody
	const sourceFilePath2 = "/path/sourceFileBody2.json" // corresponds to sourceFileBody2
	const sourceFileName = "gs://skia-perf" + sourceFilePath
	const sourceFileName2 = "gs://skia-perf" + sourceFilePath2

	// Configure the traceStore mock.
	traceStore := &mocks.TraceStore{}
	traceStore.On("GetLatestTile", testutils.AnyContext).Return(types.TileNumber(3), nil)
	traceStore.On("TileSize").Return(tileSize, nil)

	// Return a traceSet that has one commit of good data which maps to CommitNumber{5}.
	v := vec32.New(int(tileSize))
	v[tileSize-1] = 1
	traceSet := types.TraceSet{traceID1: v}
	traceStore.On("ReadTracesForCommitRange", testutils.AnyContext, []string{traceID1}, types.CommitNumber(4), types.CommitNumber(5)).Return(traceSet, nil)

	// On the second iter return a good data which maps to CommitNumber{1}
	v2 := vec32.New(2 * int(tileSize)) // On the second iter the range doubles.
	v2[2*tileSize-3] = 2
	traceSet2 := types.TraceSet{traceID1: v2}
	traceStore.On("ReadTracesForCommitRange", testutils.AnyContext, []string{traceID1}, types.CommitNumber(0), types.CommitNumber(3)).Return(traceSet2, nil)

	// Return our good source file name for the right commit number.
	traceStore.On("GetSource", testutils.AnyContext, types.CommitNumber(5), traceID1).Return(sourceFileName, nil)
	traceStore.On("GetSource", testutils.AnyContext, types.CommitNumber(1), traceID1).Return(sourceFileName2, nil)

	// Configure the gcsclient mock.
	gcsclient := &test_gcsclient.GCSClient{}
	r := bytes.NewBufferString(sourceFileBody)
	gcsclient.On("FileReader", testutils.AnyContext, sourceFilePath).Return(ioutil.NopCloser(r), nil)
	r2 := bytes.NewBufferString(sourceFileBody2)
	gcsclient.On("FileReader", testutils.AnyContext, sourceFilePath2).Return(ioutil.NopCloser(r2), nil)

	s := New(traceStore, gcsclient, ingestParser())
	sourceFileNames, err := s.Load(ctx, []string{traceID1}, 2)
	require.NoError(t, err)
	assert.Equal(t, []string{sourceFileName, sourceFileName2}, sourceFileNames)
}

func TestLoad_AskForUnknownTraceID_ReturnsEmptySlice(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	const unknownTraceID = ",foo=bar,"
	const tileSize int32 = 50

	// Configure the traceStore mock.
	traceStore := &mocks.TraceStore{}
	traceStore.On("GetLatestTile", testutils.AnyContext).Return(types.TileNumber(1), nil)
	traceStore.On("TileSize").Return(tileSize, nil)

	// Return an empty TraceSet since the traceid is unknown.
	traceStore.On("ReadTracesForCommitRange", testutils.AnyContext, []string{unknownTraceID}, types.CommitNumber(0), types.CommitNumber(49)).Return(types.TraceSet{}, nil)

	gcsclient := &test_gcsclient.GCSClient{}

	s := New(traceStore, gcsclient, ingestParser())
	sourceFileNames, err := s.Load(ctx, []string{unknownTraceID}, 1)
	require.NoError(t, err)
	assert.Empty(t, sourceFileNames)
}

func TestLoad_OneGoodTraceIDAndOneUnknownTraceID_Success(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	// The two trace ids found in the sourceFileBody.
	const traceID1 = ",config=8888,name=mytest,sub_result=min_ms,"
	const traceID2 = ",config=565,name=mytest,sub_result=min_ms,"
	const unknownTraceID = ",foo=bar,"
	const tileSize int32 = 50
	const sourceFilePath = "/path/file.json"
	const sourceFileName = "gs://skia-perf" + sourceFilePath

	// Configure the traceStore mock.
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

	traceStore.On("GetSource", testutils.AnyContext, types.CommitNumber(49), traceID1).Return(sourceFileName, nil)

	traceStore.On("ReadTracesForCommitRange", testutils.AnyContext, []string{unknownTraceID}, types.CommitNumber(0), types.CommitNumber(49)).Return(types.TraceSet{}, nil)

	// Configure the gcsclient mock.
	gcsclient := &test_gcsclient.GCSClient{}
	r := bytes.NewBufferString(sourceFileBody)

	// We should only get one read, which is for the file source of the known traceid.
	gcsclient.On("FileReader", testutils.AnyContext, sourceFilePath).Return(ioutil.NopCloser(r), nil)

	s := New(traceStore, gcsclient, ingestParser())
	sourceFileNames, err := s.Load(ctx, []string{traceID1}, 1)
	require.NoError(t, err)
	assert.Equal(t, []string{sourceFileName}, sourceFileNames)
}

func TestLoad_GetLatestTileFail_Failure(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	// The two trace ids found in the sourceFileBody.
	const traceID1 = ",config=8888,name=mytest,sub_result=min_ms,"

	// Configure the traceStore mock.
	traceStore := &mocks.TraceStore{}

	// GetLatestTile failing should cause Load to fail.
	traceStore.On("GetLatestTile", testutils.AnyContext).Return(types.BadTileNumber, expectedError)

	gcsclient := &test_gcsclient.GCSClient{}

	s := New(traceStore, gcsclient, ingestParser())
	_, err := s.Load(ctx, []string{traceID1}, 1)
	require.Equal(t, expectedError, err)
}

func TestLoad_TileSizeFail_Failure(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	// The two trace ids found in the sourceFileBody.
	const traceID1 = ",config=8888,name=mytest,sub_result=min_ms,"
	const traceID2 = ",config=565,name=mytest,sub_result=min_ms,"
	const tileSize int32 = 50
	const sourceFilePath = "/path/file.json"
	const sourceFileName = "gs://skia-perf" + sourceFilePath

	// Configure the traceStore mock.
	traceStore := &mocks.TraceStore{}
	traceStore.On("GetLatestTile", testutils.AnyContext).Return(types.TileNumber(1), nil)

	// TileSize of -1 should cause the iter to fail.
	traceStore.On("TileSize").Return(int32(-1))

	gcsclient := &test_gcsclient.GCSClient{}

	s := New(traceStore, gcsclient, ingestParser())
	_, err := s.Load(ctx, []string{traceID1}, 1)
	require.Contains(t, err.Error(), "end is invalid")
}
