package metrics2

/*
   Convenience utilities for working with InfluxDB.
*/

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/util"
)

const (
	DEFAULT_REPORT_FREQUENCY = time.Minute
	PUSH_FREQUENCY           = time.Minute
)

var (
	DefaultClient *Client = &Client{
		metrics: map[string]*rawMetric{},
	}
)

// Init() initializes the metrics package.
func Init(appName string, influxClient *influxdb.Client) error {
	hostName, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Unable to retrieve hostname: %s", err)
	}
	tags := map[string]string{
		"app":  appName,
		"host": hostName,
	}
	c, err := NewClient(influxClient, tags, DEFAULT_REPORT_FREQUENCY)
	if err != nil {
		return err
	}
	// Some metrics may already be registered with DefaultClient. Copy them
	// over.
	c.metrics = DefaultClient.metrics

	// Set the default client.
	DefaultClient = c
	return nil
}

// Client is a struct used for communicating with an InfluxDB instance.
type Client struct {
	influxClient    *influxdb.Client
	defaultTags     map[string]string
	metrics         map[string]*rawMetric
	metricsMtx      sync.Mutex
	reportFrequency time.Duration
	values          *influxdb.BatchPoints
	valuesMtx       sync.Mutex
}

// NewClient returns a Client which uses the given influxdb.Client to push data.
// defaultTags specifies a set of default tag keys and values which are applied
// to all data points. reportFrequency specifies how often metrics should create
// data points.
func NewClient(influxClient *influxdb.Client, defaultTags map[string]string, reportFrequency time.Duration) (*Client, error) {
	values, err := influxClient.NewBatchPoints()
	if err != nil {
		return nil, err
	}
	c := &Client{
		influxClient:    influxClient,
		defaultTags:     defaultTags,
		metrics:         map[string]*rawMetric{},
		metricsMtx:      sync.Mutex{},
		reportFrequency: reportFrequency,
		values:          values,
		valuesMtx:       sync.Mutex{},
	}
	go func() {
		for _ = range time.Tick(PUSH_FREQUENCY) {
			if err := c.pushData(); err != nil {
				glog.Errorf("Failed to push data into InfluxDB: %s", err)
			}
		}
	}()
	go func() {
		for _ = range time.Tick(reportFrequency) {
			for _, m := range c.metrics {
				c.addPoint(m.measurement, m.tags, m.get())
			}
		}
	}()
	return c, nil
}

// addPointAtTime adds a data point with the given timestamp.
func (c *Client) addPointAtTime(measurement string, tags map[string]string, value interface{}, ts time.Time) {
	c.valuesMtx.Lock()
	defer c.valuesMtx.Unlock()
	if c.values == nil {
		glog.Errorf("Metrics client not initialized; cannot add points.")
		return
	}
	if tags == nil {
		tags = map[string]string{}
	}
	allTags := make(map[string]string, len(tags)+len(c.defaultTags))
	for k, v := range c.defaultTags {
		allTags[k] = v
	}
	for k, v := range tags {
		allTags[k] = v
	}
	if err := c.values.AddPoint(measurement, allTags, map[string]interface{}{"value": value}, ts); err != nil {
		glog.Errorf("Failed to add data point: %s", err)
	}
}

// addPoint adds a data point.
func (c *Client) addPoint(measurement string, tags map[string]string, value interface{}) {
	c.addPointAtTime(measurement, tags, value, time.Now())
}

// RawAddInt64PointAtTime adds an int64 data point to the default client at the
// given time. When possible, use one of the helpers instead.
func RawAddInt64PointAtTime(measurement string, tags map[string]string, value int64, ts time.Time) {
	DefaultClient.addPointAtTime(measurement, tags, value, ts)
}

// pushData pushes all queued data into InfluxDB.
func (c *Client) pushData() error {
	c.valuesMtx.Lock()
	defer c.valuesMtx.Unlock()
	if c.influxClient == nil {
		return fmt.Errorf("InfluxDB client is nil! Cannot push data. Did you initialize the metrics2 package?")
	}
	if err := c.influxClient.WriteBatch(c.values); err != nil {
		return err
	}
	newValues, err := c.influxClient.NewBatchPoints()
	if err != nil {
		return err
	}
	c.values = newValues
	return nil
}

// rawMetric is a metric which has no explicit type.
type rawMetric struct {
	measurement string
	mtx         sync.RWMutex
	tags        map[string]string
	value       interface{}
}

// get returns the current value of the metric.
func (m *rawMetric) get() interface{} {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	return m.value
}

// update adds a data point to the metric.
func (m *rawMetric) update(v interface{}) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.value = v
}

// getRawMetric creates or retrieves a metric with the given measurement name
// and tag set and returns it.
func (c *Client) getRawMetric(measurement string, tagsList []map[string]string, initial interface{}) *rawMetric {
	c.metricsMtx.Lock()
	defer c.metricsMtx.Unlock()

	// Make a copy of the concatenation of all provided tags.
	tags := util.AddParams(map[string]string{}, tagsList...)
	md5, err := util.MD5Params(tags)
	if err != nil {
		glog.Errorf("Failed to encode measurement tags: %s", err)
	}
	key := fmt.Sprintf("%s_%s", measurement, md5)
	m, ok := c.metrics[key]
	if !ok {
		m = &rawMetric{
			measurement: measurement,
			mtx:         sync.RWMutex{},
			tags:        tags,
			value:       initial,
		}
		c.metrics[key] = m
	}
	return m
}
