package events

import (
	"fmt"
	"io/ioutil"
	"math"
	"path"
	"testing"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
)

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

func TestEncodeDecodeValue(t *testing.T) {
	testutils.SmallTest(t)

	tc := []float64{
		0.0,
		1.0,
		1.1,
		math.MaxFloat64,
		math.SmallestNonzeroFloat64,
	}
	for _, c := range tc {
		assert.Equal(t, c, decodeValue(encodeValue(c)))
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
	assert.NoError(t, e.InsertAt(k1, v1))

	end := now.Add(time.Second)
	start := end.Add(-100 * time.Second)
	ts, vs, err := e.GetRange(start, end)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ts))
	assert.Equal(t, k1.UTC(), ts[0])
	assert.Equal(t, 1, len(vs))
	assert.Equal(t, v1, vs[0])
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
	assert.NoError(t, e.InsertAt(k1, v1))

	period := 20 * time.Minute
	called := false
	e.AggregateMetric("my-metric", nil, period, func(ts []time.Time, vs []float64) (float64, error) {
		called = true
		assert.Equal(t, 1, len(ts))
		assert.Equal(t, k1.UTC(), ts[0])
		assert.Equal(t, 1, len(vs))
		assert.Equal(t, v1, vs[0])
		return 0.0, nil
	})
	assert.False(t, called)
	assert.NoError(t, m.updateMetrics(now))
	assert.True(t, called)
}

func TestMeanMetric(t *testing.T) {
	testutils.SmallTest(t)

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	m, err := NewEventMetrics(path.Join(tmp, "events.bdb"))
	assert.NoError(t, err)

	e := m.GetEventStream("my-events")
	e.MeanMetric("mean", nil, 24*time.Hour)

	now := time.Now()

	assert.NoError(t, e.InsertAt(now.Add(-10*time.Second), 0.5))
	assert.NoError(t, m.updateMetrics(now))
	assert.Equal(t, 0.5, metrics2.GetFloat64Metric("mean", nil).Get())

	assert.NoError(t, e.InsertAt(now.Add(-9*time.Second), 0.0))
	assert.NoError(t, m.updateMetrics(now))
	assert.Equal(t, 0.25, metrics2.GetFloat64Metric("mean", nil).Get())

	assert.NoError(t, e.InsertAt(now.Add(-8*time.Second), 1.0))
	assert.NoError(t, m.updateMetrics(now))
	assert.Equal(t, 0.5, metrics2.GetFloat64Metric("mean", nil).Get())

	// Replace a data point.
	assert.NoError(t, e.InsertAt(now.Add(-8*time.Second), -0.5))
	assert.NoError(t, m.updateMetrics(now))
	assert.Equal(t, 0.0, metrics2.GetFloat64Metric("mean", nil).Get())
}
