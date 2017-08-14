package metrics2

import (
	"io/ioutil"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"

	assert "github.com/stretchr/testify/require"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestClean(t *testing.T) {
	testutils.SmallTest(t)
	assert.Equal(t, "a_b_c", clean("a.b-c"))
}

func getPromClient() *promClient {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	return newPromClient()
}

func get(t *testing.T, metric string) string {
	req := httptest.NewRequest("GET", "/metrics", nil)
	rw := httptest.NewRecorder()
	promhttp.HandlerFor(prometheus.DefaultRegisterer.(*prometheus.Registry), promhttp.HandlerOpts{
		ErrorLog:           nil,
		ErrorHandling:      promhttp.PanicOnError,
		DisableCompression: true,
	}).ServeHTTP(rw, req)
	resp := rw.Result()
	defer util.Close(resp.Body)
	b, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	for _, s := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(s, metric) {
			return strings.Split(s, " ")[1]
		}
	}
	return ""
}

func TestInt64(t *testing.T) {
	testutils.SmallTest(t)
	c := getPromClient()
	check := func(m Int64Metric, metric string, expect int64) {
		actual, err := strconv.ParseInt(get(t, metric), 10, 64)
		assert.NoError(t, err)
		assert.Equal(t, expect, actual)
		assert.Equal(t, m.Get(), expect)
	}
	g := c.GetInt64Metric("a.b", map[string]string{"some_key": "some-value"})
	assert.NotNil(t, g)
	assert.NotNil(t, c.int64GaugeVecs["a_b [some_key]"])
	assert.NotNil(t, c.int64Gauges["a_b-some_key-some-value"])
	assert.Nil(t, c.int64GaugeVecs["a.b"])
	check(g, "a_b{some_key=\"some-value\"}", 0)

	g.Update(3)
	check(g, "a_b{some_key=\"some-value\"}", 3)

	g2 := c.GetInt64Metric("a.b", map[string]string{"some_key": "some-new-value"})
	assert.NotNil(t, g2)
	g2.Update(4)

	check(g, "a_b{some_key=\"some-value\"}", 3)
	check(g2, "a_b{some_key=\"some-new-value\"}", 4)

	g2 = c.GetInt64Metric("a.b", map[string]string{"some_key": "some-new-value"})
	check(g2, "a_b{some_key=\"some-new-value\"}", 4)

	// Metric with two tags.
	g = c.GetInt64Metric("metric_name", map[string]string{"a": "2", "b": "1"})
	assert.NotNil(t, g)
	assert.NotNil(t, c.int64GaugeVecs["metric_name [a b]"])
	assert.NotNil(t, c.int64Gauges["metric_name-a-2-b-1"])
	check(g, "metric_name{a=\"2\",b=\"1\"}", 0)

	// Test delete.
	assert.NoError(t, g.Delete())
	assert.Equal(t, "", get(t, "a_c{some_key=\"some-new-value\"}"))
}

func TestFloat64(t *testing.T) {
	testutils.SmallTest(t)
	c := getPromClient()
	check := func(m Float64Metric, metric string, expect float64) {
		actual, err := strconv.ParseFloat(get(t, metric), 64)
		assert.NoError(t, err)
		assert.Equal(t, expect, actual)
		assert.Equal(t, m.Get(), expect)
	}
	g := c.GetFloat64Metric("a.c", map[string]string{"some_key": "some-value"})
	assert.NotNil(t, g)
	assert.NotNil(t, c.float64GaugeVecs["a_c [some_key]"])
	assert.NotNil(t, c.float64Gauges["a_c-some_key-some-value"])
	assert.Nil(t, c.float64GaugeVecs["a.c"])
	check(g, "a_c{some_key=\"some-value\"}", 0.0)

	g.Update(3)
	check(g, "a_c{some_key=\"some-value\"}", 3.0)

	g2 := c.GetFloat64Metric("a.c", map[string]string{"some_key": "some-new-value"})
	assert.NotNil(t, g2)
	g2.Update(4)

	check(g, "a_c{some_key=\"some-value\"}", 3.0)
	check(g2, "a_c{some_key=\"some-new-value\"}", 4.0)

	g2 = c.GetFloat64Metric("a.c", map[string]string{"some_key": "some-new-value"})
	check(g2, "a_c{some_key=\"some-new-value\"}", 4.0)

	// Metric with two tags.
	g = c.GetFloat64Metric("float_metric_name", map[string]string{"a": "2", "b": "1"})
	assert.NotNil(t, g)
	assert.NotNil(t, c.float64GaugeVecs["float_metric_name [a b]"])
	assert.NotNil(t, c.float64Gauges["float_metric_name-a-2-b-1"])
	check(g, "float_metric_name{a=\"2\",b=\"1\"}", 0.0)

	// Test delete.
	assert.NoError(t, g.Delete())
	assert.Equal(t, "", get(t, "float_metric_name{a=\"2\",b=\"1\"}"))
}

func TestCounter(t *testing.T) {
	testutils.SmallTest(t)
	c := getPromClient()
	check := func(m Counter, metric string, expect int64) {
		actual, err := strconv.ParseInt(get(t, metric), 10, 64)
		assert.NoError(t, err)
		assert.Equal(t, expect, actual)
		assert.Equal(t, m.Get(), expect)
	}
	g := c.GetCounter("c", map[string]string{"some_key": "some-value"})
	assert.NotNil(t, g)

	g.Inc(3)
	g = c.GetCounter("c", map[string]string{"some_key": "some-value"})
	check(g, "c{some_key=\"some-value\"}", 3)
	assert.Equal(t, int64(3), g.Get())

	g.Dec(2)
	check(g, "c{some_key=\"some-value\"}", 1)
	assert.Equal(t, int64(1), g.Get())

	g.Reset()
	check(g, "c{some_key=\"some-value\"}", 0)
	assert.Equal(t, int64(0), g.Get())

	// Test delete.
	assert.NoError(t, g.Delete())
	assert.Equal(t, "", get(t, "c{some_key=\"some-value\"}"))
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
