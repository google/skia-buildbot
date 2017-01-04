package metrics2

import (
	"testing"

	"go.skia.org/infra/go/testutils"

	"github.com/stretchr/testify/assert"

	"github.com/prometheus/client_golang/prometheus"
)

func TestClean(t *testing.T) {
	testutils.SmallTest(t)
	assert.Equal(t, "a_b_c", clean("a.b-c"))
}

func getPromClient() *promClient {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	return newPromClient()
}

func TestInt64(t *testing.T) {
	testutils.SmallTest(t)
	c := getPromClient()
	g := c.GetInt64Metric("a.b", map[string]string{"some_key": "some-value"})
	assert.NotNil(t, g)
	assert.NotNil(t, c.int64GaugeVecs["a_b [some_key]"])
	assert.NotNil(t, c.int64Gauges["a_b-some_key-some-value"])
	assert.Nil(t, c.int64GaugeVecs["a.b"])

	g.Update(3)
	assert.Equal(t, int64(3), g.Get())

	g2 := c.GetInt64Metric("a.b", map[string]string{"some_key": "some-new-value"})
	assert.NotNil(t, g2)
	g2.Update(4)

	assert.Equal(t, int64(3), g.Get())
	assert.Equal(t, int64(4), g2.Get())

	g2 = c.GetInt64Metric("a.b", map[string]string{"some_key": "some-new-value"})
	assert.Equal(t, int64(4), g2.Get())

	// Metric with two tags.
	g = c.GetInt64Metric("metric_name", map[string]string{"a": "2", "b": "1"})
	assert.NotNil(t, g)
	assert.NotNil(t, c.int64GaugeVecs["metric_name [a b]"])
	assert.NotNil(t, c.int64Gauges["metric_name-a-2-b-1"])
}

func TestFloat64(t *testing.T) {
	testutils.SmallTest(t)
	c := getPromClient()
	g := c.GetFloat64Metric("a.c", map[string]string{"some_key": "some-value"})
	assert.NotNil(t, g)
	assert.NotNil(t, c.float64GaugeVecs["a_c [some_key]"])
	assert.NotNil(t, c.float64Gauges["a_c-some_key-some-value"])
	assert.Nil(t, c.float64GaugeVecs["a.c"])

	g.Update(3)
	assert.Equal(t, float64(3), g.Get())

	g2 := c.GetFloat64Metric("a.c", map[string]string{"some_key": "some-new-value"})
	assert.NotNil(t, g2)
	g2.Update(4)

	assert.Equal(t, float64(3), g.Get())
	assert.Equal(t, float64(4), g2.Get())

	g2 = c.GetFloat64Metric("a.c", map[string]string{"some_key": "some-new-value"})
	assert.Equal(t, float64(4), g2.Get())

	// Metric with two tags.
	g = c.GetFloat64Metric("float_metric_name", map[string]string{"a": "2", "b": "1"})
	assert.NotNil(t, g)
	assert.NotNil(t, c.float64GaugeVecs["float_metric_name [a b]"])
	assert.NotNil(t, c.float64Gauges["float_metric_name-a-2-b-1"])
}

func TestCounter(t *testing.T) {
	testutils.SmallTest(t)
	c := getPromClient()
	g := c.GetCounter("c", map[string]string{"some_key": "some-value"})
	assert.NotNil(t, g)

	g.Inc(3)
	assert.Equal(t, int64(3), g.Get())

	g.Dec(2)
	assert.Equal(t, int64(1), g.Get())

	g.Reset()
	assert.Equal(t, int64(0), g.Get())
}

func TestPanicOn(t *testing.T) {
	testutils.SmallTest(t)
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
			assert.Fail(t, "Should have panic'd by now.")
	*/
}
