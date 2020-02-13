package sqlts

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
)

type cleanup func()

func newTestDB(t *testing.T) (*SQLTraceStore, cleanup) {
	unittest.SmallTest(t)
	tmpfile, err := ioutil.TempFile("", "sqlts")
	assert.NoError(t, err)
	err = tmpfile.Close()
	assert.NoError(t, err)
	s, err := NewSQLite(tmpfile.Name(), 8)
	assert.NoError(t, err)

	return s, func() {
		os.Remove(tmpfile.Name())
	}
}

func newBenchTestDB(b *testing.B) (*SQLTraceStore, cleanup) {
	tmpfile, _ := ioutil.TempFile("", "sqlts")
	_ = tmpfile.Close()
	s, _ := NewSQLite(tmpfile.Name(), 8)

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
		paramtools.ParamSet{
			"config": []string{"8888", "565"},
			"arch":   []string{"x86"},
		},
		"gs://perf-bucket/2020/02/08/11/testdata.json",
		time.Now())
	assert.NoError(t, err)
}

func TestUpdateSourceFile(t *testing.T) {
	s, cleanup := newTestDB(t)
	defer cleanup()

	id, err := s.updateSourceFile("foo.txt")
	assert.NoError(t, err)
	assert.Equal(t, id, int64(1))

	id, err = s.updateSourceFile("foo.txt")
	assert.NoError(t, err)
	assert.Equal(t, id, int64(1))

	id, err = s.updateSourceFile("bar.txt")
	assert.NoError(t, err)
	assert.Equal(t, id, int64(2))

	id, err = s.updateSourceFile("bar.txt")
	assert.NoError(t, err)
	assert.Equal(t, id, int64(2))
}

func BenchmarkUpdateSourceFile(b *testing.B) {
	s, cleanup := newBenchTestDB(b)
	defer cleanup()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_, _ = s.updateSourceFile("foo.txt")
	}
}
