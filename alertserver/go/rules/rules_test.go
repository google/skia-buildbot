package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/jmoiron/sqlx"
	"github.com/skia-dev/influxdb/client"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/alertserver/go/alerting"
	"go.skia.org/infra/go/database/testutil"
	"go.skia.org/infra/go/testutils"
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
		Category:  "testing",
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
		AutoDismiss: int64(time.Second),
		Actions:     []string{"Print"},
	}
	return r
}

// clearDB initializes the database, upgrading it if needed, and removes all
// data to ensure that the test begins with a clean slate. Returns a MySQLTestDatabase
// which must be closed after the test finishes.
func clearDB(t *testing.T) *testutil.MySQLTestDatabase {
	failMsg := "Database initialization failed. Do you have the test database set up properly?  Details: %v"

	// Set up the database.
	testDb := testutil.SetupMySQLTestDatabase(t, alerting.MigrationSteps())

	conf := testutil.LocalTestDatabaseConfig(alerting.MigrationSteps())
	DB, err := sqlx.Open("mysql", conf.MySQLString)
	assert.Nil(t, err, failMsg)
	alerting.DB = DB

	return testDb
}

func TestRuleTriggeringE2E(t *testing.T) {
	testutils.SkipIfShort(t)
	d := clearDB(t)
	defer d.Close(t)

	am, err := alerting.MakeAlertManager(50*time.Millisecond, nil)
	assert.Nil(t, err)

	getAlerts := func() []*alerting.Alert {
		b := bytes.NewBuffer([]byte{})
		assert.Nil(t, am.WriteActiveAlertsJson(b))
		var active []*alerting.Alert
		assert.Nil(t, json.Unmarshal(b.Bytes(), &active))
		return active
	}
	getAlert := func() *alerting.Alert {
		active := getAlerts()
		assert.Equal(t, 1, len(active))
		return active[0]
	}
	assert.Equal(t, 0, len(getAlerts()))

	// Ensure that the rule triggers an alert.
	r := getRule()
	assert.Nil(t, r.tick(am))
	getAlert()

	// Ensure that the rule auto-dismisses.
	// Hack the condition so that it's no longer true with the fake query results.
	r.Condition = "x > 10"
	assert.Nil(t, r.tick(am))
	time.Sleep(2 * time.Second)
	assert.Equal(t, 0, len(getAlerts()))

	// Stop the AlertManager.
	am.Stop()
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

func TestRuleParsing(t *testing.T) {
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
		_, err = newRule(cfg.Rule[0], nil, false, 10)
		actualErrStr := "nil"
		if err != nil {
			actualErrStr = err.Error()
		}
		if actualErrStr != expectedErrStr {
			t.Errorf(errorStr, c.Name, expectedErrStr, actualErrStr)
		}
	}
}
