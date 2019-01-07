// timer makes timing operations easier.
package timer

import (
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

// Timer is for timing events. When finished the duration is reported
// via sklog.
//
// The standard way to use Timer is at the top of the func you
// want to measure:
//
//     defer timer.New("database sync time").Stop()
//
// If you need to do something with the duration then you can do:
//
//     timerMetric := timer.New("database sync time")
//     defer func() {
//       duration := timerMetric.Stop()
//       // Do something with duration here.
//     }()
//

// updateable is an interface that Float64Metrics implement.
type updateable interface {
	Update(float64)
}

// summaryWrapper is an adaptor that turns a Float64SummaryMetric
// into an 'updateable'.
type summaryWrapper struct {
	m metrics2.Float64SummaryMetric
}

// See updateable.
func (s summaryWrapper) Update(v float64) {
	s.m.Observe(v)
}

type Timer struct {
	Begin  time.Time
	Name   string
	Metric updateable
}

func New(name string) *Timer {
	return &Timer{
		Begin: time.Now(),
		Name:  name,
	}
}

func NewWithMetric(name string, m metrics2.Float64Metric) *Timer {
	return &Timer{
		Begin:  time.Now(),
		Name:   name,
		Metric: m,
	}
}

func NewWithSummary(name string, m metrics2.Float64SummaryMetric) *Timer {
	return &Timer{
		Begin:  time.Now(),
		Name:   name,
		Metric: summaryWrapper{m: m},
	}
}

func (t Timer) Stop() time.Duration {
	duration := time.Now().Sub(t.Begin)
	sklog.Infof("%s %v", t.Name, duration)
	if t.Metric != nil {
		t.Metric.Update(duration.Seconds())
	}
	return duration
}
