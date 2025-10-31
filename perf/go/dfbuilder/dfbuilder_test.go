package dfbuilder

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/cache/local"
	mockCache "go.skia.org/infra/go/cache/mock"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/gittest"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/tracecache"
	"go.skia.org/infra/perf/go/tracestore"
	mockTraceStore "go.skia.org/infra/perf/go/tracestore/mocks"
	"go.skia.org/infra/perf/go/tracestore/sqltracestore"
	"go.skia.org/infra/perf/go/types"
)

var (
	cfg = &config.InstanceConfig{
		DataStoreConfig: config.DataStoreConfig{
			TileSize:      256,
			DataStoreType: config.SpannerDataStoreType,
		},
	}
)

const preflightSubqueriesForExistingKeysFeatureFlag = false

func getSqlTraceStore(t *testing.T, db pool.Pool, cfg config.DataStoreConfig) (*sqltracestore.SQLTraceStore, *sqltracestore.InMemoryTraceParams) {
	traceParamStore := sqltracestore.NewTraceParamStore(db)
	inMemoryTraceParams, err := sqltracestore.NewInMemoryTraceParams(context.Background(), db, 1)
	assert.NoError(t, err)
	store, err := sqltracestore.New(db, cfg, traceParamStore, inMemoryTraceParams)
	require.NoError(t, err)
	return store, inMemoryTraceParams
}
func TestBuildTraceMapper(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "dfbuilder")
	store, _ := getSqlTraceStore(t, db, cfg.DataStoreConfig)
	tileMap := sliceOfTileNumbersFromCommits([]types.CommitNumber{0, 1, 255, 256, 257}, store)
	expected := []types.TileNumber{0, 1}
	assert.Equal(t, expected, tileMap)

	tileMap = sliceOfTileNumbersFromCommits([]types.CommitNumber{}, store)
	expected = []types.TileNumber{}
	assert.Equal(t, expected, tileMap)
}

// The keys of values are structured keys, not encoded keys.
func addValuesAtIndex(store tracestore.TraceStore, inMemoryTraceParams *sqltracestore.InMemoryTraceParams, index types.CommitNumber, keyValues map[string]float32, filename string, ts time.Time) error {
	ctx := context.Background()
	ps := paramtools.ParamSet{}
	params := []paramtools.Params{}
	values := []float32{}
	for k, v := range keyValues {
		p, err := query.ParseKey(k)
		if err != nil {
			return err
		}
		ps.AddParams(p)
		params = append(params, p)
		values = append(values, v)
	}
	err := store.WriteTraces(ctx, index, params, values, ps, filename, ts)
	if err != nil {
		return err
	}
	return inMemoryTraceParams.Refresh(ctx)
}

