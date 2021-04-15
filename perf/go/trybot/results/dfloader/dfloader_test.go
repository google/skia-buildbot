// Package dfloader implements results.Loader using a DataFrameBuilder.
package dfloader

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dataframe/mocks"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/gittest"
	"go.skia.org/infra/perf/go/trybot/results"
	"go.skia.org/infra/perf/go/trybot/store"
	storeMocks "go.skia.org/infra/perf/go/trybot/store/mocks"
	"go.skia.org/infra/perf/go/types"
)

var errFromMock = fmt.Errorf("MockError")

const testTileSize = 4

const e = vec32.MissingDataSentinel

func setupForTest(t *testing.T) (context.Context, *perfgit.Git, []string) {
	ctx, db, _, hashes, instanceConfig, gitCleanup := gittest.NewForTest(t)
	instanceConfig.DataStoreConfig.TileSize = testTileSize
	g, err := perfgit.New(ctx, true, db, instanceConfig)
	require.NoError(t, err)
	t.Cleanup(gitCleanup)
	return ctx, g, hashes
}

func TestLoader_UnknownCommit_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, _ := setupForTest(t)

	dfb := &mocks.DataFrameBuilder{}
	storeMock := &storeMocks.TryBotStore{}
	loader := New(dfb, storeMock, g)
	request := results.TryBotRequest{
		Kind:         results.Commit,
		Query:        "config=8888",
		CommitNumber: 200, // Not a valid commit.
	}
	_, err := loader.Load(ctx, request, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to get details for CommitNumber")
}

func TestLoader_InvalidQuery_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, _ := setupForTest(t)

	dfb := &mocks.DataFrameBuilder{}
	storeMock := &storeMocks.TryBotStore{}
	loader := New(dfb, storeMock, g)
	request := results.TryBotRequest{
		Kind:         results.Commit,
		CommitNumber: 2, // Valid commit that gittest.NewForTest has added.
		Query:        "%gh&%ij",
	}
	_, err := loader.Load(ctx, request, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid URL")
}

func TestLoader_EmptyQuery_LoadReturnsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, _ := setupForTest(t)

	dfb := &mocks.DataFrameBuilder{}
	storeMock := &storeMocks.TryBotStore{}
	loader := New(dfb, storeMock, g)
	request := results.TryBotRequest{
		Kind:         results.Commit,
		Query:        "",
		CommitNumber: 2, // Valid commit that gittest.NewForTest has added.
	}
	_, err := loader.Load(ctx, request, nil)
	require.Error(t, err)
	assert.Equal(t, ErrQueryMustNotBeEmpty, err)
}

func TestLoader_LoadWithDataFrameBuilderThatErrorsNewNFromQuery_LoadReturnsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, _ := setupForTest(t)

	dfb := &mocks.DataFrameBuilder{}
	dfb.On("NewNFromQuery", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errFromMock)

	storeMock := &storeMocks.TryBotStore{}
	loader := New(dfb, storeMock, g)
	request := results.TryBotRequest{
		Kind:         results.Commit,
		Query:        "config=8888",
		CommitNumber: 2, // Valid commit that gittest.NewForTest has added.
	}
	_, err := loader.Load(ctx, request, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errFromMock.Error())
}

func TestLoader_LoadWithTryBotStoreThatErrors_LoadReturnsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, _ := setupForTest(t)

	dfb := &mocks.DataFrameBuilder{}
	storeMock := &storeMocks.TryBotStore{}
	const cl = types.CL("123456")
	const patch = int(1)
	storeMock.On("Get", mock.Anything, cl, patch).Return(nil, errFromMock)

	loader := New(dfb, storeMock, g)
	request := results.TryBotRequest{
		Kind:        results.TryBot,
		CL:          cl,
		PatchNumber: patch,
	}
	_, err := loader.Load(ctx, request, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errFromMock.Error())
}

func TestLoader_LoadDataFrameBuilderThatErrorsNewNFromKeys_LoadReturnsError(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, _ := setupForTest(t)

	dfb := &mocks.DataFrameBuilder{}
	dfb.On("NewNFromKeys", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errFromMock)

	storeMock := &storeMocks.TryBotStore{}
	const cl = types.CL("123456")
	const patch = int(1)
	storeMock.On("Get", mock.Anything, cl, patch).Return(nil, nil)

	loader := New(dfb, storeMock, g)
	request := results.TryBotRequest{
		Kind:        results.TryBot,
		CL:          cl,
		PatchNumber: patch,
	}
	_, err := loader.Load(ctx, request, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errFromMock.Error())
}

