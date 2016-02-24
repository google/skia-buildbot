package metrics2

import (
	"fmt"
	"sync"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/util"
)

const (
	MEASUREMENT_COUNTER = "counter"
)

// Counter is a struct used for tracking metrics which increment or decrement.
type Counter struct {
	m   *Int64Metric
	mtx sync.Mutex
}

// GetCounter creates or retrieves a Counter with the given name and tag set and
// returns it.
func (c *Client) GetCounter(name string, tagsList ...map[string]string) *Counter {
	c.countersMtx.Lock()
	defer c.countersMtx.Unlock()

	// Make a copy of the concatenation of all provided tags.
	tags := util.AddParams(map[string]string{}, tagsList...)
	tags["name"] = name
	md5, err := util.MD5Params(tags)
	if err != nil {
		glog.Errorf("Failed to encode measurement tags: %s", err)
	}
	key := fmt.Sprintf("%s_%s", MEASUREMENT_COUNTER, md5)
	m, ok := c.counters[key]
	if !ok {
		m = &Counter{
			m: c.GetInt64Metric(MEASUREMENT_COUNTER, tags),
		}
		c.counters[key] = m
	}
	return m
}

// GetCounter creates and returns a new Counter using the default client.
func GetCounter(name string, tags map[string]string) *Counter {
	return DefaultClient.GetCounter(name, tags)
}

// Inc increments the counter by the given quantity.
func (c *Counter) Inc(i int64) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.m.Update(c.m.Get() + i)
}

// Dec decrements the counter by the given quantity.
func (c *Counter) Dec(i int64) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.m.Update(c.m.Get() - i)
}

// Reset sets the counter to zero.
func (c *Counter) Reset() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.m.Update(0)
}

// Get returns the current value in the counter.
func (c *Counter) Get() int64 {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.m.Get()
}
