package sqltracestore

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockCache "go.skia.org/infra/go/cache/mock"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/gittest"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/tracecache"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/types"
)

const (
	// e is a shorter more readable stand-in for the wordy vec32.MISSING_DATA_SENTINEL.
	e = vec32.MissingDataSentinel

	// testTileSize is the size of tiles we use for tests.
	testTileSize = int32(8)
)

var cfg = config.DataStoreConfig{
	TileSize:      testTileSize,
	DataStoreType: config.SpannerDataStoreType,
}

var (
	file1 = "gs://perf-bucket/2020/02/08/11/testdata.json"
	file2 = "gs://perf-bucket/2020/02/08/12/testdata.json"
	file3 = "gs://perf-bucket/2020/02/08/13/testdata.json"
	file4 = "gs://perf-bucket/2020/02/08/11/new_testdata.json"
)

func commonTestSetup(t *testing.T, populateTraces bool) (context.Context, *SQLTraceStore) {
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTests(t, "tracestore")

	store, err := New(db, cfg)
	require.NoError(t, err)

	if populateTraces {
		populatedTestDB(t, ctx, store)
	}

	return ctx, store
}

func commonTestSetupWithCommits(t *testing.T) (context.Context, *SQLTraceStore) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	_, err := git.New(ctx, true, db, instanceConfig)
	require.NoError(t, err)

	store, err := New(db, cfg)
	require.NoError(t, err)

	populatedTestDB(t, ctx, store)

	return ctx, store
}

func TestUpdateSourceFile(t *testing.T) {
	ctx, s := commonTestSetup(t, false)

	// Do each update twice to ensure the IDs don't change.
	id, err := s.updateSourceFile(ctx, "foo.txt")
	assert.NoError(t, err)

	id2, err := s.updateSourceFile(ctx, "foo.txt")
	assert.NoError(t, err)
	assert.Equal(t, id, id2)

	id, err = s.updateSourceFile(ctx, "bar.txt")
	assert.NoError(t, err)

	id2, err = s.updateSourceFile(ctx, "bar.txt")
	assert.NoError(t, err)
	assert.Equal(t, id, id2)
}

func assertCommitNumbersMatch(t *testing.T, commits []provider.Commit, commitNumbers []types.CommitNumber) {
	assert.Len(t, commits, len(commitNumbers), "Must be the same length.")
	for i, c := range commits {
		assert.Equal(t, c.CommitNumber, commitNumbers[i])
	}
}

func TestReadTraces(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	keys := []string{
		",arch=x86,config=8888,",
		",arch=x86,config=565,",
	}

	ts, commits, err := s.ReadTraces(ctx, types.TileNumber(0), keys)
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {e, 2.3, e, 3.3, e, e, e, e},
		",arch=x86,config=8888,": {e, 1.5, e, 2.5, e, e, e, e},
	}, ts)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7})

	ts, commits, err = s.ReadTraces(ctx, types.TileNumber(1), keys)
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{8, 9, 10, 11, 12, 13, 14, 15})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {4.3, e, e, e, e, e, e, e},
		",arch=x86,config=8888,": {3.5, e, e, e, e, e, e, e},
	}, ts)
}

func TestReadTraces_InvalidKey_AreIngored(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	keys := []string{
		",arch=x86,config='); DROP TABLE TraceValues,",
		",arch=x86,config=565,",
	}

	ts, commits, err := s.ReadTraces(ctx, types.TileNumber(0), keys)
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,": {e, 2.3, e, 3.3, e, e, e, e},
	}, ts)
}

func TestReadTraces_NoResults(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	keys := []string{
		",arch=unknown,",
	}

	ts, commits, err := s.ReadTraces(ctx, types.TileNumber(0), keys)
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7})

	assert.Equal(t, ts, types.TraceSet{
		",arch=unknown,": {e, e, e, e, e, e, e, e},
	})
}

func TestReadTraces_EmptyTileReturnsNoData(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	keys := []string{
		",arch=x86,config=8888,",
		",arch=x86,config=565,",
	}

	// Reading from a tile we haven't written to should succeed and return no data.
	ts, commits, err := s.ReadTraces(ctx, types.TileNumber(2), keys)
	assert.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{16, 17, 18, 19, 20, 21, 22, 23})
	assert.Equal(t, ts, types.TraceSet{
		",arch=x86,config=565,":  {e, e, e, e, e, e, e, e},
		",arch=x86,config=8888,": {e, e, e, e, e, e, e, e},
	})
}

