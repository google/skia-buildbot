package metrics2

import "sync"

const (
	MEASUREMENT_COUNTER = "counter"
)

// Counter is a struct used for tracking metrics which increment or decrement.
type Counter struct {
	m   *Int64Metric
	mtx sync.Mutex
}

// NewCounter creates and returns a new Counter.
func (c *Client) NewCounter(name string, tags map[string]string) *Counter {
	// Add the name to the tags.
	t := make(map[string]string, len(tags)+1)
	for k, v := range tags {
		t[k] = v
	}
	t["name"] = name
	return &Counter{
		m:   c.GetInt64Metric(MEASUREMENT_COUNTER, t),
		mtx: sync.Mutex{},
	}
}

// NewCounter creates and returns a new Counter using the default client.
func NewCounter(name string, tags map[string]string) *Counter {
	return DefaultClient.NewCounter(name, tags)
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
