// metrics2 is a client library for recording and reporting monitoring data.
package metrics2

import (
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Timer is a struct used for measuring elapsed time. Unlike the other metrics
// helpers, timer does not continuously report data; instead, it reports a
// single data point when Stop() is called.
type Timer interface {
	// Start starts or resets the timer.
	Start()

	// Stop stops the timer and reports the elapsed time.
	Stop() time.Duration
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

	// Close stops the internal goroutine. Usually used for testing since most Liveness instances
	// live for the duration of the process.
	Close()
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
	// Clients should cache this counter, as making multiple calls with the same keys will return
	// a fresh counter (initialized to 0), which is undesirable. These counters would all compete
	// with each other; the underlying metric would reflect the result of the most-recently
	// updated instance.
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

	// Int64MetricExists returns true if the given Int64Metric already exists.
	Int64MetricExists(measurement string, tags ...map[string]string) bool
}

var (
	defaultClient Client = NewPromClient()
)

// GetDefaultClient returns the default Client.
func GetDefaultClient() Client {
	return defaultClient
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
}
