package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/jmoiron/sqlx"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/alertserver/go/alerting"
	"go.skia.org/infra/go/database/testutil"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/testutils"
)

type mockClient struct {
	Result []*influxdb.Point
}

func (c mockClient) Query(database, query string, n int) ([]*influxdb.Point, error) {
	return c.Result, nil
}

func getRule() *Rule {
	r := &Rule{
		Name:       "TestRule",
		Database:   "DefaultDatabase",
		Query:      "DummyQuery",
		Category:   "testing",
		Message:    "Dummy query meets dummy conditions!",
		Conditions: []string{"x > 0", "y > 10 * x", "x < 4"},
		client: &mockClient{
			Result: []*influxdb.Point{
				&influxdb.Point{
					Tags: map[string]string{
						"tagKey": "tagValue",
					},
					Values: []json.Number{"1.0", "15.0"},
				},
			},
		},
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
	DB, err := sqlx.Open("mysql", conf.MySQLString())
	assert.NoError(t, err, failMsg)
	alerting.DB = DB

	return testDb
}

func TestRuleTriggeringE2E(t *testing.T) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)
	d := clearDB(t)
	defer d.Close(t)

	am, err := alerting.MakeAlertManager(50*time.Millisecond, nil)
	assert.NoError(t, err)

	getAlerts := func() []*alerting.Alert {
		b := bytes.NewBuffer([]byte{})
		assert.NoError(t, am.WriteActiveAlertsJson(b, func(*alerting.Alert) bool { return true }))
		var active []*alerting.Alert
		assert.NoError(t, json.Unmarshal(b.Bytes(), &active))
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
	assert.NoError(t, r.tick(am))
	getAlert()

	// Ensure that the rule auto-dismisses.
	// Hack the conditions so that it's no longer true with the fake query results.
	r.Conditions = []string{"x > 2", "y > 10 * x", "x < 4"}
	assert.NoError(t, r.tick(am))
	time.Sleep(2 * time.Second)
	assert.Equal(t, 0, len(getAlerts()))

	// Stop the AlertManager.
	am.Stop()
}

type mockAlerter struct {
	Alerts []*alerting.Alert
}

func (m *mockAlerter) AddAlert(a *alerting.Alert) error {
	m.Alerts = append(m.Alerts, a)
	return nil
}

func TestEmptyResultsError(t *testing.T) {
	testutils.SmallTest(t)
	am := &mockAlerter{}

	// Ensure that the rule triggers an alert when results are empty.
	r := &Rule{
		Name:       "TestRule",
		Database:   "DefaultDatabase",
		Query:      "DummyQuery",
		Category:   "testing",
		Message:    "Dummy query meets dummy conditions!",
		Conditions: []string{"x > 0", "y > 10 * x", "x < 4"},
		client: &mockClient{
			Result: []*influxdb.Point{},
		},
		AutoDismiss: int64(time.Second),
		Actions:     []string{"Print"},
	}
	assert.NoError(t, r.tick(am))
	assert.Equal(t, 1, len(am.Alerts))
	assert.Equal(t, "Failed to execute query for rule \"TestRule\": [ DummyQuery ]  The logs might provide more information: http://104.154.112.114:10115/alertserver.ERROR?page_y=end", am.Alerts[0].Message)
}

func TestEmptyResultsOk(t *testing.T) {
	testutils.SmallTest(t)
	am := &mockAlerter{}

	// Ensure that the rule does not trigger an alert when EmptyResultsOk is
	// true.
	r := &Rule{
		Name:           "TestRule",
		Database:       "DefaultDatabase",
		Query:          "DummyQuery",
		EmptyResultsOk: true,
		Category:       "testing",
		Message:        "Dummy query meets dummy conditions!",
		Conditions:     []string{"x > 0", "y > 10 * x", "x < 4"},
		client: &mockClient{
			Result: []*influxdb.Point{},
		},
		AutoDismiss: int64(time.Second),
		Actions:     []string{"Print"},
	}
	assert.NoError(t, r.tick(am))
	assert.Equal(t, 0, len(am.Alerts))
}