func TestReadTracesForCommitRange_OneCommit_Success(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	keys := []string{
		",arch=x86,config=8888,",
		",arch=x86,config=565,",
	}

	ts, commits, err := s.ReadTracesForCommitRange(ctx, keys, types.CommitNumber(1), types.CommitNumber(1))
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{1})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {2.3},
		",arch=x86,config=8888,": {1.5},
	}, ts)
}

func TestReadTracesForCommitRange_TwoCommits_Success(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	keys := []string{
		",arch=x86,config=8888,",
		",arch=x86,config=565,",
	}

	ts, commits, err := s.ReadTracesForCommitRange(ctx, keys, types.CommitNumber(1), types.CommitNumber(2))
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{1, 2})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {2.3, e},
		",arch=x86,config=8888,": {1.5, e},
	}, ts)
}

func TestRestrictByCounting_EmptyPlan_ReturnsEmptyRestrictClause(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	const emptyTileNumber = types.TileNumber(1)
	clause, key, planDisposition := s.restrictByCounting(ctx, emptyTileNumber, paramtools.NewParamSet())
	require.Empty(t, key)
	require.Empty(t, clause)
	require.Equal(t, runnable, planDisposition)
}

func TestRestrictByCounting_OneKeyInPlan_ReturnsEmptyRestrictClause(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	const emptyTileNumber = types.TileNumber(1)
	clause, key, planDisposition := s.restrictByCounting(ctx, emptyTileNumber, paramtools.ParamSet{"arch": []string{"x86"}})
	require.Empty(t, key)
	require.Empty(t, clause)
	require.Equal(t, runnable, planDisposition)
}

func TestRestrictByCounting_TwoKeysInPlan_ReturnsNonEmptyRestrictClause(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	const emptyTileNumber = types.TileNumber(1)

	plan := paramtools.ParamSet{
		"config": []string{"565"}, // Matches one trace.
		"arch":   []string{"x86"}, // Matches two traces.
	}

	// Should return 'config' as the key with the least matches.
	clause, key, planDisposition := s.restrictByCounting(ctx, emptyTileNumber, plan)
	require.Equal(t, "config", key)
	expectedClause := `
    AND trace_ID IN
    (
            '\x277262a9236d571883d47dab102070bc'
    )`

	require.Equal(t, expectedClause, clause)
	require.Equal(t, runnable, planDisposition)
}

func TestRestrictByCounting_TwoKeysInPlanButOneKeyDoesNotMatchAnything_ReturnsSkippableDisposition(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	const emptyTileNumber = types.TileNumber(1)

	plan := paramtools.ParamSet{
		"unknownKey": []string{"blah-blah-blah"}, // Matches one trace.
		"arch":       []string{"x86"},            // Matches two traces.
	}

	// Should return 'config' as the key with the least matches.
	clause, key, planDisposition := s.restrictByCounting(ctx, emptyTileNumber, plan)
	require.Equal(t, "", key)
	require.Equal(t, "", clause)
	require.Equal(t, skippable, planDisposition)
}

func TestQueryTracesIDOnly_EmptyQueryReturnsError(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	// Query that matches one trace.
	q, err := query.NewFromString("")
	assert.NoError(t, err)
	const emptyTileNumber = types.TileNumber(5)
	_, err = s.QueryTracesIDOnly(ctx, emptyTileNumber, q)
	assert.Error(t, err)
}

// paramSetFromParamsChan is a utility func that reads all the Params from the
// channel and returns them in a ParamSet.
func paramSetFromParamsChan(ch <-chan paramtools.Params) paramtools.ParamSet {
	ret := paramtools.NewParamSet()
	for p := range ch {
		ret.AddParams(p)
	}
	ret.Normalize()
	return ret
}

func TestQueryTracesIDOnly_EmptyTileReturnsEmptyParamset(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	// Query that matches one trace.
	q, err := query.NewFromString("config=565")
	assert.NoError(t, err)
	ch, err := s.QueryTracesIDOnly(ctx, 5, q)
	require.NoError(t, err)
	assert.Empty(t, paramSetFromParamsChan(ch))
}

