package ignore

import (
	"context"
	"net/url"
	"time"

	"go.skia.org/infra/go/skerr"
)

// RuleMatcher returns a list of rules that match the given set of parameters.
type RuleMatcher func(map[string]string) ([]*Rule, bool)

// Store is an interface for a database that saves ignore rules.
type Store interface {
	// Create adds a new rule to the ignore store. The ID will be set if this call is successful.
	Create(context.Context, *Rule) error

	// List returns all ignore rules in the ignore store.
	List(context.Context) ([]*Rule, error)

	// Update sets a Rule. TODO(kjlubick) this API is strange - why do we need an id *and* a rule?
	Update(ctx context.Context, id string, rule *Rule) error

	// Delete removes a Rule from the store. The return value is the number of
	// records that were deleted (either 0 or 1).
	Delete(ctx context.Context, id string) (int, error)
}

// Rule is the GUI struct for dealing with Ignore rules.
type Rule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	UpdatedBy string    `json:"updatedBy"`
	Expires   time.Time `json:"expires"`
	Query     string    `json:"query"`
	Note      string    `json:"note"`
}

// toQuery makes a slice of url.Values from the given slice of Rules.
func toQuery(ignores []*Rule) ([]url.Values, error) {
	var ret []url.Values
	for _, ignore := range ignores {
		v, err := url.ParseQuery(ignore.Query)
		if err != nil {
			return nil, skerr.Wrapf(err, "invalid ignore rule id %q; query %q", ignore.ID, ignore.Query)
		}
		ret = append(ret, v)
	}
	return ret, nil
}

// NewRule creates a new ignore rule with the given data.
func NewRule(createdByUser string, expires time.Time, queryStr string, note string) *Rule {
	return &Rule{
		Name:      createdByUser,
		UpdatedBy: createdByUser,
		Expires:   expires,
		Query:     queryStr,
		Note:      note,
	}
}
