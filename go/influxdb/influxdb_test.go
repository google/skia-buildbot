package influxdb

import (
	"encoding/json"
	"fmt"
	"testing"

	"go.skia.org/infra/go/testutils"

	client "github.com/influxdata/influxdb/client/v2"
	"github.com/influxdata/influxdb/models"
	assert "github.com/stretchr/testify/require"
)

type dummyClient struct {
	queryFn func(client.Query) (*client.Response, error)
}

func (c dummyClient) Query(q client.Query) (*client.Response, error) {
	return c.queryFn(q)
}

func (c dummyClient) Write(client.BatchPoints) error {
	return nil
}

func TestQueryNumber(t *testing.T) {
	type queryCase struct {
		Name        string
		QueryFunc   func(client.Query) (*client.Response, error)
		ExpectedVal []*Point
		ExpectedErr error
	}
	cases := []queryCase{
		queryCase{
			Name: "QueryFailed",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return nil, fmt.Errorf("<dummy error>")
			},
			ExpectedVal: nil,
			ExpectedErr: fmt.Errorf("Failed to query InfluxDB with query \"<dummy query>\": <dummy error>"),
		},
		queryCase{
			Name: "EmptyResults",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return &client.Response{}, nil
			},
			ExpectedVal: nil,
			ExpectedErr: fmt.Errorf("Query returned no results: d=\"nodatabase\" q=\"<dummy query>\""),
		},
		queryCase{
			Name: "MultipleResults",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return &client.Response{
					Results: []client.Result{
						client.Result{},
						client.Result{},
					},
					Err: "",
				}, nil
			},
			ExpectedVal: nil,
			ExpectedErr: fmt.Errorf("Query returned more than one result: d=\"nodatabase\" q=\"<dummy query>\""),
		},
		queryCase{
			Name: "NoSeries",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return &client.Response{
					Results: []client.Result{
						client.Result{
							Series: []models.Row{},
						},
					},
					Err: "",
				}, nil
			},
			ExpectedVal: []*Point{},
			ExpectedErr: nil,
		},
		queryCase{
			Name: "MultipleSeries",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return &client.Response{
					Results: []client.Result{
						client.Result{
							Series: []models.Row{

								models.Row{
									Columns: []string{"time", "value"},
									Values: [][]interface{}{
										[]interface{}{
											interface{}(12345),
											interface{}(json.Number("1.5")),
										},
									},
								},
								models.Row{
									Columns: []string{"time", "value"},
									Values: [][]interface{}{
										[]interface{}{
											interface{}(12345),
											interface{}(json.Number("3.5")),
										},
									},
								},
							},
						},
					},
					Err: "",
				}, nil
			},
			ExpectedVal: []*Point{
				&Point{
					Tags:  nil,
					Value: json.Number("1.5"),
				},
				&Point{
					Tags:  nil,
					Value: json.Number("3.5"),
				},
			},
			ExpectedErr: nil,
		},
		queryCase{
			Name: "NotEnoughCols",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return &client.Response{
					Results: []client.Result{
						client.Result{
							Series: []models.Row{
								models.Row{
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
					Err: "",
				}, nil
			},
			ExpectedVal: nil,
			ExpectedErr: fmt.Errorf("Invalid data from InfluxDB: Point data does not match column spec:\nCols:\n[value]\nVals:\n[12345 1.004]"),
		},
		queryCase{
			Name: "TooManyCols",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return &client.Response{
					Results: []client.Result{
						client.Result{
							Series: []models.Row{
								models.Row{
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
					Err: "",
				}, nil
			},
			ExpectedVal: nil,
			ExpectedErr: fmt.Errorf("Query returned an incorrect set of columns: \"<dummy query>\" [time label value]"),
		},
		queryCase{
			Name: "NoPoints",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return &client.Response{
					Results: []client.Result{
						client.Result{
							Series: []models.Row{
								models.Row{
									Columns: []string{"time", "value"},
									Values:  [][]interface{}{},
								},
							},
						},
					},
					Err: "",
				}, nil
			},
			ExpectedVal: nil,
			ExpectedErr: fmt.Errorf("Query returned no points: \"<dummy query>\""),
		},
		queryCase{
			Name: "GoodData",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return &client.Response{
					Results: []client.Result{
						client.Result{
							Series: []models.Row{
								models.Row{
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
					Err: "",
				}, nil
			},
			ExpectedVal: []*Point{
				&Point{
					Tags:  nil,
					Value: json.Number("1.5"),
				},
			},
			ExpectedErr: nil,
		},
		queryCase{
			Name: "GoodWithSequenceNumber",
			QueryFunc: func(q client.Query) (*client.Response, error) {
				return &client.Response{
					Results: []client.Result{
						client.Result{
							Series: []models.Row{
								models.Row{
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
					Err: "",
				}, nil
			},
			ExpectedVal: []*Point{
				&Point{
					Tags:  nil,
					Value: json.Number("1.5"),
				},
			},
			ExpectedErr: nil,
		},
	}

	errorStr := "Case %s:\nExpected:\n%v\nActual:\n%v"
	for _, c := range cases {
		client := Client{
			Database:     "nodatabase",
			influxClient: dummyClient{c.QueryFunc},
		}
		res, err := client.Query(client.Database, "<dummy query>")
		assert.Equal(t, c.ExpectedErr, err, fmt.Sprintf(errorStr, c.Name, c.ExpectedErr, err))
		if err != nil {
			continue
		}
		testutils.AssertDeepEqual(t, res, c.ExpectedVal)
	}

}