func TestQueryTracesIDOnly_MatchesOneTrace(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	// Query that matches one trace.
	q, err := query.NewFromString("config=565")
	require.NoError(t, err)
	ch, err := s.QueryTracesIDOnly(ctx, 0, q)
	require.NoError(t, err)
	expected := paramtools.ParamSet{
		"arch":   []string{"x86"},
		"config": []string{"565"},
	}
	assert.Equal(t, expected, paramSetFromParamsChan(ch))
}

func TestQueryTracesIDOnly_QueryThatTriggersUserOfARestrictClause_Success(t *testing.T) {
	ctx, s := commonTestSetup(t, true)
	s.queryUsesRestrictClause.Reset()

	// "config=565" Matches one trace. "arch=x86" Matches two traces. So the
	// query will use a restrict clause to speed up the query.
	q, err := query.NewFromString("arch=x86&config=565")
	require.NoError(t, err)
	ch, err := s.QueryTracesIDOnly(ctx, 0, q)
	require.NoError(t, err)
	expected := paramtools.ParamSet{
		"arch":   []string{"x86"},
		"config": []string{"565"},
	}
	assert.Equal(t, expected, paramSetFromParamsChan(ch))
	assert.Equal(t, int64(1), s.queryUsesRestrictClause.Get())
}

func TestQueryTracesIDOnly_MatchesTwoTraces(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	// Query that matches two traces.
	q, err := query.NewFromString("arch=x86")
	assert.NoError(t, err)
	ch, err := s.QueryTracesIDOnly(ctx, 0, q)
	require.NoError(t, err)
	expected := paramtools.ParamSet{
		"arch":   []string{"x86"},
		"config": []string{"565", "8888"},
	}
	assert.Equal(t, expected, paramSetFromParamsChan(ch))
}

func TestQueryTraces_MatchesOneTrace(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	// Query that matches one trace.
	q, err := query.NewFromString("config=565")
	assert.NoError(t, err)
	ts, commits, err := s.QueryTraces(ctx, 0, q, nil)
	assert.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7})
	assert.Equal(t, ts, types.TraceSet{
		",arch=x86,config=565,": {e, 2.3, e, 3.3, e, e, e, e},
	})
}

func TestQueryTraces_NegativeQuery(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	// Query with a negative match that matches one trace.
	q, err := query.NewFromString("config=!565")
	require.NoError(t, err)
	ts, commits, err := s.QueryTraces(ctx, 0, q, nil)
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=8888,": {e, 1.5, e, 2.5, e, e, e, e},
	}, ts)
}

func TestQueryTraces_MatchesOneTraceInTheSecondTile(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	// Query that matches one trace second tile.
	q, err := query.NewFromString("config=565")
	assert.NoError(t, err)
	ts, commits, err := s.QueryTraces(ctx, 1, q, nil)
	assert.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{8, 9, 10, 11, 12, 13, 14, 15})
	assert.Equal(t, ts, types.TraceSet{
		",arch=x86,config=565,": {4.3, e, e, e, e, e, e, e},
	})
}

func TestQueryTraces_MatchesTwoTraces(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	// Query that matches two traces.
	q, err := query.NewFromString("arch=x86")
	assert.NoError(t, err)
	ts, commits, err := s.QueryTraces(ctx, 0, q, nil)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7})
	assert.NoError(t, err)
	assert.Equal(t, ts, types.TraceSet{
		",arch=x86,config=565,":  {e, 2.3, e, 3.3, e, e, e, e},
		",arch=x86,config=8888,": {e, 1.5, e, 2.5, e, e, e, e},
	})
}

func TestQueryTraces_QueryHasUnknownParamReturnsNoError(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	// Query that has no matching params in the given tile.
	q, err := query.NewFromString("arch=unknown")
	assert.NoError(t, err)
	ts, commits, err := s.QueryTraces(ctx, 0, q, nil)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7})
	assert.NoError(t, err)
	assert.Empty(t, ts)
}

func TestQueryTraces_QueryAgainstTileWithNoDataReturnsNoError(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	// Query that has no Postings for the given tile.
	q, err := query.NewFromString("arch=unknown")
	assert.NoError(t, err)
	ts, commits, err := s.QueryTraces(ctx, 2, q, nil)
	assert.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{16, 17, 18, 19, 20, 21, 22, 23})
	assert.Empty(t, ts)
}

