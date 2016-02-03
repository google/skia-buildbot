package rules

import (
	"fmt"
	"go/token"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/alertserver/go/alerting"
	"go.skia.org/infra/go/influxdb"
	"golang.org/x/tools/go/exact"
	"golang.org/x/tools/go/types"
)

/*
	Rules for triggering alerts.
*/

// Rule is an object used for triggering Alerts.
type Rule struct {
	Name        string        `json:"name"`
	Database    string        `json:"database"`
	Query       string        `json:"query"`
	Category    string        `json:"category"`
	Condition   string        `json:"condition"`
	Message     string        `json:"message"`
	Nag         time.Duration `json:"nag"`
	client      queryable
	AutoDismiss int64 `json:"autoDismiss"`
	Actions     []string
}

func formatMsg(msg string, tags map[string]string) string {
	rv := msg
	for k, v := range tags {
		rv = strings.Replace(rv, fmt.Sprintf("%%(%s)s", k), v, -1)
	}
	return rv
}

// Fire causes the Alert to become Active() and not Snoozed(), and causes each
// action to be performed. Active Alerts do not perform new queries.
func (r *Rule) fire(am *alerting.AlertManager, tags map[string]string) error {
	actions, err := alerting.ParseActions(r.Actions)
	if err != nil {
		return fmt.Errorf("Could not fire alert: %v", err)
	}
	a := alerting.Alert{
		Name:        formatMsg(r.Name, tags),
		Category:    r.Category,
		Message:     formatMsg(r.Message, tags),
		Nag:         int64(r.Nag),
		AutoDismiss: r.AutoDismiss,
		Actions:     actions,
	}
	return am.AddAlert(&a)
}

func (r *Rule) queryExecutionAlert(queryErr error, am *alerting.AlertManager) error {
	actions, err := alerting.ParseActions([]string{"Email(infra-alerts@skia.org)"})
	if err != nil {
		return err
	}
	name := "Failed to execute query"
	msg := fmt.Sprintf("Failed to execute query for rule \"%s\": [ %s ]", r.Name, r.Query)
	glog.Errorf("%s\nFull error:\n%v", msg, queryErr)
	return am.AddAlert(&alerting.Alert{
		Name:        name,
		Category:    alerting.INFRA_ALERT,
		Message:     msg,
		Nag:         int64(1 * time.Hour),
		AutoDismiss: int64(15 * time.Minute),
		Actions:     actions,
	})
}

func (r *Rule) queryEvaluationAlert(queryErr error, am *alerting.AlertManager) error {
	actions, err := alerting.ParseActions([]string{"Email(infra-alerts@skia.org)"})
	if err != nil {
		return err
	}
	name := "Failed to evaluate query"
	msg := fmt.Sprintf("Failed to evaluate query for rule \"%s\": [ %s ]", r.Name, r.Condition)
	glog.Errorf("%s\nFull error:\n%v", msg, queryErr)
	return am.AddAlert(&alerting.Alert{
		Name:        name,
		Category:    alerting.INFRA_ALERT,
		Message:     msg,
		Nag:         int64(1 * time.Hour),
		AutoDismiss: int64(15 * time.Minute),
		Actions:     actions,
	})
}

