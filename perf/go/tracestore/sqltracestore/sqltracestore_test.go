package sqltracestore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/gittest"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/sql/sqltest"
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
	traceParamStore := NewTraceParamStore(db)
	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60)
	require.NoError(t, err)
	store, err := New(db, cfg, traceParamStore, inMemoryTraceParams)
	require.NoError(t, err)

	if populateTraces {
		populatedTestDB(t, ctx, store)
		err := inMemoryTraceParams.Refresh(ctx)
		require.NoError(t, err)
	}

	return ctx, store
}

func commonTestSetupWithCommits(t *testing.T) (context.Context, *SQLTraceStore) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	_, err := git.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	traceParamStore := NewTraceParamStore(db)
	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60)
	require.NoError(t, err)
	store, err := New(db, cfg, traceParamStore, inMemoryTraceParams)
	require.NoError(t, err)

	populatedTestDB(t, ctx, store)
	err = inMemoryTraceParams.Refresh(ctx)
	require.NoError(t, err)

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

	ts, commits, _, err := s.ReadTraces(ctx, types.TileNumber(0), keys)
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {e, 2.3, e, 3.3, e, e, e, e},
		",arch=x86,config=8888,": {e, 1.5, e, 2.5, e, e, e, e},
	}, ts)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7})

	ts, commits, _, err = s.ReadTraces(ctx, types.TileNumber(1), keys)
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

	ts, commits, _, err := s.ReadTraces(ctx, types.TileNumber(0), keys)
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

	ts, commits, _, err := s.ReadTraces(ctx, types.TileNumber(0), keys)
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
	ts, commits, _, err := s.ReadTraces(ctx, types.TileNumber(2), keys)
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

	ts, commits, _, err := s.ReadTracesForCommitRange(ctx, keys, types.CommitNumber(1), types.CommitNumber(1))
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

	ts, commits, _, err := s.ReadTracesForCommitRange(ctx, keys, types.CommitNumber(1), types.CommitNumber(2))
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{1, 2})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {2.3, e},
		",arch=x86,config=8888,": {1.5, e},
	}, ts)
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
	ts, commits, _, err := s.QueryTraces(ctx, 0, q, nil)
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
	ts, commits, _, err := s.QueryTraces(ctx, 0, q, nil)
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
	ts, commits, _, err := s.QueryTraces(ctx, 1, q, nil)
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
	ts, commits, _, err := s.QueryTraces(ctx, 0, q, nil)
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
	ts, commits, _, err := s.QueryTraces(ctx, 0, q, nil)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7})
	assert.NoError(t, err)
	assert.Empty(t, ts)
}

func TestQueryTraces_QueryAgainstTileWithNoDataReturnsNoError(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	// Query that has no Postings for the given tile.
	q, err := query.NewFromString("arch=unknown")
	assert.NoError(t, err)
	ts, commits, _, err := s.QueryTraces(ctx, 2, q, nil)
	assert.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{16, 17, 18, 19, 20, 21, 22, 23})
	assert.Empty(t, ts)
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

func TestWriteTraces_InsertDifferentValueAndFile_OverwriteExistingTraceValues(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)
	traceName1 := ",arch=x86,config=8888,"
	sourceFile, err := s.GetSource(ctx, types.CommitNumber(1), traceName1)
	assert.NoError(t, err)
	assert.Equal(t, file1, sourceFile)

	traceNameStrings := []string{traceName1}
	traceSet, commits, _, err := s.ReadTraces(ctx, types.TileNumber(0), traceNameStrings)
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

	traceSet, commits, _, err = s.ReadTraces(ctx, types.TileNumber(0), traceNameStrings)
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

	traceSet, commits, _, err = s.ReadTraces(ctx, types.TileNumber(0), []string{traceName2})
	assert.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7})
	trace, ok = traceSet[traceName2]
	assert.True(t, ok)
	assert.Equal(t, float32(2.4), trace[1])
}

