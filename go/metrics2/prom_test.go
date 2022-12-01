package metrics2

import (
	"strconv"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	metrics_util "go.skia.org/infra/go/metrics2/testutils"
)

func TestClean(t *testing.T) {
	require.Equal(t, "a_b_c", clean("a.b-c"))
}

// getPromClient creates a fresh Prometheus Registry and
// a fresh Prometheus Client. This wipes out all previous metrics.
func getPromClient() *promClient {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	return NewPromClient()
}

func TestInt64(t *testing.T) {
	c := getPromClient()
	check := func(m Int64Metric, metric string, tags map[string]string, expect float64) {
		actual, err := strconv.ParseFloat(metrics_util.GetRecordedMetric(t, metric, tags), 64)
		require.NoError(t, err)
		require.Equal(t, expect, actual)
		require.Equal(t, float64(m.Get()), expect)
	}
	labels := map[string]string{"some_key": "some-value"}
	g := c.GetInt64Metric("a.b", labels)
	require.NotNil(t, g)
	require.NotNil(t, c.int64GaugeVecs["a_b [some_key]"])
	require.NotNil(t, c.int64Gauges["a_b-some_key-some-value"])
	require.Nil(t, c.int64GaugeVecs["a.b"])
	check(g, "a_b", labels, 0)

	g.Update(3)
	check(g, "a_b", labels, 3)

	labels2 := map[string]string{"some_key": "some-new-value"}
	g2 := c.GetInt64Metric("a.b", labels2)
	require.NotNil(t, g2)
	g2.Update(4)

	check(g, "a_b", labels, 3)
	check(g2, "a_b", labels2, 4)

	g2 = c.GetInt64Metric("a.b", labels2)
	check(g2, "a_b", labels2, 4)

	// Metric with two tags.
	labels = map[string]string{"b": "1", "a": "2"}
	g = c.GetInt64Metric("metric_name", labels)
	require.NotNil(t, g)
	require.NotNil(t, c.int64GaugeVecs["metric_name [a b]"])
	require.NotNil(t, c.int64Gauges["metric_name-a-2-b-1"])
	check(g, "metric_name", labels, 0)

	// Test delete.
	require.NoError(t, g.Delete())
	require.Equal(t, `Could not find anything for metric_name{a="2",b="1"}`, metrics_util.GetRecordedMetric(t, "metric_name", labels))
}

func TestFloat64(t *testing.T) {
	c := getPromClient()
	check := func(m Float64Metric, metric string, tags map[string]string, expect float64) {
		actual, err := strconv.ParseFloat(metrics_util.GetRecordedMetric(t, metric, tags), 64)
		require.NoError(t, err)
		require.Equal(t, expect, actual)
		require.Equal(t, m.Get(), expect)
	}
	labels := map[string]string{"some_key": "some-value"}
	g := c.GetFloat64Metric("a.c", labels)
	require.NotNil(t, g)
	require.NotNil(t, c.float64GaugeVecs["a_c [some_key]"])
	require.NotNil(t, c.float64Gauges["a_c-some_key-some-value"])
	require.Nil(t, c.float64GaugeVecs["a.c"])
	check(g, "a_c", labels, 0.0)

	g.Update(3)
	check(g, "a_c", labels, 3.0)

	labels2 := map[string]string{"some_key": "some-new-value"}
	g2 := c.GetFloat64Metric("a.c", labels2)
	require.NotNil(t, g2)
	g2.Update(4)

	check(g, "a_c", labels, 3.0)
	check(g2, "a_c", labels2, 4.0)

	g2 = c.GetFloat64Metric("a.c", labels2)
	check(g2, "a_c", labels2, 4.0)

	// Metric with two tags.
	labels = map[string]string{"a": "2", "b": "1"}
	g = c.GetFloat64Metric("float_metric_name", labels)
	require.NotNil(t, g)
	require.NotNil(t, c.float64GaugeVecs["float_metric_name [a b]"])
	require.NotNil(t, c.float64Gauges["float_metric_name-a-2-b-1"])
	check(g, "float_metric_name", labels, 0.0)

	// Test delete.
	require.NoError(t, g.Delete())
	require.Equal(t, `Could not find anything for float_metric_name{a="2",b="1"}`, metrics_util.GetRecordedMetric(t, "float_metric_name", labels))
}

func TestCounter(t *testing.T) {
	c := getPromClient()
	check := func(m Counter, metric string, tags map[string]string, expect float64) {
		actual, err := strconv.ParseFloat(metrics_util.GetRecordedMetric(t, metric, tags), 64)
		require.NoError(t, err)
		require.Equal(t, expect, actual)
		require.Equal(t, float64(m.Get()), expect)
	}
	labels := map[string]string{"some_key": "some-value"}
	g := c.GetCounter("c", labels)
	require.NotNil(t, g)

	g.Inc(3)
	g = c.GetCounter("c", labels)
	check(g, "c", labels, 3)
	require.Equal(t, int64(3), g.Get())

	g.Dec(2)
	check(g, "c", labels, 1)
	require.Equal(t, int64(1), g.Get())

	g.Reset()
	check(g, "c", labels, 0)
	require.Equal(t, int64(0), g.Get())

	// Test delete.
	require.NoError(t, g.Delete())
	require.Equal(t, `Could not find anything for c{some_key="some-value"}`, metrics_util.GetRecordedMetric(t, "c", labels))
}

func TestBool(t *testing.T) {
	c := getPromClient()
	labels := map[string]string{"some_key": "some-value"}
	g := c.GetBoolMetric("c", labels)
	require.NotNil(t, g)

	g.Update(true)
	g = c.GetBoolMetric("c", labels)
	require.Equal(t, true, g.Get())

	g.Update(false)
	require.Equal(t, false, g.Get())

	// Test delete.
	require.NoError(t, g.Delete())
	require.Equal(t, `Could not find anything for c{some_key="some-value"}`, metrics_util.GetRecordedMetric(t, "c", labels))
}

func TestPanicOn(t *testing.T) {
	/*
		  We need a sklog stand-in that just panics on Fatal*.

			defer func() {
				if r := recover(); r != nil {
					fmt.Println("Recovered in f", r)
				}
			}()
			p := newPromClient()
			_ = p.GetInt64Metric("a.b", map[string]string{"some_key": "some-value"})
			_ = p.GetInt64Metric("a.b", map[string]string{"some_key": "some-new-value", "wrong_number_of_keys": "2"})
			require.Fail(t, "Should have panic'd by now.")
	*/
}
