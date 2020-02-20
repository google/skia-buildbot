package sqlts

import (
	"context"
	"database/sql"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/types"
)

const e = vec32.MISSING_DATA_SENTINEL

type cleanup func()

func newTestDB(t *testing.T) (*SQLTraceStore, cleanup) {
	unittest.MediumTest(t)
	tmpfile, err := ioutil.TempFile("", "sqlts")
	assert.NoError(t, err)
	err = tmpfile.Close()
	assert.NoError(t, err)

	db, err := sql.Open("sqlite3", tmpfile.Name())
	assert.NoError(t, err)

	s, err := NewSQLite(db, SQLiteDialect, 8)
	assert.NoError(t, err)

	return s, func() {
		err := os.Remove(tmpfile.Name())
		assert.NoError(t, err)
	}
}

func newPopulatedTestDB(t *testing.T) (*SQLTraceStore, cleanup) {
	s, cleanup := newTestDB(t)

	traceNames := []paramtools.Params{
		{"config": "8888", "arch": "x86"},
		{"config": "565", "arch": "x86"},
	}
	err := s.WriteTraces(1, traceNames,
		[]float32{1.5, 2.3},
		paramtools.ParamSet{},
		"gs://perf-bucket/2020/02/08/11/testdata.json",
		time.Now())
	assert.NoError(t, err)
	err = s.WriteTraces(2, traceNames,
		[]float32{2.5, 3.3},
		paramtools.ParamSet{},
		"gs://perf-bucket/2020/02/08/12/testdata.json",
		time.Now())
	assert.NoError(t, err)
	err = s.WriteTraces(8, traceNames,
		[]float32{3.5, 4.3},
		paramtools.ParamSet{},
		"gs://perf-bucket/2020/02/08/13/testdata.json",
		time.Now())
	assert.NoError(t, err)

	return s, cleanup
}

func newBenchTestDB(b *testing.B) (*SQLTraceStore, cleanup) {
	tmpfile, _ := ioutil.TempFile("", "sqlts")
	_ = tmpfile.Close()
	db, _ := sql.Open("sqlite3", tmpfile.Name())
	s, _ := NewSQLite(db, SQLiteDialect, 8)

	return s, func() {
		_ = os.Remove(tmpfile.Name())
	}
}

func TestNewSQLite(t *testing.T) {
	s, cleanup := newTestDB(t)
	defer cleanup()

	err := s.WriteTraces(0, []paramtools.Params{
		{"config": "8888", "arch": "x86"},
		{"config": "565", "arch": "x86"},
	},
		[]float32{1.5, 2.3},
		paramtools.ParamSet{},
		"gs://perf-bucket/2020/02/08/11/testdata.json",
		time.Now())
	assert.NoError(t, err)
}

func TestUpdateSourceFile(t *testing.T) {
	s, cleanup := newTestDB(t)
	defer cleanup()

	// Do each update twice to ensure the IDs don't change.
	id, err := s.updateSourceFile("foo.txt")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), id)

	id, err = s.updateSourceFile("foo.txt")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), id)

	id, err = s.updateSourceFile("bar.txt")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), id)

	id, err = s.updateSourceFile("bar.txt")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), id)
}

