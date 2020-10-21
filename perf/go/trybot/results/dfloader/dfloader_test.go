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
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dataframe/mocks"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/gittest"
	"go.skia.org/infra/perf/go/trybot/results"
	storeMocks "go.skia.org/infra/perf/go/trybot/store/mocks"
	"go.skia.org/infra/perf/go/types"
)

var errFromMock = fmt.Errorf("MockError")

const testTileSize = 4

// CleanupFunc is the type of clean up function that NewForTest returns.
type CleanupFunc func()

func setupForTest(t *testing.T) (context.Context, *perfgit.Git, CleanupFunc, []string) {
	ctx, db, _, hashes, instanceConfig, _, gitCleanup := gittest.NewForTest(t)
	instanceConfig.DataStoreConfig.TileSize = testTileSize
	g, err := perfgit.New(ctx, true, db, instanceConfig)
	require.NoError(t, err)
	return ctx, g, CleanupFunc(gitCleanup), hashes
}

func TestLoader_UnknownCommit_ReturnsError(t *testing.T) {
	unittest.MediumTest(t)
	ctx, g, cleanup, _ := setupForTest(t)
	defer cleanup()

	dfb := &mocks.DataFrameBuilder{}
	storeMock := &storeMocks.TryBotStore{}
	loader := New(dfb, storeMock, g)
	request := results.TryBotRequest{
		Kind:         results.Commit,
		CommitNumber: 200, // Not a valid commit.
	}
	_, err := loader.Load(ctx, request, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to get details for CommitNumber")
}

func TestLoader_InvalidQuery_ReturnsError(t *testing.T) {
	unittest.MediumTest(t)
	ctx, g, cleanup, _ := setupForTest(t)
	defer cleanup()

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

func TestLoader_LoadWithDataFrameBuilderThatErrorsNewNFromQuery_LoadReturnsError(t *testing.T) {
	unittest.MediumTest(t)
	ctx, g, cleanup, _ := setupForTest(t)
	defer cleanup()

	dfb := &mocks.DataFrameBuilder{}
	dfb.On("NewNFromQuery", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errFromMock)

	storeMock := &storeMocks.TryBotStore{}
	loader := New(dfb, storeMock, g)
	request := results.TryBotRequest{
		Kind:         results.Commit,
		CommitNumber: 2, // Valid commit that gittest.NewForTest has added.
	}
	_, err := loader.Load(ctx, request, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errFromMock.Error())
}

func TestLoader_LoadWithTryBotStoreThatErrors_LoadReturnsError(t *testing.T) {
	unittest.MediumTest(t)
	ctx, g, cleanup, _ := setupForTest(t)
	defer cleanup()

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
	unittest.MediumTest(t)
	ctx, g, cleanup, _ := setupForTest(t)
	defer cleanup()

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
	unittest.MediumTest(t)
	ctx, g, cleanup, _ := setupForTest(t)
	defer cleanup()

	dfb := &mocks.DataFrameBuilder{}
	df := &dataframe.DataFrame{
		Header:   []*dataframe.ColumnHeader{},
		ParamSet: paramtools.ParamSet{},
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
