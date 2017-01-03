package metrics2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/prometheus/client_golang/prometheus"
)

func TestClean(t *testing.T) {
	assert.Equal(t, "a_b_c", clean("a.b-c"))
}

func getPromClient() *promClient {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	return newPromClient()
}

func TestInt64(t *testing.T) {
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

func testClient(t *testing.T, c Client) {
	// Int64Metric
	g := c.GetInt64Metric("a.b", map[string]string{"some_key": "some-value"})
	assert.NotNil(t, g)

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

	// Float64Metric
	gf := c.GetFloat64Metric("a.c", map[string]string{"some_key": "some-value"})
	assert.NotNil(t, gf)

	gf.Update(3)
	assert.Equal(t, float64(3), gf.Get())

	gf2 := c.GetFloat64Metric("a.c", map[string]string{"some_key": "some-new-value"})
	assert.NotNil(t, gf2)
	gf2.Update(4)

	assert.Equal(t, float64(3), gf.Get())
	assert.Equal(t, float64(4), gf2.Get())

	gf2 = c.GetFloat64Metric("a.c", map[string]string{"some_key": "some-new-value"})
	assert.Equal(t, float64(4), gf2.Get())

	// Metric with two tags.
	gf = c.GetFloat64Metric("float_metric_name", map[string]string{"a": "2", "b": "1"})
	assert.NotNil(t, gf)

	// Counter
	gc := c.GetCounter("c", map[string]string{"some_key": "some-value"})
	assert.NotNil(t, gc)

	gc.Inc(3)
	assert.Equal(t, int64(3), gc.Get())

	gc.Dec(2)
	assert.Equal(t, int64(1), gc.Get())

	gc.Reset()
	assert.Equal(t, int64(0), gc.Get())
}

func TestClients(t *testing.T) {
	// Test Prometheus client.
	var c Client = getPromClient()
	testClient(t, c)

	// Test Mux using a single Prometheus client.
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	var err error
	c, err = newMuxClient([]Client{c})
	assert.NoError(t, err)
	testClient(t, c)

	// Test Mux using two Prometheus clients.
	c = getPromClient()
	c, err = newMuxClient([]Client{c, c})
	assert.NoError(t, err)
	testClient(t, c)
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
			assert.Fail(t, "Should have panic'd by now.")
	*/
}
