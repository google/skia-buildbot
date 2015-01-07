package alerting

import (
	"fmt"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/golang/glog"
	"github.com/influxdb/influxdb/client"
	"golang.org/x/tools/go/exact"
	"golang.org/x/tools/go/types"
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

// Comment is an object representing a comment on an alert.
type Comment struct {
	Time    time.Time
	User    string
	Message string
}

// Alert is an object which represents an active alert.
type Alert struct {
	Id            int64
	lastTriggered time.Time
	snoozedUntil  time.Time
	dismissedAt   time.Time
	Comments      []*Comment
	Rule          *Rule
}

// addComment adds a comment about the alert.
func (a *Alert) addComment(c *Comment) {
	a.Comments = append(a.Comments, c)
	for _, action := range a.Rule.actions {
		go action.Followup(c.Message)
	}
}

// Snoozed indicates whether the Alert has been Snoozed.
func (a *Alert) Snoozed() bool {
	return a.snoozedUntil != time.Time{}
}

// Triggered gives the time when the alert was triggered.
func (a Alert) Triggered() time.Time {
	return a.lastTriggered
}

// SnoozedUntil gives the time until which the alert is snoozed.
func (a Alert) SnoozedUntil() time.Time {
	return a.snoozedUntil
}

// Rule is an object used for triggering Alerts.
type Rule struct {
	Id          string
	Name        string
	Query       string
	Condition   string
	Message     string
	nag         time.Duration
	client      queryable
	autoDismiss bool
	actions     []Action
	activeAlert *Alert
}

// Fire causes the Alert to become Active() and not Snoozed(), and causes each
// action to be performed. Active Alerts do not perform new queries.
func (r *Rule) fire() {
	a := Alert{
		Id:            0,
		lastTriggered: time.Now().UTC(),
		Comments:      []*Comment{},
		Rule:          r,
	}
	r.activeAlert = &a
	for _, action := range r.actions {
		go action.Fire()
	}
}

func (r *Rule) tick() {
	if r.activeAlert != nil && r.activeAlert.Snoozed() {
		if r.activeAlert.snoozedUntil.Before(time.Now().UTC()) {
			r.activeAlert.addComment(&Comment{
				Time:    time.Now().UTC(),
				User:    "AlertServer",
				Message: "Dismissing; snooze period expired.",
			})
			r.activeAlert = nil
		}
	} else if r.autoDismiss || r.activeAlert == nil {
		glog.Infof("Executing query [%s]", r.Query)
		d, err := executeQuery(r.client, r.Query)
		if err != nil {
			glog.Error(err)
			return
		}
		glog.Infof("Query [%s] returned %v", r.Query, d)
		doAlert, err := r.evaluate(d)
		if err != nil {
			glog.Error(err)
			return
		}
		if r.activeAlert != nil && r.autoDismiss && !doAlert {
			r.activeAlert.addComment(&Comment{
				Time:    time.Now().UTC(),
				User:    "AlertServer",
				Message: "Auto-dismissing; condition is no longer true.",
			})
			r.activeAlert = nil
		} else if r.activeAlert == nil && doAlert {
			r.fire()
		}
	}
	r.maybeNag()
}

func (r *Rule) maybeNag() {
	a := r.activeAlert
	if a != nil && !a.Snoozed() && r.nag != time.Duration(0) {
		lastMsgTime := a.Triggered()
		if len(a.Comments) > 0 {
			lastMsgTime = a.Comments[len(a.Comments)-1].Time
		}
		if time.Since(lastMsgTime) > r.nag {
			a.addComment(&Comment{
				Time:    time.Now().UTC(),
				User:    "AlertServer",
				Message: fmt.Sprintf(NAG_MSG_TMPL, r.nag.String()),
			})
		}
	}
}

func (r *Rule) evaluate(d float64) (bool, error) {
	pkg := types.NewPackage("evaluateme", "evaluateme")
	v := exact.MakeFloat64(d)
	pkg.Scope().Insert(types.NewConst(0, pkg, "x", types.Typ[types.Float64], v))
	typ, val, err := types.Eval(r.Condition, pkg, pkg.Scope())
	if err != nil {
		return false, fmt.Errorf("Failed to evaluate condition %q: %s", r.Condition, err)
	}
	if typ.String() != "untyped bool" {
		return false, fmt.Errorf("Rule \"%v\" does not return boolean type.", r.Condition)
	}
	return exact.BoolVal(val), nil
}

type parsedRule map[string]interface{}

func newRule(r parsedRule, client *client.Client, emailAuth *email.GMail, testing bool) (*Rule, error) {
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
	rule := Rule{
		Id:          id,
		Name:        name,
		Query:       query,
		Condition:   condition,
		Message:     message,
		nag:         nagDuration,
		client:      client,
		autoDismiss: autoDismiss,
		actions:     nil,
		activeAlert: nil,
	}
	if err := rule.parseActions(actionsInterface, emailAuth, testing); err != nil {
		return nil, err
	}
	// Verify that the condition can be evaluated.
	_, err = rule.evaluate(0.0)
	if err != nil {
		return nil, err
	}
	return &rule, nil
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

func makeRules(cfgFile string, dbClient *client.Client, emailAuth *email.GMail, testing bool) ([]*Rule, error) {
	parsedRules, err := parseAlertRules(cfgFile)
	if err != nil {
		return nil, err
	}
	rules := []*Rule{}
	for _, r := range parsedRules {
		r, err := newRule(r, dbClient, emailAuth, testing)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}