func TestUpdateTraceID(t *testing.T) {
	s, cleanup := newTestDB(t)
	defer cleanup()

	p := paramtools.NewParams(",config=8888,arch=x86,")

	// Do each update twice to ensure the IDs don't change.
	traceID, err := s.updateTraceID(p, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), traceID)

	traceID, err = s.updateTraceID(p, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), traceID)

	p2 := paramtools.NewParams(",config=8888,arch=arm,")

	traceID, err = s.updateTraceID(p2, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), traceID)

	traceID, err = s.updateTraceID(p2, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), traceID)

	// Confirm the created postings are correct. Here we confirm the postings
	// for config=8888 are the trace ids for both traces, since they both
	// contain that key value pair.
	row, err := s.db.Query(
		`SELECT tile_number, key_value, trace_id FROM Postings
		 WHERE key_value="config=8888"`,
	)
	assert.NoError(t, err)
	count := 0
	for row.Next() {
		var tileNumber int64
		var keyValue string
		var traceID int64
		err := row.Scan(&tileNumber, &keyValue, &traceID)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), tileNumber)
		assert.Equal(t, "config=8888", keyValue)
		assert.Contains(t, []int64{1, 2}, traceID)
		count++
	}
	assert.Equal(t, 2, count)

	// Here we confirm the postings for arch=arm are the trace ids for just the
	// second trace, since it only appears in that trace name.
	row, err = s.db.Query(
		`SELECT tile_number, key_value, trace_id FROM Postings WHERE key_value="arch=arm"`,
	)
	assert.NoError(t, err)
	count = 0
	for row.Next() {
		var tileNumber int64
		var keyValue string
		var traceID int64
		err := row.Scan(&tileNumber, &keyValue, &traceID)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), tileNumber)
		assert.Equal(t, "arch=arm", keyValue)
		assert.Contains(t, []int64{2}, traceID)
		count++
	}
	assert.Equal(t, 1, count)
}

func TestReadTraces(t *testing.T) {
	s, cleanup := newPopulatedTestDB(t)
	defer cleanup()

	keys := []string{
		",arch=x86,config=8888,",
		",arch=x86,config=565,",
	}

	ts, err := s.ReadTraces(0, keys)
	assert.NoError(t, err)
	assert.Equal(t, ts, map[string][]float32{
		",arch=x86,config=565,":  {e, 2.3, 3.3, e, e, e, e, e},
		",arch=x86,config=8888,": {e, 1.5, 2.5, e, e, e, e, e},
	})
}

func TestReadTraces_empty_tile_returns_no_data(t *testing.T) {
	s, cleanup := newPopulatedTestDB(t)
	defer cleanup()

	keys := []string{
		",arch=x86,config=8888,",
		",arch=x86,config=565,",
	}

	// Reading from a tile we haven't written to should succeed and return no data.
	ts, err := s.ReadTraces(2, keys)
	assert.NoError(t, err)
	assert.Equal(t, ts, map[string][]float32{
		",arch=x86,config=565,":  {e, e, e, e, e, e, e, e},
		",arch=x86,config=8888,": {e, e, e, e, e, e, e, e},
	})
}

func paramSetFromParamsChan(ch <-chan paramtools.Params) paramtools.ParamSet {
	ret := paramtools.NewParamSet()
	for p := range ch {
		ret.AddParams(p)
	}
	ret.Normalize()
	return ret
}

func TestQueryTracesIDOnlyByIndex_empty_query_returns_error(t *testing.T) {
	s, cleanup := newPopulatedTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Query that matches one trace.
	q, err := query.NewFromString("")
	assert.NoError(t, err)
	_, err = s.QueryTracesIDOnlyByIndex(ctx, 5, q)
	assert.Error(t, err)
}

func TestQueryTracesIDOnlyByIndex_empty_tile_returns_empty_paramset(t *testing.T) {
	s, cleanup := newPopulatedTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Query that matches one trace.
	q, err := query.NewFromString("config=565")
	assert.NoError(t, err)
	ch, err := s.QueryTracesIDOnlyByIndex(ctx, 5, q)
	assert.NoError(t, err)
	assert.Equal(t, paramtools.ParamSet{}, paramSetFromParamsChan(ch))
}

func TestQueryTracesIDOnlyByIndex_matches_one_trace(t *testing.T) {
	s, cleanup := newPopulatedTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Query that matches one trace.
	q, err := query.NewFromString("config=565")
	assert.NoError(t, err)
	ch, err := s.QueryTracesIDOnlyByIndex(ctx, 0, q)
	assert.NoError(t, err)
	expected := paramtools.ParamSet{
		"arch":   []string{"x86"},
		"config": []string{"565"},
	}
	assert.Equal(t, expected, paramSetFromParamsChan(ch))
}