func TestQueryTraces_WithTraceCache_Success(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	// Query that matches two traces.
	q, err := query.NewFromString("arch=x86")
	assert.NoError(t, err)
	cache := mockCache.NewCache(t)
	traceCache := tracecache.New(cache)

	// Configure the cache to return the below params.
	params := []paramtools.Params{
		paramtools.NewParams(",arch=x86,config=565,"),
		paramtools.NewParams(",arch=x86,config=8888,"),
	}
	b, err := json.Marshal(params)
	assert.NoError(t, err)
	cache.On("GetValue", testutils.AnyContext, mock.Anything).Return(string(b), nil)

	ts, commits, err := s.QueryTraces(ctx, 0, q, traceCache)
	assert.NoError(t, err)
	cache.AssertExpectations(t)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7})
	assert.Equal(t, ts, types.TraceSet{
		",arch=x86,config=565,":  {e, 2.3, e, 3.3, e, e, e, e},
		",arch=x86,config=8888,": {e, 1.5, e, 2.5, e, e, e, e},
	})
}

func TestQueryTraces_WithTraceCache_Miss_Success(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	// Query that matches two traces.
	q, err := query.NewFromString("arch=x86")
	assert.NoError(t, err)
	cache := mockCache.NewCache(t)
	traceCache := tracecache.New(cache)
	// Make the cache return nil.
	cache.On("GetValue", testutils.AnyContext, mock.Anything).Return("", nil)

	ts, commits, err := s.QueryTraces(ctx, 0, q, traceCache)
	assert.NoError(t, err)
	// Makes sure the cache GetValue function was invoked.
	cache.AssertExpectations(t)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7})
	assert.Equal(t, ts, types.TraceSet{
		",arch=x86,config=565,":  {e, 2.3, e, 3.3, e, e, e, e},
		",arch=x86,config=8888,": {e, 1.5, e, 2.5, e, e, e, e},
	})
}

