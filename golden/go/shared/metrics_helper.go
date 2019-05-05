package shared

import (
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/timer"
)

// NewMetricsTimer wraps a metric and a timer together so we can
// have both a metric produced and the times in the logs.
// Use of this helper can make sure all the gold_timers stick
// together.
// TODO(kjlubick): If, at some point, these timers in the logs get
// noisy, we can just replace this with metrics2.NewTimer directly.
func NewMetricsTimer(name string) *timer.Timer {
	m := metrics2.GetFloat64SummaryMetric("gold_timer", map[string]string{
		"name": name,
	})
	return timer.NewWithSummary(name, m)
}