func TestWriteTraces_DuplicateTraceIDsInBatch_Succeeds(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)
	// We are simulating a scenario where the ingestion process sends two values
	// for the exactly same trace in a single batch.
	traceNames := []paramtools.Params{
		{"config": "8888", "arch": "x86"}, // Entry A
		{"config": "8888", "arch": "x86"}, // Entry A (Duplicate of above)
	}
	values := []float32{1.0, 99.0}
	ps := paramtools.ParamSet{
		"config": {"8888"},
		"arch":   {"x86"},
	}

	err := s.WriteTraces(ctx, types.CommitNumber(2), traceNames, values, ps, file1, time.Time{})
	require.NoError(t, err)

	keys := []string{",arch=x86,config=8888,"}
	ts, _, _, err := s.ReadTraces(ctx, types.TileNumber(0), keys)
	require.NoError(t, err)
	trace := ts[",arch=x86,config=8888,"]
	actualValue := trace[2]
	// Verify that the stored value is one of the inputs.
	// We don't care if it's 1.0 (first-write-wins) or 99.0 (last-write-wins),
	// as long as the write succeeded and data is present.
	assert.Contains(t, []float32{1.0, 99.0}, actualValue, "The stored value should be one of the input values from the batch")
}

func TestReadTraces_WithDiscontinueCommitNumbers_Succeed(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	keys := []string{
		",arch=x86,config=8888,",
		",arch=x86,config=565,",
	}

	ts, commits, _, err := s.ReadTraces(ctx, types.TileNumber(0), keys)
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {e, 2.3, e, 3.3, e, e, e, e},
		",arch=x86,config=8888,": {e, 1.5, e, 2.5, e, e, e, e},
	}, ts)

	ts, commits, _, err = s.ReadTraces(ctx, types.TileNumber(1), keys)
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{8, 9, 10, 11, 12, 13, 14, 15})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {4.3, e, e, e, e, e, e, e},
		",arch=x86,config=8888,": {3.5, e, e, e, e, e, e, e},
	}, ts)

	err = s.deleteCommit(ctx, types.CommitNumber(2))
	require.NoError(t, err)

	ts, commits, _, err = s.ReadTraces(ctx, types.TileNumber(0), keys)
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{0, 1, 3, 4, 5, 6, 7})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {e, 2.3, 3.3, e, e, e, e},
		",arch=x86,config=8888,": {e, 1.5, 2.5, e, e, e, e},
	}, ts)

	ts, commits, _, err = s.ReadTraces(ctx, types.TileNumber(1), keys)
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{8, 9, 10, 11, 12, 13, 14, 15})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {4.3, e, e, e, e, e, e, e},
		",arch=x86,config=8888,": {3.5, e, e, e, e, e, e, e},
	}, ts)

	err = s.deleteCommit(ctx, types.CommitNumber(0))
	require.NoError(t, err)

	ts, commits, _, err = s.ReadTraces(ctx, types.TileNumber(0), keys)
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{1, 3, 4, 5, 6, 7})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {2.3, 3.3, e, e, e, e},
		",arch=x86,config=8888,": {1.5, 2.5, e, e, e, e},
	}, ts)

	ts, commits, _, err = s.ReadTraces(ctx, types.TileNumber(1), keys)
	require.NoError(t, err)
	assertCommitNumbersMatch(t, commits, []types.CommitNumber{8, 9, 10, 11, 12, 13, 14, 15})
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {4.3, e, e, e, e, e, e, e},
		",arch=x86,config=8888,": {3.5, e, e, e, e, e, e, e},
	}, ts)
}

func TestGetSourceIds_Success(t *testing.T) {
	ctx, s := commonTestSetupWithCommits(t)

	traceIds := []string{
		",arch=x86,config=8888,",
		",arch=x86,config=565,",
	}
	commitNumbers := []types.CommitNumber{0, 1, 2, 3, 4, 5, 6, 7}
	sourceInfo, err := s.GetSourceIds(ctx, commitNumbers, traceIds)
	assert.NoError(t, err)
	assert.NotNil(t, sourceInfo)
	for _, traceId := range traceIds {
		sourceIdsForTrace, ok := sourceInfo[traceId]
		assert.True(t, ok)
		assert.NotNil(t, sourceIdsForTrace)
		assert.True(t, len(sourceIdsForTrace) > 0)
	}
}