func (r *Rule) tick(am *alerting.AlertManager) error {
	res, err := executeQuery(r.client, r.Database, r.Query)
	if err != nil {
		// We shouldn't fail to execute a query. Trigger an alert.
		return r.queryExecutionAlert(err, am)
	}
	// Evaluate the query comparison for each returned value.
	for _, v := range res {
		f, err := v.Value.Float64()
		if err != nil {
			return r.queryEvaluationAlert(err, am)
		}
		doAlert, err := r.evaluate(f)
		if err != nil {
			return r.queryEvaluationAlert(err, am)
		}
		if doAlert {
			if err := r.fire(am, v.Tags); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Rule) evaluate(d float64) (bool, error) {
	pkg := types.NewPackage("evaluateme", "evaluateme")
	v := exact.MakeFloat64(d)
	pkg.Scope().Insert(types.NewConst(0, pkg, "x", types.Typ[types.Float64], v))
	tv, err := types.Eval(token.NewFileSet(), pkg, token.NoPos, r.Condition)
	if err != nil {
		return false, fmt.Errorf("Failed to evaluate condition %q: %s", r.Condition, err)
	}
	if tv.Type.String() != "untyped bool" {
		return false, fmt.Errorf("Rule \"%v\" does not return boolean type.", r.Condition)
	}
	return exact.BoolVal(tv.Value), nil
}

type parsedRule map[string]interface{}

func newRule(r parsedRule, client *influxdb.Client, testing bool, tickInterval time.Duration) (*Rule, error) {
	errString := "Alert rule missing field %q"
	name, ok := r["name"].(string)
	if !ok {
		return nil, fmt.Errorf(errString, "name")
	}
	query, ok := r["query"].(string)
	if !ok {
		return nil, fmt.Errorf(errString, "query")
	}
	category, ok := r["category"].(string)
	if !ok {
		return nil, fmt.Errorf(errString, "category")
	}
	condition, ok := r["condition"].(string)
	if !ok {
		return nil, fmt.Errorf(errString, "condition")
	}
	database, ok := r["database"].(string)
	if !ok {
		return nil, fmt.Errorf(errString, "database")
	}
	message, ok := r["message"].(string)
	if !ok {
		return nil, fmt.Errorf(errString, "message")
	}
	autoDismiss, ok := r["auto-dismiss"].(bool)
	if !ok {
		return nil, fmt.Errorf(errString, "auto-dismiss")
	}
	dismissInterval := int64(0)
	if autoDismiss {
		dismissInterval = int64(10 * tickInterval)
	}
	actionsInterface, ok := r["actions"]
	if !ok {
		return nil, fmt.Errorf(errString, "actions")
	}
	actionsInterfaceList := actionsInterface.([]interface{})
	actionStrings := make([]string, 0, len(actionsInterfaceList))
	for _, iface := range actionsInterfaceList {
		actionStrings = append(actionStrings, iface.(string))
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
	rule := Rule{
		Name:        name,
		Database:    database,
		Query:       query,
		Category:    category,
		Condition:   condition,
		Message:     message,
		Nag:         nagDuration,
		client:      client,
		AutoDismiss: dismissInterval,
		Actions:     actionStrings,
	}
	// Verify that the condition can be evaluated.
	_, err := rule.evaluate(0.0)
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

func MakeRules(cfgFile string, dbClient *influxdb.Client, tickInterval time.Duration, am *alerting.AlertManager, testing bool) ([]*Rule, error) {
	parsedRules, err := parseAlertRules(cfgFile)
	if err != nil {
		return nil, err
	}
	rules := map[string]*Rule{}
	for _, r := range parsedRules {
		r, err := newRule(r, dbClient, testing, tickInterval)
		if err != nil {
			return nil, err
		}
		if _, ok := rules[r.Name]; ok {
			return nil, fmt.Errorf("Found multiple rules with the same name: %s", r.Name)
		}
		rules[r.Name] = r
	}

	// Start the goroutines.
	rv := make([]*Rule, 0, len(rules))
	for _, r := range rules {
		rv = append(rv, r)
		go func(rule *Rule) {
			if err := rule.tick(am); err != nil {
				glog.Error(err)
			}
			for _ = range time.Tick(tickInterval) {
				if err := rule.tick(am); err != nil {
					glog.Error(err)
				}
			}
		}(r)
	}

	return rv, nil
}

type queryable interface {
	// Query sends a query to the database and returns a slice of points
	// along with any error. The parameters are the name of the database
	// and the query to perform.
	Query(string, string) ([]*influxdb.Point, error)
}

func executeQuery(c queryable, database, q string) ([]*influxdb.Point, error) {
	return c.Query(database, q)
}
