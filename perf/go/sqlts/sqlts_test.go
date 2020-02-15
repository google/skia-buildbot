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
	unittest.SmallTest(t)
	tmpfile, err := ioutil.TempFile("", "sqlts")
	assert.NoError(t, err)
	err = tmpfile.Close()
	assert.NoError(t, err)

	db, err := sql.Open("sqlite3", tmpfile.Name())
	assert.NoError(t, err)

	s, err := NewSQLite(db, SQLiteDialect, 8)
	assert.NoError(t, err)

	return s, func() {
		os.Remove(tmpfile.Name())
	}
}

func newBenchTestDB(b *testing.B) (*SQLTraceStore, cleanup) {
	tmpfile, _ := ioutil.TempFile("", "sqlts")
	_ = tmpfile.Close()
	db, _ := sql.Open("sqlite3", tmpfile.Name())
	s, _ := NewSQLite(db, SQLiteDialect, 8)

	return s, func() {
		os.Remove(tmpfile.Name())
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
		row.Scan(&tileNumber, &keyValue, &traceID)
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
		row.Scan(&tileNumber, &keyValue, &traceID)
		assert.Equal(t, int64(1), tileNumber)
		assert.Equal(t, "arch=arm", keyValue)
		assert.Contains(t, []int64{2}, traceID)
		count++
	}
	assert.Equal(t, 1, count)
}

func TestReadTraces(t *testing.T) {
	s, cleanup := newTestDB(t)
	defer cleanup()

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

	keys := make([]string, len(traceNames))
	for i, p := range traceNames {
		keys[i], err = query.MakeKeyFast(p)
		assert.NoError(t, err)
	}

	ts, err := s.ReadTraces(0, keys)
	assert.NoError(t, err)
	assert.Equal(t, ts, map[string][]float32{
		",arch=x86,config=565,":  {e, 2.3, 3.3, e, e, e, e, e},
		",arch=x86,config=8888,": {e, 1.5, 2.5, e, e, e, e, e},
	})

	// Reading from a tile we haven't written to should succeed and return no data.
	ts, err = s.ReadTraces(2, keys)
	assert.NoError(t, err)
	assert.Equal(t, ts, map[string][]float32{
		",arch=x86,config=565,":  {e, e, e, e, e, e, e, e},
		",arch=x86,config=8888,": {e, e, e, e, e, e, e, e},
	})
}

func TestQueryTracesByIndex(t *testing.T) {
	s, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()

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

	q, err := query.NewFromString("config=565")
	assert.NoError(t, err)
	ts, err := s.QueryTracesByIndex(ctx, 0, q)
	assert.NoError(t, err)
	assert.Equal(t, ts, types.TraceSet{
		",arch=x86,config=565,": {e, 2.3, 3.3, e, e, e, e, e},
	})

	q, err = query.NewFromString("arch=x86")
	assert.NoError(t, err)
	ts, err = s.QueryTracesByIndex(ctx, 0, q)
	assert.NoError(t, err)
	assert.Equal(t, ts, types.TraceSet{
		",arch=x86,config=565,":  {e, 2.3, 3.3, e, e, e, e, e},
		",arch=x86,config=8888,": {e, 1.5, 2.5, e, e, e, e, e},
	})

	// Reading from a tile with no matching params.
	q, err = query.NewFromString("arch=unknown")
	assert.NoError(t, err)
	ts, err = s.QueryTracesByIndex(ctx, 0, q)
	assert.NoError(t, err)
	assert.Nil(t, ts)

}

