package rules

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"go.skia.org/infra/alertserver/go/alerting"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/sklog"
)

/*
	Rules for triggering alerts.
*/

// Rule is an object used for triggering Alerts.
type Rule struct {
	Name           string        `json:"name"`
	Database       string        `json:"database"`
	Query          string        `json:"query"`
	EmptyResultsOk bool          `json:"emptyResultsOk"`
	Category       string        `json:"category"`
	Conditions     []string      `json:"conditions"`
	Message        string        `json:"message"`
	Nag            time.Duration `json:"nag"`
	client         queryable
	AutoDismiss    int64 `json:"autoDismiss"`
	Actions        []string
}

// Alerter is a target for adding alerts.
type Alerter interface {
	// AddAlert inserts the given Alert into the Alerter, if one does not
	// already exist for its rule, and fires its actions if inserted.
	AddAlert(a *alerting.Alert) error
}

// The first three return values of a clause will be loaded into x, y z.  If there are fewer than
// three return values, the others will just be undefined.
var CONDITION_VARIABLES = []string{"x", "y", "z"}

func formatMsg(msg string, tags map[string]string) string {
	rv := msg
	for k, v := range tags {
		rv = strings.Replace(rv, fmt.Sprintf("%%(%s)s", k), v, -1)
	}
	return rv
}

