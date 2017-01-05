// metrics2 is a client library for recording and reporting monitoring data.
package metrics2

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/influxdb"
)

// Timer is a struct used for measuring elapsed time. Unlike the other metrics
// helpers, timer does not continuously report data; instead, it reports a
// single data point when Stop() is called.
type Timer interface {
	// Start starts or resets the timer.
	Start()

	// Stop stops the timer and reports the elapsed time.
	Stop()
}

// Liveness keeps a time-since-last-successful-update metric.
//
// The unit of the metrics is in seconds.
//
// It is used to keep track of periodic processes to make sure that they are running
// successfully. Every liveness metric should have a corresponding alert set up that
// will fire of the time-since-last-successful-update metric gets too large.
type Liveness interface {
	// Get returns the current value of the Liveness.
	Get() int64

	// ManualReset sets the last-successful-update time of the Liveness to a specific value. Useful for tracking processes whose lifetimes are outside of that of the current process, but should not be needed in most cases.
	ManualReset(lastSuccessfulUpdate time.Time)

	// Reset should be called when some work has been successfully completed.
	Reset()
}

// Int64Metric is a metric which reports an int64 value.
type Int64Metric interface {
	// Delete removes the metric from its Client's registry.
	Delete() error

	// Get returns the current value of the metric.
	Get() int64

	// Update adds a data point to the metric.
	Update(v int64)
}

// Float64Metric is a metric which reports a float64 value.
type Float64Metric interface {
	// Delete removes the metric from its Client's registry.
	Delete() error

	// Get returns the current value of the metric.
	Get() float64

	// Update adds a data point to the metric.
	Update(v float64)
}

// Float64SummaryMetric is a metric which reports a summary of many float64 values.
type Float64SummaryMetric interface {
	// Observe adds a data point to the metric.
	Observe(v float64)
}

// Counter is a struct used for tracking metrics which increment or decrement.
type Counter interface {
	// Dec decrements the counter by the given quantity.
	Dec(i int64)

	// Delete removes the counter from metrics.
	Delete() error

	// Get returns the current value in the counter.
	Get() int64

	// Inc increments the counter by the given quantity.
	Inc(i int64)

	// Reset sets the counter to zero.
	Reset()
}

// Client represents a set of metrics.
type Client interface {
	// Flush pushes any queued data immediately. Long running apps shouldn't worry about this as Client will auto-push every so often.
	Flush() error

	// GetCounter creates or retrieves a Counter with the given name and tag set and returns it.
	GetCounter(name string, tagsList ...map[string]string) Counter

	// GetFloat64Metric returns a Float64Metric instance.
	GetFloat64Metric(measurement string, tags ...map[string]string) Float64Metric

	// GetInt64Metric returns an Int64Metric instance.
	GetInt64Metric(measurement string, tags ...map[string]string) Int64Metric

	// GetFloat64SummaryMetric returns an Float64SummaryMetric instance.
	GetFloat64SummaryMetric(measurement string, tags ...map[string]string) Float64SummaryMetric

	// NewLiveness creates a new Liveness metric helper.
	NewLiveness(name string, tagsList ...map[string]string) Liveness

	// NewTimer creates and returns a new started timer.
	NewTimer(name string, tagsList ...map[string]string) Timer
}

var (
	defaultInfluxClient *influxClient = &influxClient{
		aggMetrics:      map[string]*aggregateMetric{},
		counters:        map[string]*counter{},
		metrics:         map[string]*rawMetric{},
		reportFrequency: time.Minute,
	}
	defaultClient Client = defaultInfluxClient
)

// GetDefaultClient returns the default Client.
func GetDefaultClient() Client {
	return defaultClient
}

// Init() initializes the metrics package.
func Init(appName string, influxDbClient *influxdb.Client) error {
	hostName, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Unable to retrieve hostname: %s", err)
	}
	tags := map[string]string{
		"app":  appName,
		"host": hostName,
	}
	clientI, err := NewClient(influxDbClient, tags, DEFAULT_REPORT_FREQUENCY)
	if err != nil {
		return err
	}
	c := clientI.(*influxClient)
	// Some metrics may already be registered with defaultInfluxClient. Copy them
	// over.
	c.aggMetrics = defaultInfluxClient.aggMetrics
	c.counters = defaultInfluxClient.counters
	c.metrics = defaultInfluxClient.metrics

	// Set the default client.
	defaultClient = c
	defaultInfluxClient = c
	return nil
}

// InitPrometheus initializes metrics to be reported to Prometheus.
//
// port - string, The port on which to serve the metrics, e.g. ":10110".
func InitPrometheus(port string) {
	r := mux.NewRouter()
	r.Handle("/metrics", promhttp.Handler())
	go func() {
		glog.Fatal(http.ListenAndServe(port, r))
	}()
	defaultClient = newPromClient()
	defaultInfluxClient = nil
}

// InitPromInflux initializes metrics to be reported to both InfluxDB and Prometheus.
//
// port - string, The port on which to serve the metrics, e.g. ":10110".
func InitPromInflux(appName string, influxDbClient *influxdb.Client, port string) error {
	if err := Init(appName, influxDbClient); err != nil {
		return fmt.Errorf("Failed to Init Influx metrics: %s", err)
	}
	influxClient := defaultClient
	InitPrometheus(port)
	promClient := defaultClient
	var err error
	defaultClient, err = newMuxClient([]Client{promClient, influxClient})
	if err != nil {
		return fmt.Errorf("Failed to create MuxClient: %s", err)
	}
	return nil
}

// NewClient returns a Client which uses the given influxdb.Client to push data.
// defaultTags specifies a set of default tag keys and values which are applied
// to all data points. reportFrequency specifies how often metrics should create
// data points.
func NewClient(influxDbClient *influxdb.Client, defaultTags map[string]string, reportFrequency time.Duration) (Client, error) {
	values, err := influxDbClient.NewBatchPoints()
	if err != nil {
		return nil, err
	}
	c := &influxClient{
		aggMetrics:      map[string]*aggregateMetric{},
		counters:        map[string]*counter{},
		influxDbClient:  influxDbClient,
		defaultTags:     defaultTags,
		metrics:         map[string]*rawMetric{},
		reportFrequency: reportFrequency,
		values:          values,
	}
	go func() {
		for _ = range time.Tick(PUSH_FREQUENCY) {
			byMeasurement, err := c.pushData()
			if err != nil {
				glog.Errorf("Failed to push data into InfluxDB: %s", err)
			} else {
				total := int64(0)
				for k, v := range byMeasurement {
					c.GetInt64Metric("metrics.points-pushed.by-measurement", map[string]string{"measurement": k}).Update(v)
					total += v
				}
				c.GetInt64Metric("metrics.points-pushed.total", nil).Update(total)
			}
		}
	}()
	go func() {
		for _ = range time.Tick(reportFrequency) {
			c.collectMetrics()
			c.collectAggregateMetrics()
		}
	}()
	return c, nil
}
