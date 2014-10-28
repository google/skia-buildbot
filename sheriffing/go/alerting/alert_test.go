package alerting

import (
	"fmt"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/influxdb/influxdb/client"
)

type mockClient struct {
	mockQuery func(string) ([]*client.Series, error)
}

func (c mockClient) Query(query string, precision ...client.TimePrecision) ([]*client.Series, error) {
	return c.mockQuery(query)
}

func getAlert() *Alert {
	return &Alert{
		Name:      "TestAlert",
		Query:     "DummyQuery",
		Condition: "x > 0",
		client: &mockClient{func(string) ([]*client.Series, error) {
			s := client.Series{
				Name:    "Results",
				Columns: []string{"func", "value"},
				Points:  [][]interface{}{[]interface{}{"mean", 1.0}},
			}
			return []*client.Series{&s}, nil
		}},
		actions:       []func(*Alert){},
		lastTriggered: time.Time{},
		snoozedUntil:  time.Time{},
	}
}

func TestAlert(t *testing.T) {
	// TODO(borenet): This test is really racy. Is there a good fix?
	a := getAlert()
	if a.Active() {
		t.Errorf("Alert is active before firing.")
	}
	if a.Snoozed() {
		t.Errorf("Alert is snoozed before firing.")
	}
	a.tick()
	time.Sleep(10 * time.Millisecond)
	if !a.Active() {
		t.Errorf("Alert did not fire as expected.")
	}
	a.snooze(time.Now().Add(30 * time.Millisecond))
	time.Sleep(10 * time.Millisecond)
	if !a.Snoozed() {
		t.Errorf("Alert did not snooze as expected.")
	}
	// Wait for alert to wake itself up.
	time.Sleep(50 * time.Millisecond)
	a.tick()
	if a.Active() || a.Snoozed() {
		t.Errorf("Alert did not dismiss itself.")
	}
}

func TestExecuteQuery(t *testing.T) {
	type queryCase struct {
		Name        string
		QueryFunc   func(string) ([]*client.Series, error)
		ExpectedVal float64
		ExpectedErr error
	}
	cases := []queryCase{
		queryCase{
			Name: "QueryFailed",
			QueryFunc: func(q string) ([]*client.Series, error) {
				return nil, fmt.Errorf("<dummy error>")
			},
			ExpectedVal: 0.0,
			ExpectedErr: fmt.Errorf("Failed to query InfluxDB with query \"<dummy query>\": <dummy error>"),
		},
		queryCase{
			Name: "EmptyResults",
			QueryFunc: func(q string) ([]*client.Series, error) {
				return []*client.Series{}, nil
			},
			ExpectedVal: 0.0,
			ExpectedErr: fmt.Errorf("Query returned no data: \"<dummy query>\""),
		},
		queryCase{
			Name: "EmptySeries",
			QueryFunc: func(q string) ([]*client.Series, error) {
				s := client.Series{
					Name:    "Empty",
					Columns: []string{},
					Points:  [][]interface{}{},
				}
				return []*client.Series{&s}, nil
			},
			ExpectedVal: 0.0,
			ExpectedErr: fmt.Errorf("Query returned no points: \"<dummy query>\""),
		},
		queryCase{
			Name: "TooManyPoints",
			QueryFunc: func(q string) ([]*client.Series, error) {
				s := client.Series{
					Name:    "TooMany",
					Columns: []string{"time", "value"},
					Points:  [][]interface{}{[]interface{}{}, []interface{}{}},
				}
				return []*client.Series{&s}, nil
			},
			ExpectedVal: 0.0,
			ExpectedErr: fmt.Errorf("Query returned more than one point: \"<dummy query>\""),
		},
		queryCase{
			Name: "NotEnoughCols",
			QueryFunc: func(q string) ([]*client.Series, error) {
				s := client.Series{
					Name:    "NotEnoughCols",
					Columns: []string{"value"},
					Points:  [][]interface{}{[]interface{}{}},
				}
				return []*client.Series{&s}, nil
			},
			ExpectedVal: 0.0,
			ExpectedErr: fmt.Errorf("Query returned fewer than two columns: \"<dummy query>\""),
		},
		queryCase{
			Name: "TooManyCols",
			QueryFunc: func(q string) ([]*client.Series, error) {
				s := client.Series{
					Name:    "NotEnoughCols",
					Columns: []string{"func", "value", "extraCol"},
					Points:  [][]interface{}{[]interface{}{}},
				}
				return []*client.Series{&s}, nil
			},
			ExpectedVal: 0.0,
			ExpectedErr: fmt.Errorf("Query returned more than two columns: \"<dummy query>\""),
		},
		queryCase{
			Name: "ColsPointsMismatch",
			QueryFunc: func(q string) ([]*client.Series, error) {
				s := client.Series{
					Name:    "BadData",
					Columns: []string{"func", "value"},
					Points:  [][]interface{}{[]interface{}{}},
				}
				return []*client.Series{&s}, nil
			},
			ExpectedVal: 0.0,
			ExpectedErr: fmt.Errorf("Invalid data from InfluxDB: Point data does not match column spec."),
		},
		queryCase{
			Name: "GoodData",
			QueryFunc: func(q string) ([]*client.Series, error) {
				s := client.Series{
					Name:    "GoodData",
					Columns: []string{"func", "value"},
					Points:  [][]interface{}{[]interface{}{"mean", 1.5}},
				}
				return []*client.Series{&s}, nil
			},
			ExpectedVal: 1.5,
			ExpectedErr: nil,
		},
	}
	errorStr := "Case %s:\nExpected:\n%v\nActual:\n%v"
	for _, c := range cases {
		client := mockClient{c.QueryFunc}
		actualErrStr := "nil"
		expectedErrStr := "nil"
		if c.ExpectedErr != nil {
			expectedErrStr = c.ExpectedErr.Error()
		}
		val, err := executeQuery(client, "<dummy query>")
		if err != nil {
			actualErrStr = err.Error()
		}
		if expectedErrStr != actualErrStr {
			t.Errorf(errorStr, c.Name, expectedErrStr, actualErrStr)
		}
		if val != c.ExpectedVal {
			t.Errorf(errorStr, c.Name, c.ExpectedVal, val)
		}
	}
}

