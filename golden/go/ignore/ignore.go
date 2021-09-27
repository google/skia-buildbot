package ignore

import (
	"context"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// Store is an interface for a database that saves ignore rules.
type Store interface {
	// Create adds a new rule to the ignore store. The ID will be set if this call is successful.
	Create(context.Context, Rule) error

	// List returns all ignore rules in the ignore store.
	List(context.Context) ([]Rule, error)

	// Update sets a Rule. The "Name" field should be ignored, but all other fields should be
	// applied to the existing data.
	Update(ctx context.Context, rule Rule) error

	// Delete removes a Rule from the store. If the rule didn't exist before, there will be no error.
	Delete(ctx context.Context, id string) error
}

// Rule defines a single ignore rule, matching zero or more traces based on
// Query.
type Rule struct {
	// ID is the id used to store this Rule in a Store. They should be unique.
	ID string
	// Name is the email of the user who created the rule.
	CreatedBy string
	// UpdatedBy is the email of the user who last updated the rule.
	UpdatedBy string
	// Expires indicates a time at which a human should re-consider the rule and see if
	// it still needs to be applied.
	Expires time.Time
	// Query is a url-encoded set of key-value pairs that can be used to match traces.
	// For example: "config=angle_d3d9_es2&cpu_or_gpu_value=RadeonHD7770"
	Query string
	// Note is a comment by a developer, typically a bug.
	Note string
}

// NewRule creates a new ignore rule with the given data.
func NewRule(createdByUser string, expires time.Time, queryStr string, note string) Rule {
	return Rule{
		CreatedBy: createdByUser,
		UpdatedBy: createdByUser,
		Expires:   expires,
		Query:     queryStr,
		Note:      note,
	}
}

// oneStep counts the number of ignore rules in the given store that are expired.
func oneStep(ctx context.Context, store Store, metric metrics2.Int64Metric) error {
	list, err := store.List(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	n := 0
	for _, rule := range list {
		if time.Now().After(rule.Expires) {
			n += 1
		}
	}
	metric.Update(int64(n))
	return nil
}

// StartMetrics starts a new monitoring routine for the given
// ignore.Store that counts expired ignore rules and pushes
// that info into a metric.
func StartMetrics(ctx context.Context, store Store, interval time.Duration) error {
	numExpired := metrics2.GetInt64Metric("gold_num_expired_ignore_rules", nil)
	liveness := metrics2.NewLiveness("gold_expired_ignore_rules_monitoring")

	if err := oneStep(ctx, store, numExpired); err != nil {
		return skerr.Wrapf(err, "starting to monitor ignore rules")
	}
	go util.RepeatCtx(ctx, interval, func(ctx context.Context) {
		if err := oneStep(ctx, store, numExpired); err != nil {
			sklog.Errorf("Failed one step of monitoring ignore rules: %s", err)
			return
		}
		liveness.Reset()
	})
	return nil
}
