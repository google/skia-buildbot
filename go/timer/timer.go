// timer makes timing operations easier.
package timer

import (
	"time"

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
type Timer struct {
	Begin time.Time
	Name  string
}

func New(name string) *Timer {
	return &Timer{
		Begin: time.Now(),
		Name:  name,
	}
}

func (t Timer) Stop() {
	sklog.Infof("%s %v", t.Name, time.Now().Sub(t.Begin))
}