func TestBuildNew(t *testing.T) {
	ctx := context.Background()

	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	// TODO(b/451967534) Temporary - remove config.Config modifications after Redis is implemented.
	config.Config = &config.InstanceConfig{}
	config.Config.Experiments = config.Experiments{ProgressUseRedisCache: false}

	instanceConfig.DataStoreConfig.TileSize = 6

	store, inMemoryTraceParams := getSqlTraceStore(t, db, instanceConfig.DataStoreConfig)

	builder := NewDataFrameBuilderFromTraceStore(g, store, nil, 2, doNotFilterParentTraces, instanceConfig.QueryConfig.CommitChunkSize, instanceConfig.QueryConfig.MaxEmptyTilesForQuery, preflightSubqueriesForExistingKeysFeatureFlag)

	// Add some points to the first and second tile.
	err = addValuesAtIndex(store, inMemoryTraceParams, 0, map[string]float32{
		",arch=x86,config=8888,": 1.2,
		",arch=x86,config=565,":  2.1,
		",arch=arm,config=8888,": 100.5,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)
	err = addValuesAtIndex(store, inMemoryTraceParams, 1, map[string]float32{
		",arch=x86,config=8888,": 1.3,
		",arch=x86,config=565,":  2.2,
		",arch=arm,config=8888,": 100.6,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)
	err = addValuesAtIndex(store, inMemoryTraceParams, 7, map[string]float32{
		",arch=x86,config=8888,": 1.0,
		",arch=x86,config=565,":  2.5,
		",arch=arm,config=8888,": 101.1,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)

	// NewFromQueryAndRange
	q, err := query.New(url.Values{"config": []string{"8888"}})
	assert.NoError(t, err)
	now := gittest.StartTime.Add(7 * time.Minute)

	df, err := builder.NewFromQueryAndRange(ctx, now.Add(-8*time.Minute), now.Add(time.Second), q, false, progress.New())
	require.NoError(t, err)
	assert.Len(t, df.TraceSet, 2)
	assert.Len(t, df.Header, 3)
	assert.Equal(t, types.CommitNumber(0), df.Header[0].Offset)
	assert.Equal(t, dataframe.TimestampSeconds(1680000000), df.Header[0].Timestamp)
	assert.Equal(t, types.CommitNumber(1), df.Header[1].Offset)
	assert.Equal(t, dataframe.TimestampSeconds(1680000060), df.Header[1].Timestamp)
	assert.Equal(t, types.CommitNumber(7), df.Header[2].Offset)
	assert.Equal(t, dataframe.TimestampSeconds(1680000420), df.Header[2].Timestamp)
	assert.Equal(t, types.Trace{1.2, 1.3, 1}, df.TraceSet[",arch=x86,config=8888,"])
	assert.Equal(t, types.Trace{100.5, 100.6, 101.1}, df.TraceSet[",arch=arm,config=8888,"])

	// A dense response from NewNFromQuery().
	df, err = builder.NewNFromQuery(ctx, now, q, 4, progress.New())
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 2)
	assert.Len(t, df.Header, 3)
	assert.Equal(t, df.Header[0].Offset, types.CommitNumber(0))
	assert.Equal(t, df.Header[1].Offset, types.CommitNumber(1))
	assert.Equal(t, df.Header[2].Offset, types.CommitNumber(7))
	assert.Equal(t, df.TraceSet[",arch=x86,config=8888,"][0], float32(1.2))
	assert.Equal(t, df.TraceSet[",arch=x86,config=8888,"][1], float32(1.3))
	assert.Equal(t, df.TraceSet[",arch=x86,config=8888,"][2], float32(1.0))

	df, err = builder.NewNFromQuery(ctx, now, q, 2, progress.New())
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 2)
	assert.Len(t, df.Header, 2)
	assert.Equal(t, df.Header[1].Offset, types.CommitNumber(7))
	assert.Equal(t, df.TraceSet[",arch=x86,config=8888,"][1], float32(1.0))

	// NewFromQueryAndRange where query doesn't encode.
	q, err = query.New(url.Values{"config": []string{"nvpr"}})
	assert.NoError(t, err)

	df, err = builder.NewFromQueryAndRange(ctx, now.Add(-8*time.Minute), now.Add(time.Second), q, false, progress.New())
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 0)
	assert.Len(t, df.Header, 0)

	// NewFromKeysAndRange.
	df, err = builder.NewFromKeysAndRange(ctx, []string{",arch=x86,config=8888,", ",arch=x86,config=565,"}, now.Add(-8*time.Minute), now.Add(time.Second), false, progress.New())
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 2)
	assert.Len(t, df.Header, 3)
	assert.Len(t, df.ParamSet, 2)
	assert.Len(t, df.TraceSet[",arch=x86,config=8888,"], 3)
	assert.Len(t, df.TraceSet[",arch=x86,config=565,"], 3)

	// NewNFromKeys.
	df, err = builder.NewNFromKeys(ctx, now, []string{",arch=x86,config=8888,", ",arch=x86,config=565,"}, 2, progress.New())
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 2)
	assert.Len(t, df.Header, 2)
	assert.Len(t, df.ParamSet, 2)
	assert.Len(t, df.TraceSet[",arch=x86,config=8888,"], 2)
	assert.Len(t, df.TraceSet[",arch=x86,config=565,"], 2)

	df, err = builder.NewNFromKeys(ctx, now, []string{",arch=x86,config=8888,", ",arch=x86,config=565,"}, 3, progress.New())
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 2)
	assert.Len(t, df.Header, 3)
	assert.Len(t, df.TraceSet[",arch=x86,config=8888,"], 3)
	assert.Len(t, df.TraceSet[",arch=x86,config=565,"], 3)

	df, err = builder.NewNFromKeys(ctx, now, []string{",arch=x86,config=8888,"}, 3, progress.New())
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 1)
	assert.Len(t, df.Header, 3)
	assert.Equal(t, df.TraceSet[",arch=x86,config=8888,"], types.Trace{1.2, 1.3, 1})

	df, err = builder.NewNFromKeys(ctx, now, []string{}, 3, progress.New())
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 0)
	assert.Len(t, df.Header, 0)

	// Empty set of keys should not fail.
	df, err = builder.NewFromKeysAndRange(ctx, []string{}, now.Add(-8*time.Minute), now.Add(time.Second), false, progress.New())
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 0)
	assert.Len(t, df.Header, 0)

	// Add a value that only appears in one of the tiles.
	err = addValuesAtIndex(store, inMemoryTraceParams, 7, map[string]float32{
		",config=8888,model=Pixel,": 3.0,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)
	store.ClearOrderedParamSetCache()

	// This query will only encode for one tile and should still succeed.
	q, err = query.New(url.Values{"model": []string{"Pixel"}})
	assert.NoError(t, err)
	df, err = builder.NewFromQueryAndRange(ctx, now.Add(-8*time.Minute), now.Add(time.Second), q, false, progress.New())
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 1)
	assert.Len(t, df.Header, 1)
}

