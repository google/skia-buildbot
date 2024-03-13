package metrics

import (
	"math"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/metrics2"
)

func newClient() metrics2.Client {
	// This wipes out all previous metrics.
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	return metrics2.GetDefaultClient()
}

func TestHandler_WithSameCounter_ReturnSameCounter(t *testing.T) {
	const cn = "some_counter"
	h := NewMetricsHandler(map[string]string{}, newClient())
	require.Equal(t, h.Counter(cn), h.Counter(cn))
	require.NotEqual(t, h.Counter("other"), h.Counter(cn))
}

func TestHandler_WithSameGauge_ReturnSameGauge(t *testing.T) {
	const gn = "some_gauge"
	h := NewMetricsHandler(map[string]string{}, newClient())
	require.Equal(t, h.Gauge(gn), h.Gauge(gn))
	require.NotEqual(t, h.Gauge("other"), h.Gauge(gn))
}

func TestHandler_OnSameTimer_ReturnSameTimer(t *testing.T) {
	const tn = "some_timer"
	h := NewMetricsHandler(map[string]string{}, newClient())
	require.Equal(t, h.Timer(tn), h.Timer(tn))
	require.NotEqual(t, h.Timer("other"), h.Timer(tn))
}

func TestHandler_WithCounter_RecordMetric(t *testing.T) {
	const cn = "some_counter"
	h := NewMetricsHandler(map[string]string{}, newClient())
	h.Counter(cn).Inc(1)
	require.EqualValues(t, 1, h.Counter(cn).(metrics2.Counter).Get())
	require.NotEqualValues(t, 1, h.Counter("other").(metrics2.Counter).Get())
}

func TestHandler_WithGauge_RecordMetric(t *testing.T) {
	const gn = "some_gauge"
	h := NewMetricsHandler(map[string]string{}, newClient())
	h.Gauge(gn).Update(2.2)
	require.InDelta(t, 2.2, h.Gauge(gn).(metrics2.Float64Metric).Get(), 1e-9)
	require.Greater(t, math.Abs(2.2-h.Gauge("other").(metrics2.Float64Metric).Get()), 1e-9)
}

func TestHanlder_RecordMetrics_InParallel(t *testing.T) {
	var wg sync.WaitGroup
	const count = 500

	const ct1, ct2 = "some_counter1", "some_counter2"
	const gg1, gg2 = "some_gauge1", "some_gauge2"

	// Spawn many goroutines for 4 different metrics
	wg.Add(count * 4)

	h := NewMetricsHandler(map[string]string{}, newClient())
	for i := 0; i < count; i++ {
		go func() {
			h.Counter(ct1).Inc(1)
			wg.Done()
		}()
		go func() {
			h.Counter(ct2).Inc(1)
			wg.Done()
		}()
		go func() {
			h.Gauge(gg1).Update(2.0)
			wg.Done()
		}()
		go func() {
			h.Gauge(gg2).Update(4.0)
			wg.Done()
		}()
	}
	wg.Wait()
	require.EqualValues(t, count, h.Counter(ct1).(metrics2.Counter).Get())
	require.EqualValues(t, count, h.Counter(ct2).(metrics2.Counter).Get())
	require.InDelta(t, 2.0, h.Gauge(gg1).(metrics2.Float64Metric).Get(), 1e-9)
	require.InDelta(t, 4.0, h.Gauge(gg2).(metrics2.Float64Metric).Get(), 1e-9)
}
