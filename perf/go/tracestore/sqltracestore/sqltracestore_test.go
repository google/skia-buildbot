package sqltracestore

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
	"go.skia.org/infra/go/vec32"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/types"
)

const (
	// e is a shorter more readable stand-in for the wordy vec32.MISSING_DATA_SENTINEL.
	e = vec32.MissingDataSentinel

	// testTileSize is the size of tiles we use for tests.
	testTileSize = 8
)

func TestCockroachDB(t *testing.T) {
	unittest.LargeTest(t)

	for name, subTest := range subTests {
		t.Run(name, func(t *testing.T) {
			db, cleanup := sqltest.NewCockroachDBForTests(t, "tracestore", sqltest.ApplyMigrations)
			// Commenting out the defer cleanup() can sometimes make failures
			// easier to understand.
			defer cleanup()

			store, err := New(db, perfsql.CockroachDBDialect, testTileSize)
			require.NoError(t, err)

			subTest(t, store)
		})
	}
}

func testUpdateSourceFile(t *testing.T, s *SQLTraceStore) {
	// Do each update twice to ensure the IDs don't change.
	id, err := s.updateSourceFile("foo.txt")
	assert.NoError(t, err)

	id2, err := s.updateSourceFile("foo.txt")
	assert.NoError(t, err)
	assert.Equal(t, id, id2)

	id, err = s.updateSourceFile("bar.txt")
	assert.NoError(t, err)

	id2, err = s.updateSourceFile("bar.txt")
	assert.NoError(t, err)
	assert.Equal(t, id, id2)
}

func testWriteTraceIDAndPostings(t *testing.T, s *SQLTraceStore) {
	const traceName = ",arch=x86,config=8888,"
	p := paramtools.NewParams(traceName)
	const tileNumber types.TileNumber = 1

	// Do each update twice to ensure the IDs don't change.
	traceID, err := s.writeTraceIDAndPostings(p, tileNumber)
	assert.NoError(t, err)

	traceID2, err := s.writeTraceIDAndPostings(p, tileNumber)
	assert.NoError(t, err)
	assert.Equal(t, traceID, traceID2)

	// Confirm the cache entries exist.
	got, ok := s.cache.Get(getHashedTraceName(traceName))
	assert.True(t, ok)
	assert.Equal(t, traceID, got.(traceIDFromSQL))
	assert.True(t, s.cache.Contains(getPostingsCacheEntryKey(traceID, tileNumber)))

	const traceName2 = ",arch=arm,config=8888,"
	p2 := paramtools.NewParams(traceName2)

	traceID, err = s.writeTraceIDAndPostings(p2, tileNumber)
	assert.NoError(t, err)
	assert.NotEqual(t, traceID, traceID2)

	traceID2, err = s.writeTraceIDAndPostings(p2, tileNumber)
	assert.NoError(t, err)
	assert.Equal(t, traceID, traceID2)
}

func testWriteTraces_MultipleBatches_Success(t *testing.T, s *SQLTraceStore) {
	ctx := context.Background()

	const commitNumber = types.CommitNumber(1)

	// Add enough values to force it to be done in batches.
	const testLength = 2*traceValuesInsertBatchSize + 1

	const tileNumber = types.TileNumber(0)

	traceNames := make([]paramtools.Params, 0, testLength)
	values := make([]float32, 0, testLength)

	for i := 0; i < testLength; i++ {
		traceNames = append(traceNames, paramtools.Params{
			"traceid": fmt.Sprintf("%d", i),
			"config":  "8888",
		})
		values = append(values, float32(i))
	}
	err := s.WriteTraces(
		commitNumber,
		traceNames,
		values,
		paramtools.ParamSet{}, // ParamSet is empty because WriteTraces doesn't use it in this impl.
		"gs://not-tested-as-part-of-this-test.json",
		time.Time{}) // time is unused in this impl of TraceStore.
	require.NoError(t, err)

	// Confirm all traces were written.
	q, err := query.NewFromString("config=8888")
	require.NoError(t, err)
	ts, err := s.QueryTracesByIndex(ctx, tileNumber, q)
	assert.NoError(t, err)
	assert.Len(t, ts, testLength)

	// Spot test some values.
	q, err = query.NewFromString("config=8888&traceid=0")
	require.NoError(t, err)
	ts, err = s.QueryTracesByIndex(ctx, tileNumber, q)
	assert.NoError(t, err)

	assert.Equal(t, float32(0), ts[",config=8888,traceid=0,"][s.OffsetFromCommitNumber(commitNumber)])

	q, err = query.NewFromString(fmt.Sprintf("config=8888&traceid=%d", testLength-1))
	require.NoError(t, err)
	ts, err = s.QueryTracesByIndex(ctx, tileNumber, q)
	assert.NoError(t, err)
	assert.Equal(t, float32(testLength-1), ts[fmt.Sprintf(",config=8888,traceid=%d,", testLength-1)][s.OffsetFromCommitNumber(commitNumber)])
}