func TestQueryTracesIDOnlyByIndex_matches_two_traces(t *testing.T) {
	s, cleanup := newPopulatedTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Query that matches two traces.
	q, err := query.NewFromString("arch=x86")
	assert.NoError(t, err)
	ch, err := s.QueryTracesIDOnlyByIndex(ctx, 0, q)
	assert.NoError(t, err)
	expected := paramtools.ParamSet{
		"arch":   []string{"x86"},
		"config": []string{"565", "8888"},
	}
	assert.Equal(t, expected, paramSetFromParamsChan(ch))
}

func TestQueryTracesByIndex_matches_one_trace(t *testing.T) {
	s, cleanup := newPopulatedTestDB(t)
	defer cleanup()

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

func TestQueryTracesByIndex_matches_one_trace_in_the_second_tile(t *testing.T) {
	s, cleanup := newPopulatedTestDB(t)
	defer cleanup()

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

func TestQueryTracesByIndex_matches_two_traces(t *testing.T) {
	s, cleanup := newPopulatedTestDB(t)
	defer cleanup()

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

func TestQueryTracesByIndex_query_has_unknow_param_returns_no_error(t *testing.T) {
	s, cleanup := newPopulatedTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Query that has no matching params in the given tile.
	q, err := query.NewFromString("arch=unknown")
	assert.NoError(t, err)
	ts, err := s.QueryTracesByIndex(ctx, 0, q)
	assert.NoError(t, err)
	assert.Nil(t, ts)
}

func TestQueryTracesByIndex_query_against_tile_with_no_data_returns_no_erro(t *testing.T) {
	s, cleanup := newPopulatedTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Query that has no Postings for the given tile.
	q, err := query.NewFromString("arch=unknown")
	assert.NoError(t, err)
	ts, err := s.QueryTracesByIndex(ctx, 2, q)
	assert.NoError(t, err)
	assert.Nil(t, ts)
}

func TestTraceCount(t *testing.T) {
	s, cleanup := newPopulatedTestDB(t)
	defer cleanup()

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

func TestParamSetForTile(t *testing.T) {
	s, cleanup := newTestDB(t)
	defer cleanup()

	// Now add some trace ids.
	_, err := s.updateTraceID(paramtools.NewParams(",config=8888,arch=x86,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=565,arch=arm,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=8888,arch=arm64,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=gpu,arch=x86_64,"), 1)
	assert.NoError(t, err)

	ps, err := s.paramSetForTile(1)
	assert.NoError(t, err)
	expected := paramtools.ParamSet{
		"arch":   []string{"arm", "arm64", "x86", "x86_64"},
		"config": []string{"565", "8888", "gpu"},
	}
	assert.Equal(t, expected, ps)
}

func TestParamSetForTile_empty(t *testing.T) {
	s, cleanup := newTestDB(t)
	defer cleanup()

	// Test the empty case where there is no data in datastore.
	ps, err := s.paramSetForTile(1)
	assert.NoError(t, err)
	assert.Equal(t, paramtools.ParamSet{}, ps)
}

func TestGetLatestTile(t *testing.T) {
	s, cleanup := newTestDB(t)
	defer cleanup()

	// Now add some trace ids.
	_, err := s.updateTraceID(paramtools.NewParams(",config=8888,arch=x86,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=8888,arch=arm64,"), 5)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=gpu,arch=x86_64,"), 7)
	assert.NoError(t, err)

	tileNumber, err := s.GetLatestTile()
	assert.NoError(t, err)
	assert.Equal(t, types.TileNumber(7), tileNumber)
}

func TestGetLatestTile_empty(t *testing.T) {
	s, cleanup := newTestDB(t)
	defer cleanup()

	// Test the empty case where there is no data in datastore.
	tileNumber, err := s.GetLatestTile()
	assert.Error(t, err)
	assert.Equal(t, types.BadTileNumber, tileNumber)
}

func TestGetOrderedParamSet(t *testing.T) {
	s, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Now add some trace ids.
	_, err := s.updateTraceID(paramtools.NewParams(",config=8888,arch=x86,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=565,arch=arm,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=8888,arch=arm64,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=gpu,arch=x86_64,"), 1)
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

func TestGetOrderedParamSet_empty(t *testing.T) {
	s, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test the empty case where there is no data in datastore.
	ops, err := s.GetOrderedParamSet(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, paramtools.ParamSet{}, ops.ParamSet)
}

func TestCountIndices(t *testing.T) {
	s, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Now add some trace ids.
	_, err := s.updateTraceID(paramtools.NewParams(",config=8888,arch=x86,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=565,arch=arm,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=8888,arch=arm64,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=gpu,arch=x86_64,"), 1)
	assert.NoError(t, err)

	count, err := s.CountIndices(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(8), count)
}

func TestCountIndices_empty(t *testing.T) {
	s, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test the empty case where there is no data in datastore.
	count, err := s.CountIndices(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestGetSource(t *testing.T) {
	s, cleanup := newPopulatedTestDB(t)
	defer cleanup()

	ctx := context.Background()

	filename, err := s.GetSource(ctx, types.CommitNumber(2), ",arch=x86,config=8888,")
	assert.NoError(t, err)
	assert.Equal(t, "gs://perf-bucket/2020/02/08/12/testdata.json", filename)

}

func TestGetSource_empty(t *testing.T) {
	s, cleanup := newTestDB(t)
	defer cleanup()

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

func BenchmarkUpdateSourceFile(b *testing.B) {
	s, cleanup := newBenchTestDB(b)
	defer cleanup()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_, _ = s.updateSourceFile("foo.txt")
	}
}

func BenchmarkUpdateTraceID(b *testing.B) {
	s, cleanup := newBenchTestDB(b)
	p := paramtools.NewParams(",config=8888,arch=x86,")
	defer cleanup()

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_, _ = s.updateTraceID(p, 1)
	}
}

func BenchmarkReadTraces(b *testing.B) {
	s, cleanup := newBenchTestDB(b)
	defer cleanup()

	traceNames := []paramtools.Params{
		{"config": "8888", "arch": "x86"},
		{"config": "565", "arch": "x86"},
	}
	_ = s.WriteTraces(1, traceNames,
		[]float32{1.5, 2.3},
		paramtools.ParamSet{},
		"gs://perf-bucket/2020/02/08/11/testdata.json",
		time.Now())
	_ = s.WriteTraces(2, traceNames,
		[]float32{2.5, 3.3},
		paramtools.ParamSet{},
		"gs://perf-bucket/2020/02/08/12/testdata.json",
		time.Now())
	keys := make([]string, len(traceNames))
	for i, p := range traceNames {
		keys[i], _ = query.MakeKeyFast(p)
	}

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_, _ = s.ReadTraces(0, keys)
	}
}

func BenchmarkQueryTracesByIndex(b *testing.B) {
	s, cleanup := newBenchTestDB(b)
	defer cleanup()

	traceNames := []paramtools.Params{
		{"config": "8888", "arch": "x86"},
		{"config": "565", "arch": "x86"},
	}
	_ = s.WriteTraces(1, traceNames,
		[]float32{1.5, 2.3},
		paramtools.ParamSet{},
		"gs://perf-bucket/2020/02/08/11/testdata.json",
		time.Now())
	_ = s.WriteTraces(2, traceNames,
		[]float32{2.5, 3.3},
		paramtools.ParamSet{},
		"gs://perf-bucket/2020/02/08/12/testdata.json",
		time.Now())
	q, _ := query.NewFromString("config=565")
	ctx := context.Background()

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_, _ = s.QueryTracesByIndex(ctx, 0, q)
	}
}
