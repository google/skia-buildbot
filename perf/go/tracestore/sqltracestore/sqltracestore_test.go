package sqltracestore

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/types"
)

const (
	// e is a shorter more readable stand-in for the wordy vec32.MISSING_DATA_SENTINEL.
	e = vec32.MissingDataSentinel

	// testTileSize is the size of tiles we use for tests.
	testTileSize = int32(8)
)

var cfg = config.DataStoreConfig{
	TileSize: testTileSize,
}

func commonTestSetup(t *testing.T, populateTraces bool) (context.Context, *SQLTraceStore, sqltest.Cleanup) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db, cleanup := sqltest.NewCockroachDBForTests(t, fmt.Sprintf("tracestore%d", rand.Int63()))

	store, err := New(db, cfg)
	require.NoError(t, err)

	if populateTraces {
		populatedTestDB(t, ctx, store)
	}

	return ctx, store, cleanup
}

func TestUpdateSourceFile(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, false)
	defer cleanup()

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

func TestReadTraces(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	keys := []string{
		",arch=x86,config=8888,",
		",arch=x86,config=565,",
	}

	ts, err := s.ReadTraces(ctx, 0, keys)
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {e, 2.3, 3.3, e, e, e, e, e},
		",arch=x86,config=8888,": {e, 1.5, 2.5, e, e, e, e, e},
	}, ts)

	ts, err = s.ReadTraces(ctx, 1, keys)
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {4.3, e, e, e, e, e, e, e},
		",arch=x86,config=8888,": {3.5, e, e, e, e, e, e, e},
	}, ts)
}

func TestReadTraces_InvalidKey_AreIngored(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	keys := []string{
		",arch=x86,config='); DROP TABLE TraceValues,",
		",arch=x86,config=565,",
	}

	ts, err := s.ReadTraces(ctx, 0, keys)
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,": {e, 2.3, 3.3, e, e, e, e, e},
	}, ts)
}

func TestReadTraces_NoResults(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	keys := []string{
		",arch=unknown,",
	}

	ts, err := s.ReadTraces(ctx, 0, keys)
	require.NoError(t, err)
	assert.Equal(t, ts, types.TraceSet{
		",arch=unknown,": {e, e, e, e, e, e, e, e},
	})
}

func TestReadTraces_EmptyTileReturnsNoData(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	keys := []string{
		",arch=x86,config=8888,",
		",arch=x86,config=565,",
	}

	// Reading from a tile we haven't written to should succeed and return no data.
	ts, err := s.ReadTraces(ctx, 2, keys)
	assert.NoError(t, err)
	assert.Equal(t, ts, types.TraceSet{
		",arch=x86,config=565,":  {e, e, e, e, e, e, e, e},
		",arch=x86,config=8888,": {e, e, e, e, e, e, e, e},
	})
}

func TestReadTracesForCommitRange_OneCommit_Success(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	keys := []string{
		",arch=x86,config=8888,",
		",arch=x86,config=565,",
	}

	ts, err := s.ReadTracesForCommitRange(ctx, keys, types.CommitNumber(1), types.CommitNumber(1))
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {2.3},
		",arch=x86,config=8888,": {1.5},
	}, ts)
}

func TestReadTracesForCommitRange_TwoCommits_Success(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	keys := []string{
		",arch=x86,config=8888,",
		",arch=x86,config=565,",
	}

	ts, err := s.ReadTracesForCommitRange(ctx, keys, types.CommitNumber(1), types.CommitNumber(2))
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {2.3, 3.3},
		",arch=x86,config=8888,": {1.5, 2.5},
	}, ts)
}

func TestQueryTracesIDOnly_EmptyQueryReturnsError(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

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
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	// Query that matches one trace.
	q, err := query.NewFromString("config=565")
	assert.NoError(t, err)
	ch, err := s.QueryTracesIDOnly(ctx, 5, q)
	require.NoError(t, err)
	assert.Empty(t, paramSetFromParamsChan(ch))
}

func TestQueryTracesIDOnly_MatchesOneTrace(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

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
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

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
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	// Query that matches one trace.
	q, err := query.NewFromString("config=565")
	assert.NoError(t, err)
	ts, err := s.QueryTraces(ctx, 0, q)
	assert.NoError(t, err)
	assert.Equal(t, ts, types.TraceSet{
		",arch=x86,config=565,": {e, 2.3, 3.3, e, e, e, e, e},
	})
}