func testReadTraces(t *testing.T, s *SQLTraceStore) {
	populatedTestDB(t, s)

	keys := []string{
		",arch=x86,config=8888,",
		",arch=x86,config=565,",
	}

	ts, err := s.ReadTraces(0, keys)
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {e, 2.3, 3.3, e, e, e, e, e},
		",arch=x86,config=8888,": {e, 1.5, 2.5, e, e, e, e, e},
	}, ts)

	ts, err = s.ReadTraces(1, keys)
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  {4.3, e, e, e, e, e, e, e},
		",arch=x86,config=8888,": {3.5, e, e, e, e, e, e, e},
	}, ts)
}

func testReadTraces_InvalidKey(t *testing.T, s *SQLTraceStore) {
	populatedTestDB(t, s)

	keys := []string{
		",arch=x86,config='); DROP TABLE TraceValues,",
		",arch=x86,config=565,",
	}

	_, err := s.ReadTraces(0, keys)
	require.Error(t, err)
}

func testReadTraces_NoResults(t *testing.T, s *SQLTraceStore) {
	populatedTestDB(t, s)

	keys := []string{
		",arch=unknown,",
	}

	ts, err := s.ReadTraces(0, keys)
	require.NoError(t, err)
	assert.Equal(t, ts, types.TraceSet{
		",arch=unknown,": {e, e, e, e, e, e, e, e},
	})
}

func testReadTraces_EmptyTileReturnsNoData(t *testing.T, s *SQLTraceStore) {
	populatedTestDB(t, s)

	keys := []string{
		",arch=x86,config=8888,",
		",arch=x86,config=565,",
	}

	// Reading from a tile we haven't written to should succeed and return no data.
	ts, err := s.ReadTraces(2, keys)
	assert.NoError(t, err)
	assert.Equal(t, ts, types.TraceSet{
		",arch=x86,config=565,":  {e, e, e, e, e, e, e, e},
		",arch=x86,config=8888,": {e, e, e, e, e, e, e, e},
	})
}

