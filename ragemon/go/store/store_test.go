package store

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"testing"
	"time"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/ragemon/go/ts"

	"github.com/stretchr/testify/assert"
)

var (
	tmpDir = ""
)

func setupStoreDir(t *testing.T) {
	var err error
	tmpDir, err = ioutil.TempDir("", "ptracestore")
	assert.NoError(t, err)
}

func cleanup() {
	if err := os.RemoveAll(tmpDir); err != nil {
		fmt.Printf("Failed to clean up %q: %s", tmpDir, err)
	}
}

// queries runs a series of queries against the given store.
func queries(t *testing.T, now time.Time, m []Measurement, st *StoreImpl) {
	// Search in a narrow window that only gets 2 of the 3 points.
	matches := st.Match(now.Add(-time.Second), now.Add(time.Second), &query.Query{})
	assert.Equal(t, 2, len(matches))
	assert.Equal(t, []ts.Point{m[1].Point}, matches[",host=foo,metric=cpu,"].Points())
	// m is sorted when added so value=102 is actually in position 2.
	assert.Equal(t, []ts.Point{m[2].Point}, matches[",host=bar,metric=cpu,"].Points())

	// Widen the window to get all 3 points.
	matches = st.Match(now.Add(-time.Second), now.Add(2*time.Minute), &query.Query{})
	assert.Equal(t, 2, len(matches))
	assert.Equal(t, []ts.Point{m[1].Point}, matches[",host=foo,metric=cpu,"].Points())
	assert.Equal(t, []ts.Point{m[2].Point, m[3].Point}, matches[",host=bar,metric=cpu,"].Points())

	// Narrow the query to get just 1 timeseries.
	q, err := query.New(url.Values{"host": []string{"bar"}})
	assert.NoError(t, err)
	matches = st.Match(now.Add(-time.Second), now.Add(2*time.Minute), q)
	assert.Equal(t, 1, len(matches))
	assert.Equal(t, []ts.Point{m[2].Point, m[3].Point}, matches[",host=bar,metric=cpu,"].Points())

	// Narrow the query to get 0 timeseries.
	q, err = query.New(url.Values{"host": []string{"quux"}})
	matches = st.Match(now.Add(-time.Second), now.Add(2*time.Minute), q)
	assert.Equal(t, 0, len(matches))

	// Narrow the time window to get 0 points.
	matches = st.Match(now.Add(2*time.Minute), now.Add(3*time.Minute), &query.Query{})
	assert.Equal(t, 0, len(matches))
}

func TestNew(t *testing.T) {
	testutils.MediumTest(t)
	setupStoreDir(t)
	defer cleanup()

	st, err := New(tmpDir)
	assert.NoError(t, err)
	assert.Equal(t, 3, st.cache.Len())

	tileIndices := st.tileIndices()
	assert.Equal(t, 3, len(tileIndices))

	now := time.Now()
	m := []Measurement{
		Measurement{
			Key: "", // An invalid key, should be ignored.
			Point: ts.Point{
				Timestamp: now.Unix(),
				Value:     101,
			},
		},
		Measurement{
			Key: ",host=foo,metric=cpu,",
			Point: ts.Point{
				Timestamp: now.Unix(),
				Value:     10,
			},
		},
		Measurement{
			Key: ",host=bar,metric=cpu,",
			Point: ts.Point{
				Timestamp: now.Add(time.Minute).Unix(),
				Value:     103,
			},
		},
		Measurement{
			Key: ",host=bar,metric=cpu,",
			Point: ts.Point{
				Timestamp: now.Unix(),
				Value:     102,
			},
		},
	}
	err = st.Add(m)
	assert.NoError(t, err)
	expectedParamSet := paramtools.ParamSet{
		"host":   []string{"foo", "bar"},
		"metric": []string{"cpu"},
	}
	assert.Equal(t, expectedParamSet, st.ParamSet())

	queries(t, now, m, st)

	// Flush to disk.
	errors := st.oneStep(now)
	assert.Equal(t, 0, len(errors))

	// Test tile rotation by jumping ahead in time 2 hours.
	tileIndices = st.tileIndices()
	assert.Equal(t, 3, len(tileIndices))
	errors = st.oneStep(now.Add(120 * time.Minute))
	assert.Equal(t, 0, len(errors))
	tileIndicesAfter := st.tileIndices()
	assert.Equal(t, 3, len(tileIndicesAfter))
	assert.NotEqual(t, tileIndices, tileIndicesAfter)

	// Re-run the queries.
	queries(t, now, m, st)

	// Now purge the lru cache and re-run the queries.
	st.cache.Purge()
	queries(t, now, m, st)

	// Now purge the lru cache and create a new StoreImpl and re-run the queries.
	st.cache.Purge()
	st, err = New(tmpDir)
	assert.NoError(t, err)
	queries(t, now, m, st)

	// Try adding a point that is out of range, i.e. too old.
	m = []Measurement{
		Measurement{
			Key: ",host=bar,metric=cpu,",
			Point: ts.Point{
				Timestamp: now.Add(-time.Hour * 24).Unix(),
				Value:     101,
			},
		},
	}
	err = st.Add(m)
	assert.Error(t, err)

}