func TestAlertParsing(t *testing.T) {
	type parseCase struct {
		Name        string
		Input       string
		ExpectedErr error
	}
	cases := []parseCase{
		parseCase{
			Name: "Good",
			Input: `[[rule]]
name = "randombits generates more 1's than 0's in last 5 seconds"
query = "select mean(value) from random_bits where time > now() - 5s"
condition = "x > 0.5"
actions = ["Print"]
`,
			ExpectedErr: nil,
		},
		parseCase{
			Name: "NoName",
			Input: `[[rule]]
query = "select mean(value) from random_bits where time > now() - 5s"
condition = "x > 0.5"
actions = ["Print"]
`,
			ExpectedErr: fmt.Errorf("Alert rule missing field \"name\""),
		},
		parseCase{
			Name: "NoQuery",
			Input: `[[rule]]
name = "randombits generates more 1's than 0's in last 5 seconds"
condition = "x > 0.5"
actions = ["Print"]
`,
			ExpectedErr: fmt.Errorf("Alert rule missing field \"query\""),
		},
		parseCase{
			Name: "NoCondition",
			Input: `[[rule]]
name = "randombits generates more 1's than 0's in last 5 seconds"
query = "select mean(value) from random_bits where time > now() - 5s"
actions = ["Print"]
`,
			ExpectedErr: fmt.Errorf("Alert rule missing field \"condition\""),
		},
		parseCase{
			Name: "NoActions",
			Input: `[[rule]]
name = "randombits generates more 1's than 0's in last 5 seconds"
query = "select mean(value) from random_bits where time > now() - 5s"
condition = "x > 0.5"
`,
			ExpectedErr: fmt.Errorf("Alert rule missing field \"actions\""),
		},
		parseCase{
			Name: "UnknownAction",
			Input: `[[rule]]
name = "randombits generates more 1's than 0's in last 5 seconds"
query = "select mean(value) from random_bits where time > now() - 5s"
condition = "x > 0.5"
actions = ["Print", "UnknownAction"]
`,
			ExpectedErr: fmt.Errorf("Unknown action: \"UnknownAction\""),
		},
		parseCase{
			Name: "BadCondition",
			Input: `[[rule]]
name = "randombits generates more 1's than 0's in last 5 seconds"
query = "select mean(value) from random_bits where time > now() - 5s"
condition = "x > y"
actions = ["Print"]
`,
			ExpectedErr: fmt.Errorf("Failed to evaluate condition \"x > y\": 1:1: undeclared name: y"),
		},
	}
	errorStr := "Case %s:\nExpected:\n%v\nActual:\n%v"
	for _, c := range cases {
		expectedErrStr := "nil"
		if c.ExpectedErr != nil {
			expectedErrStr = c.ExpectedErr.Error()
		}
		var cfg struct {
			Rule []parsedRule
		}
		_, err := toml.Decode(c.Input, &cfg)
		if err != nil {
			t.Errorf("Failed to parse:\n%v", c.Input)
		}
		_, err = newAlert(cfg.Rule[0], nil, nil)
		actualErrStr := "nil"
		if err != nil {
			actualErrStr = err.Error()
		}
		if actualErrStr != expectedErrStr {
			t.Errorf(errorStr, c.Name, expectedErrStr, actualErrStr)
		}
	}
}
