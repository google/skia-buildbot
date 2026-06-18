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
//
// Returns:
//   - addedCount (int): Number of missing expected rules that were newly saved to the database.
//   - removedCount (int): Number of extra rules found in the database that were deleted.
//   - error: An error if the check operation failed.
func (c *Checker) Check(ctx context.Context) (int, int, error) {
	sklog.Info("Starting check and sync of visibility rules...")

	dbConfigs, err := c.store.GetAll(ctx)
	if err != nil {
		return 0, 0, skerr.Wrapf(err, "failed to fetch current visibility configs from DB")
	}

	dbRules := make(map[string]bool)
	for _, cfg := range dbConfigs {
		dbRules[cfg.RuleExpression] = true
	}

	expectedRules, err := c.provider.GetExpectedRules(ctx)
	if err != nil {
		return 0, 0, skerr.Wrapf(err, "failed to fetch expected rules")
	}

	var missingRules []string
	for rule := range expectedRules {
		if !dbRules[rule] {
			missingRules = append(missingRules, rule)
		}
	}

	missingByPrefixCount := make(map[string]int)
	addedCount := 0
	if len(missingRules) > 0 {
		sklog.Infof("Auto-remediation: Saving %d missing expected rules to DB: %s", len(missingRules), strings.Join(missingRules, ", "))
		for _, rule := range missingRules {
			if err := c.store.Set(ctx, rule); err != nil {
				sklog.Errorf("Failed to save expected rule %q: %s", rule, err)
			} else {
				addedCount++
			}
			missingByPrefixCount[extractRulePrefix(rule)]++
		}
	}

	extraByPrefixCount := make(map[string]int)
	allPrefixes := make(map[string]bool)
	removedCount := 0
	for rule := range expectedRules {
		allPrefixes[extractRulePrefix(rule)] = true
	}

	for rule := range dbRules {
		rulePrefix := extractRulePrefix(rule)
		allPrefixes[rulePrefix] = true

		if !expectedRules[rule] {
			// Delete the rule row from the configuration store.
			// Note: We will not perform auto demotion of the actual traces in the TraceParams table
			// because demoting traces can break historical public links (e.g. in Buganizer).
			if err := c.store.Delete(ctx, rule); err != nil {
				sklog.Errorf("Failed to delete outdated rule %q: %s", rule, err)
			} else {
				removedCount++
			}
			extraByPrefixCount[rulePrefix]++
		}
	}

	for prefix := range allPrefixes {
		tagsExtra := map[string]string{"type": "extra", "source": prefix}
		metrics2.GetInt64Metric("perf_visibility_rules_diff", tagsExtra).Update(int64(extraByPrefixCount[prefix]))

		tagsMissing := map[string]string{"type": "missing", "source": prefix}
		metrics2.GetInt64Metric("perf_visibility_rules_diff", tagsMissing).Update(int64(missingByPrefixCount[prefix]))
	}

	return addedCount, removedCount, nil
}
