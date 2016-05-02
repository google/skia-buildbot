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
		aggMetrics:      map[string]*aggregateMetric{},
		counters:        map[string]*Counter{},
		metrics:         map[string]*rawMetric{},
		reportFrequency: time.Minute,
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
	c.aggMetrics = DefaultClient.aggMetrics
	c.counters = DefaultClient.counters
	c.metrics = DefaultClient.metrics

	// Set the default client.
	DefaultClient = c
	return nil
}

// Client is a struct used for communicating with an InfluxDB instance.
type Client struct {
	aggMetrics    map[string]*aggregateMetric
	aggMetricsMtx sync.Mutex

	counters    map[string]*Counter
	countersMtx sync.Mutex

	influxClient *influxdb.Client
	defaultTags  map[string]string

	metrics    map[string]*rawMetric
	metricsMtx sync.Mutex

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
		aggMetrics:      map[string]*aggregateMetric{},
		counters:        map[string]*Counter{},
		influxClient:    influxClient,
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

// collectMetrics collects data points from all raw metrics.
func (c *Client) collectMetrics() {
	c.metricsMtx.Lock()
	defer c.metricsMtx.Unlock()
	for _, m := range c.metrics {
		c.addPoint(m.measurement, m.tags, m.get())
	}
}

// collectAggregateMetrics collects data points from all aggregate metrics.
func (c *Client) collectAggregateMetrics() {
	c.aggMetricsMtx.Lock()
	defer c.aggMetricsMtx.Unlock()
	for _, m := range c.aggMetrics {
		c.addPoint(m.measurement, m.tags, m.reset())
	}
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
func (c *Client) pushData() (map[string]int64, error) {
	c.valuesMtx.Lock()
	defer c.valuesMtx.Unlock()

	// Always clear out the values after pushing, even if we failed.
	newValues, err := c.influxClient.NewBatchPoints()
	if err != nil {
		return nil, err
	}
	defer func() {
		c.values = newValues
	}()

	if c.influxClient == nil {
		return nil, fmt.Errorf("InfluxDB client is nil! Cannot push data. Did you initialize the metrics2 package?")
	}

	// Push the points.
	if err := c.influxClient.WriteBatch(c.values); err != nil {
		return nil, err
	}

	// Record the number of points.
	byMeasurement := map[string]int64{}
	points := c.values.Points()
	for _, pt := range points {
		count := byMeasurement[pt.Name()]
		byMeasurement[pt.Name()] = count + 1
	}

	return byMeasurement, nil
}

// rawMetric is a metric which has no explicit type.
type rawMetric struct {
	client      *Client
	key         string
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

// Delete removes the metric from its Client's registry.
func (m *rawMetric) Delete() error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	return m.client.deleteRawMetric(m.key)
}

// makeMetricKey generates a key for the given metric based on its measurement
// name and tags.
func makeMetricKey(measurement string, tags map[string]string) string {
	md5, err := util.MD5Params(tags)
	if err != nil {
		glog.Errorf("Failed to encode measurement tags: %s", err)
	}
	return fmt.Sprintf("%s_%s", measurement, md5)
}

// getRawMetric creates or retrieves a metric with the given measurement name
// and tag set and returns it.
func (c *Client) getRawMetric(measurement string, tagsList []map[string]string, initial interface{}) *rawMetric {
	c.metricsMtx.Lock()
	defer c.metricsMtx.Unlock()

	// Make a copy of the concatenation of all provided tags.
	tags := util.AddParams(map[string]string{}, tagsList...)
	key := makeMetricKey(measurement, tags)
	m, ok := c.metrics[key]
	if !ok {
		m = &rawMetric{
			client:      c,
			key:         key,
			measurement: measurement,
			tags:        tags,
			value:       initial,
		}
		c.metrics[key] = m
	}
	return m
}

// getAggregateMetric creates or retrieves an aggregateMetric with the given
// measurement name and tag set and returns it.
func (c *Client) getAggregateMetric(measurement string, tagsList []map[string]string, aggFn func([]interface{}) interface{}) *aggregateMetric {
	c.aggMetricsMtx.Lock()
	defer c.aggMetricsMtx.Unlock()

	// Make a copy of the concatenation of all provided tags.
	tags := util.AddParams(map[string]string{}, tagsList...)
	key := makeMetricKey(measurement, tags)
	m, ok := c.aggMetrics[key]
	if !ok {
		m = &aggregateMetric{
			aggFn:       aggFn,
			client:      c,
			key:         key,
			measurement: measurement,
			tags:        tags,
			values:      []interface{}{},
		}
		c.aggMetrics[key] = m
	}
	return m
}

// deleteRawMetric removes the given raw metric.
func (c *Client) deleteRawMetric(key string) error {
	c.metricsMtx.Lock()
	defer c.metricsMtx.Unlock()

	if _, ok := c.metrics[key]; ok {
		delete(c.metrics, key)
	} else {
		return fmt.Errorf("Unable to delete unknown metric: %s", key)
	}
	return nil
}

// deleteAggregateMetric removes the given aggregate metric.
func (c *Client) deleteAggregateMetric(key string) error {
	c.aggMetricsMtx.Lock()
	defer c.aggMetricsMtx.Unlock()

	if _, ok := c.aggMetrics[key]; ok {
		delete(c.aggMetrics, key)
	} else {
		return fmt.Errorf("Unable to delete unknown metric: %s", key)
	}
	return nil
}
