package metrics2

import (
	"runtime"
	"strings"
	"time"
)

const (
	MEASUREMENT_TIMER = "timer"
	NAME_FUNC_TIMER   = "func-timer"
)

// Timer is a struct used for measuring elapsed time. Unlike the other metrics
// helpers, Timer does not continuously report data; instead, it reports a
// single data point when Stop() is called.
type Timer struct {
	begin       time.Time
	client      *Client
	measurement string
	tags        map[string]string
}

// NewTimer creates and returns a new Timer.
func (c *Client) NewTimer(name string, tags map[string]string) *Timer {
	// Add the name to the tags.
	t := make(map[string]string, len(tags)+1)
	for k, v := range tags {
		t[k] = v
	}
	t["name"] = name
	return &Timer{
		begin:       time.Now(),
		client:      c,
		measurement: MEASUREMENT_TIMER,
		tags:        t,
	}
}

// NewTimer creates and returns a new Timer using the default client.
func NewTimer(name string, tags map[string]string) *Timer {
	return DefaultClient.NewTimer(name, tags)
}

// Stop stops the timer and reports the elapsed time.
func (t Timer) Stop() {
	v := int64(time.Now().Sub(t.begin))
	t.client.addPoint(t.measurement, t.tags, v)
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
func FuncTimer() *Timer {
	pc, _, _, _ := runtime.Caller(1)
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
