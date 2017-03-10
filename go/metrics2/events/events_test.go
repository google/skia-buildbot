package events

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"math"
	"path"
	"testing"
	"time"

	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
)

func encodeEvent(v float64) []byte {
	rv := make([]byte, 8)
	binary.BigEndian.PutUint64(rv, math.Float64bits(v))
	return rv
}

func decodeEvent(b []byte) float64 {
	return math.Float64frombits(binary.BigEndian.Uint64(b))
}

func TestEncodeDecodeKey(t *testing.T) {
	testutils.SmallTest(t)

	tc := []time.Time{
		time.Unix(0, 0),
		time.Now(),
		time.Now().UTC(),
	}
	for _, c := range tc {
		enc, err := encodeKey(c)
		assert.NoError(t, err)
		dec, err := decodeKey(enc)
		assert.NoError(t, err, fmt.Sprintf("%s", c))
		assert.Equal(t, c.UTC(), dec.UTC())
	}

	// Errors.
	tc = []time.Time{
		time.Time{},
		time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC),
	}
	for _, c := range tc {
		_, err := encodeKey(c)
		assert.Error(t, err)
	}
}

func TestInsertRetrieve(t *testing.T) {
	testutils.SmallTest(t)

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	m, err := NewEventMetrics(path.Join(tmp, "events.bdb"))
	assert.NoError(t, err)

	e := m.GetEventStream("my-events")
	now := time.Now()
	k1 := now.Add(-3 * time.Second)
	v1 := 0.05
	assert.NoError(t, e.InsertAt(k1, encodeEvent(v1)))

	end := now.Add(time.Second)
	start := end.Add(-100 * time.Second)
	ts, vs, err := e.GetRange(start, end)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ts))
	assert.Equal(t, k1.UTC(), ts[0])
	assert.Equal(t, 1, len(vs))
	assert.Equal(t, v1, decodeEvent(vs[0]))
}

func TestAggregateMetric(t *testing.T) {
	testutils.SmallTest(t)

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	m, err := NewEventMetrics(path.Join(tmp, "events.bdb"))
	assert.NoError(t, err)

	e := m.GetEventStream("my-events")
	now := time.Now()
	k1 := now.Add(-3 * time.Second)
	v1 := 0.05
	assert.NoError(t, e.InsertAt(k1, encodeEvent(v1)))

	period := 20 * time.Minute
	called := false
	e.AggregateMetric("my-metric", nil, period, func(ts []time.Time, vs []Event) (float64, error) {
		called = true
		assert.Equal(t, 1, len(ts))
		assert.Equal(t, k1.UTC(), ts[0])
		assert.Equal(t, 1, len(vs))
		assert.Equal(t, v1, decodeEvent(vs[0]))
		return 0.0, nil
	})
	assert.False(t, called)
	assert.NoError(t, m.updateMetrics(now))
	assert.True(t, called)
}
