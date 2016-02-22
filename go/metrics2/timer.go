package metrics2

import (
	"runtime"
	"strings"
	"time"

	"go.skia.org/infra/go/util"
)

const (
	MEASUREMENT_TIMER = "timer"
	NAME_FUNC_TIMER   = "func-timer"
)

// Timer is a struct used for measuring elapsed time. Unlike the other metrics
// helpers, Timer does not continuously report data; instead, it reports a
// single data point when Stop() is called.
type Timer struct {
	begin time.Time
	m     *Int64MeanMetric
}

// NewTimer creates and returns a new started timer.
func (c *Client) NewTimer(name string, tagsList ...map[string]string) *Timer {
	// Make a copy of the tags and add the name.
	tags := util.AddParams(map[string]string{}, tagsList...)
	tags["name"] = name
	ret := &Timer{
		m: c.GetInt64MeanMetric(MEASUREMENT_TIMER, tags),
	}
	ret.Start()
	return ret
}

// NewTimer creates and returns a new Timer using the default client.
func NewTimer(name string, tags ...map[string]string) *Timer {
	return DefaultClient.NewTimer(name, tags...)
}

// Start starts or resets the timer.
func (t *Timer) Start() {
	t.begin = time.Now()
}

// Stop stops the timer and reports the elapsed time.
func (t *Timer) Stop() {
	v := int64(time.Now().Sub(t.begin))
	t.m.update(v)
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
