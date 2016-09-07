package client

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/golang/glog"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/util"
)

const (
	APP_NAME_KEY         = "app"
	MEASUREMENT_NAME_KEY = "meas"
)

var (
	metrics map[string]*Metric = map[string]*Metric{}
	appName string
	client  *http.Client
	mutex   sync.Mutex
)

func GetOrRegister(measurement string, tags map[string]string) (*Metric, error) {
	mutex.Lock()
	defer mutex.Unlock()

	params := util.CopyStringMap(tags)
	params[MEASUREMENT_NAME_KEY] = measurement
	if appName != "" {
		params[APP_NAME_KEY] = appName
	}

	key, err := query.MakeKey(params)
	if err != nil {
		return nil, fmt.Errorf("Not a valid Metric parameters: %s", err)
	}
	ret := &Metric{}
	metrics[key] = ret

	return ret, nil
}

func MustGetOrRegister(measurement string, tags map[string]string) *Metric {
	m, err := GetOrRegister(measurement, tags)
	if err != nil {
		glog.Fatalf("Faile to create metric: %s", err)
	}
	return m
}

func Init(appName string, client *http.Client) {
	mutex.Lock()
	defer mutex.Unlock()

	// If there are any existing metrics then we need to rewrite their keys
	// to include the appName.

	// Now start the Go routine.
}

// Metric is the standard implementation of a Counter and uses the
// sync/atomic package to manage a single int64 value.
type Metric struct {
	value int64
}

// Clear sets the counter to zero.
func (c *Metric) Clear() {
	atomic.StoreInt64(&c.value, 0)
}

// Value returns the current value.
func (c *Metric) Value() int64 {
	return atomic.LoadInt64(&c.value)
}

// Dec decrements the counter by the given amount.
func (c *Metric) Dec(i int64) {
	atomic.AddInt64(&c.value, -i)
}

// Inc increments the counter by the given amount.
func (c *Metric) Inc(i int64) {
	atomic.AddInt64(&c.value, i)
}
