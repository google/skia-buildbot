package sqlts

import (
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
