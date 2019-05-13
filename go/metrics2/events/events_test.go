package events

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	bt_testutil "go.skia.org/infra/go/bt/testutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestAggregateMetric(t *testing.T) {
	unittest.LargeTest(t)

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	project, instance, cleanup := bt_testutil.SetupBigTable(t, BT_TABLE, BT_COLUMN_FAMILY)
	defer cleanup()
	db, err := NewBTEventDB(context.Background(), project, instance, nil)
	assert.NoError(t, err)
	m, err := NewEventMetrics(db, "test-metrics")
	assert.NoError(t, err)

	s := "my-events"
	now := time.Now()
	k1 := now.Add(-3 * time.Second)
	v1 := 0.05
	assert.NoError(t, m.db.Insert(&Event{
		Stream:    s,
		Timestamp: k1,
		Data:      encodeEvent(v1),
	}))

	period := 20 * time.Minute
	called := false
	assert.NoError(t, m.AggregateMetric(s, nil, period, func(vs []*Event) (float64, error) {
		called = true
		assert.Equal(t, 1, len(vs))
		assert.Equal(t, v1, decodeEvent(vs[0].Data))
		return 0.0, nil
	}))
	assert.False(t, called)
	assert.NoError(t, m.updateMetrics(now))
	assert.True(t, called)
}

func TestDynamicMetric(t *testing.T) {
	unittest.LargeTest(t)

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	project, instance, cleanup := bt_testutil.SetupBigTable(t, BT_TABLE, BT_COLUMN_FAMILY)
	defer cleanup()
	db, err := NewBTEventDB(context.Background(), project, instance, nil)
	assert.NoError(t, err)
	m, err := NewEventMetrics(db, "test-dynamic-metrics")
	assert.NoError(t, err)

	s := "my-events"
	now := time.Now()
	k := now.Add(-5 * 20 * time.Minute)
	for i := 0; i < 20; i++ {
		v := 0.05 * float64(i)
		assert.NoError(t, m.db.Insert(&Event{
			Stream:    s,
			Timestamp: k,
			Data:      encodeEvent(v),
		}))
		k = k.Add(5 * time.Minute)
	}

	period := 100 * time.Minute
	assert.NoError(t, m.DynamicMetric(s, nil, period, func(vs []*Event) ([]map[string]string, []float64, error) {
		tags := []map[string]string{}
		vals := []float64{}
		for _, e := range vs {
			t := map[string]string{}
			v := decodeEvent(e.Data)
			if v >= 0.5 {
				t["category"] = "large"
			} else {
				t["category"] = "small"
			}
			tags = append(tags, t)
			vals = append(vals, v)
		}
		return tags, vals, nil
	}))
	assert.NoError(t, m.updateMetrics(now))

	// Ensure that we got the right dynamic metrics.
	assert.Equal(t, 2, len(m.currentDynamicMetrics))

	// Wait for the "small" events to scroll off, ensure that we deleted the
	// old metric.
	t1 := now.Add(50 * time.Minute)
	assert.NoError(t, m.updateMetrics(t1))
	assert.Equal(t, 1, len(m.currentDynamicMetrics))
}