// Fire causes the Alert to become Active() and not Snoozed(), and causes each
// action to be performed. Active Alerts do not perform new queries.
func (r *Rule) fire(am Alerter, tags map[string]string) error {
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

func (r *Rule) queryExecutionAlert(queryErr error, am Alerter) error {
	actions, err := alerting.ParseActions([]string{"Email(infra-alerts@skia.org)"})
	if err != nil {
		return err
	}
	name := "Failed to execute query"
	msg := fmt.Sprintf("Failed to execute query for rule \"%s\": [ %s ]  The logs might provide more information: http://104.154.112.114:10115/alertserver.ERROR?page_y=end", r.Name, r.Query)
	sklog.Errorf("%s\nFull error:\n%v", msg, queryErr)
	return am.AddAlert(&alerting.Alert{
		Name:        name,
		Category:    alerting.INFRA_ALERT,
		Message:     msg,
		Nag:         int64(1 * time.Hour),
		AutoDismiss: int64(15 * time.Minute),
		Actions:     actions,
	})
}

func (r *Rule) queryEvaluationAlert(queryErr error, am Alerter) error {
	actions, err := alerting.ParseActions([]string{"Email(infra-alerts@skia.org)"})
	if err != nil {
		return err
	}
	name := "Failed to evaluate query"
	msg := fmt.Sprintf("Failed to evaluate query for rule \"%s\": [ %s ]", r.Name, r.Conditions)
	sklog.Errorf("%s\nFull error:\n%v", msg, queryErr)
	return am.AddAlert(&alerting.Alert{
		Name:        name,
		Category:    alerting.INFRA_ALERT,
		Message:     msg,
		Nag:         int64(1 * time.Hour),
		AutoDismiss: int64(15 * time.Minute),
		Actions:     actions,
	})
}

func (r *Rule) tick(am Alerter) error {
	res, err := executeQuery(r.client, r.Database, r.Query, len(r.Conditions))
	if err != nil {
		// We shouldn't fail to execute a query. Trigger an alert.
		return r.queryExecutionAlert(err, am)
	}
	if len(res) == 0 && !r.EmptyResultsOk {
		return r.queryExecutionAlert(fmt.Errorf("Query returned no series: %q", r.Query), am)
	}
	// Evaluate the query comparison for each returned value.
	for _, v := range res {
		values := v.Values
		xf := make([]float64, 0, len(values))
		for _, vf := range values {
			f, err := vf.Float64()
			if err != nil {
				return r.queryEvaluationAlert(err, am)
			}
			xf = append(xf, f)
		}

		doAlert, err := r.evaluate(xf)
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

// evaluate takes a slice of floats to represent the value(s) returned from the query.  It creates
// variables (e.g. 'x', 'y', etc) in scope of a go program and then attempts to evaluate the
// conditions.  It returns the result of anding all of the results of the conditions or an error.
func (r *Rule) evaluate(xf []float64) (bool, error) {
	if len(xf) > len(CONDITION_VARIABLES) {
		return false, fmt.Errorf("Too many return values for query.  We support a max of %d (%q), but there were %d (%f)", len(CONDITION_VARIABLES), CONDITION_VARIABLES, len(xf), xf)
	}
	pkg := types.NewPackage("evaluateme", "evaluateme")
	result := true
	// Load all the inputs into CONDITION_VARIABLES
	for i, f := range xf {
		v := constant.MakeFloat64(f)
		pkg.Scope().Insert(types.NewConst(0, pkg, CONDITION_VARIABLES[i], types.Typ[types.Float64], v))
	}

	for _, condition := range r.Conditions {
		tv, err := types.Eval(token.NewFileSet(), pkg, token.NoPos, condition)
		if err != nil {
			return false, fmt.Errorf("Failed to evaluate condition %q: %s", condition, err)
		}
		if tv.Type.String() != "untyped bool" {
			return false, fmt.Errorf("Rule \"%v\" does not return boolean type.", condition)
		}
		result = constant.BoolVal(tv.Value) && result
	}
	return result, nil
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
	emptyResultsOk, ok := r["empty-results-ok"].(bool)
	if !ok {
		emptyResultsOk = false
	}
	category, ok := r["category"].(string)
	if !ok {
		return nil, fmt.Errorf(errString, "category")
	}
	conditionsInterface, ok := r["conditions"]
	if !ok {
		return nil, fmt.Errorf(errString, "conditions")
	}
	conditionsInterfaceList := conditionsInterface.([]interface{})
	conditionsStrings := make([]string, 0, len(conditionsInterfaceList))
	for _, iface := range conditionsInterfaceList {
		conditionsStrings = append(conditionsStrings, iface.(string))
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
	numQueryReturns := countNumQueryReturns(query)
	if numQueryReturns > len(CONDITION_VARIABLES) {
		return nil, fmt.Errorf("Too many return values in query %q.  We only support %d variables and found %d return values", query, len(CONDITION_VARIABLES), numQueryReturns)
	}
	for i := numQueryReturns; i < len(CONDITION_VARIABLES); i++ {
		undefinedVar := CONDITION_VARIABLES[i]
		for _, condition := range conditionsStrings {
			if j := strings.Index(condition, undefinedVar); j != -1 {
				return nil, fmt.Errorf("Failed to evaluate condition %q: eval:1:%d: undeclared name: %s", condition, j+1, undefinedVar)
			}
		}
	}

	rule := Rule{
		Name:           name,
		Database:       database,
		Query:          query,
		EmptyResultsOk: emptyResultsOk,
		Category:       category,
		Conditions:     conditionsStrings,
		Message:        message,
		Nag:            nagDuration,
		client:         client,
		AutoDismiss:    dismissInterval,
		Actions:        actionStrings,
	}
	// Verify that the condition can be evaluated.
	_, err := rule.evaluate(make([]float64, len(CONDITION_VARIABLES)))
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

var commaCounter = regexp.MustCompile("(?i)select(.*)from")

// countNumQueryReturns returns a heuristic-based guess as to how many return values the
// given influxdb query has.  It basically counts the number of commas between select and from.
func countNumQueryReturns(query string) int {
	q := commaCounter.FindStringSubmatch(query)
	if q == nil {
		// This should never happen.  An error will be thrown because "x" will not be defined.
		return 0
	}
	commas := strings.Count(q[0], ",")
	// # of commas == # of variables - 1, so we add one to offset
	return commas + 1
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

func MakeRules(cfgFile string, dbClient *influxdb.Client, tickInterval time.Duration, am Alerter, testing bool) ([]*Rule, error) {
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
	if testing {
		return nil, nil
	}

	// Start the goroutines.
	rv := make([]*Rule, 0, len(rules))
	for _, r := range rules {
		rv = append(rv, r)
		go func(rule *Rule) {
			if err := rule.tick(am); err != nil {
				sklog.Error(err)
			}
			for _ = range time.Tick(tickInterval) {
				if err := rule.tick(am); err != nil {
					sklog.Error(err)
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
	Query(string, string, int) ([]*influxdb.Point, error)
}

func executeQuery(c queryable, database, q string, numConditions int) ([]*influxdb.Point, error) {
	return c.Query(database, q, numConditions)
}
