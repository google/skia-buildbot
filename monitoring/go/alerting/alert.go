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

const (
	NAG_MSG_TMPL = "This alert has been active for %s since the last update. Please verify that it is still valid and either fix the issue or dismiss/snooze the alert."
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
	valueColumn := 0
	for _, label := range results[0].Columns {
		if label == "time" || label == "sequence_number" {
			valueColumn++
		} else {
			break
		}
	}
	if len(results[0].Columns) != valueColumn+1 {
		return 0.0, fmt.Errorf("Query returned an incorrect set of columns: %q %v", q, results[0].Columns)
	}
	if len(results[0].Columns) != len(points[0]) {
		return 0.0, fmt.Errorf("Invalid data from InfluxDB: Point data does not match column spec.")
	}
	return points[0][valueColumn].(float64), nil
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
	nag           time.Duration
	client        queryable
	autoDismiss   bool
	actions       []Action
	lastTriggered time.Time
	snoozedUntil  time.Time
	lastMsgTime   time.Time
}

// Fire causes the Alert to become Active() and not Snoozed(), and causes each
// action to be performed. Active Alerts do not perform new queries.
func (a *Alert) fire() {
	a.lastTriggered = time.Now()
	a.snoozedUntil = time.Time{}
	a.lastMsgTime = time.Now()
	for _, action := range a.actions {
		go action.Fire()
	}
}

// Followup sends a followup message about the alert.
func (a *Alert) followup(msg string) {
	a.lastMsgTime = time.Now()
	for _, action := range a.actions {
		go action.Followup(msg)
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
			a.dismiss("Dismissing; snooze period expired.")
		}
	} else if a.autoDismiss || !a.Active() {
		glog.Infof("Executing query [%s]", a.Query)
		d, err := executeQuery(a.client, a.Query)
		if err != nil {
			glog.Error(err)
			return
		}
		glog.Infof("Query [%s] returned %v", a.Query, d)
		doAlert, err := a.evaluate(d)
		if err != nil {
			glog.Error(err)
			return
		}
		if a.Active() && a.autoDismiss && !doAlert {
			a.dismiss("Auto-dismissing; condition is no longer true.")
		} else if !a.Active() && doAlert {
			a.fire()
		}
	}
	a.maybeNag()
}

func (a *Alert) maybeNag() {
	if a.Active() && !a.Snoozed() && a.nag != time.Duration(0) && time.Now().Sub(a.lastMsgTime) > a.nag {
		a.followup(fmt.Sprintf(NAG_MSG_TMPL, a.nag.String()))
	}
}

func (a *Alert) dismiss(msg string) {
	a.lastTriggered = time.Time{}
	a.snoozedUntil = time.Time{}
	a.followup(msg)
}

func (a *Alert) snooze(until time.Time, msg string) {
	a.snoozedUntil = until
	a.followup(msg)
}

func (a *Alert) unsnooze(msg string) {
	a.snoozedUntil = time.Time{}
	a.followup(msg)
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

func newAlert(r parsedRule, client *client.Client, emailAuth *email.GMail, testing bool) (*Alert, error) {
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
	autoDismiss, ok := r["auto-dismiss"].(bool)
	if !ok {
		return nil, fmt.Errorf(errString, "auto-dismiss")
	}
	actionsInterface, ok := r["actions"]
	if !ok {
		return nil, fmt.Errorf(errString, "actions")
	}
	nagDuration := time.Duration(0)
	nag, ok := r["nag"].(string)
	if ok {
		var err error
		nagDuration, err = time.ParseDuration(nag)
		if err != nil {
			return nil, fmt.Errorf("Invalid nag duration %q: %v", nag, err)
		}
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
		nag:           nagDuration,
		client:        client,
		autoDismiss:   autoDismiss,
		actions:       nil,
		lastTriggered: time.Time{},
		snoozedUntil:  time.Time{},
	}
	if err := alert.parseActions(actionsInterface, emailAuth, testing); err != nil {
		return nil, err
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

func makeAlerts(cfgFile string, dbClient *client.Client, emailAuth *email.GMail, testing bool) ([]*Alert, error) {
	parsedRules, err := parseAlertRules(cfgFile)
	if err != nil {
		return nil, err
	}
	alerts := []*Alert{}
	for _, r := range parsedRules {
		a, err := newAlert(r, dbClient, emailAuth, testing)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}