func TestQueryTraces_NegativeQuery(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	// Query with a negative match that matches one trace.
	q, err := query.NewFromString("config=!565")
	require.NoError(t, err)
	ts, err := s.QueryTraces(ctx, 0, q)
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=8888,": {e, 1.5, 2.5, e, e, e, e, e},
	}, ts)
}

func TestQueryTraces_MatchesOneTraceInTheSecondTile(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	// Query that matches one trace second tile.
	q, err := query.NewFromString("config=565")
	assert.NoError(t, err)
	ts, err := s.QueryTraces(ctx, 1, q)
	assert.NoError(t, err)
	assert.Equal(t, ts, types.TraceSet{
		",arch=x86,config=565,": {4.3, e, e, e, e, e, e, e},
	})
}

func TestQueryTraces_MatchesTwoTraces(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	// Query that matches two traces.
	q, err := query.NewFromString("arch=x86")
	assert.NoError(t, err)
	ts, err := s.QueryTraces(ctx, 0, q)
	assert.NoError(t, err)
	assert.Equal(t, ts, types.TraceSet{
		",arch=x86,config=565,":  {e, 2.3, 3.3, e, e, e, e, e},
		",arch=x86,config=8888,": {e, 1.5, 2.5, e, e, e, e, e},
	})
}

func TestQueryTraces_QueryHasUnknownParamReturnsNoError(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	// Query that has no matching params in the given tile.
	q, err := query.NewFromString("arch=unknown")
	assert.NoError(t, err)
	ts, err := s.QueryTraces(ctx, 0, q)
	assert.NoError(t, err)
	assert.Empty(t, ts)
}

func TestQueryTraces_QueryAgainstTileWithNoDataReturnsNoError(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, false)
	defer cleanup()

	// Query that has no Postings for the given tile.
	q, err := query.NewFromString("arch=unknown")
	assert.NoError(t, err)
	ts, err := s.QueryTraces(ctx, 2, q)
	assert.NoError(t, err)
	assert.Empty(t, ts)
}

func TestTraceCount(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

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
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	ps, err := s.paramSetForTile(ctx, 1)
	assert.NoError(t, err)
	expected := paramtools.ParamSet{
		"arch":   []string{"x86"},
		"config": []string{"565", "8888"},
	}
	assert.Equal(t, expected, ps)
}

func TestParamSetForTile_Empty(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, false)
	defer cleanup()

	// Test the empty case where there is no data in the store.
	ps, err := s.paramSetForTile(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, paramtools.ParamSet{}, ps)
}

func TestGetLatestTile(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	tileNumber, err := s.GetLatestTile(ctx)
	assert.NoError(t, err)
	assert.Equal(t, types.TileNumber(1), tileNumber)
}

func TestGetLatestTile_Empty(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, false)
	defer cleanup()

	// Test the empty case where there is no data in datastore.
	tileNumber, err := s.GetLatestTile(ctx)
	assert.Error(t, err)
	assert.Equal(t, types.BadTileNumber, tileNumber)
}

func TestGetOrderedParamSet(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	tileNumber := types.TileNumber(1)
	assert.False(t, s.orderedParamSetCache.Contains(tileNumber))

	ops, err := s.GetOrderedParamSet(ctx, tileNumber)
	assert.NoError(t, err)
	expected := paramtools.ParamSet{
		"arch":   []string{"x86"},
		"config": []string{"565", "8888"},
	}
	assert.Equal(t, expected, ops.ParamSet)
	assert.Equal(t, []string{"arch", "config"}, ops.KeyOrder)

	assert.True(t, s.orderedParamSetCache.Contains(tileNumber))
}

func TestGetOrderedParamSet_CacheEntriesAreWrittenForParamSets(t *testing.T) {
	_, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	tileNumber := types.TileNumber(0)

	assert.True(t, s.cache.Exists(cacheKeyForParamSets(tileNumber, "arch", "x86")))
	assert.True(t, s.cache.Exists(cacheKeyForParamSets(tileNumber, "config", "565")))
	assert.True(t, s.cache.Exists(cacheKeyForParamSets(tileNumber, "config", "8888")))
}