func testQueryTracesIDOnlyByIndex_EmptyQueryReturnsError(t *testing.T, s *SQLTraceStore) {
	populatedTestDB(t, s)
	ctx := context.Background()

	// Query that matches one trace.
	q, err := query.NewFromString("")
	assert.NoError(t, err)
	const emptyTileNumber = types.TileNumber(5)
	_, err = s.QueryTracesIDOnlyByIndex(ctx, emptyTileNumber, q)
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

func testQueryTracesIDOnlyByIndex_EmptyTileReturnsEmptyParamset(t *testing.T, s *SQLTraceStore) {
	populatedTestDB(t, s)
	ctx := context.Background()

	// Query that matches one trace.
	q, err := query.NewFromString("config=565")
	assert.NoError(t, err)
	ch, err := s.QueryTracesIDOnlyByIndex(ctx, 5, q)
	require.NoError(t, err)
	assert.Empty(t, paramSetFromParamsChan(ch))
}

func testQueryTracesIDOnlyByIndex_MatchesOneTrace(t *testing.T, s *SQLTraceStore) {
	populatedTestDB(t, s)
	ctx := context.Background()

	// Query that matches one trace.
	q, err := query.NewFromString("config=565")
	assert.NoError(t, err)
	ch, err := s.QueryTracesIDOnlyByIndex(ctx, 0, q)
	require.NoError(t, err)
	expected := paramtools.ParamSet{
		"arch":   []string{"x86"},
		"config": []string{"565"},
	}
	assert.Equal(t, expected, paramSetFromParamsChan(ch))
}

func testQueryTracesIDOnlyByIndex_MatchesTwoTraces(t *testing.T, s *SQLTraceStore) {
	populatedTestDB(t, s)
	ctx := context.Background()

	// Query that matches two traces.
	q, err := query.NewFromString("arch=x86")
	assert.NoError(t, err)
	ch, err := s.QueryTracesIDOnlyByIndex(ctx, 0, q)
	require.NoError(t, err)
	expected := paramtools.ParamSet{
		"arch":   []string{"x86"},
		"config": []string{"565", "8888"},
	}
	assert.Equal(t, expected, paramSetFromParamsChan(ch))
}

func testQueryTracesByIndex_MatchesOneTrace(t *testing.T, s *SQLTraceStore) {
	populatedTestDB(t, s)
	ctx := context.Background()

	// Query that matches one trace.
	q, err := query.NewFromString("config=565")
	assert.NoError(t, err)
	ts, err := s.QueryTracesByIndex(ctx, 0, q)
	assert.NoError(t, err)
	assert.Equal(t, ts, types.TraceSet{
		",arch=x86,config=565,": {e, 2.3, 3.3, e, e, e, e, e},
	})
}

func testQueryTracesByIndex_MatchesOneTraceInTheSecondTile(t *testing.T, s *SQLTraceStore) {
	populatedTestDB(t, s)
	ctx := context.Background()

	// Query that matches one trace second tile.
	q, err := query.NewFromString("config=565")
	assert.NoError(t, err)
	ts, err := s.QueryTracesByIndex(ctx, 1, q)
	assert.NoError(t, err)
	assert.Equal(t, ts, types.TraceSet{
		",arch=x86,config=565,": {4.3, e, e, e, e, e, e, e},
	})
}

func testQueryTracesByIndex_MatchesTwoTraces(t *testing.T, s *SQLTraceStore) {
	populatedTestDB(t, s)
	ctx := context.Background()

	// Query that matches two traces.
	q, err := query.NewFromString("arch=x86")
	assert.NoError(t, err)
	ts, err := s.QueryTracesByIndex(ctx, 0, q)
	assert.NoError(t, err)
	assert.Equal(t, ts, types.TraceSet{
		",arch=x86,config=565,":  {e, 2.3, 3.3, e, e, e, e, e},
		",arch=x86,config=8888,": {e, 1.5, 2.5, e, e, e, e, e},
	})
}

func testQueryTracesByIndex_QueryHasUnknownParamReturnsNoError(t *testing.T, s *SQLTraceStore) {
	populatedTestDB(t, s)
	ctx := context.Background()

	// Query that has no matching params in the given tile.
	q, err := query.NewFromString("arch=unknown")
	assert.NoError(t, err)
	ts, err := s.QueryTracesByIndex(ctx, 0, q)
	assert.NoError(t, err)
	assert.Nil(t, ts)
}

func testQueryTracesByIndex_QueryAgainstTileWithNoDataReturnsNoError(t *testing.T, s *SQLTraceStore) {
	ctx := context.Background()

	// Query that has no Postings for the given tile.
	q, err := query.NewFromString("arch=unknown")
	assert.NoError(t, err)
	ts, err := s.QueryTracesByIndex(ctx, 2, q)
	assert.NoError(t, err)
	assert.Nil(t, ts)
}

func testTraceCount(t *testing.T, s *SQLTraceStore) {
	populatedTestDB(t, s)
	ctx := context.Background()

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

func testParamSetForTile(t *testing.T, s *SQLTraceStore) {
	const tileNumber types.TileNumber = 1
	_, err := s.writeTraceIDAndPostings(paramtools.NewParams(",config=8888,arch=x86,"), tileNumber)
	assert.NoError(t, err)
	_, err = s.writeTraceIDAndPostings(paramtools.NewParams(",config=565,arch=arm,"), tileNumber)
	assert.NoError(t, err)
	_, err = s.writeTraceIDAndPostings(paramtools.NewParams(",config=8888,arch=arm64,"), tileNumber)
	assert.NoError(t, err)
	_, err = s.writeTraceIDAndPostings(paramtools.NewParams(",config=gpu,arch=x86_64,"), tileNumber)
	assert.NoError(t, err)

	ps, err := s.paramSetForTile(1)
	assert.NoError(t, err)
	expected := paramtools.ParamSet{
		"arch":   []string{"arm", "arm64", "x86", "x86_64"},
		"config": []string{"565", "8888", "gpu"},
	}
	assert.Equal(t, expected, ps)
}

func testParamSetForTile_Empty(t *testing.T, s *SQLTraceStore) {
	// Test the empty case where there is no data in the store.
	ps, err := s.paramSetForTile(1)
	assert.NoError(t, err)
	assert.Equal(t, paramtools.ParamSet{}, ps)
}

func testGetLatestTile(t *testing.T, s *SQLTraceStore) {
	_, err := s.writeTraceIDAndPostings(paramtools.NewParams(",config=8888,arch=x86,"), types.TileNumber(1))
	assert.NoError(t, err)
	_, err = s.writeTraceIDAndPostings(paramtools.NewParams(",config=8888,arch=arm64,"), types.TileNumber(5))
	assert.NoError(t, err)
	_, err = s.writeTraceIDAndPostings(paramtools.NewParams(",config=gpu,arch=x86_64,"), types.TileNumber(7))
	assert.NoError(t, err)

	tileNumber, err := s.GetLatestTile()
	assert.NoError(t, err)
	assert.Equal(t, types.TileNumber(7), tileNumber)
}

func testGetLatestTile_Empty(t *testing.T, s *SQLTraceStore) {
	// Test the empty case where there is no data in datastore.
	tileNumber, err := s.GetLatestTile()
	assert.Error(t, err)
	assert.Equal(t, types.BadTileNumber, tileNumber)
}

func testGetOrderedParamSet(t *testing.T, s *SQLTraceStore) {
	ctx := context.Background()

	const tileNumber types.TileNumber = 1
	// Now add some trace ids.
	_, err := s.writeTraceIDAndPostings(paramtools.NewParams(",config=8888,arch=x86,"), tileNumber)
	assert.NoError(t, err)
	_, err = s.writeTraceIDAndPostings(paramtools.NewParams(",config=565,arch=arm,"), tileNumber)
	assert.NoError(t, err)
	_, err = s.writeTraceIDAndPostings(paramtools.NewParams(",config=8888,arch=arm64,"), tileNumber)
	assert.NoError(t, err)
	_, err = s.writeTraceIDAndPostings(paramtools.NewParams(",config=gpu,arch=x86_64,"), tileNumber)
	assert.NoError(t, err)

	ops, err := s.GetOrderedParamSet(ctx, 1)
	assert.NoError(t, err)
	expected := paramtools.ParamSet{
		"arch":   []string{"arm", "arm64", "x86", "x86_64"},
		"config": []string{"565", "8888", "gpu"},
	}
	assert.Equal(t, expected, ops.ParamSet)
	assert.Equal(t, []string{"arch", "config"}, ops.KeyOrder)
}

func testGetOrderedParamSet_Empty(t *testing.T, s *SQLTraceStore) {
	ctx := context.Background()

	// Test the empty case where there is no data in datastore.
	ops, err := s.GetOrderedParamSet(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, paramtools.ParamSet{}, ops.ParamSet)
}

func testCountIndices(t *testing.T, s *SQLTraceStore) {
	ctx := context.Background()

	const tileNumber types.TileNumber = 1
	// Now add some trace ids.
	_, err := s.writeTraceIDAndPostings(paramtools.NewParams(",config=8888,arch=x86,"), tileNumber)
	assert.NoError(t, err)
	_, err = s.writeTraceIDAndPostings(paramtools.NewParams(",config=565,arch=arm,"), tileNumber)
	assert.NoError(t, err)
	_, err = s.writeTraceIDAndPostings(paramtools.NewParams(",config=8888,arch=arm64,"), tileNumber)
	assert.NoError(t, err)
	_, err = s.writeTraceIDAndPostings(paramtools.NewParams(",config=gpu,arch=x86_64,"), tileNumber)
	assert.NoError(t, err)

	count, err := s.CountIndices(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(8), count)
}

func testCountIndices_Empty(t *testing.T, s *SQLTraceStore) {
	ctx := context.Background()

	// Test the empty case where there is no data in datastore.
	count, err := s.CountIndices(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func testGetSource(t *testing.T, s *SQLTraceStore) {
	populatedTestDB(t, s)
	ctx := context.Background()

	filename, err := s.GetSource(ctx, types.CommitNumber(2), ",arch=x86,config=8888,")
	assert.NoError(t, err)
	assert.Equal(t, "gs://perf-bucket/2020/02/08/12/testdata.json", filename)
}

func testGetSource_Empty(t *testing.T, s *SQLTraceStore) {
	ctx := context.Background()

	// Confirm the call works with an empty tracestore.
	filename, err := s.GetSource(ctx, types.CommitNumber(5), ",arch=x86,config=8888,")
	assert.Error(t, err)
	assert.Equal(t, "", filename)
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

func populatedTestDB(t *testing.T, store *SQLTraceStore) {
	traceNames := []paramtools.Params{
		{"config": "8888", "arch": "x86"},
		{"config": "565", "arch": "x86"},
	}
	err := store.WriteTraces(types.CommitNumber(1), traceNames,
		[]float32{1.5, 2.3},
		paramtools.ParamSet{}, // ParamSet is empty because WriteTraces doesn't use it in this impl.
		"gs://perf-bucket/2020/02/08/11/testdata.json",
		time.Time{}) // time is unused in this impl of TraceStore.
	require.NoError(t, err)
	err = store.WriteTraces(types.CommitNumber(2), traceNames,
		[]float32{2.5, 3.3},
		paramtools.ParamSet{}, // ParamSet is empty because WriteTraces doesn't use it in this impl.
		"gs://perf-bucket/2020/02/08/12/testdata.json",
		time.Time{}) // time is unused in this impl of TraceStore.
	require.NoError(t, err)
	err = store.WriteTraces(types.CommitNumber(8), traceNames,
		[]float32{3.5, 4.3},
		paramtools.ParamSet{}, // ParamSet is empty because WriteTraces doesn't use it in this impl.
		"gs://perf-bucket/2020/02/08/13/testdata.json",
		time.Time{}) // time is unused in this impl of TraceStore.
	require.NoError(t, err)
}

// subTestFunction is a func we will call to test one aspect of *SQLTraceStore.
type subTestFunction func(t *testing.T, s *SQLTraceStore)

// subTests are all the tests we have for *SQLTraceStore.
var subTests = map[string]subTestFunction{
	"testUpdateSourceFile":                                            testUpdateSourceFile,
	"testWriteTraceIDAndPostings":                                     testWriteTraceIDAndPostings,
	"testParamSetForTile":                                             testParamSetForTile,
	"testParamSetForTile_Empty":                                       testParamSetForTile_Empty,
	"testGetLatestTile":                                               testGetLatestTile,
	"testGetLatestTile_Empty":                                         testGetLatestTile_Empty,
	"testGetOrderedParamSet":                                          testGetOrderedParamSet,
	"testGetOrderedParamSet_Empty":                                    testGetOrderedParamSet_Empty,
	"testCountIndices":                                                testCountIndices,
	"testCountIndices_Empty":                                          testCountIndices_Empty,
	"testGetSource_Empty":                                             testGetSource_Empty,
	"testReadTraces":                                                  testReadTraces,
	"testWriteTraces_MultipleBatches_Success":                         testWriteTraces_MultipleBatches_Success,
	"testReadTraces_InvalidKey":                                       testReadTraces_InvalidKey,
	"testReadTraces_NoResults":                                        testReadTraces_NoResults,
	"testReadTraces_EmptyTileReturnsNoData":                           testReadTraces_EmptyTileReturnsNoData,
	"testQueryTracesIDOnlyByIndex_EmptyQueryReturnsError":             testQueryTracesIDOnlyByIndex_EmptyQueryReturnsError,
	"testQueryTracesIDOnlyByIndex_EmptyTileReturnsEmptyParamset":      testQueryTracesIDOnlyByIndex_EmptyTileReturnsEmptyParamset,
	"testQueryTracesIDOnlyByIndex_MatchesOneTrace":                    testQueryTracesIDOnlyByIndex_MatchesOneTrace,
	"testQueryTracesIDOnlyByIndex_MatchesTwoTraces":                   testQueryTracesIDOnlyByIndex_MatchesTwoTraces,
	"testQueryTracesByIndex_MatchesOneTrace":                          testQueryTracesByIndex_MatchesOneTrace,
	"testQueryTracesByIndex_MatchesOneTraceInTheSecondTile":           testQueryTracesByIndex_MatchesOneTraceInTheSecondTile,
	"testQueryTracesByIndex_MatchesTwoTraces":                         testQueryTracesByIndex_MatchesTwoTraces,
	"testQueryTracesByIndex_QueryHasUnknownParamReturnsNoError":       testQueryTracesByIndex_QueryHasUnknownParamReturnsNoError,
	"testQueryTracesByIndex_QueryAgainstTileWithNoDataReturnsNoError": testQueryTracesByIndex_QueryAgainstTileWithNoDataReturnsNoError,
	"testTraceCount":                                                  testTraceCount,
	"testGetSource":                                                   testGetSource,
}
