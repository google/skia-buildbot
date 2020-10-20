// Package dfloader implements results.Loader using a DataFrameBuilder.
package dfloader

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/dataframe"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/gittest"
	"go.skia.org/infra/perf/go/trybot"
	"go.skia.org/infra/perf/go/trybot/results"
	"go.skia.org/infra/perf/go/trybot/store"
	"go.skia.org/infra/perf/go/types"
)

const testTileSize = 4

var errFromMock = fmt.Errorf("MockError")

// storeErr implements store.TryBotStore and only returns errors.
type storeErr struct{}

func (storeErr) Write(ctx context.Context, tryFile trybot.TryFile) error { return errFromMock }
func (storeErr) List(ctx context.Context, since time.Time) ([]store.ListResult, error) {
	return nil, errFromMock
}
func (storeErr) Get(ctx context.Context, cl types.CL, patch int) ([]store.GetResult, error) {
	return nil, errFromMock
}

// dfbError implements dataframe.DataFrameBuilder and only returns errors.
type dfbError struct{}

func (dfbError) NewFromQueryAndRange(ctx context.Context, begin, end time.Time, q *query.Query, downsample bool, progress types.Progress) (*dataframe.DataFrame, error) {
	return nil, errFromMock
}
func (dfbError) NewFromKeysAndRange(ctx context.Context, keys []string, begin, end time.Time, downsample bool, progress types.Progress) (*dataframe.DataFrame, error) {
	return nil, errFromMock
}
func (dfbError) NewNFromQuery(ctx context.Context, end time.Time, q *query.Query, n int32, progress types.Progress) (*dataframe.DataFrame, error) {
	return nil, errFromMock
}
func (dfbError) NewNFromKeys(ctx context.Context, end time.Time, keys []string, n int32, progress types.Progress) (*dataframe.DataFrame, error) {
	return nil, errFromMock
}
func (dfbError) PreflightQuery(ctx context.Context, end time.Time, q *query.Query) (int64, paramtools.ParamSet, error) {
	return 0, nil, errFromMock
}

// CleanupFunc is the type of clean up function that NewForTest returns.
type CleanupFunc func()

func setupForTest(t *testing.T) (context.Context, *perfgit.Git, CleanupFunc, []string) {
	ctx, db, _, hashes, instanceConfig, _, gitCleanup := gittest.NewForTest(t)
	instanceConfig.DataStoreConfig.TileSize = testTileSize
	g, err := perfgit.New(ctx, true, db, instanceConfig)
	require.NoError(t, err)
	return ctx, g, CleanupFunc(gitCleanup), hashes
}

func TestLoader_LoadWithDataFrameBuilderThatErrors_LoadReturnsError(t *testing.T) {
	unittest.MediumTest(t)
	ctx, g, cleanup, _ := setupForTest(t)
	defer cleanup()

	loader := New(dfbError{}, storeErr{}, g)

	request := results.TryBotRequest{
		Kind:         results.Commit,
		CommitNumber: 2,
	}
	_, err := loader.Load(ctx, request, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errFromMock.Error())
}