func TestParamSetForTile(t *testing.T) {
	unittest.SmallTest(t)

	s, cleanup := newTestDB(t)
	defer cleanup()

	// Test the empty case where there is no data in datastore.
	ps, err := s.paramSetForTile(1)
	assert.NoError(t, err)
	assert.Equal(t, paramtools.ParamSet{}, ps)

	// Now add some trace ids.
	_, err = s.updateTraceID(paramtools.NewParams(",config=8888,arch=x86,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=565,arch=arm,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=8888,arch=arm64,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=gpu,arch=x86_64,"), 1)
	assert.NoError(t, err)

	ps, err = s.paramSetForTile(1)
	assert.NoError(t, err)
	expected := paramtools.ParamSet{
		"arch":   []string{"arm", "arm64", "x86", "x86_64"},
		"config": []string{"565", "8888", "gpu"},
	}
	assert.Equal(t, expected, ps)
}

func TestGetLatestTile(t *testing.T) {
	unittest.SmallTest(t)

	s, cleanup := newTestDB(t)
	defer cleanup()

	// Test the empty case where there is no data in datastore.
	tileNumber, err := s.GetLatestTile()
	assert.Error(t, err)
	assert.Equal(t, types.BadTileNumber, tileNumber)

	// Now add some trace ids.
	_, err = s.updateTraceID(paramtools.NewParams(",config=8888,arch=x86,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=565,arch=arm,"), 2)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=8888,arch=arm64,"), 5)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=gpu,arch=x86_64,"), 7)
	assert.NoError(t, err)

	tileNumber, err = s.GetLatestTile()
	assert.NoError(t, err)
	assert.Equal(t, types.TileNumber(7), tileNumber)
}

func TestGetOrderedParamSet(t *testing.T) {
	unittest.SmallTest(t)

	s, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test the empty case where there is no data in datastore.
	ops, err := s.GetOrderedParamSet(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, paramtools.ParamSet{}, ops.ParamSet)

	// Now add some trace ids.
	_, err = s.updateTraceID(paramtools.NewParams(",config=8888,arch=x86,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=565,arch=arm,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=8888,arch=arm64,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=gpu,arch=x86_64,"), 1)
	assert.NoError(t, err)

	ops, err = s.GetOrderedParamSet(ctx, 1)
	assert.NoError(t, err)
	expected := paramtools.ParamSet{
		"arch":   []string{"arm", "arm64", "x86", "x86_64"},
		"config": []string{"565", "8888", "gpu"},
	}
	assert.Equal(t, expected, ops.ParamSet)
	assert.Equal(t, []string{"arch", "config"}, ops.KeyOrder)
}

func TestCountIndices(t *testing.T) {
	unittest.SmallTest(t)

	s, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test the empty case where there is no data in datastore.
	count, err := s.CountIndices(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Now add some trace ids.
	_, err = s.updateTraceID(paramtools.NewParams(",config=8888,arch=x86,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=565,arch=arm,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=8888,arch=arm64,"), 1)
	assert.NoError(t, err)
	_, err = s.updateTraceID(paramtools.NewParams(",config=gpu,arch=x86_64,"), 1)
	assert.NoError(t, err)

	count, err = s.CountIndices(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(8), count)
}

func TestGetSource(t *testing.T) {
	unittest.SmallTest(t)

	s, cleanup := newTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Confirm the call works with an empty tracestore.
	filename, err := s.GetSource(ctx, types.CommitNumber(2), ",arch=x86,config=8888,")
	assert.Error(t, err)
	assert.Equal(t, "", filename)

	traceNames := []paramtools.Params{
		{"config": "8888", "arch": "x86"},
		{"config": "565", "arch": "x86"},
	}
	err = s.WriteTraces(1, traceNames,
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

	filename, err = s.GetSource(ctx, types.CommitNumber(2), ",arch=x86,config=8888,")
	assert.NoError(t, err)
	assert.Equal(t, "gs://perf-bucket/2020/02/08/12/testdata.json", filename)

	filename, err = s.GetSource(ctx, types.CommitNumber(1), ",arch=x86,config=8888,")
	assert.NoError(t, err)
	assert.Equal(t, "gs://perf-bucket/2020/02/08/11/testdata.json", filename)
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
