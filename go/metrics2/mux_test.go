package metrics2

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
)

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
	gc.Reset()
	assert.Equal(t, int64(0), gc.Get())

	gc.Inc(3)
	assert.NotZero(t, gc.Get())

	gc.Reset()
	assert.Equal(t, int64(0), gc.Get())
}

func TestClients(t *testing.T) {
	testutils.SmallTest(t)
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