func TestTraceCount(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	count, err := s.TraceCount(ctx, 0)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)

	count, err = s.TraceCount(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)

	count, err = s.TraceCount(ctx, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestParamSetForTile(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	ps, err := s.paramSetForTile(ctx, 1)
	assert.NoError(t, err)
	expected := paramtools.ReadOnlyParamSet{
		"arch":   []string{"x86"},
		"config": []string{"565", "8888"},
	}
	assert.Equal(t, expected, ps)
}

func TestParamSetForTile_Empty(t *testing.T) {
	ctx, s := commonTestSetup(t, false)

	// Test the empty case where there is no data in the store.
	ps, err := s.paramSetForTile(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, paramtools.NewReadOnlyParamSet(), ps)
}

func TestGetLatestTile(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	tileNumber, err := s.GetLatestTile(ctx)
	assert.NoError(t, err)
	assert.Equal(t, types.TileNumber(1), tileNumber)
}

func TestGetLatestTile_Empty(t *testing.T) {
	ctx, s := commonTestSetup(t, false)

	// Test the empty case where there is no data in datastore.
	tileNumber, err := s.GetLatestTile(ctx)
	assert.Error(t, err)
	assert.Equal(t, types.BadTileNumber, tileNumber)
}

func TestGetParamSet(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	tileNumber := types.TileNumber(1)
	assert.False(t, s.orderedParamSetCache.Contains(tileNumber))

	ps, err := s.GetParamSet(ctx, tileNumber)
	assert.NoError(t, err)
	expected := paramtools.ReadOnlyParamSet{
		"arch":   []string{"x86"},
		"config": []string{"565", "8888"},
	}
	assert.Equal(t, expected, ps)

	assert.True(t, s.orderedParamSetCache.Contains(tileNumber))
}

func TestGetParamSet_CacheEntriesAreWrittenForParamSets(t *testing.T) {
	_, s := commonTestSetup(t, true)

	tileNumber := types.TileNumber(0)

	assert.True(t, s.cache.Exists(cacheKeyForParamSets(tileNumber, "arch", "x86")))
	assert.True(t, s.cache.Exists(cacheKeyForParamSets(tileNumber, "config", "565")))
	assert.True(t, s.cache.Exists(cacheKeyForParamSets(tileNumber, "config", "8888")))
}

func TestGetParamSet_ParamSetCacheIsClearedAfterTTL(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	tileNumber := types.TileNumber(0)
	assert.False(t, s.orderedParamSetCache.Contains(tileNumber))

	ps, err := s.GetParamSet(ctx, tileNumber)
	assert.NoError(t, err)
	expected := paramtools.ReadOnlyParamSet{
		"arch":   []string{"x86"},
		"config": []string{"565", "8888"},
	}
	assert.Equal(t, expected, ps)
	assert.True(t, s.orderedParamSetCache.Contains(tileNumber))

	// Add new points that will expand the ParamSet.
	traceNames := []paramtools.Params{
		{"config": "8888", "arch": "risc-v"},
		{"config": "565", "arch": "risc-v"},
	}
	err = s.WriteTraces(ctx, types.CommitNumber(1), traceNames,
		[]float32{1.5, 2.3},
		paramtools.ParamSet{
			"config": {"565", "8888"},
			"arch":   {"risc-v"},
		}, // ParamSet is empty because WriteTraces doesn't use it in this impl.
		file1,
		time.Time{}) // time is unused in this impl of TraceStore.
	require.NoError(t, err)

	// The cached version should be returned.
	ps, err = s.GetParamSet(ctx, tileNumber)
	assert.NoError(t, err)
	assert.Equal(t, expected, ps)

	// But if we query past the TTL we should get an updated OPS.
	updatedExpected := paramtools.ReadOnlyParamSet{
		"arch":   []string{"risc-v", "x86"},
		"config": []string{"565", "8888"},
	}

	// Set the time with a time past the TTL.
	ctx = context.WithValue(ctx, now.ContextKey, time.Now().Add(orderedParamSetCacheTTL*2))
	ps, err = s.GetParamSet(ctx, tileNumber)
	assert.NoError(t, err)
	assert.Equal(t, updatedExpected, ps)
}

func TestGetParamSet_Empty(t *testing.T) {
	ctx, s := commonTestSetup(t, false)

	// Test the empty case where there is no data in datastore.
	ps, err := s.GetParamSet(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, paramtools.ReadOnlyParamSet{}, ps)
}

func TestGetSource(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	filename, err := s.GetSource(ctx, types.CommitNumber(3), ",arch=x86,config=8888,")
	require.NoError(t, err)
	assert.Equal(t, file2, filename)
}

func TestGetSource_Empty(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	// Confirm the call works with an empty tracestore.
	filename, err := s.GetSource(ctx, types.CommitNumber(5), ",arch=x86,config=8888,")
	assert.Error(t, err)
	assert.Equal(t, "", filename)
}

func TestSQLTraceStore_TileNumber(t *testing.T) {
	_, s := commonTestSetup(t, false)

	assert.Equal(t, types.TileNumber(0), s.TileNumber(types.CommitNumber(1)))
	assert.Equal(t, types.TileNumber(1), s.TileNumber(types.CommitNumber(9)))
}

func TestSQLTraceStore_TileSize(t *testing.T) {
	_, s := commonTestSetup(t, false)

	assert.Equal(t, testTileSize, s.TileSize())
}

func TestCommitNumberOfTileStart(t *testing.T) {
	s := &SQLTraceStore{
		tileSize: 8,
	}
	assert.Equal(t, types.CommitNumber(0), s.CommitNumberOfTileStart(0))
	assert.Equal(t, types.CommitNumber(0), s.CommitNumberOfTileStart(1))
	assert.Equal(t, types.CommitNumber(0), s.CommitNumberOfTileStart(7))
	assert.Equal(t, types.CommitNumber(8), s.CommitNumberOfTileStart(8))
	assert.Equal(t, types.CommitNumber(8), s.CommitNumberOfTileStart(9))
}

func populatedTestDB(t *testing.T, ctx context.Context, store *SQLTraceStore) {
	traceNames := []paramtools.Params{
		{"config": "8888", "arch": "x86"},
		{"config": "565", "arch": "x86"},
	}
	ps := paramtools.ParamSet{
		"config": {"565", "8888"},
		"arch":   {"x86"},
	}

	err := store.WriteTraces(ctx, types.CommitNumber(1), traceNames,
		[]float32{1.5, 2.3},
		ps,
		file1,
		time.Time{}) // time is unused in this impl of TraceStore.
	require.NoError(t, err)
	err = store.WriteTraces(ctx, types.CommitNumber(3), traceNames,
		[]float32{2.5, 3.3},
		ps,
		file2,
		time.Time{}) // time is unused in this impl of TraceStore.
	require.NoError(t, err)
	err = store.WriteTraces(ctx, types.CommitNumber(8), traceNames,
		[]float32{3.5, 4.3},
		ps,
		file3,
		time.Time{}) // time is unused in this impl of TraceStore.
	require.NoError(t, err)
}

func Test_traceIDForSQLFromTraceName_Success(t *testing.T) {
	/*
	   $ python3
	   Python 3.7.3 (default, Dec 20 2019, 18:57:59)
	   [GCC 8.3.0] on linux
	   Type "help", "copyright", "credits" or "license" for more information.
	   >>> import hashlib
	   >>> hashlib.md5(b",arch=x86,config=8888,").hexdigest()
	   'fe385b159ff55dca481069805e5ff050'
	*/
	assert.Equal(t, traceIDForSQL(`\xfe385b159ff55dca481069805e5ff050`), traceIDForSQLFromTraceName(",arch=x86,config=8888,"))
}

func Test_ExpandConvertTraceIDs_Success(t *testing.T) {
	context := convertTraceIDsContext{
		TileNumber: 12,
		TraceIDs:   []traceIDForSQL{"foo", "bar", "baz"},
		AsOf:       "AS OF SYSTEM TIME '-5s'",
	}

	tmpl, err := template.New("").Parse(templates[convertTraceIDs])
	require.NoError(t, err)
	var b bytes.Buffer
	err = tmpl.Execute(&b, context)
	require.NoError(t, err)
	expected := `
        
        SELECT
            key_value, trace_id
        FROM
            Postings@by_trace_id
            AS OF SYSTEM TIME '-5s'
        WHERE
            tile_number = 12
            AND trace_id IN (
                'foo'
                ,'bar'
                ,'baz'
                )
    `
	assert.Equal(t, expected, b.String())
}

func TestGetLsatNSources_MoreCommitsMatchThanAreAskedFor_Success(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	sources, err := s.GetLastNSources(ctx, ",arch=x86,config=8888,", 2)
	require.NoError(t, err)
	expected := []tracestore.Source{
		{
			Filename:     file3,
			CommitNumber: 8,
		},
		{
			Filename:     file2,
			CommitNumber: 3,
		},
	}
	require.Equal(t, expected, sources)
}

func TestGetLsatNSources_LessCommitsMatchThanAreAskedFor_Success(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	sources, err := s.GetLastNSources(ctx, ",arch=x86,config=8888,", 4)
	require.NoError(t, err)
	expected := []tracestore.Source{
		{
			Filename:     file3,
			CommitNumber: 8,
		},
		{
			Filename:     file2,
			CommitNumber: 3,
		},
		{
			Filename:     file1,
			CommitNumber: 1,
		},
	}
	require.Equal(t, expected, sources)
}

func TestGetLsatNSources_NoMatchesForTraceID_ReturnsEmptySlice(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	sources, err := s.GetLastNSources(ctx, ",this=key,does=not,match=anything,", 4)
	require.NoError(t, err)
	expected := []tracestore.Source{}
	require.Equal(t, expected, sources)
}

func TestGetTraceIDsBySource_SourceInSecondTile_Success(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	secondTile := types.TileNumber(1)
	traceIDs, err := s.GetTraceIDsBySource(ctx, file3, secondTile)
	require.NoError(t, err)
	expected := []string{",arch=x86,config=565,", ",arch=x86,config=8888,"}
	require.ElementsMatch(t, expected, traceIDs)
}

func TestGetTraceIDsBySource_LookForSourceThatDoesNotExist_ReturnsEmptySlice(t *testing.T) {
	ctx, s := commonTestSetup(t, true)

	secondTile := types.TileNumber(1)
	traceIDs, err := s.GetTraceIDsBySource(ctx, "gs://perf-bucket/this-file-does-not-exist.json", secondTile)
	require.NoError(t, err)
	require.Empty(t, traceIDs)
}

func TestWriteTraces_InsertDifferentValueAndFile_OverwriteExistingTraceValues(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)
	traceName1 := ",arch=x86,config=8888,"
	sourceFile, err := s.GetSource(ctx, types.CommitNumber(1), traceName1)
	assert.NoError(t, err)
	assert.Equal(t, file1, sourceFile)

	traceNameStrings := []string{traceName1}
	traceSet, commits, err := s.ReadTraces(ctx, types.TileNumber(0), traceNameStrings)
	assert.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7})

	trace, ok := traceSet[traceName1]
	assert.True(t, ok)
	assert.Equal(t, float32(1.5), trace[1])

	// Write traces with new and conflict value and file.
	traceNames := []paramtools.Params{
		{"config": "8888", "arch": "x86"},
		{"config": "565", "arch": "risc-v"},
	}
	err = s.WriteTraces(ctx, types.CommitNumber(1), traceNames,
		[]float32{1.6, 2.4},
		paramtools.ParamSet{
			"config": {"565", "8888"},
			"arch":   {"x86", "risc-v"},
		},
		file4,
		time.Time{}) // time is unused in this impl of TraceStore.
	require.NoError(t, err)

	// Verify traceName1 is updated
	sourceFile, err = s.GetSource(ctx, types.CommitNumber(1), traceName1)
	assert.NoError(t, err)
	assert.Equal(t, file4, sourceFile)

	traceSet, commits, err = s.ReadTraces(ctx, types.TileNumber(0), traceNameStrings)
	assert.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7})

	trace, ok = traceSet[traceName1]
	assert.True(t, ok)
	assert.Equal(t, float32(1.6), trace[1])

	// Verify traceName2 is inserted
	traceName2 := ",arch=risc-v,config=565,"
	sourceFile, err = s.GetSource(ctx, types.CommitNumber(1), traceName2)
	assert.NoError(t, err)
	assert.Equal(t, file4, sourceFile)

	traceSet, commits, err = s.ReadTraces(ctx, types.TileNumber(0), []string{traceName2})
	assert.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7})
	trace, ok = traceSet[traceName2]
	assert.True(t, ok)
	assert.Equal(t, float32(2.4), trace[1])
}

