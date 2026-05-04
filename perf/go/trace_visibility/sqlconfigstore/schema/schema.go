package schema

// PublicTraceRulesSchema represents the PublicTraceRules table in the database.
type PublicTraceRulesSchema struct {
	RuleExpression string `sql:"public_rule_expr TEXT PRIMARY KEY"`
}
