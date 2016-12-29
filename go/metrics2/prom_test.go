package metrics2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClean(t *testing.T) {
	assert.Equal(t, "a_b_c", clean("a.b-c"))
}

func TestInt64(t *testing.T) {
	p := newPromClient()
	g := p.GetInt64Metric("a.b", map[string]string{"some_key": "some-value"})
	assert.NotNil(t, g)
	assert.NotNil(t, p.int64GaugeVecs["a_b [some_key]"])
	assert.NotNil(t, p.int64Gauges["a_b-some_key-some-value"])
	assert.Nil(t, p.int64GaugeVecs["a.b"])

	g.Update(3)
	assert.Equal(t, int64(3), g.Get())

	g2 := p.GetInt64Metric("a.b", map[string]string{"some_key": "some-new-value"})
	assert.NotNil(t, g2)
	g2.Update(4)

	assert.Equal(t, int64(3), g.Get())
	assert.Equal(t, int64(4), g2.Get())

	g2 = p.GetInt64Metric("a.b", map[string]string{"some_key": "some-new-value"})
	assert.Equal(t, int64(4), g2.Get())

	// Metric with two tags.
	g = p.GetInt64Metric("metric_name", map[string]string{"a": "2", "b": "1"})
	assert.NotNil(t, g)
	assert.NotNil(t, p.int64GaugeVecs["metric_name [a b]"])
	assert.NotNil(t, p.int64Gauges["metric_name-a-2-b-1"])
}

func TestFloat64(t *testing.T) {
	p := newPromClient()
	g := p.GetFloat64Metric("a.c", map[string]string{"some_key": "some-value"})
	assert.NotNil(t, g)
	assert.NotNil(t, p.float64GaugeVecs["a_c [some_key]"])
	assert.NotNil(t, p.float64Gauges["a_c-some_key-some-value"])
	assert.Nil(t, p.float64GaugeVecs["a.c"])

	g.Update(3)
	assert.Equal(t, float64(3), g.Get())

	g2 := p.GetFloat64Metric("a.c", map[string]string{"some_key": "some-new-value"})
	assert.NotNil(t, g2)
	g2.Update(4)

	assert.Equal(t, float64(3), g.Get())
	assert.Equal(t, float64(4), g2.Get())

	g2 = p.GetFloat64Metric("a.c", map[string]string{"some_key": "some-new-value"})
	assert.Equal(t, float64(4), g2.Get())

	// Metric with two tags.
	g = p.GetFloat64Metric("float_metric_name", map[string]string{"a": "2", "b": "1"})
	assert.NotNil(t, g)
	assert.NotNil(t, p.float64GaugeVecs["float_metric_name [a b]"])
	assert.NotNil(t, p.float64Gauges["float_metric_name-a-2-b-1"])
}

func TestCounter(t *testing.T) {
	p := newPromClient()
	g := p.GetCounter("c", map[string]string{"some_key": "some-value"})
	assert.NotNil(t, g)

	g.Inc(3)
	assert.Equal(t, int64(3), g.Get())

	g.Dec(2)
	assert.Equal(t, int64(1), g.Get())

	g.Reset()
	assert.Equal(t, int64(0), g.Get())
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
