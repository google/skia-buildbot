package client

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
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

// Metric is the interface that all metrics must support.
//
// The Value() method must be Go routine safe.
type Metric interface {
	Value() int64
}

// rageWriteClient is a client for sending metrics to a ragemon server.
//
// Metrics are sent to the server via HTTP Post of a content-type text/plain,
// one line per metric, of the form:
//
//    <structured key> <value>
//
// For example:
//
//    ,app=perf,host=skia-perf,meas=requests, 100
//    ,app=perf,host=skia-perf,meas=errors, 27
//
// The ragemon server will stamp points with the current time as they arrive,
// so we don't need to include timestamps.
type rageWriteClient struct {
	metrics    map[string]Metric
	appName    string
	hostName   string
	serverURL  string
	httpClient *http.Client
	mutex      sync.Mutex
}

var (
	// safeRe is used in Init() to replace unsafe chars in a hostname.
	safeRe = regexp.MustCompile("[^a-zA-Z0-9]")

	// client is the single instance of rageWriteClient we need.
	client *rageWriteClient
)

func init() {
	client = &rageWriteClient{
		metrics: map[string]Metric{},
	}
}

// GetOrRegister returns a Counter with the given measurement name and tags.
func GetOrRegister(measurement string, tags map[string]string) (*Counter, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	params := util.CopyStringMap(tags)
	if params == nil {
		params = map[string]string{}
	}
	params[MEASUREMENT_NAME_KEY] = measurement
	if client.appName != "" {
		params[APP_NAME_KEY] = client.appName
	}
	if client.hostName != "" {
		params[HOST_NAME_KEY] = client.hostName
	}

	key, err := query.MakeKey(params)
	if err != nil {
		return nil, fmt.Errorf("Not a valid Metric parameters: %s", err)
	}
	ret := &Counter{}
	client.metrics[key] = ret

	return ret, nil
}

// MustGetOrRegister is GetOrRegister that glog.Fatals if there was an error creating the metric.
func MustGetOrRegister(measurement string, tags map[string]string) *Counter {
	m, err := GetOrRegister(measurement, tags)
	if err != nil {
		glog.Fatalf("Failed to create Counter: %s", err)
	}
	return m
}

// serializedBody returns the serialized metrics suitable for putting in an
// HTTP Post request body.
func serializedBody() string {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	ret := []string{}
	for k, m := range client.metrics {
		ret = append(ret, fmt.Sprintf("%s %d", k, m.Value()))
	}
	return strings.Join(ret, "\n")
}

// oneStep does a single step of sending all metric values to the server.
func oneStep() {
	buf := bytes.NewBufferString(serializedBody())
	resp, err := client.httpClient.Post(client.serverURL, "text/plain", buf)
	if err != nil || resp.StatusCode >= 400 {
		glog.Errorf("Error sending metrics to ragemon server: %s", err)
	}
}

// pushMetrics runs as a Go routine and pushes metrics every minute.
func pushMetrics() {
	oneStep()
	for _ = range time.Tick(time.Minute) {
		oneStep()
	}
}

// Init initializes the metrics client, setting the appName and hostName for
// all metrics, and sending metric updates to 'serverURL' using the passed in
// client 'c'.
func Init(appName, serverURL string, c *http.Client, hostName string) error {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	if hostName == "" {
		var err error
		hostName, err = os.Hostname()
		if err != nil {
			return fmt.Errorf("Unable to retrieve hostname: %s", err)
		}
		hostName = safeRe.ReplaceAllLiteralString(hostName, "_")
	}

	if appName == "" {
		return fmt.Errorf("Not a valid app name: %q", appName)
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