func TestReadTraces_WithDiscontinueCommitNumbers_Succeed(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	keys := []string{
		",arch=x86,config=8888,",
		",arch=x86,config=565,",
	}

	ts, commits, err := s.ReadTraces(ctx, types.TileNumber(0), keys)
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {e, 2.3, e, 3.3, e, e, e, e},
		",arch=x86,config=8888,": {e, 1.5, e, 2.5, e, e, e, e},
	}, ts)

	ts, commits, err = s.ReadTraces(ctx, types.TileNumber(1), keys)
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{8, 9, 10, 11, 12, 13, 14, 15})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {4.3, e, e, e, e, e, e, e},
		",arch=x86,config=8888,": {3.5, e, e, e, e, e, e, e},
	}, ts)

	err = s.deleteCommit(ctx, types.CommitNumber(2))
	require.NoError(t, err)

	ts, commits, err = s.ReadTraces(ctx, types.TileNumber(0), keys)
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 3, 4, 5, 6, 7})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {e, 2.3, 3.3, e, e, e, e},
		",arch=x86,config=8888,": {e, 1.5, 2.5, e, e, e, e},
	}, ts)

	ts, commits, err = s.ReadTraces(ctx, types.TileNumber(1), keys)
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{8, 9, 10, 11, 12, 13, 14, 15})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {4.3, e, e, e, e, e, e, e},
		",arch=x86,config=8888,": {3.5, e, e, e, e, e, e, e},
	}, ts)

	err = s.deleteCommit(ctx, types.CommitNumber(0))
	require.NoError(t, err)

	ts, commits, err = s.ReadTraces(ctx, types.TileNumber(0), keys)
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{1, 3, 4, 5, 6, 7})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {2.3, 3.3, e, e, e, e},
		",arch=x86,config=8888,": {1.5, 2.5, e, e, e, e},
	}, ts)

	ts, commits, err = s.ReadTraces(ctx, types.TileNumber(1), keys)
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{8, 9, 10, 11, 12, 13, 14, 15})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {4.3, e, e, e, e, e, e, e},
		",arch=x86,config=8888,": {3.5, e, e, e, e, e, e, e},
	}, ts)
}
