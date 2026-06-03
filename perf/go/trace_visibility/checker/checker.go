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

// Check fetches visibility config and compares them to the database,
// auto-remediating any discrepancies.
func (c *Checker) Check(ctx context.Context) error {
	sklog.Info("Starting check and sync of visibility rules...")

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

	var missingRules []string
	for rule := range expectedRules {
		if !dbRules[rule] {
			missingRules = append(missingRules, rule)
		}
	}

	missingByPrefixCount := make(map[string]int)
	if len(missingRules) > 0 {
		sklog.Infof("Auto-remediation: Saving %d missing expected rules to DB: %s", len(missingRules), strings.Join(missingRules, ", "))
		for _, rule := range missingRules {
			if err := c.store.Set(ctx, rule); err != nil {
				sklog.Errorf("Failed to save expected rule %q: %s", rule, err)
				missingByPrefixCount[extractRulePrefix(rule)]++
			}
		}
	}

	extraByPrefixCount := make(map[string]int)
	allPrefixes := make(map[string]bool)
	for rule := range expectedRules {
		allPrefixes[extractRulePrefix(rule)] = true
	}

	for rule := range dbRules {
		rulePrefix := extractRulePrefix(rule)
		allPrefixes[rulePrefix] = true

		if !expectedRules[rule] {
			// Auto-remediate extra rule
			// TODO(sergeirudenkov): The promoter does not yet know how to demote/revert public traces
			// when their rule is deleted. This will be handled in a separate CL.
			sklog.Warningf("Outdated extra rule %q found in database that is not in expected provider configs", rule)
			extraByPrefixCount[rulePrefix]++
		}
	}

	for prefix := range allPrefixes {
		tagsExtra := map[string]string{"type": "extra", "source": prefix}
		metrics2.GetInt64Metric("perf_visibility_rules_diff", tagsExtra).Update(int64(extraByPrefixCount[prefix]))

		tagsMissing := map[string]string{"type": "missing", "source": prefix}
		metrics2.GetInt64Metric("perf_visibility_rules_diff", tagsMissing).Update(int64(missingByPrefixCount[prefix]))
	}

	return nil
}
