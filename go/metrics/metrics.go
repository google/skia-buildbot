// metrics is a package for helper functions for working go-metrics.
package metrics

import (
	"time"

	"github.com/rcrowley/go-metrics"
)

// Liveness keeps a time-since-last-successful-update metric.
//
// The unit of the metrics is in seconds.
//
// It is used to keep track of periodic processes to make sure that they are running
// successfully. Every liveness metric should have a corresponding alert set up that
// will fire of the time-since-last-successful-update metric gets too large.
type Liveness struct {
	lastSuccessfulUpdate           time.Time
	timeSinceLastSucceessfulUpdate metrics.Gauge
}

// Update should be called when some work has been successfully completed.
func (l *Liveness) Update() {
	l.lastSuccessfulUpdate = time.Now()
}

// NewLiveness creates a new Liveness metric helper.
func NewLiveness(name string) *Liveness {
	l := &Liveness{
		lastSuccessfulUpdate:           time.Now(),
		timeSinceLastSucceessfulUpdate: metrics.NewRegisteredGauge(name+".time-since-last-successful-update", metrics.DefaultRegistry),
	}
	l.timeSinceLastSucceessfulUpdate.Update(int64(time.Since(l.lastSuccessfulUpdate).Seconds()))
	go func() {
		for _ = range time.Tick(time.Minute) {
			l.timeSinceLastSucceessfulUpdate.Update(int64(time.Since(l.lastSuccessfulUpdate).Seconds()))
		}
	}()
	return l
}