func TestFromIndexRange_Success(t *testing.T) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	columnHeaders, commitNumbers, _, err := fromIndexRange(ctx, g, types.CommitNumber(0), types.CommitNumber(2))
	require.NoError(t, err)
	assert.Equal(t, 3, len(columnHeaders))
	assert.Equal(t, types.CommitNumber(0), columnHeaders[0].Offset)
	assert.Equal(t, dataframe.TimestampSeconds(gittest.StartTime.Unix()), columnHeaders[0].Timestamp)
	assert.Equal(t, types.CommitNumber(1), columnHeaders[1].Offset)
	assert.Equal(t, dataframe.TimestampSeconds(gittest.StartTime.Add(time.Minute).Unix()), columnHeaders[1].Timestamp)
	assert.Equal(t, types.CommitNumber(2), columnHeaders[2].Offset)
	assert.Equal(t, dataframe.TimestampSeconds(gittest.StartTime.Add(2*time.Minute).Unix()), columnHeaders[2].Timestamp)

	assert.Equal(t, []types.CommitNumber{0, 1, 2}, commitNumbers)
}

func TestFromIndexRange_EmptySliceOnBadCommitNumber(t *testing.T) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	columnHeaders, commitNumbers, _, err := fromIndexRange(ctx, g, types.BadCommitNumber, types.BadCommitNumber)
	require.NoError(t, err)

	assert.Empty(t, columnHeaders)
	assert.Empty(t, commitNumbers)
}

func TestPreflightQuery_EmptyQuery_ReturnsError(t *testing.T) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	instanceConfig.DataStoreConfig.TileSize = 6

	store, inMemoryTraceParams := getSqlTraceStore(t, db, instanceConfig.DataStoreConfig)

	builder := NewDataFrameBuilderFromTraceStore(g, store, nil, 2, doNotFilterParentTraces, instanceConfig.QueryConfig.CommitChunkSize, instanceConfig.QueryConfig.MaxEmptyTilesForQuery, preflightSubqueriesForExistingKeysFeatureFlag)

	// Add some points to the first tile.
	err = addValuesAtIndex(store, inMemoryTraceParams, 0, map[string]float32{
		",arch=x86,config=8888,": 1.2,
		",arch=x86,config=565,":  2.1,
		",arch=arm,config=8888,": 100.5,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)

	q, err := query.NewFromString("")
	require.NoError(t, err)
	_, _, err = builder.PreflightQuery(ctx, q, paramtools.NewReadOnlyParamSet())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Can not pre-flight an empty query")
}

