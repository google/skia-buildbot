package client

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/util"
)

const (
	APP_NAME_KEY         = "app"
	MEASUREMENT_NAME_KEY = "meas"
	HOST_NAME_KEY        = "host"
)

type rageWriteClient struct {
	metrics    map[string]*Metric
	appName    string
	hostName   string
	serverURL  string
	httpClient *http.Client
	mutex      sync.Mutex
}

var (
	client *rageWriteClient
)

func init() {

	client = &rageWriteClient{
		metrics: map[string]*Metric{},
	}
}

func GetOrRegister(measurement string, tags map[string]string) (*Metric, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	params := util.CopyStringMap(tags)
	params[MEASUREMENT_NAME_KEY] = measurement
	params[APP_NAME_KEY] = client.appName
	params[HOST_NAME_KEY] = client.hostName

	key, err := query.MakeKey(params)
	if err != nil {
		return nil, fmt.Errorf("Not a valid Metric parameters: %s", err)
	}
	ret := &Metric{}
	client.metrics[key] = ret

	return ret, nil
}

func MustGetOrRegister(measurement string, tags map[string]string) *Metric {
	m, err := GetOrRegister(measurement, tags)
	if err != nil {
		glog.Fatalf("Failed to create Metric: %s", err)
	}
	return m
}

func serializedBody() string {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	ret := []string{}
	now := time.Now()
	for k, m := range client.metrics {
		ret = append(ret, fmt.Sprintf("%s %d %d", k, now.Unix(), m.Value()))
	}
	return strings.Join(ret, "\n")
}

func oneStep() {
	buf := bytes.NewBufferString(serializedBody())
	resp, err := client.httpClient.Post(client.serverURL, "text/plain", buf)
	if err != nil || resp.StatusCode >= 400 {
		glog.Errorf("Error sending metrics to ragemon server: %s", err)
	}
}

func pushMetrics() {
	oneStep()
	for _ = range time.Tick(time.Minute) {
		oneStep()
	}
}

func Init(appName, serverURL string, c *http.Client) error {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	hostName, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Unable to retrieve hostname: %s", err)
	}

	client.httpClient = c
	client.appName = appName
	client.serverURL = serverURL
	client.hostName = hostName

	// If there are any existing metrics then we need to rewrite their keys
	// to include the appName and hostName.
	for k, m := range client.metrics {
		params, err := query.ParseKey(k)
		if err != nil {
			return fmt.Errorf("Found invalid metric name %q: %s", k, err)
		}
		params[APP_NAME_KEY] = appName
		params[HOST_NAME_KEY] = hostName
		delete(client.metrics, k)
		newKey, err := query.MakeKey(params)
		if err != nil {
			return fmt.Errorf("Failed to rewrite existing metrics: %s", err)
		}
		client.metrics[newKey] = m
	}

	// Now start the Go routine.
	go pushMetrics()

	return nil
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
