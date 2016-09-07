package client

import "sync/atomic"

// Metric is the standard implementation of an int64 metric, and uses the
// sync/atomic package to manage the value.
type Metric struct {
	value int64
}

// Clear sets the metric to zero.
func (c *Metric) Clear() {
	atomic.StoreInt64(&c.value, 0)
}

// Value returns the metric value.
func (c *Metric) Value() int64 {
	return atomic.LoadInt64(&c.value)
}

// Dec decrements the metric by the given amount.
func (c *Metric) Dec(i int64) {
	atomic.AddInt64(&c.value, -i)
}

// Inc increments the metric by the given amount.
func (c *Metric) Inc(i int64) {
	atomic.AddInt64(&c.value, i)
}
