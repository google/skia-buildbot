package influxdb

/*
   Convenience utilities for working with InfluxDB.
*/

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/polling_status"

	influx_client "github.com/skia-dev/influxdb/client/v2"
)

const (
	DEFAULT_HOST     = "localhost:8086"
	DEFAULT_USER     = "root"
	DEFAULT_PASSWORD = "root"
	DEFAULT_DATABASE = "graphite"
)

type queryClient interface {
	Query(influx_client.Query) (*influx_client.Response, error)
	Write(influx_client.BatchPoints) error
}

// Client is a struct used for communicating with an InfluxDB instance.
type Client struct {
	Database     string
	influxClient queryClient
	defaultTags  map[string]string
	mtx          sync.Mutex
	values       influx_client.BatchPoints
}

// NewClient returns a Client with the given credentials.
func NewClient(host, user, password, database string) (*Client, error) {
	if !(strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://")) {
		host = fmt.Sprintf("http://%s", host)
	}
	influxClient, err := influx_client.NewHTTPClient(influx_client.HTTPConfig{
		Addr:     host,
		Username: user,
		Password: password,
		Timeout:  3 * time.Minute,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize InfluxDB client: %s", err)
	}
	values, err := influx_client.NewBatchPoints(influx_client.BatchPointsConfig{
		Database: database,
	})
	if err != nil {
		return nil, err
	}
	return &Client{
		Database:     database,
		influxClient: influxClient,
		mtx:          sync.Mutex{},
		values:       values,
	}, nil
}

// Point is a struct representing a data point with multiple values in InfluxDB.
type Point struct {
	Tags   map[string]string
	Values []json.Number
}

// Query issues a query to the InfluxDB instance and returns a slice of Points.
// The query must return series which have a single point with n values, or an error is returned.
func (c *Client) Query(database, q string, n int) ([]*Point, error) {
	response, err := c.influxClient.Query(influx_client.Query{
		Command:  q,
		Database: database,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to query InfluxDB with query %q: %s", q, err)
	}
	if response.Err != "" {
		return nil, fmt.Errorf(response.Err)
	}

	results := response.Results
	if err != nil {
		return nil, err
	}
	// We want exactly one result.
	if len(results) < 1 {
		return nil, fmt.Errorf("Query returned no results: d=%q q=%q", database, q)
	}
	if len(results) > 1 {
		return nil, fmt.Errorf("Query returned more than one result: d=%q q=%q", database, q)
	}

	// Collect all data points.
	series := results[0].Series
	rv := make([]*Point, 0, len(series))
	for _, s := range series {
		valueColumn := 0
		// Skip over the non value columns, if any exist
		for _, label := range s.Columns {
			if label == "time" || label == "sequence_number" {
				valueColumn++
			} else {
				break
			}
		}

		// The column containing the values should be the last n columns.
		if len(s.Columns) != valueColumn+n {
			return nil, fmt.Errorf("Query returned an incorrect set of columns.  Wanted %d of them: %q %v", valueColumn+n, q, s.Columns)
		}
		points := s.Values
		// There should still be exactly one point.  This point will have n + 1 values which we extract below.
		if len(points) < 1 {
			return nil, fmt.Errorf("Query returned too few points.  Wanted 1, but was %d: %q", len(points), q)
		}
		if len(points) > 1 {
			return nil, fmt.Errorf("Query returned too many points.  Wanted 1, but was %d: %q", len(points), q)
		}
		point := points[0]

		// Ensure that the columns are correct for the point.
		if len(s.Columns) != len(point) {
			return nil, fmt.Errorf("Invalid data from InfluxDB: Point data does not match column spec:\nCols:\n%v\nVals:\n%v", series[0].Columns, point)
		}
		values := make([]json.Number, 0, len(point)-1)
		for ; valueColumn < len(point); valueColumn++ {
			if point[valueColumn] == nil {
				return nil, fmt.Errorf("Query returned nil value: %q", q)
			}
			values = append(values, point[valueColumn].(json.Number))
		}
		rv = append(rv, &Point{
			Tags:   s.Tags,
			Values: values,
		})
	}
	return rv, nil
}

// QueryInt64 issues a query to the InfluxDB instance and returns a
// single int64 point value. The query must return a single series with a
// single point, otherwise QueryInt64 returns an error.
func (c *Client) QueryInt64(database, q string) (int64, error) {
	res, err := c.Query(database, q, 1)
	if err != nil {
		return 0.0, err
	}
	if len(res) != 1 {
		return 0.0, fmt.Errorf("Query returned %d series (db = %q): %q\n%v", len(res), database, q, res)
	}
	return res[0].Values[0].Int64()
}

// PollingStatus returns a PollingStatus which runs the given
// query at the given interval.
func (c *Client) Int64PollingStatus(database, query string, interval time.Duration) *polling_status.PollingStatus {
	return polling_status.NewPollingStatus(func() (interface{}, error) {
		return c.QueryInt64(database, query)
	}, interval)
}

// BatchPoints is a struct used for writing batches of points into InfluxDB.
type BatchPoints struct {
	bp  influx_client.BatchPoints
	mtx sync.Mutex
}

// AddPoint adds a point to the BatchPoints.
func (bp *BatchPoints) AddPoint(measurement string, tags map[string]string, fields map[string]interface{}, ts time.Time) error {
	bp.mtx.Lock()
	defer bp.mtx.Unlock()
	pt, err := influx_client.NewPoint(measurement, tags, fields, ts)
	if err != nil {
		return err
	}
	bp.bp.AddPoint(pt)
	return nil
}

// NewBatchPoints returns an InfluxDB BatchPoints instance.
func (c *Client) NewBatchPoints() (*BatchPoints, error) {
	bp, err := influx_client.NewBatchPoints(influx_client.BatchPointsConfig{
		Database: c.Database,
	})
	if err != nil {
		return nil, err
	}
	return &BatchPoints{
		bp:  bp,
		mtx: sync.Mutex{},
	}, nil
}

// NumPoints returns the number of points in the batch.
func (bp *BatchPoints) Points() []*influx_client.Point {
	bp.mtx.Lock()
	defer bp.mtx.Unlock()
	return bp.bp.Points()
}

// WriteBatch writes the BatchPoints into InfluxDB.
func (c *Client) WriteBatch(batch *BatchPoints) error {
	batch.mtx.Lock()
	defer batch.mtx.Unlock()
	return c.influxClient.Write(batch.bp)
}
