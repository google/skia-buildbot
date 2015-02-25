package alerting

import (
	"fmt"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/skia-dev/influxdb/client"
)

type mockClient struct {
	mockQuery func(string) ([]*client.Series, error)
}

func (c mockClient) Query(query string, precision ...client.TimePrecision) ([]*client.Series, error) {
	return c.mockQuery(query)
}

func getRule() *Rule {
	r := &Rule{
		Name:      "TestRule",
		Query:     "DummyQuery",
		Message:   "Dummy query meets dummy condition!",
		Condition: "x > 0",
		client: &mockClient{func(string) ([]*client.Series, error) {
			s := client.Series{
				Name:    "Results",
				Columns: []string{"time", "value"},
				Points:  [][]interface{}{[]interface{}{1234567, 1.0}},
			}
			return []*client.Series{&s}, nil
		}},
		AutoDismiss: false,
		actions:     nil,
	}
	r.actions = []Action{NewPrintAction(r)}
	return r
}

func getAlert() *Alert {
	a := &Alert{
		Id:            0,
		lastTriggered: time.Time{},
		snoozedUntil:  time.Time{},
		Comments:      []*Comment{},
		Rule:          getRule(),
	}
	return a
}

func TestRule(t *testing.T) {
	// TODO(borenet): This test is really racy. Is there a good fix?
	r := getRule()
	if r.activeAlert != nil {
		t.Errorf("Alert is active before firing.")
	}
	r.tick()
	time.Sleep(10 * time.Millisecond)
	if r.activeAlert == nil {
		t.Errorf("Alert did not fire as expected.")
	}
	r.activeAlert.snoozedUntil = time.Now().Add(30 * time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	if !r.activeAlert.Snoozed() {
		t.Errorf("Alert did not snooze as expected.")
	}
	// Wait for alert to dismiss itself.
	time.Sleep(50 * time.Millisecond)
	r.tick()
	if r.activeAlert != nil {
		t.Errorf("Alert did not dismiss itself after snooze period ended.")
	}
}

func TestAutoDismiss(t *testing.T) {
	r := getRule()
	r.AutoDismiss = true
	if r.activeAlert != nil {
		t.Errorf("Alert is active before firing.")
	}
	r.tick()
	time.Sleep(10 * time.Millisecond)
	if r.activeAlert == nil {
		t.Errorf("Alert did not fire as expected.")
	}
	// Hack the condition so that it's no longer true with the fake query results.
	r.Condition = "x > 10"
	r.tick()
	time.Sleep(10 * time.Millisecond)
	if r.activeAlert != nil {
		t.Errorf("Alert did not auto-dismiss.")
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
					Columns: []string{"time"},
					Points:  [][]interface{}{[]interface{}{}},
				}
				return []*client.Series{&s}, nil
			},
			ExpectedVal: 0.0,
			ExpectedErr: fmt.Errorf("Query returned an incorrect set of columns: \"<dummy query>\" [time]"),
		},
		queryCase{
			Name: "TooManyCols",
			QueryFunc: func(q string) ([]*client.Series, error) {
				s := client.Series{
					Name:    "NotEnoughCols",
					Columns: []string{"time", "value", "extraCol"},
					Points:  [][]interface{}{[]interface{}{}},
				}
				return []*client.Series{&s}, nil
			},
			ExpectedVal: 0.0,
			ExpectedErr: fmt.Errorf("Query returned an incorrect set of columns: \"<dummy query>\" [time value extraCol]"),
		},
		queryCase{
			Name: "ColsPointsMismatch",
			QueryFunc: func(q string) ([]*client.Series, error) {
				s := client.Series{
					Name:    "BadData",
					Columns: []string{"time", "value"},
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
					Columns: []string{"time", "value"},
					Points:  [][]interface{}{[]interface{}{"mean", 1.5}},
				}
				return []*client.Series{&s}, nil
			},
			ExpectedVal: 1.5,
			ExpectedErr: nil,
		},
		queryCase{
			Name: "GoodWithSequenceNumber",
			QueryFunc: func(q string) ([]*client.Series, error) {
				s := client.Series{
					Name:    "GoodData",
					Columns: []string{"time", "sequence_number", "value"},
					Points:  [][]interface{}{[]interface{}{1234567, 10, 1.5}},
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
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
query = "select mean(value) from random_bits where time > now() - 5s"
condition = "x > 0.5"
actions = ["Print"]
auto-dismiss = false
`,
			ExpectedErr: nil,
		},
		parseCase{
			Name: "NoName",
			Input: `[[rule]]
message = "randombits generates more 1's than 0's in last 5 seconds"
query = "select mean(value) from random_bits where time > now() - 5s"
condition = "x > 0.5"
actions = ["Print"]
auto-dismiss = false
`,
			ExpectedErr: fmt.Errorf("Alert rule missing field \"name\""),
		},
		parseCase{
			Name: "NoQuery",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
condition = "x > 0.5"
actions = ["Print"]
auto-dismiss = false
`,
			ExpectedErr: fmt.Errorf("Alert rule missing field \"query\""),
		},
		parseCase{
			Name: "NoCondition",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
query = "select mean(value) from random_bits where time > now() - 5s"
actions = ["Print"]
auto-dismiss = false
`,
			ExpectedErr: fmt.Errorf("Alert rule missing field \"condition\""),
		},
		parseCase{
			Name: "NoActions",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
query = "select mean(value) from random_bits where time > now() - 5s"
condition = "x > 0.5"
auto-dismiss = false
`,
			ExpectedErr: fmt.Errorf("Alert rule missing field \"actions\""),
		},
		parseCase{
			Name: "UnknownAction",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
query = "select mean(value) from random_bits where time > now() - 5s"
condition = "x > 0.5"
actions = ["Print", "UnknownAction"]
auto-dismiss = false
`,
			ExpectedErr: fmt.Errorf("Unknown action: \"UnknownAction\""),
		},
		parseCase{
			Name: "BadCondition",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
query = "select mean(value) from random_bits where time > now() - 5s"
condition = "x > y"
actions = ["Print"]
auto-dismiss = false
`,
			ExpectedErr: fmt.Errorf("Failed to evaluate condition \"x > y\": 1:1: undeclared name: y"),
		},
		parseCase{
			Name: "NoMessage",
			Input: `[[rule]]
name = "randombits"
query = "select mean(value) from random_bits where time > now() - 5s"
condition = "x > 0.5"
actions = ["Print"]
auto-dismiss = false
`,
			ExpectedErr: fmt.Errorf("Alert rule missing field \"message\""),
		},
		parseCase{
			Name: "NoAutoDismiss",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
query = "select mean(value) from random_bits where time > now() - 5s"
condition = "x > 0.5"
actions = ["Print"]
`,
			ExpectedErr: fmt.Errorf("Alert rule missing field \"auto-dismiss\""),
		},
		parseCase{
			Name: "Nag",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
query = "select mean(value) from random_bits where time > now() - 5s"
condition = "x > 0.5"
actions = ["Print"]
auto-dismiss = false
nag = "1h10m"
`,
			ExpectedErr: nil,
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
		_, err = newRule(cfg.Rule[0], nil, nil, false)
		actualErrStr := "nil"
		if err != nil {
			actualErrStr = err.Error()
		}
		if actualErrStr != expectedErrStr {
			t.Errorf(errorStr, c.Name, expectedErrStr, actualErrStr)
		}
	}
}
