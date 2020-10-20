// Package dfloader implements results.Loader using a DataFrameBuilder.
package dfloader

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/dataframe/mocks"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/gittest"
	"go.skia.org/infra/perf/go/trybot/results"
	storeMocks "go.skia.org/infra/perf/go/trybot/store/mocks"
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

func TestLoader_LoadWithDataFrameBuilderThatErrors_LoadReturnsError(t *testing.T) {
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
