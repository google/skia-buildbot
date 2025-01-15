package splitter

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs/mocks"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/ingest/format"
)

var results []format.Format

type resultWriteCloser struct {
}

func newResultWriteCloser() resultWriteCloser {
	results = []format.Format{}
	return resultWriteCloser{}
}

func (r resultWriteCloser) Write(p []byte) (n int, err error) {
	var data format.Format
	fmt.Println("Writing json data")
	if err := json.Unmarshal(p, &data); err != nil {
		fmt.Println(err)
		return 0, skerr.Wrap(err)
	}

	sklog.Infof("Adding %v to cache", data)
	results = append(results, data)
	sklog.Infof("Length of objs is %d", len(results))
	return 1, nil
}

func (resultWriteCloser) Close() error { return nil }

const (
	secondaryGCSPath = "gs://secondaryBucket/rootDir"
)

var inputData = format.Format{
	Version: 1,
	GitHash: "f99dc31a4f78ac6574055c7b7c6beb9466333895",
	Issue:   "12345",
	Key: map[string]string{
		"bot": "testbot",
	},
	Results: []format.Result{
		{
			Key: map[string]string{
				"name": "r1",
			},
			Measurement: 1.0,
		},
		{
			Key: map[string]string{
				"name": "r2",
			},
			Measurement: 2.0,
		},
		{
			Key: map[string]string{
				"name": "r3",
			},
			Measurement: 3.0,
		},
		{
			Key: map[string]string{
				"name": "r4",
			},
			Measurement: 4.0,
		},
		{
			Key: map[string]string{
				"name": "r5",
			},
			Measurement: 5.0,
		},
		{
			Key: map[string]string{
				"name": "r6",
			},
			Measurement: 6.0,
		},
		{
			Key: map[string]string{
				"name": "r7",
			},
			Measurement: 7.0,
		},
		{
			Key: map[string]string{
				"name": "r8",
			},
			Measurement: 8.0,
		},
		{
			Key: map[string]string{
				"name": "r9",
			},
			Measurement: 9.0,
		},
		{
			Key: map[string]string{
				"name": "r10",
			},
			Measurement: 10.0,
		},
	},
}

func TestSplitLargeFile_EqualParts_Success(t *testing.T) {
	storageClientMock := mocks.NewGCSClient(t)
	fileName := "gs://primarybucket/dir/large_file.json"

	expectedSplits := []string{"large_file_0.json", "large_file_1.json"}

	writer := newResultWriteCloser()
	for _, split := range expectedSplits {
		storageClientMock.On("FileWriter", testutils.AnyContext, fmt.Sprintf("rootDir/%s", split), mock.Anything).Return(writer, nil)
	}

	ctx := context.Background()
	splitter, err := NewIngestionDataSplitter(ctx, 5, secondaryGCSPath, storageClientMock)
	require.NoError(t, err)
	assert.NotNil(t, splitter)

	err = splitter.SplitAndPublishFormattedData(ctx, inputData, fileName)
	require.NoError(t, err)

	// Expected to have 2 splits.
	storageClientMock.AssertNumberOfCalls(t, "FileWriter", 2)
	assert.Equal(t, 2, len(results))
	for _, res := range results {
		assert.Equal(t, 5, len(res.Results))
	}
}

func TestSplitLargeFile_UnequalParts_Success(t *testing.T) {
	storageClientMock := mocks.NewGCSClient(t)
	fileName := "gs://primarybucket/dir/large_file.json"

	expectedSplits := []string{"large_file_0.json", "large_file_1.json"}

	writer := newResultWriteCloser()
	for _, split := range expectedSplits {
		storageClientMock.On("FileWriter", testutils.AnyContext, fmt.Sprintf("rootDir/%s", split), mock.Anything).Return(writer, nil)
	}

	ctx := context.Background()
	splitter, err := NewIngestionDataSplitter(ctx, 7, secondaryGCSPath, storageClientMock)
	require.NoError(t, err)
	assert.NotNil(t, splitter)

	err = splitter.SplitAndPublishFormattedData(ctx, inputData, fileName)
	require.NoError(t, err)

	// Expected to have 2 splits.
	storageClientMock.AssertNumberOfCalls(t, "FileWriter", 2)
	assert.Equal(t, 2, len(results))
	assert.Equal(t, 7, len(results[0].Results))
	assert.Equal(t, 3, len(results[1].Results))
}

func TestSplitLargeFile_Single_Success(t *testing.T) {
	storageClientMock := mocks.NewGCSClient(t)
	fileName := "gs://primarybucket/dir/large_file.json"

	expectedSplits := []string{"large_file_0.json"}

	writer := newResultWriteCloser()
	for _, split := range expectedSplits {
		storageClientMock.On("FileWriter", testutils.AnyContext, fmt.Sprintf("rootDir/%s", split), mock.Anything).Return(writer, nil)
	}

	ctx := context.Background()
	splitter, err := NewIngestionDataSplitter(ctx, 10, secondaryGCSPath, storageClientMock)
	require.NoError(t, err)
	assert.NotNil(t, splitter)

	err = splitter.SplitAndPublishFormattedData(ctx, inputData, fileName)
	require.NoError(t, err)

	// Expected to have 2 splits.
	storageClientMock.AssertNumberOfCalls(t, "FileWriter", 1)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, inputData, results[0])
}

func TestSplitLargeFile_Single_GCSPath_Success(t *testing.T) {
	storageClientMock := mocks.NewGCSClient(t)
	fileName := "gs://primarybucket/dir/2024/12/31/00/large_file.json"

	expectedSplits := []string{"2024/12/31/00/large_file_0.json"}

	writer := newResultWriteCloser()
	for _, split := range expectedSplits {
		storageClientMock.On("FileWriter", testutils.AnyContext, fmt.Sprintf("rootDir/%s", split), mock.Anything).Return(writer, nil)
	}

	ctx := context.Background()
	splitter, err := NewIngestionDataSplitter(ctx, 10, secondaryGCSPath, storageClientMock)
	require.NoError(t, err)
	assert.NotNil(t, splitter)

	err = splitter.SplitAndPublishFormattedData(ctx, inputData, fileName)
	require.NoError(t, err)

	// Expected to have 2 splits.
	storageClientMock.AssertNumberOfCalls(t, "FileWriter", 1)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, inputData, results[0])
}
