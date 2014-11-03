package alerting

import (
	"fmt"
	"time"

	"code.google.com/p/go.tools/go/exact"
	"code.google.com/p/go.tools/go/types"
	"github.com/BurntSushi/toml"
	"github.com/golang/glog"
	"github.com/influxdb/influxdb/client"
	"skia.googlesource.com/buildbot.git/go/email"
	"skia.googlesource.com/buildbot.git/go/util"
)

type queryable interface {
	Query(string, ...client.TimePrecision) ([]*client.Series, error)
}

func executeQuery(c queryable, q string) (float64, error) {
	results, err := c.Query(q)
	if err != nil {
		return 0.0, fmt.Errorf("Failed to query InfluxDB with query %q: %s", q, err)
	}
	if len(results) < 1 {
		return 0.0, fmt.Errorf("Query returned no data: %q", q)
	}
	points := results[0].Points
	if len(points) < 1 {
		return 0.0, fmt.Errorf("Query returned no points: %q", q)
	}
	if len(points) > 1 {
		return 0.0, fmt.Errorf("Query returned more than one point: %q", q)
	}
	if len(results[0].Columns) < 2 {
		return 0.0, fmt.Errorf("Query returned fewer than two columns: %q", q)
	}
	if len(results[0].Columns) > 2 {
		return 0.0, fmt.Errorf("Query returned more than two columns: %q", q)
	}
	if len(results[0].Columns) != len(points[0]) {
		return 0.0, fmt.Errorf("Invalid data from InfluxDB: Point data does not match column spec.")
	}
	return points[0][1].(float64), nil
}

// Alert represents a set of actions which are performed when a given database
// query satisfies a given condition. There are a few valid states for Alerts:
//
// A. Active: The Alert has been triggered.
//   1. Snoozed: The alert will dismiss itself after a specified time.
//   2. Not snoozed: The alert needs attention.
// B. Inactive.
type Alert struct {
	Id            string
	Name          string
	Query         string
	Condition     string
	Message       string
	client        queryable
	actions       []func(*Alert)
	lastTriggered time.Time
	snoozedUntil  time.Time
}

// Fire causes the Alert to become Active() and not Snoozed(), and causes each
// action to be performed. Active Alerts do not perform new queries.
func (a *Alert) fire() {
	a.lastTriggered = time.Now()
	a.snoozedUntil = time.Time{}
	for _, f := range a.actions {
		go f(a)
	}
}

// Active indicates whether the Alert has fired.
func (a *Alert) Active() bool {
	return a.lastTriggered != time.Time{}
}

// Snoozed indicates whether the Alert fired sometime in the past and was
// subsequently Snoozed.
func (a *Alert) Snoozed() bool {
	return a.Active() && a.snoozedUntil != time.Time{}
}

// Triggered gives the time when the alert was triggered.
func (a Alert) Triggered() time.Time {
	return a.lastTriggered
}

// SnoozedUntil gives the time until which the alert is snoozed.
func (a Alert) SnoozedUntil() time.Time {
	return a.snoozedUntil
}

func (a *Alert) tick() {
	if a.Snoozed() {
		if a.snoozedUntil.Before(time.Now()) {
			a.dismiss()
		}
	} else if !a.Active() {
		d, err := executeQuery(a.client, a.Query)
		if err != nil {
			glog.Error(err)
			return
		}
		doAlert, err := a.evaluate(d)
		if err != nil {
			glog.Error(err)
			return
		}
		if doAlert {
			a.fire()
		}
	}
}

func (a *Alert) dismiss() {
	a.lastTriggered = time.Time{}
	a.snoozedUntil = time.Time{}
}

func (a *Alert) snooze(until time.Time) {
	a.snoozedUntil = until
}

func (a *Alert) unsnooze() {
	a.snoozedUntil = time.Time{}
}

func (a *Alert) evaluate(d float64) (bool, error) {
	pkg := types.NewPackage("evaluateme", "evaluateme")
	v := exact.MakeFloat64(d)
	pkg.Scope().Insert(types.NewConst(0, pkg, "x", types.Typ[types.Float64], v))
	typ, val, err := types.Eval(a.Condition, pkg, pkg.Scope())
	if err != nil {
		return false, fmt.Errorf("Failed to evaluate condition %q: %s", a.Condition, err)
	}
	if typ.String() != "untyped bool" {
		return false, fmt.Errorf("Rule \"%v\" does not return boolean type.", a.Condition)
	}
	return exact.BoolVal(val), nil
}

type parsedRule map[string]interface{}

func newAlert(r parsedRule, client *client.Client, emailAuth *email.GMail) (*Alert, error) {
	errString := "Alert rule missing field %q"
	name, ok := r["name"].(string)
	if !ok {
		return nil, fmt.Errorf(errString, "name")
	}
	query, ok := r["query"].(string)
	if !ok {
		return nil, fmt.Errorf(errString, "query")
	}
	condition, ok := r["condition"].(string)
	if !ok {
		return nil, fmt.Errorf(errString, "condition")
	}
	message, ok := r["message"].(string)
	if !ok {
		return nil, fmt.Errorf(errString, "message")
	}
	actionsInterface, ok := r["actions"]
	if !ok {
		return nil, fmt.Errorf(errString, "actions")
	}
	actionsList, err := parseActions(actionsInterface, emailAuth)
	if err != nil {
		return nil, err
	}
	id, err := util.GenerateID()
	if err != nil {
		return nil, err
	}
	alert := Alert{
		Id:            id,
		Name:          name,
		Query:         query,
		Condition:     condition,
		Message:       message,
		client:        client,
		actions:       actionsList,
		lastTriggered: time.Time{},
		snoozedUntil:  time.Time{},
	}
	// Verify that the condition can be evaluated.
	_, err = alert.evaluate(0.0)
	if err != nil {
		return nil, err
	}
	return &alert, nil
}

func parseAlertRules(cfgFile string) ([]parsedRule, error) {
	var cfg struct {
		Rule []parsedRule
	}
	_, err := toml.DecodeFile(cfgFile, &cfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse %s: %s", cfgFile, err)
	}
	return cfg.Rule, nil
}

func makeAlerts(cfgFile string, dbClient *client.Client, emailAuth *email.GMail) ([]*Alert, error) {
	parsedRules, err := parseAlertRules(cfgFile)
	if err != nil {
		return nil, err
	}
	alerts := []*Alert{}
	for _, r := range parsedRules {
		a, err := newAlert(r, dbClient, emailAuth)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}
