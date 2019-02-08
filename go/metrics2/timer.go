package metrics2

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"go.skia.org/infra/go/util"
)

const (
	MEASUREMENT_TIMER = "timer"
	NAME_FUNC_TIMER   = "func_timer"
)

// timer implements Timer.
type timer struct {
	begin time.Time
	m     Float64SummaryMetric
}

// NewTimer creates and returns a new started timer.
//
// makeUnique - True if the measurement name needs to made unique, which means
//              to append 'name' to 'timer'.
func newTimer(c Client, name string, makeUnique bool, tagsList ...map[string]string) Timer {
	// Make a copy of the tags and add the name.
	tags := util.AddParams(map[string]string{}, tagsList...)
	tags["name"] = name
	measurement := MEASUREMENT_TIMER
	if makeUnique {
		measurement = fmt.Sprintf("%s_%s_ns", MEASUREMENT_TIMER, name)
		tags["type"] = MEASUREMENT_TIMER
	}
	ret := &timer{
		m: c.GetFloat64SummaryMetric(measurement, tags),
	}
	ret.Start()
	return ret
}

// Start starts or resets the timer.
func (t *timer) Start() {
	t.begin = time.Now()
}

// Stop stops the timer and reports the elapsed time.
func (t *timer) Stop() time.Duration {
	dur := time.Now().Sub(t.begin)
	v := float64(dur)
	t.m.Observe(v)
	return dur
}

// NewTimer creates and returns a new Timer using the default client.
func NewTimer(name string, tags ...map[string]string) Timer {
	return defaultClient.NewTimer(name, tags...)
}

// FuncTimer is specifically intended for measuring the duration of functions.
// It uses the default client.
//
// The standard way to use FuncTimer is at the top of the func you
// want to measure:
//
// func myfunc() {
//    defer metrics2.FuncTimer().Stop()
//    ...
// }
//
func FuncTimer() Timer {
	return FuncTimerWithStackOffset(1)
}

// FuncTimerWithStackOffset returns a FuncTimer with the specified stack offset.
// This allows the caller to fine-tune which function gets timed, eg. using
// helper functions. The offset is from the calling function, ie. to time the
// current function you'd use zero.
func FuncTimerWithStackOffset(offset int) Timer {
	pc, _, _, _ := runtime.Caller(offset + 1)
	f := runtime.FuncForPC(pc)
	split := strings.Split(f.Name(), ".")
	fn := "unknown"
	pkg := "unknown"
	if len(split) >= 2 {
		fn = split[len(split)-1]
		pkg = strings.Join(split[:len(split)-1], ".")
	}
	return NewTimer(NAME_FUNC_TIMER, map[string]string{"package": pkg, "func": fn})
}

// Verify that timer implements the Timer interface.
var _ Timer = (*timer)(nil)
