package influxdb

/*
   Convenience utilities for working with InfluxDB.
*/

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	influx_client "github.com/influxdata/influxdb/client/v2"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/util"
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
	influxClient, err := influx_client.NewHTTPClient(influx_client.HTTPConfig{
		Addr:     fmt.Sprintf("http://%s", host),
		Username: user,
		Password: password,
		Timeout:  time.Minute,
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

// NewClientFromParamsAndMetadata returns a Client with credentials obtained
// from a combination of the given parameters and metadata, depending on whether
// the program is running in local mode.
func NewClientFromParamsAndMetadata(host, user, password, database string, local bool) (*Client, error) {
	if !local {
		var err error
		user, err = metadata.ProjectGet(metadata.INFLUXDB_NAME)
		if err != nil {
			return nil, err
		}
		password, err = metadata.ProjectGet(metadata.INFLUXDB_PASSWORD)
		if err != nil {
			return nil, err
		}
		database, err = metadata.ProjectGet(metadata.INFLUXDB_DATABASE)
		if err != nil {
			return nil, err
		}
		host, err = metadata.ProjectGet(metadata.INFLUXDB_HOST)
		if err != nil {
			return nil, err
		}
	}
	return NewClient(host, user, password, database)
}

// Point is a struct representing a single data point in InfluxDB.
type Point struct {
	Tags  map[string]string
	Value json.Number
}

// Query issues a query to the InfluxDB instance and returns a slice of Points.
// The query must return series which have a single point, otherwise an error is
// returned.
func (c *Client) Query(database, q string) ([]*Point, error) {
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
	// Allow queries to return no series.
	series := results[0].Series
	if len(series) < 1 {
		return []*Point{}, nil
	}
	// Collect all data points.
	rv := make([]*Point, 0, len(series))
	for _, s := range series {
		valueColumn := 0
		for _, label := range s.Columns {
			if label == "time" || label == "sequence_number" {
				valueColumn++
			} else {
				break
			}
		}
		// The column containing the value should be the last column.
		if len(s.Columns) != valueColumn+1 {
			return nil, fmt.Errorf("Query returned an incorrect set of columns: %q %v", q, s.Columns)
		}
		// We want exactly one point.
		points := s.Values
		if len(points) < 1 {
			return nil, fmt.Errorf("Query returned no points: %q", q)
		}
		if len(points) > 1 {
			return nil, fmt.Errorf("Query returned more than one point: %q", q)
		}
		point := points[0]

		// Ensure that the columns are correct for the point.
		if len(s.Columns) != len(point) {
			return nil, fmt.Errorf("Invalid data from InfluxDB: Point data does not match column spec:\nCols:\n%v\nVals:\n%v", series[0].Columns, point)
		}
		if point[valueColumn] == nil {
			return nil, fmt.Errorf("Query returned nil value: %q", q)
		}
		rv = append(rv, &Point{
			Tags:  s.Tags,
			Value: point[valueColumn].(json.Number),
		})
	}
	return rv, nil
}

// QueryFloat64 issues a query to the InfluxDB instance and returns a
// single float64 point value. The query must return a single series with a
// single point, otherwise QueryFloat64 returns an error.
func (c *Client) QueryFloat64(database, q string) (float64, error) {
	res, err := c.Query(database, q)
	if err != nil {
		return 0.0, err
	}
	if len(res) != 1 {
		return 0.0, fmt.Errorf("Query returned more than one series: %q", q)
	}
	return res[0].Value.Float64()
}

// QueryInt64 issues a query to the InfluxDB instance and returns a
// single int64 point value. The query must return a single series with a
// single point, otherwise QueryInt64 returns an error.
func (c *Client) QueryInt64(database, q string) (int64, error) {
	res, err := c.Query(database, q)
	if err != nil {
		return 0.0, err
	}
	if len(res) != 1 {
		return 0.0, fmt.Errorf("Query returned more than one series: %q", q)
	}
	return res[0].Value.Int64()
}

// PollingStatus returns a util.PollingStatus which runs the given
// query at the given interval.
func (c *Client) Int64PollingStatus(database, query string, interval time.Duration) *util.PollingStatus {
	return util.NewPollingStatus(func() (interface{}, error) {
		return c.QueryInt64(database, query)
	}, interval)
}

// BatchPoints is a struct used for writing batches of points into InfluxDB.
type BatchPoints struct {
	bp influx_client.BatchPoints
}

// AddPoint adds a point to the BatchPoints.
func (bp *BatchPoints) AddPoint(measurement string, tags map[string]string, fields map[string]interface{}, ts time.Time) error {
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
	return &BatchPoints{bp: bp}, nil
}

// WriteBatch writes the BatchPoints into InfluxDB.
func (c *Client) WriteBatch(batch *BatchPoints) error {
	return c.influxClient.Write(batch.bp)
}
