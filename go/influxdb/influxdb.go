package influxdb

/*
   Convenience utilities for working with InfluxDB.
*/

import (
	"flag"
	"fmt"
	"reflect"

	"github.com/skia-dev/glog"
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

// Client is a struct used for communicating with an InfluxDB instance.
type Client struct {
	dbClient *client.Client
}

// NewClient returns a Client with the given credentials.
func NewClient(host, user, password, database string) (*Client, error) {
	dbClient, err := client.New(&client.ClientConfig{
		Host:       host,
		Username:   user,
		Password:   password,
		Database:   database,
		HttpClient: nil,
		IsSecure:   false,
		IsUDP:      false,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize InfluxDB client: %s", err)
	}
	return &Client{
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

// structToSeriesList converts a struct to an InfluxDB Series. Struct fields
// with the "influxdb" tag will be mapped to InfluxDB columns with the same
// names. All types of ints, floats, uints, and strings are supported. No
// nested structs are supported.
func structToSeries(data interface{}, series string) (*client.Series, error) {
	v := reflect.Indirect(reflect.ValueOf(data))
	t := v.Type()
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("Data point is required to be a struct (is a %s)", t.Kind())
	}
	cols := []string{}
	vals := []interface{}{}
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get(TAG_NAME)
		if tag != "" {
			cols = append(cols, tag)
			vals = append(vals, v.Field(i).Interface())
		}
	}
	return &client.Series{
		Name:    series,
		Columns: cols,
		Points:  [][]interface{}{vals},
	}, nil
}

// seriesToStruct converts an InfluxDB Series to a struct. Attempts to map
// columns from the series onto the struct based on the "influxdb" tag on the
// struct fields. Returns an error if all of the result columns could not be
// mapped onto the struct, with the exception of the "time" and
// "sequence_number" columns. All types of ints, floats, uints, and strings
// are supported. No nested structs are supported.
func seriesToStruct(rv interface{}, s *client.Series) error {
	ptr := reflect.ValueOf(rv)
	v := reflect.ValueOf(rv)
	for v.Type().Kind() == reflect.Interface || v.Type().Kind() == reflect.Ptr {
		ptr = v
		v = v.Elem()
	}
	if ptr.Type().Kind() != reflect.Ptr {
		return fmt.Errorf("Return value is required to be a pointer to a struct (is a %s)", ptr.Type().Kind())
	}
	t := v.Type()
	if t.Kind() != reflect.Struct {
		return fmt.Errorf("Return value is required to be a pointer to a struct (is a %s)", t.Kind())
	}
	points := s.Points
	if len(points) < 1 {
		return fmt.Errorf("Query returned no points: %v", s)
	}
	if len(points) > 1 {
		return fmt.Errorf("Query returned more than one point: %v", s)
	}
	p := points[0]
	if len(p) != len(s.Columns) {
		return fmt.Errorf("Query returned mismatched column/point dimensions: %v", s)
	}
	for i, c := range s.Columns {
		foundField := false
		for j := 0; j < t.NumField(); j++ {
			f := v.Field(j)
			tag := t.Field(j).Tag.Get(TAG_NAME)
			val := reflect.ValueOf(p[i])
			if tag != "" && tag == c {
				fieldType := f.Type()
				fieldKind := fieldType.Kind()
				if fieldKind == reflect.Bool {
					v.Field(j).SetBool(val.Convert(fieldType).Bool())
				} else if fieldKind == reflect.Float32 || fieldKind == reflect.Float64 {
					v.Field(j).SetFloat(val.Convert(fieldType).Float())
				} else if fieldKind == reflect.Int || fieldKind == reflect.Int8 || fieldKind == reflect.Int16 || fieldKind == reflect.Int32 || fieldKind == reflect.Int64 {
					v.Field(j).SetInt(val.Convert(fieldType).Int())
				} else if fieldKind == reflect.String {
					v.Field(j).SetString(p[i].(string))
				} else if fieldKind == reflect.Uint || fieldKind == reflect.Uint8 || fieldKind == reflect.Uint16 || fieldKind == reflect.Uint32 || fieldKind == reflect.Uint64 {
					v.Field(j).SetUint(val.Convert(fieldType).Uint())
				} else {
					return fmt.Errorf("Field has unsupported type %s", fieldKind)
				}
				foundField = true
				break
			}
		}
		if !foundField {
			if !(c == "time" || c == "sequence_number") {
				return fmt.Errorf("Destination struct contains no field with tag \"%s\"", c)
			}
		}
	}
	return nil
}

// WriteDataPoint pushes the data point into InfluxDB. Struct fields with the
// "influxdb" tag will be mapped to InfluxDB columns with the same names. All
// types of ints, floats, uints, and strings are supported. No nested structs
// are supported.
func (c *Client) WriteDataPoint(data interface{}, series string) error {
	s, err := structToSeries(data, series)
	if err != nil {
		return err
	}
	seriesList := []*client.Series{s}
	glog.Infof("Pushing datapoint to %s: %v", series, s)
	return c.dbClient.WriteSeries(seriesList)
}

// Query issues a query to the InfluxDB instance and places its results in rv.
// Attempts to map columns from the query result onto the struct based on
// the "influxdb" tag on the struct fields. Returns an error if all of the
// result columns could not be mapped onto the struct, with the exception of
// the "time" and "sequence_number" columns. All types of ints, floats, uints,
// and strings are supported. No nested structs are supported.
func (c *Client) Query(rv interface{}, q string) error {
	results, err := c.dbClient.Query(q)
	if err != nil {
		return fmt.Errorf("Failed to query InfluxDB with query %q: %s", q, err)
	}
	if len(results) < 1 {
		return fmt.Errorf("Query returned no data: %q", q)
	}
	if len(results) > 1 {
		return fmt.Errorf("Query returned more than one series: %q", q)
	}
	return seriesToStruct(rv, results[0])
}
