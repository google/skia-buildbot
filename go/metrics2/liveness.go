package metrics2

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	LIVENESS_REPORT_FREQUENCY = time.Minute
	MEASUREMENT_LIVENESS      = "liveness"
)

// liveness implements Liveness.
type liveness struct {
	cancelFn             func()
	lastSuccessfulUpdate time.Time
	m                    Int64Metric
	mtx                  sync.Mutex
}

// newLiveness creates a new Liveness metric helper. The current value is
// reported at the given frequency; if the report frequency is zero, the value
// is only reported when it changes.
func newLiveness(c Client, name string, makeUnique bool, tagsList ...map[string]string) Liveness {
	// Make a copy of the tags and add the name.
	tags := util.AddParams(map[string]string{}, tagsList...)
	tags["name"] = name

	measurement := MEASUREMENT_LIVENESS
	if makeUnique {
		measurement = fmt.Sprintf("%s_%s_s", MEASUREMENT_LIVENESS, name)
		tags["type"] = MEASUREMENT_LIVENESS
	}
	if c.Int64MetricExists(measurement, tags) {
		sklog.Errorf("Liveness metric %s (%v) already exists! This may cause a goroutine leak.", measurement, tags)
	}
	ctx, cancelFn := context.WithCancel(context.Background())
	l := &liveness{
		cancelFn:             cancelFn,
		lastSuccessfulUpdate: time.Now(),
		m:                    c.GetInt64Metric(measurement, tags),
		mtx:                  sync.Mutex{},
	}
	go util.RepeatCtx(LIVENESS_REPORT_FREQUENCY, ctx, func(_ context.Context) { l.update() })
	return l
}

// Close implements the Liveness interface.
func (l *liveness) Close() {
	l.cancelFn()
}

// getLocked returns the current value of the Liveness. Assumes the caller holds a lock.
func (l *liveness) getLocked() int64 {
	return int64(time.Since(l.lastSuccessfulUpdate).Seconds())
}

// Get returns the current value of the Liveness.
func (l *liveness) Get() int64 {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	return l.getLocked()
}

// updateLocked sets the value of the Liveness. Assumes the caller holds a lock.
func (l *liveness) updateLocked() {
	l.m.Update(l.getLocked())
}

// update sets the value of the Liveness.
func (l *liveness) update() {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.updateLocked()
}

// Reset should be called when some work has been successfully completed.
func (l *liveness) Reset() {
	l.ManualReset(time.Now())
}

// ManualReset sets the last-successful-update time of the Liveness to a
// specific value. Useful for tracking processes whose lifetimes are outside
// of that of the current process, but should not be needed in most cases.
func (l *liveness) ManualReset(lastSuccessfulUpdate time.Time) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.lastSuccessfulUpdate = lastSuccessfulUpdate
	l.updateLocked()
}

// NewLiveness creates a new Liveness metric helper using the default client.
// The current value is reported at the given frequency; if the report frequency
// is zero, the value is only reported when it changes.
func NewLiveness(name string, tags ...map[string]string) Liveness {
	return defaultClient.NewLiveness(name, tags...)
}

// Validate that the concrete liveness faithfully implements the Liveness interface.
var _ Liveness = (*liveness)(nil)