func TestPreflightQuery_NonEmptyQuery_Success(t *testing.T) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	instanceConfig.DataStoreConfig.TileSize = 6

	store, inMemoryTraceParams := getSqlTraceStore(t, db, instanceConfig.DataStoreConfig)

	builder := NewDataFrameBuilderFromTraceStore(g, store, nil, 2, doNotFilterParentTraces, instanceConfig.QueryConfig.CommitChunkSize, instanceConfig.QueryConfig.MaxEmptyTilesForQuery, preflightSubqueriesForExistingKeysFeatureFlag)

	// Add some points to the first tile.
	err = addValuesAtIndex(store, inMemoryTraceParams, 0, map[string]float32{
		",arch=x86,config=8888,": 1.2,
		",arch=x86,config=565,":  2.1,
		",arch=arm,config=8888,": 100.5,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)

	// Create a query that will match two of the points.
	q, err := query.NewFromString("config=8888")
	require.NoError(t, err)

	// The referenceParamSet contains values that should not appear in the
	// returned ParamSet, and some that get retained.
	referenceParamSet := paramtools.ReadOnlyParamSet{
		"arch":   {"x86", "arm", "should-disappear"},
		"config": {"565", "8888", "should-be-retained"},
		// 'should-be-retained' is retained because 'config' is in the query and
		// so all 'config' values should be returned in the ParamSet.
	}
	count, ps, err := builder.PreflightQuery(ctx, q, referenceParamSet)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	expectedParamSet := paramtools.ParamSet{
		"arch":   {"arm", "x86"},
		"config": {"565", "8888", "should-be-retained"},
	}
	assert.Equal(t, expectedParamSet, ps)
}

func TestPreflightQuery_TilesContainDifferentNumberOfMatches_ReturnedParamSetReflectsBothTiles(t *testing.T) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	instanceConfig.DataStoreConfig.TileSize = 6

	store, inMemoryTraceParams := getSqlTraceStore(t, db, instanceConfig.DataStoreConfig)

	builder := NewDataFrameBuilderFromTraceStore(g, store, nil, 2, doNotFilterParentTraces, instanceConfig.QueryConfig.CommitChunkSize, instanceConfig.QueryConfig.MaxEmptyTilesForQuery, preflightSubqueriesForExistingKeysFeatureFlag)

	// Add some points to the first tile.
	err = addValuesAtIndex(store, inMemoryTraceParams, 0, map[string]float32{
		",arch=x86,config=8888,": 1.2,
		",arch=x86,config=565,":  2.1,
		",arch=arm,config=8888,": 100.5,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)

	// Add some points to the second tile.
	err = addValuesAtIndex(store, inMemoryTraceParams, types.CommitNumber(instanceConfig.DataStoreConfig.TileSize), map[string]float32{
		",arch=riscv,config=8888,": 1.2,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)

	// Create a query that will match two of the points in tile 0 and one of the points in tile 1.
	q, err := query.NewFromString("config=8888")
	require.NoError(t, err)

	// The reference ParamSet contains values that should not appear in the
	// returned ParamSet, and some that get retained.
	referenceParamSet := paramtools.ReadOnlyParamSet{
		"arch":   {"x86", "arm", "should-disappear"},
		"config": {"565", "8888", "should-be-retained"},
		// 'should-be-retained' is retained because 'config' is in the query and
		// so all 'config' values should be returned in the ParamSet.
	}
	count, ps, err := builder.PreflightQuery(ctx, q, referenceParamSet)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	expectedParamSet := paramtools.ParamSet{
		"arch":   {"arm", "riscv", "x86"},
		"config": {"565", "8888", "should-be-retained"},
	}
	assert.Equal(t, expectedParamSet, ps)
}

func TestNumMatches_EmptyQuery_ReturnsError(t *testing.T) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	instanceConfig.DataStoreConfig.TileSize = 6

	store, _ := getSqlTraceStore(t, db, instanceConfig.DataStoreConfig)

	builder := NewDataFrameBuilderFromTraceStore(g, store, nil, 2, doNotFilterParentTraces, instanceConfig.QueryConfig.CommitChunkSize, instanceConfig.QueryConfig.MaxEmptyTilesForQuery, preflightSubqueriesForExistingKeysFeatureFlag)
	q, err := query.NewFromString("")
	require.NoError(t, err)
	_, err = builder.NumMatches(ctx, q)
	require.Error(t, err)
}

