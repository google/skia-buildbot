package influxdb

/*
   Convenience utilities for working with InfluxDB.
*/

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"time"

	"github.com/skia-dev/influxdb/client"
	"go.skia.org/infra/go/metadata"
)

const (
	DEFAULT_HOST     = "localhost:8086"
	DEFAULT_USER     = "root"
	DEFAULT_PASSWORD = "root"
	DEFAULT_DATABASE = "graphite"

	TAG_NAME = "influxdb"
)

var (
	host     *string
	user     *string
	password *string
	database *string
)

// SetupFlags adds command-line flags for InfluxDB.
func SetupFlags() {
	host = flag.String("influxdb_host", DEFAULT_HOST, "The InfluxDB hostname.")
	user = flag.String("influxdb_name", DEFAULT_USER, "The InfluxDB username.")
	password = flag.String("influxdb_password", DEFAULT_PASSWORD, "The InfluxDB password.")
	database = flag.String("influxdb_database", DEFAULT_DATABASE, "The InfluxDB database.")
}

type queryClient interface {
	Query(client.Query) (*client.Response, error)
}

// Client is a struct used for communicating with an InfluxDB instance.
type Client struct {
	database string
	dbClient queryClient
}

// NewClient returns a Client with the given credentials.
func NewClient(host, user, password, database string) (*Client, error) {
	u, err := url.Parse(fmt.Sprintf("http://%s", host))
	if err != nil {
		return nil, err
	}
	dbClient, err := client.NewClient(client.Config{
		URL:      *u,
		Username: user,
		Password: password,
		Timeout:  time.Minute,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize InfluxDB client: %s", err)
	}
	return &Client{
		database: database,
		dbClient: dbClient,
	}, nil
}

// NewClientFromFlags returns a Client with credentials obtained from flags.
// Assumes that SetupFlags() and flag.Parse() have been called.
func NewClientFromFlags() (*Client, error) {
	return NewClient(*host, *user, *password, *database)
}

// NewClientFromFlagsAndMetadata returns a Client with credentials obtained
// from a combination of flags and metadata, depending on whether the program
// is running in local mode.
func NewClientFromFlagsAndMetadata(local bool) (*Client, error) {
	if !local {
		userMeta, err := metadata.ProjectGet(metadata.INFLUXDB_NAME)
		if err != nil {
			return nil, err
		}
		passMeta, err := metadata.ProjectGet(metadata.INFLUXDB_PASSWORD)
		if err != nil {
			return nil, err
		}
		*user = userMeta
		*password = passMeta
	}
	return NewClientFromFlags()
}

// Query issues a query to the InfluxDB instance and returns its results.
func (c *Client) Query(q string) ([]client.Result, error) {
	response, err := c.dbClient.Query(client.Query{
		Command:  q,
		Database: c.database,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to query InfluxDB with query %q: %s", q, err)
	}
	if response.Err != nil {
		return nil, fmt.Errorf("Failed to query InfluxDB with query %q: %s", q, response.Err)
	}
	return response.Results, nil
}

// QuerySingle issues a query to the InfluxDB instance and returns a single
// point value. The query must return a single series with a single point,
// otherwise QuerySingle returns an error.
func (c *Client) QuerySingle(q string) (json.Number, error) {
	results, err := c.Query(q)
	if err != nil {
		return "", err
	}
	// We want exactly one series.
	if len(results) < 1 {
		return "", fmt.Errorf("Query returned no data: %q", q)
	}
	if len(results) > 1 {
		return "", fmt.Errorf("Query returned more than one series: %q", q)
	}
	series := results[0].Series
	if len(series) < 1 {
		return "", fmt.Errorf("Query returned no series: %q", q)
	}
	if len(series) > 1 {
		return "", fmt.Errorf("Query returned more than one series: %q", q)
	}
	valueColumn := 0
	for _, label := range series[0].Columns {
		if label == "time" || label == "sequence_number" {
			valueColumn++
		} else {
			break
		}
	}
	// The column containing the value should be the last column.
	if len(series[0].Columns) != valueColumn+1 {
		return "", fmt.Errorf("Query returned an incorrect set of columns: %q %v", q, series[0].Columns)
	}
	// We want exactly one point.
	points := series[0].Values
	if len(points) < 1 {
		return "", fmt.Errorf("Query returned no points: %q", q)
	}
	if len(points) > 1 {
		return "", fmt.Errorf("Query returned more than one point: %q", q)
	}
	point := points[0]

	// Ensure that the columns are correct for the point.
	if len(series[0].Columns) != len(point) {
		return "", fmt.Errorf("Invalid data from InfluxDB: Point data does not match column spec:\nCols:\n%v\nVals:\n%v", series[0].Columns, point)
	}
	if point[valueColumn] == nil {
		return "", fmt.Errorf("Query returned nil value: %q", q)
	}
	return point[valueColumn].(json.Number), nil
}

// QuerySingleFloat64 issues a query to the InfluxDB instance and returns a
// single float64 point value. The query must return a single series with a
// single point, otherwise QuerySingleFloat64 returns an error.
func (c *Client) QuerySingleFloat64(q string) (float64, error) {
	n, err := c.QuerySingle(q)
	if err != nil {
		return 0.0, err
	}
	return n.Float64()
}

// QuerySingleInt64 issues a query to the InfluxDB instance and returns a
// single int64 point value. The query must return a single series with a
// single point, otherwise QuerySingleInt64 returns an error.
func (c *Client) QuerySingleInt64(q string) (int64, error) {
	n, err := c.QuerySingle(q)
	if err != nil {
		return 0.0, err
	}
	return n.Int64()
}