func TestRuleParsing(t *testing.T) {
	testutils.SmallTest(t)
	type parseCase struct {
		Name           string
		Input          string
		ExpectedErr    error
		ExpectedErr1_6 error
	}
	cases := []parseCase{
		parseCase{
			Name: "Good",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
database = "graphite"
query = "select mean(value) from random_bits where time > now() - 5s"
category = "testing"
conditions = ["x > 0.5"]
actions = ["Print"]
auto-dismiss = false
`,
			ExpectedErr: nil,
		},
		parseCase{
			Name: "GoodTwoVariables",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
database = "graphite"
query = "select mean(value), sum(value) from random_bits where time > now() - 5s"
category = "testing"
conditions = ["x > 0.5", "y < 0.5"]
actions = ["Print"]
auto-dismiss = false
`,
			ExpectedErr: nil,
		},
		parseCase{
			Name: "GoodOneVariableTwice",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
database = "graphite"
query = "select mean(value) from random_bits where time > now() - 5s"
category = "testing"
conditions = ["x > 0.5", "x < 1.0"]
actions = ["Print"]
auto-dismiss = false
nag = "1h10m"
`,
			ExpectedErr: nil,
		},
		parseCase{
			Name: "GoodTwoVariables, One condition",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
database = "graphite"
query = "select max(value), min(value) from random_bits where time > now() - 5s"
category = "testing"
conditions = ["x > y"]
actions = ["Print"]
auto-dismiss = false
`,
			ExpectedErr: nil,
		},
		parseCase{
			Name: "GoodEmptyResultsOk",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
database = "graphite"
query = "select mean(value) from random_bits where time > now() - 5s"
category = "testing"
conditions = ["x > 0.5"]
actions = ["Print"]
auto-dismiss = false
empty-results-ok = true
`,
			ExpectedErr: nil,
		},
		parseCase{
			Name: "NoName",
			Input: `[[rule]]
message = "randombits generates more 1's than 0's in last 5 seconds"
database = "graphite"
query = "select mean(value) from random_bits where time > now() - 5s"
category = "testing"
conditions = ["x > 0.5"]
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
database = "graphite"
category = "testing"
conditions = ["x > 0.5"]
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
database = "graphite"
query = "select mean(value) from random_bits where time > now() - 5s"
category = "testing"
actions = ["Print"]
auto-dismiss = false
`,
			ExpectedErr: fmt.Errorf("Alert rule missing field \"conditions\""),
		},
		parseCase{
			Name: "NoActions",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
database = "graphite"
query = "select mean(value) from random_bits where time > now() - 5s"
category = "testing"
conditions = ["x > 0.5"]
auto-dismiss = false
`,
			ExpectedErr: fmt.Errorf("Alert rule missing field \"actions\""),
		},
		parseCase{
			Name: "BadVariables, 'a' never exists",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
database = "graphite"
query = "select mean(value) from random_bits where time > now() - 5s"
category = "testing"
conditions = ["x > a"]
actions = ["Print"]
auto-dismiss = false
`,
			ExpectedErr1_6: fmt.Errorf("Failed to evaluate condition \"x > a\": eval:1:5: undeclared name: a"),
			// TODO(kjlubick): This behaves differently on golang 1.7 vs 1.6.
			// Consolidate this once all the bots are updated.
			ExpectedErr: fmt.Errorf("Failed to evaluate condition \"x > a\": Unknown error"),
		},
		parseCase{
			Name: "BadVariables, 'z' isn't defined",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
database = "graphite"
query = "select mean(value), sum(value) from random_bits where time > now() - 5s"
category = "testing"
conditions = ["x > y", "x < z"]
actions = ["Print"]
auto-dismiss = false
`,
			ExpectedErr: fmt.Errorf("Failed to evaluate condition \"x < z\": eval:1:5: undeclared name: z"),
		},
		parseCase{
			Name: "TooManyVariables",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
database = "graphite"
query = "select mean(value), sum(value), min(value), max(value) from random_bits where time > now() - 5s"
category = "testing"
conditions = ["x > y", "x < z"]
actions = ["Print"]
auto-dismiss = false
`,
			ExpectedErr: fmt.Errorf(`Too many return values in query "select mean(value), sum(value), min(value), max(value) from random_bits where time > now() - 5s".  We only support 3 variables and found 4 return values`),
		},
		parseCase{
			Name: "NoMessage",
			Input: `[[rule]]
name = "randombits"
database = "graphite"
query = "select mean(value) from random_bits where time > now() - 5s"
category = "testing"
conditions = ["x > 0.5"]
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
category = "testing"
database = "graphite"
query = "select mean(value) from random_bits where time > now() - 5s"
conditions = ["x > 0.5"]
actions = ["Print"]
`,
			ExpectedErr: fmt.Errorf("Alert rule missing field \"auto-dismiss\""),
		},
		parseCase{
			Name: "Nag",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
database = "graphite"
query = "select mean(value) from random_bits where time > now() - 5s"
category = "testing"
conditions = ["x > 0.5"]
actions = ["Print"]
auto-dismiss = false
nag = "1h10m"
`,
			ExpectedErr: nil,
		},
		parseCase{
			Name: "NoCategory",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
database = "graphite"
query = "select mean(value) from random_bits where time > now() - 5s"
conditions = ["x > 0.5"]
actions = ["Print"]
auto-dismiss = false
nag = "1h10m"
`,
			ExpectedErr: fmt.Errorf("Alert rule missing field \"category\""),
		},
		parseCase{
			Name: "NoDatabase",
			Input: `[[rule]]
name = "randombits"
message = "randombits generates more 1's than 0's in last 5 seconds"
query = "select mean(value) from random_bits where time > now() - 5s"
category = "testing"
conditions = ["x > 0.5"]
actions = ["Print"]
auto-dismiss = false
nag = "1h10m"
`,
			ExpectedErr: fmt.Errorf("Alert rule missing field \"database\""),
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
			if c.ExpectedErr1_6 == nil {
				t.Errorf(errorStr, c.Name, expectedErrStr, actualErrStr)
			} else {
				if expectedError2 := c.ExpectedErr1_6.Error(); actualErrStr != expectedError2 {
					t.Errorf(errorStr, c.Name, expectedErrStr+"[ or ]"+expectedError2, actualErrStr)
				}
			}
		}
	}
}