func TestGetOrderedParamSet_ParamSetCacheIsClearedAfterTTL(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	tileNumber := types.TileNumber(0)
	assert.False(t, s.orderedParamSetCache.Contains(tileNumber))

	ops, err := s.GetOrderedParamSet(ctx, tileNumber)
	assert.NoError(t, err)
	expected := paramtools.ParamSet{
		"arch":   []string{"x86"},
		"config": []string{"565", "8888"},
	}
	assert.Equal(t, expected, ops.ParamSet)
	assert.Equal(t, []string{"arch", "config"}, ops.KeyOrder)
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
		"gs://perf-bucket/2020/02/08/11/testdata.json",
		time.Time{}) // time is unused in this impl of TraceStore.

	// The cached version should be returned.
	ops, err = s.GetOrderedParamSet(ctx, tileNumber)
	assert.NoError(t, err)
	assert.Equal(t, expected, ops.ParamSet)

	// But if we query past the TTL we should get an updated OPS.
	updatedExpected := paramtools.ParamSet{
		"arch":   []string{"risc-v", "x86"},
		"config": []string{"565", "8888"},
	}

	// Swap out timeNow with a time past the TTL.
	s.timeNow = func() time.Time {
		return time.Now().Add(orderedParamSetCacheTTL * 2)
	}
	ops, err = s.GetOrderedParamSet(ctx, tileNumber)
	assert.NoError(t, err)
	assert.Equal(t, updatedExpected, ops.ParamSet)
}

func TestGetOrderedParamSet_Empty(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, false)
	defer cleanup()

	// Test the empty case where there is no data in datastore.
	ops, err := s.GetOrderedParamSet(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, paramtools.ParamSet{}, ops.ParamSet)
}

func TestGetSource(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	filename, err := s.GetSource(ctx, types.CommitNumber(2), ",arch=x86,config=8888,")
	require.NoError(t, err)
	assert.Equal(t, "gs://perf-bucket/2020/02/08/12/testdata.json", filename)
}

func TestGetSource_Empty(t *testing.T) {
	ctx, s, cleanup := commonTestSetup(t, true)
	defer cleanup()

	// Confirm the call works with an empty tracestore.
	filename, err := s.GetSource(ctx, types.CommitNumber(5), ",arch=x86,config=8888,")
	assert.Error(t, err)
	assert.Equal(t, "", filename)
}

func TestSQLTraceStore_TileNumber(t *testing.T) {
	_, s, cleanup := commonTestSetup(t, false)
	defer cleanup()

	assert.Equal(t, types.TileNumber(0), s.TileNumber(types.CommitNumber(1)))
	assert.Equal(t, types.TileNumber(1), s.TileNumber(types.CommitNumber(9)))
}

func TestSQLTraceStore_TileSize(t *testing.T) {
	_, s, cleanup := commonTestSetup(t, false)
	defer cleanup()

	assert.Equal(t, testTileSize, s.TileSize())
}

func TestCommitNumberOfTileStart(t *testing.T) {
	unittest.SmallTest(t)
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
		"gs://perf-bucket/2020/02/08/11/testdata.json",
		time.Time{}) // time is unused in this impl of TraceStore.
	require.NoError(t, err)
	err = store.WriteTraces(ctx, types.CommitNumber(2), traceNames,
		[]float32{2.5, 3.3},
		ps,
		"gs://perf-bucket/2020/02/08/12/testdata.json",
		time.Time{}) // time is unused in this impl of TraceStore.
	require.NoError(t, err)
	err = store.WriteTraces(ctx, types.CommitNumber(8), traceNames,
		[]float32{3.5, 4.3},
		ps,
		"gs://perf-bucket/2020/02/08/13/testdata.json",
		time.Time{}) // time is unused in this impl of TraceStore.
	require.NoError(t, err)
}

func Test_traceIDForSQLFromTraceName_Success(t *testing.T) {
	unittest.SmallTest(t)
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
	unittest.SmallTest(t)
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
        ORDER BY
            trace_id
    `
	assert.Equal(t, expected, b.String())
}