func TestNumMatches_NonEmptyQuery_Success(t *testing.T) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	instanceConfig.DataStoreConfig.TileSize = 6

	store, inMemoryTraceParams := getSqlTraceStore(t, db, instanceConfig.DataStoreConfig)

	builder := NewDataFrameBuilderFromTraceStore(g, store, nil, 2, doNotFilterParentTraces, instanceConfig.QueryConfig.CommitChunkSize, instanceConfig.QueryConfig.MaxEmptyTilesForQuery, preflightSubqueriesForExistingKeysFeatureFlag)

	// Add some points to the first tile.
	err = addValuesAtIndex(store, inMemoryTraceParams, 0, map[string]float32{
		",arch=x86,config=8888,": 1.2,
		",arch=x86,config=565,":  2.1,
		",arch=arm,config=8888,": 100.5,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)

	// Create a query that will match two of the points.
	q, err := query.NewFromString("config=8888")
	require.NoError(t, err)

	count, err := builder.NumMatches(ctx, q)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestNumMatches_TilesContainDifferentNumberOfMatches_TheLargerOfTheTwoCountsIsReturned(t *testing.T) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	instanceConfig.DataStoreConfig.TileSize = 6

	store, inMemoryTraceParams := getSqlTraceStore(t, db, instanceConfig.DataStoreConfig)

	builder := NewDataFrameBuilderFromTraceStore(g, store, nil, 2, doNotFilterParentTraces, instanceConfig.QueryConfig.CommitChunkSize, instanceConfig.QueryConfig.MaxEmptyTilesForQuery, preflightSubqueriesForExistingKeysFeatureFlag)

	// Add some points to the latest tile.
	err = addValuesAtIndex(store, inMemoryTraceParams, types.CommitNumber(instanceConfig.DataStoreConfig.TileSize+1), map[string]float32{
		",arch=x86,config=8888,": 1.2,
		",arch=x86,config=565,":  2.1,
		",arch=arm,config=8888,": 100.5,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)

	// Add some points to the previous tile.
	err = addValuesAtIndex(store, inMemoryTraceParams, 1, map[string]float32{
		",arch=x86,config=8888,":   1.2,
		",arch=riscv,config=8888,": 2.1,
		",arch=arm,config=8888,":   100.5,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)

	// Create a query that will match two of the points in tile 1, but three
	// points in tile 0.
	q, err := query.NewFromString("config=8888")
	require.NoError(t, err)

	count, err := builder.NumMatches(ctx, q)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestPreflightQuery_Cache_Success(t *testing.T) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	instanceConfig.DataStoreConfig.TileSize = 6

	store, inMemoryTraceParams := getSqlTraceStore(t, db, instanceConfig.DataStoreConfig)
	cache, err := local.New(10)
	require.NoError(t, err)

	traceCache := tracecache.New(cache)

	builder := NewDataFrameBuilderFromTraceStore(g, store, traceCache, 2, doNotFilterParentTraces, instanceConfig.QueryConfig.CommitChunkSize, instanceConfig.QueryConfig.MaxEmptyTilesForQuery, preflightSubqueriesForExistingKeysFeatureFlag)

	// Add some points to the first tile.
	err = addValuesAtIndex(store, inMemoryTraceParams, 0, map[string]float32{
		",arch=x86,config=8888,": 1.2,
		",arch=x86,config=565,":  2.1,
		",arch=arm,config=8888,": 100.5,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)

	// Create a query that will match two of the points.
	q, err := query.NewFromString("config=8888")
	require.NoError(t, err)

	// The referenceParamSet contains values that should not appear in the
	// returned ParamSet, and some that get retained.
	referenceParamSet := paramtools.ReadOnlyParamSet{
		"arch":   {"x86", "arm", "should-disappear"},
		"config": {"565", "8888", "should-be-retained"},
		// 'should-be-retained' is retained because 'config' is in the query and
		// so all 'config' values should be returned in the ParamSet.
	}
	count, ps, err := builder.PreflightQuery(ctx, q, referenceParamSet)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	expectedParamSet := paramtools.ParamSet{
		"arch":   {"arm", "x86"},
		"config": {"565", "8888", "should-be-retained"},
	}
	assert.Equal(t, expectedParamSet, ps)

	traceIds, err := traceCache.GetTraceIds(ctx, 0, q)
	require.NoError(t, err)
	assert.NotEmpty(t, traceIds)
	assert.Equal(t, 2, len(traceIds))
}

func TestPreflightQuery_Cache_Query_Success(t *testing.T) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	store := mockTraceStore.NewTraceStore(t)
	mock_cache := mockCache.NewCache(t)

	traceCache := tracecache.New(mock_cache)

	store.On("TileSize").Return(int32(6))
	store.On("GetLatestTile", testutils.AnyContext).Return(types.TileNumber(0), nil)
	builder := NewDataFrameBuilderFromTraceStore(g, store, traceCache, 2, doNotFilterParentTraces, instanceConfig.QueryConfig.CommitChunkSize, instanceConfig.QueryConfig.MaxEmptyTilesForQuery, preflightSubqueriesForExistingKeysFeatureFlag)

	// Create a query that will match two of the points.
	q, err := query.NewFromString("config=8888")
	require.NoError(t, err)

	// The referenceParamSet contains values that should not appear in the
	// returned ParamSet, and some that get retained.
	referenceParamSet := paramtools.ReadOnlyParamSet{
		"arch":   {"x86", "arm", "should-disappear"},
		"config": {"565", "8888", "should-be-retained"},
		// 'should-be-retained' is retained because 'config' is in the query and
		// so all 'config' values should be returned in the ParamSet.
	}
	expectedParamSet := paramtools.ParamSet{
		"arch":   {"arm", "x86"},
		"config": {"565", "8888", "should-be-retained"},
	}

	// This is the first call, so let's configure the cache to return empty.
	mock_cache.On("GetValue", testutils.AnyContext, mock.Anything).Return("", nil)
	var key, value string
	mock_cache.On("SetValue", testutils.AnyContext, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		key = args[1].(string)
		value = args[2].(string)
	}).Return(nil)

	expectedTraces := []paramtools.Params{
		{
			"arch":   "arm",
			"config": "565",
		},
		{
			"arch":   "x86",
			"config": "8888",
		},
	}
	paramChannel := make(chan paramtools.Params, 10)
	for _, traceId := range expectedTraces {
		paramChannel <- traceId
	}
	close(paramChannel)
	store.On("QueryTracesIDOnly", testutils.AnyContext, mock.Anything, mock.Anything).Return((<-chan paramtools.Params)(paramChannel), nil)
	count, ps, err := builder.PreflightQuery(ctx, q, referenceParamSet)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
	assert.Equal(t, expectedParamSet, ps)

	// Check that values were set in the cache.
	assert.NotEmpty(t, key)
	assert.NotEmpty(t, value)

	// Now lets run the same query but with cache returning data.
	b, err := json.Marshal(expectedTraces)
	assert.NoError(t, err)
	mock_cache.AssertExpectations(t)
	store.AssertExpectations(t)

	// Reset the mocks.
	mock_cache.ExpectedCalls = []*mock.Call{}
	store.ExpectedCalls = []*mock.Call{}

	// QueryTracesIDOnly should not be called since data is returned from cache.
	store.On("TileSize").Return(int32(6))
	store.On("GetLatestTile", testutils.AnyContext).Return(types.TileNumber(0), nil)

	// SetValue should not be called since the data is returned from cache.
	mock_cache.On("GetValue", testutils.AnyContext, mock.Anything).Return(string(b), nil)

	count, ps, err = builder.PreflightQuery(ctx, q, referenceParamSet)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
	assert.Equal(t, expectedParamSet, ps)
	mock_cache.AssertExpectations(t)
	store.AssertExpectations(t)
}

func TestPreflightQuery_SingleConstraintFiltering(t *testing.T) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	instanceConfig.DataStoreConfig.TileSize = 6

	store, inMemoryTraceParams := getSqlTraceStore(t, db, instanceConfig.DataStoreConfig)
	builder := NewDataFrameBuilderFromTraceStore(g, store, nil, 2, doNotFilterParentTraces, instanceConfig.QueryConfig.CommitChunkSize, instanceConfig.QueryConfig.MaxEmptyTilesForQuery, true)

	// Add traces that will exercise the filtering logic.
	err = addValuesAtIndex(store, inMemoryTraceParams, 0, map[string]float32{
		// The target trace that matches both query parameters.
		",benchmark=speedometer3,bot=mac,test=ChartJS,": 1.0,
		// A trace that matches the benchmark but not the test.
		",benchmark=speedometer3,bot=mac,test=OtherTest,": 2.0,
		// A trace that matches the test but not the benchmark.
		",benchmark=OtherBench,bot=win,test=ChartJS,": 3.0,
		// A trace that matches the benchmark but has a different test.
		",benchmark=speedometer3,bot=mac,test=ThirdTest,": 4.0,
		// And a complete mismatch.
		// A trace that matches the benchmark but has a different test.
		",benchmark=xstream4,bot=z100,test=Ztest,": 5.0,
	}, "gs://foo.json", time.Now())
	require.NoError(t, err)

	// The query that should restrict the results significantly.
	queryString := "benchmark=speedometer3"
	q, err := query.NewFromString(queryString)
	require.NoError(t, err)

	// The reference paramset contains all possible values.
	referenceParamSet := paramtools.ReadOnlyParamSet{
		"benchmark": {"speedometer3", "OtherBench", "xtream4"},
		"bot":       {"mac", "win", "z100"},
		"test":      {"ChartJS", "OtherTest", "ThirdTest", "Ztest"},
	}

	// Execute the function under test.
	count, ps, err := builder.PreflightQuery(ctx, q, referenceParamSet)
	require.NoError(t, err)

	// Only one trace should match the full query.
	assert.Equal(t, int64(3), count)

	// The crucial part: the returned paramset should be built *only* from the
	// single matching trace.
	expectedParamSet := paramtools.ParamSet{
		"benchmark": {"speedometer3", "OtherBench", "xtream4"},
		"bot":       {"mac"},
		"test":      {"ChartJS", "OtherTest", "ThirdTest"},
	}
	// Sort the string slices for consistent comparison.
	for _, v := range expectedParamSet {
		sort.Strings(v)
	}

	assert.Equal(t, expectedParamSet, ps)
}

