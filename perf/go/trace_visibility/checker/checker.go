package checker

import (
	"context"
	"strings"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/trace_visibility/provider"
	"go.skia.org/infra/perf/go/trace_visibility/store"
)

// Checker handles the checking of trace visibility rules against the database.
type Checker struct {
	provider provider.Provider
	store    store.Store
}

// NewChecker creates a new Checker instance.
func NewChecker(store store.Store, provider provider.Provider) *Checker {
	return &Checker{
		provider: provider,
		store:    store,
	}
}

func extractRulePrefix(rule string) string {
	idx := strings.Index(rule, "=")
	if idx == -1 {
		return "unknown"
	}
	return rule[:idx+1]
}

// Check fetches visibility config and compares them to the database.
func (c *Checker) Check(ctx context.Context) error {
	sklog.Info("Starting check of visibility rules...")

	dbConfigs, err := c.store.GetAll(ctx)
	if err != nil {
		return skerr.Wrapf(err, "failed to fetch current visibility configs from DB")
	}

	dbRules := make(map[string]bool)
	for _, cfg := range dbConfigs {
		dbRules[cfg.RuleExpression] = true
	}

	expectedRules, err := c.provider.GetExpectedRules(ctx)
	if err != nil {
		return skerr.Wrapf(err, "failed to fetch expected rules")
	}

	missingByRule := make(map[string][]string)
	extraByRule := make(map[string][]string)

	for rule := range expectedRules {
		if !dbRules[rule] {
			rulePrefix := extractRulePrefix(rule)
			missingByRule[rulePrefix] = append(missingByRule[rulePrefix], rule)
		}
	}

	for rule := range dbRules {
		if !expectedRules[rule] {
			rulePrefix := extractRulePrefix(rule)
			extraByRule[rulePrefix] = append(extraByRule[rulePrefix], rule)
		}
	}

	allRules := make(map[string]bool)
	for rule := range expectedRules {
		allRules[extractRulePrefix(rule)] = true
	}
	for rule := range dbRules {
		allRules[extractRulePrefix(rule)] = true
	}

	for rulePrefix := range allRules {
		missing := missingByRule[rulePrefix]
		extra := extraByRule[rulePrefix]
		if len(missing) > 0 || len(extra) > 0 {
			sklog.Infof(
				"Visibility rules diff found for source %q. Missing in DB: %v, Extra in DB: %v",
				rulePrefix,
				missing,
				extra,
			)
		} else {
			sklog.Infof(
				"Successfully verified visibility rules for source %q. No differences found.",
				rulePrefix,
			)
		}

		tagsMissing := map[string]string{"type": "missing", "source": rulePrefix}
		metrics2.GetInt64Metric(
			"perf_visibility_rules_diff",
			tagsMissing).Update(int64(len(missing)))

		tagsExtra := map[string]string{"type": "extra", "source": rulePrefix}
		metrics2.GetInt64Metric("perf_visibility_rules_diff", tagsExtra).Update(int64(len(extra)))
	}

	return nil
}
