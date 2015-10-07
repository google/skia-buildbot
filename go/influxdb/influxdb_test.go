package influxdb

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/skia-dev/influxdb/client"
	"github.com/skia-dev/influxdb/influxql"
	assert "github.com/stretchr/testify/require"
)

type dummyClient struct {
	queryFn func(client.Query) (*client.Response, error)
}

func (c dummyClient) Query(q client.Query) (*client.Response, error) {
	return c.queryFn(q)
}

func TestQueryNumber(t *testing.T) {
	type queryCase struct {
		Name        string
		QueryFunc   func(client.Query) (*client.Response, error)
		ExpectedVal float64
		ExpectedErr error
	}
	cases := []queryCase{
		queryCase{
			Name: "QueryFailed",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return nil, fmt.Errorf("<dummy error>")
			},
			ExpectedVal: 0.0,
			ExpectedErr: fmt.Errorf("Failed to query InfluxDB with query \"<dummy query>\": <dummy error>"),
		},
		queryCase{
			Name: "EmptyResults",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return &client.Response{}, nil
			},
			ExpectedVal: 0.0,
			ExpectedErr: fmt.Errorf("Query returned no data: \"<dummy query>\""),
		},
		queryCase{
			Name: "NoSeries",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return &client.Response{
					Results: []client.Result{
						client.Result{
							Series: []influxql.Row{},
						},
					},
					Err: nil,
				}, nil
			},
			ExpectedVal: 0.0,
			ExpectedErr: fmt.Errorf("Query returned no series: \"<dummy query>\""),
		},
		queryCase{
			Name: "TooManySeries",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return &client.Response{
					Results: []client.Result{
						client.Result{
							Series: []influxql.Row{
								influxql.Row{},
								influxql.Row{},
							},
						},
					},
					Err: nil,
				}, nil
			},
			ExpectedVal: 0.0,
			ExpectedErr: fmt.Errorf("Query returned more than one series: \"<dummy query>\""),
		},
		queryCase{
			Name: "NotEnoughCols",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return &client.Response{
					Results: []client.Result{
						client.Result{
							Series: []influxql.Row{
								influxql.Row{
									Columns: []string{"value"},
									Values: [][]interface{}{
										[]interface{}{
											interface{}(12345),
											interface{}(1.004),
										},
									},
								},
							},
						},
					},
					Err: nil,
				}, nil
			},
			ExpectedVal: 0.0,
			ExpectedErr: fmt.Errorf("Invalid data from InfluxDB: Point data does not match column spec:\nCols:\n[value]\nVals:\n[12345 1.004]"),
		},
		queryCase{
			Name: "TooManyCols",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return &client.Response{
					Results: []client.Result{
						client.Result{
							Series: []influxql.Row{
								influxql.Row{
									Columns: []string{"time", "label", "value"},
									Values: [][]interface{}{
										[]interface{}{
											interface{}(12345),
											interface{}(1.004),
										},
									},
								},
							},
						},
					},
					Err: nil,
				}, nil
			},
			ExpectedVal: 0.0,
			ExpectedErr: fmt.Errorf("Query returned an incorrect set of columns: \"<dummy query>\" [time label value]"),
		},
		queryCase{
			Name: "NoPoints",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return &client.Response{
					Results: []client.Result{
						client.Result{
							Series: []influxql.Row{
								influxql.Row{
									Columns: []string{"time", "value"},
									Values:  [][]interface{}{},
								},
							},
						},
					},
					Err: nil,
				}, nil
			},
			ExpectedVal: 0.0,
			ExpectedErr: fmt.Errorf("Query returned no points: \"<dummy query>\""),
		},
		queryCase{
			Name: "GoodData",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return &client.Response{
					Results: []client.Result{
						client.Result{
							Series: []influxql.Row{
								influxql.Row{
									Columns: []string{"time", "value"},
									Values: [][]interface{}{
										[]interface{}{
											interface{}(12345),
											interface{}(json.Number("1.5")),
										},
									},
								},
							},
						},
					},
					Err: nil,
				}, nil
			},
			ExpectedVal: 1.5,
			ExpectedErr: nil,
		},
		queryCase{
			Name: "GoodWithSequenceNumber",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return &client.Response{
					Results: []client.Result{
						client.Result{
							Series: []influxql.Row{
								influxql.Row{
									Columns: []string{"time", "sequence_number", "value"},
									Values: [][]interface{}{
										[]interface{}{
											interface{}(12345),
											interface{}(10001),
											interface{}(json.Number("1.5")),
										},
									},
								},
							},
						},
					},
					Err: nil,
				}, nil
			},
			ExpectedVal: 1.5,
			ExpectedErr: nil,
		},
	}

	errorStr := "Case %s:\nExpected:\n%v\nActual:\n%v"
	for _, c := range cases {
		client := Client{
			database: "nodatabase",
			dbClient: dummyClient{c.QueryFunc},
		}
		val, err := client.QueryNumber("<dummy query>")
		assert.Equal(t, c.ExpectedErr, err, fmt.Sprintf(errorStr, c.Name, c.ExpectedErr, err))
		if err != nil {
			continue
		}
		v, err := val.Float64()
		assert.Nil(t, err, fmt.Sprintf(errorStr, c.Name, nil, err))
		assert.Equal(t, c.ExpectedVal, v, fmt.Sprintf(errorStr, c.Name, c.ExpectedVal, v))
	}

}