func TestPreflightQuery_MultiConstraintFiltering(t *testing.T) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	instanceConfig.DataStoreConfig.TileSize = 6
	store, inMemoryTraceParams := getSqlTraceStore(t, db, instanceConfig.DataStoreConfig)

	// Add traces that will exercise the filtering logic.
	err = addValuesAtIndex(store, inMemoryTraceParams, 0, map[string]float32{
		// The target trace that matches both query parameters.
		",benchmark=speedometer3,bot=mac,test=ChartJS,": 1.0,
		// A trace that matches the benchmark but not the test.
		",benchmark=speedometer3,bot=mac,test=OtherTest,": 2.0,
		// A trace that matches the test but not the benchmark.
		",benchmark=OtherBench,bot=win,test=ChartJS,": 3.0,
		// A trace that matches the benchmark but has a different test.
		",benchmark=speedometer3,bot=mac,test=ThirdTest,": 4.0,
		// And a complete mismatch.
		",benchmark=xstream4,bot=z100,test=Ztest,": 5.0,
	}, "gs://foo.json", time.Now())
	require.NoError(t, err)

	// The query that should restrict the results significantly.
	queryString := "benchmark=speedometer3&test=ChartJS"
	q, err := query.NewFromString(queryString)
	require.NoError(t, err)

	// The reference paramset contains all possible values.
	referenceParamSet := paramtools.ReadOnlyParamSet{
		"benchmark": {"speedometer3", "OtherBench", "xtream4"},
		"bot":       {"mac", "win", "z100"},
		"test":      {"ChartJS", "OtherTest", "ThirdTest", "Ztest"},
	}

	testCases := []struct {
		name              string
		enableSubqueries  bool
		numMatchingTraces int
		resultPS          paramtools.ParamSet
	}{
		{
			"filter_present_keys",
			true,
			1,
			// The crucial part: the returned paramset should be built using the logic
			// that for a key in the query, we show all possible values that match the
			// other query parameters. For keys not in the query, we only show values
			// that appear in traces that match the full query.
			paramtools.ParamSet{
				"benchmark": {"OtherBench", "speedometer3"},
				"bot":       {"mac"},
				"test":      {"ChartJS", "OtherTest", "ThirdTest"},
			},
		},
		{
			"fill_present_keys_with_all_values",
			false,
			1,
			paramtools.ParamSet{
				"benchmark": {"OtherBench", "speedometer3", "xtream4"},
				"bot":       {"mac"},
				"test":      {"ChartJS", "OtherTest", "ThirdTest", "Ztest"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := NewDataFrameBuilderFromTraceStore(g, store, nil, 2, doNotFilterParentTraces, instanceConfig.QueryConfig.CommitChunkSize, instanceConfig.QueryConfig.MaxEmptyTilesForQuery, tc.enableSubqueries)

			// Execute the function under test.
			count, ps, err := builder.PreflightQuery(ctx, q, referenceParamSet)
			require.NoError(t, err)

			// Only one trace should match the full query.
			assert.Equal(t, int64(tc.numMatchingTraces), count)

			// Sort the string slices for consistent comparison.
			for _, v := range tc.resultPS {
				sort.Strings(v)
			}

			assert.Equal(t, tc.resultPS, ps)
		})
	}
}
