package client

import "sync/atomic"

// Counter is the standard implementation of an int64 counter, and uses the
// sync/atomic package to manage the value.
//
// Counter implements Metric.
type Counter struct {
	value int64
}

// Clear sets the counter to zero.
func (c *Counter) Clear() {
	atomic.StoreInt64(&c.value, 0)
}

// Value returns the counter value.
func (c *Counter) Value() int64 {
	return atomic.LoadInt64(&c.value)
}

// Dec decrements the counter by the given amount.
func (c *Counter) Dec(i int64) {
	atomic.AddInt64(&c.value, -i)
}

// Inc increments the counter by the given amount.
func (c *Counter) Inc(i int64) {
	atomic.AddInt64(&c.value, i)
}
