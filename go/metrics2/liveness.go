package metrics2

import (
	"sync"
	"time"

	"go.skia.org/infra/go/util"
)

const (
	MEASUREMENT_LIVENESS = "liveness"
)

// Liveness keeps a time-since-last-successful-update metric.
//
// The unit of the metrics is in seconds.
//
// It is used to keep track of periodic processes to make sure that they are running
// successfully. Every liveness metric should have a corresponding alert set up that
// will fire of the time-since-last-successful-update metric gets too large.
type Liveness struct {
	lastSuccessfulUpdate time.Time
	m                    *Int64Metric
	mtx                  sync.Mutex
}

// NewLiveness creates a new Liveness metric helper. The current value is
// reported at the given frequency; if the report frequency is zero, the value
// is only reported when it changes.
func (c *Client) NewLiveness(name string, tagsList ...map[string]string) *Liveness {
	// Make a copy of the tags and add the name.
	tags := util.AddParams(map[string]string{}, tagsList...)
	tags["name"] = name
	l := &Liveness{
		lastSuccessfulUpdate: time.Now(),
		m:                    c.GetInt64Metric(MEASUREMENT_LIVENESS, tags),
		mtx:                  sync.Mutex{},
	}
	go func() {
		for _ = range time.Tick(c.reportFrequency) {
			l.update()
		}
	}()
	return l
}

// NewLiveness creates a new Liveness metric helper using the default client.
// The current value is reported at the given frequency; if the report frequency
// is zero, the value is only reported when it changes.
func NewLiveness(name string, tags ...map[string]string) *Liveness {
	return DefaultClient.NewLiveness(name, tags...)
}

// updateLocked sets the value of the Liveness. Assumes the caller holds a lock.
func (l *Liveness) updateLocked() {
	l.m.Update(int64(time.Since(l.lastSuccessfulUpdate).Seconds()))
}

// update sets the value of the Liveness.
func (l *Liveness) update() {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.updateLocked()
}

// Reset should be called when some work has been successfully completed.
func (l *Liveness) Reset() {
	l.ManualReset(time.Now())
}

// ManualReset sets the last-successful-update time of the Liveness to a
// specific value. Useful for tracking processes whose lifetimes are outside
// of that of the current process, but should not be needed in most cases.
func (l *Liveness) ManualReset(lastSuccessfulUpdate time.Time) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.lastSuccessfulUpdate = lastSuccessfulUpdate
	l.updateLocked()
}