func TestLoader_ZeroLengthResponseFromTryBotStore_LoadReturnsSuccess(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, _ := setupForTest(t)

	dfb := &mocks.DataFrameBuilder{}
	df := &dataframe.DataFrame{
		Header:   []*dataframe.ColumnHeader{},
		ParamSet: paramtools.NewReadOnlyParamSet(),
	}
	dfb.On("NewNFromKeys", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(df, nil)

	storeMock := &storeMocks.TryBotStore{}
	const cl = types.CL("123456")
	const patch = int(1)
	storeMock.On("Get", mock.Anything, cl, patch).Return(nil, nil)

	loader := New(dfb, storeMock, g)
	request := results.TryBotRequest{
		Kind:        results.TryBot,
		CL:          cl,
		PatchNumber: patch,
	}
	resp, err := loader.Load(ctx, request, nil)
	require.NoError(t, err)
	assert.Empty(t, resp.Results)
	assert.Empty(t, resp.Header)
	assert.Empty(t, resp.ParamSet)
}

func TestLoader_OneTraceTryBotHappyPath_LoadReturnsSuccess(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, _ := setupForTest(t)

	dfb := &mocks.DataFrameBuilder{}
	df := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{
			{Offset: 0, Timestamp: gittest.StartTime.Unix()},
			{Offset: 1, Timestamp: gittest.StartTime.Unix() + 1},
			{Offset: 2, Timestamp: gittest.StartTime.Unix() + 2},
			{Offset: 3, Timestamp: gittest.StartTime.Unix() + 3},
			{Offset: 4, Timestamp: gittest.StartTime.Unix() + 4},
			{Offset: 5, Timestamp: gittest.StartTime.Unix() + 5},
			{Offset: 6, Timestamp: gittest.StartTime.Unix() + 6},
			{Offset: 7, Timestamp: gittest.StartTime.Unix() + 7},
			{Offset: 8, Timestamp: gittest.StartTime.Unix() + 8},
			{Offset: 9, Timestamp: gittest.StartTime.Unix() + 9},
		},
		ParamSet: paramtools.ReadOnlyParamSet{"config": []string{"gpu"}},
		TraceSet: types.TraceSet{
			",config=gpu,": []float32{1, 1, 0.9, 0.9, 1.1, 1.1, 0.8, 0.8, 1.2, 1.2},
		},
	}
	dfb.On("NewNFromKeys", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(df, nil)

	storeMock := &storeMocks.TryBotStore{}
	const cl = types.CL("123456")
	const patch = int(1)
	storeResults := []store.GetResult{
		{
			TraceName: ",config=gpu,",
			Value:     3.0,
		},
	}
	storeMock.On("Get", mock.Anything, cl, patch).Return(storeResults, nil)

	loader := New(dfb, storeMock, g)
	request := results.TryBotRequest{
		Kind:        results.TryBot,
		CL:          cl,
		PatchNumber: patch,
	}
	resp, err := loader.Load(ctx, request, nil)
	require.NoError(t, err)
	expected := results.TryBotResult{
		Params:      paramtools.Params{"config": "gpu"},
		Median:      1,
		Lower:       0.1825742,
		Upper:       0.122474514,
		StdDevRatio: 16.329927,
		Values:      []float32{1, 1, 0.9, 0.9, 1.1, 1.1, 0.8, 0.8, 1.2, 3},
	}
	assert.Len(t, resp.Results, 1)
	assert.Equal(t, expected, resp.Results[0])
	assert.Equal(t, types.BadCommitNumber, df.Header[len(df.Header)-1].Offset)
	assert.Equal(t, df.ParamSet, resp.ParamSet)
}

func TestLoader_TwoTraces_LoadReturnsSuccessAndResultsAreSorted(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, _ := setupForTest(t)

	dfb := &mocks.DataFrameBuilder{}
	df := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{
			{Offset: 0, Timestamp: gittest.StartTime.Unix()},
			{Offset: 1, Timestamp: gittest.StartTime.Unix() + 1},
			{Offset: 2, Timestamp: gittest.StartTime.Unix() + 2},
			{Offset: 3, Timestamp: gittest.StartTime.Unix() + 3},
			{Offset: 4, Timestamp: gittest.StartTime.Unix() + 4},
			{Offset: 5, Timestamp: gittest.StartTime.Unix() + 5},
			{Offset: 6, Timestamp: gittest.StartTime.Unix() + 6},
			{Offset: 7, Timestamp: gittest.StartTime.Unix() + 7},
		},
		ParamSet: paramtools.ReadOnlyParamSet{"config": []string{"gpu", "cpu"}},
		TraceSet: types.TraceSet{
			",config=cpu,": []float32{1, 1, 1, 1, 2, 2, 2, 0},
			",config=gpu,": []float32{1, 1, 1, 1, 2, 2, 2, 0},
		},
	}
	dfb.On("NewNFromKeys", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(df, nil)

	storeMock := &storeMocks.TryBotStore{}
	const cl = types.CL("123456")
	const patch = int(1)
	storeResults := []store.GetResult{
		{
			TraceName: ",config=gpu,",
			Value:     6.0,
		},
		{
			TraceName: ",config=cpu,",
			Value:     4.0,
		},
	}
	storeMock.On("Get", mock.Anything, cl, patch).Return(storeResults, nil)

	loader := New(dfb, storeMock, g)
	request := results.TryBotRequest{
		Kind:        results.TryBot,
		CL:          cl,
		PatchNumber: patch,
	}
	resp, err := loader.Load(ctx, request, nil)
	require.NoError(t, err)
	assert.Len(t, resp.Results, 2)
	expected := []results.TryBotResult{
		{
			Params:      paramtools.Params{"config": "gpu"},
			Median:      1,
			Lower:       0,
			Upper:       1,
			StdDevRatio: 5,
			Values:      []float32{1, 1, 1, 1, 2, 2, 2, 6},
		},
		{
			Params:      paramtools.Params{"config": "cpu"},
			Median:      1,
			Lower:       0,
			Upper:       1,
			StdDevRatio: 3,
			Values:      []float32{1, 1, 1, 1, 2, 2, 2, 4},
		},
	}
	assert.Equal(t, expected, resp.Results)

	assert.Equal(t, types.BadCommitNumber, df.Header[len(df.Header)-1].Offset)
	assert.Equal(t, df.ParamSet, resp.ParamSet)
}

func TestLoader_UnknownTracesAreIgnored_LoadReturnsSuccess(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, _ := setupForTest(t)

	// The trybotStore has two results, but there are trace values for only one of those results (config=8888).
	dfb := &mocks.DataFrameBuilder{}
	df := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{
			{Offset: 0, Timestamp: gittest.StartTime.Unix()},
			{Offset: 1, Timestamp: gittest.StartTime.Unix() + 1},
			{Offset: 2, Timestamp: gittest.StartTime.Unix() + 2},
			{Offset: 3, Timestamp: gittest.StartTime.Unix() + 3},
			{Offset: 4, Timestamp: gittest.StartTime.Unix() + 4},
			{Offset: 5, Timestamp: gittest.StartTime.Unix() + 5},
			{Offset: 6, Timestamp: gittest.StartTime.Unix() + 6},
			{Offset: 7, Timestamp: gittest.StartTime.Unix() + 7},
			{Offset: 8, Timestamp: gittest.StartTime.Unix() + 8},
			{Offset: 9, Timestamp: gittest.StartTime.Unix() + 9},
		},
		ParamSet: paramtools.ReadOnlyParamSet{"config": []string{"565", "8888"}},
		TraceSet: types.TraceSet{
			",config=8888,": []float32{1, 1, 0.9, 0.9, 1.1, 1.1, 0.8, 0.8, 1.2, 1.2},
		},
	}
	dfb.On("NewNFromKeys", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(df, nil)

	storeMock := &storeMocks.TryBotStore{}
	const cl = types.CL("123456")
	const patch = int(1)
	storeResults := []store.GetResult{
		{
			TraceName: ",config=8888,",
			Value:     3.0,
		},
		{
			TraceName: ",config=565,",
			Value:     4.0,
		},
	}
	storeMock.On("Get", mock.Anything, cl, patch).Return(storeResults, nil)

	loader := New(dfb, storeMock, g)
	request := results.TryBotRequest{
		Kind:        results.TryBot,
		CL:          cl,
		PatchNumber: patch,
	}
	resp, err := loader.Load(ctx, request, nil)
	require.NoError(t, err)
	expected := results.TryBotResult{
		Params:      paramtools.Params{"config": "8888"},
		Median:      1,
		Lower:       0.1825742,
		Upper:       0.122474514,
		StdDevRatio: 16.329927,
		Values:      []float32{1, 1, 0.9, 0.9, 1.1, 1.1, 0.8, 0.8, 1.2, 3},
	}
	assert.Len(t, resp.Results, 1)
	assert.Equal(t, expected, resp.Results[0])
	assert.Equal(t, types.BadCommitNumber, df.Header[len(df.Header)-1].Offset)
	assert.Equal(t, paramtools.ReadOnlyParamSet{"config": []string{"8888"}}, resp.ParamSet)
}

func TestLoader_InsufficientNonMissingDataSentinel_ResultIsSkipped(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, _ := setupForTest(t)

	// The trybotStore has two results, but there are trace values for only one of those results (config=8888).
	dfb := &mocks.DataFrameBuilder{}
	df := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{
			{Offset: 0, Timestamp: gittest.StartTime.Unix()},
			{Offset: 1, Timestamp: gittest.StartTime.Unix() + 1},
			{Offset: 2, Timestamp: gittest.StartTime.Unix() + 2},
			{Offset: 3, Timestamp: gittest.StartTime.Unix() + 3},
			{Offset: 4, Timestamp: gittest.StartTime.Unix() + 4},
			{Offset: 5, Timestamp: gittest.StartTime.Unix() + 5},
			{Offset: 6, Timestamp: gittest.StartTime.Unix() + 6},
			{Offset: 7, Timestamp: gittest.StartTime.Unix() + 7},
			{Offset: 8, Timestamp: gittest.StartTime.Unix() + 8},
			{Offset: 9, Timestamp: gittest.StartTime.Unix() + 9},
		},
		ParamSet: paramtools.ReadOnlyParamSet{"config": []string{"565", "8888"}},
		TraceSet: types.TraceSet{
			",config=8888,": []float32{1, 1, 0.9, 0.9, 1.1, 1.1, 0.8, 0.8, 1.2, 1.2},
			",config=565,":  []float32{e, e, e, e, e, e, e, e, e, 1.2}, // Should be dropped from results since there isn't enough valid data.
		},
	}
	dfb.On("NewNFromKeys", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(df, nil)

	storeMock := &storeMocks.TryBotStore{}
	const cl = types.CL("123456")
	const patch = int(1)
	storeResults := []store.GetResult{
		{
			TraceName: ",config=8888,",
			Value:     3.0,
		},
		{
			TraceName: ",config=565,",
			Value:     4.0,
		},
	}
	storeMock.On("Get", mock.Anything, cl, patch).Return(storeResults, nil)

	loader := New(dfb, storeMock, g)
	request := results.TryBotRequest{
		Kind:        results.TryBot,
		CL:          cl,
		PatchNumber: patch,
	}
	resp, err := loader.Load(ctx, request, nil)
	require.NoError(t, err)
	expected := results.TryBotResult{
		Params:      paramtools.Params{"config": "8888"},
		Median:      1,
		Lower:       0.1825742,
		Upper:       0.122474514,
		StdDevRatio: 16.329927,
		Values:      []float32{1, 1, 0.9, 0.9, 1.1, 1.1, 0.8, 0.8, 1.2, 3},
	}
	assert.Len(t, resp.Results, 1)
	assert.Equal(t, expected, resp.Results[0])
	assert.Equal(t, types.BadCommitNumber, df.Header[len(df.Header)-1].Offset)
	assert.Equal(t, paramtools.ReadOnlyParamSet{"config": []string{"8888"}}, resp.ParamSet)
}

func TestLoader_InvalidTraceKeysAreIgnored_LoadReturnsSuccess(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, _ := setupForTest(t)

	dfb := &mocks.DataFrameBuilder{}
	df := &dataframe.DataFrame{
		Header:   []*dataframe.ColumnHeader{},
		ParamSet: paramtools.ReadOnlyParamSet{},
		TraceSet: types.TraceSet{
			"this-isnt-a-valid-key": []float32{1, 1, 0.9, 0.9, 1.1, 1.1, 0.8, 0.8, 1.2, 1.2},
		},
	}
	dfb.On("NewNFromKeys", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(df, nil)

	storeMock := &storeMocks.TryBotStore{}
	const cl = types.CL("123456")
	const patch = int(1)
	storeResults := []store.GetResult{
		{
			TraceName: ",config=gpu,",
			Value:     3.0,
		},
	}
	storeMock.On("Get", mock.Anything, cl, patch).Return(storeResults, nil)

	loader := New(dfb, storeMock, g)
	request := results.TryBotRequest{
		Kind:        results.TryBot,
		CL:          cl,
		PatchNumber: patch,
	}
	resp, err := loader.Load(ctx, request, nil)
	require.NoError(t, err)
	assert.Empty(t, resp.Results)
	assert.Empty(t, resp.Header)
	assert.Empty(t, resp.ParamSet)
}
