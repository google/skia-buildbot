package events

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	bt_testutil "go.skia.org/infra/go/bt/testutil"
	"go.skia.org/infra/go/testutils"
)

func TestAggregateMetric(t *testing.T) {

	tmp, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	project, instance, cleanup := bt_testutil.SetupBigTable(t, BT_TABLE, BT_COLUMN_FAMILY)
	defer cleanup()
	db, err := NewBTEventDB(context.Background(), project, instance, nil)
	require.NoError(t, err)
	m, err := NewEventMetrics(db, "test-metrics")
	require.NoError(t, err)

	s := "my-events"
	now := time.Now()
	k1 := now.Add(-3 * time.Second)
	v1 := 0.05
	require.NoError(t, m.db.Insert(&Event{
		Stream:    s,
		Timestamp: k1,
		Data:      encodeEvent(v1),
	}))

	period := 20 * time.Minute
	called := false
	require.NoError(t, m.AggregateMetric(s, nil, period, func(vs []*Event) (float64, error) {
		called = true
		require.Equal(t, 1, len(vs))
		require.Equal(t, v1, decodeEvent(vs[0].Data))
		return 0.0, nil
	}))
	require.False(t, called)
	require.NoError(t, m.updateMetrics(now))
	require.True(t, called)
}

func TestDynamicMetric(t *testing.T) {

	tmp, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	project, instance, cleanup := bt_testutil.SetupBigTable(t, BT_TABLE, BT_COLUMN_FAMILY)
	defer cleanup()
	db, err := NewBTEventDB(context.Background(), project, instance, nil)
	require.NoError(t, err)
	m, err := NewEventMetrics(db, "test-dynamic-metrics")
	require.NoError(t, err)

	s := "my-events"
	now := time.Now()
	k := now.Add(-5 * 20 * time.Minute)
	for i := 0; i < 20; i++ {
		v := 0.05 * float64(i)
		require.NoError(t, m.db.Insert(&Event{
			Stream:    s,
			Timestamp: k,
			Data:      encodeEvent(v),
		}))
		k = k.Add(5 * time.Minute)
	}

	period := 100 * time.Minute
	require.NoError(t, m.DynamicMetric(s, nil, period, func(vs []*Event) ([]map[string]string, []float64, error) {
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
	require.NoError(t, m.updateMetrics(now))

	// Ensure that we got the right dynamic metrics.
	require.Equal(t, 2, len(m.currentDynamicMetrics))

	// Wait for the "small" events to scroll off, ensure that we deleted the
	// old metric.
	t1 := now.Add(50 * time.Minute)
	require.NoError(t, m.updateMetrics(t1))
	require.Equal(t, 1, len(m.currentDynamicMetrics))
}

func TestMean(t *testing.T) {
	test := func(name string, input []float64, expect float64) {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, expect, mean(input))
		})
	}
	test("empty", []float64{}, 0.0)
	test("singleValue", []float64{1.0}, 1.0)
	test("multipleValues", []float64{1.0, 2.0, 3.0, 4.0}, 2.5)
}

func TestVariance(t *testing.T) {
	test := func(name string, input []float64, expect float64) {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, expect, variance(input))
		})
	}
	test("empty", []float64{}, 0.0)
	test("singleValue", []float64{1.0}, 0.0)
	test("multipleValues", []float64{1.0, 2.0, 3.0, 4.0}, 1.25)
}

func TestPercentile(t *testing.T) {
	test := func(name string, input []float64, percent, expect float64) {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, expect, percentile(input, percent))
		})
	}
	test("empty", []float64{}, 0.5, 0.0)
	test("singleValue", []float64{1.0}, 0.5, 1.0)
	test("twoValues", []float64{1.0, 2.0}, 0.5, 1.5)
	test("fourValues", []float64{1.0, 2.0, 3.0, 4.0}, 0.5, 2.5)
	test("fiveValues", []float64{1.0, 2.0, 3.0, 4.0, 5.0}, 0.5, 3.0)
	test("tenValues_fiftieth", []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}, 0.5, 5.5)
	test("tenValues_ninetieth", []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}, 0.9, 9.1)
	test("tenValues_fifth", []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}, 0.05, 1.45)
}
