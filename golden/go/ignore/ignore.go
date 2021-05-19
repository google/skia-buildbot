package ignore

import (
	"context"
	"time"
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
