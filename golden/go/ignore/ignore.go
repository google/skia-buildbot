package ignore

import (
	"context"
	"net/url"
	"time"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/tiling"
)

// Store is an interface for a database that saves ignore rules.
type Store interface {
	// Create adds a new rule to the ignore store. The ID will be set if this call is successful.
	Create(context.Context, *Rule) error

	// List returns all ignore rules in the ignore store.
	List(context.Context) ([]*Rule, error)

	// Update sets a Rule.
	Update(ctx context.Context, rule *Rule) error

	// Delete removes a Rule from the store. The return value is the number of
	// records that were deleted (either 0 or 1).
	Delete(ctx context.Context, id string) (int, error)
}

// Rule defines a single ignore rule, matching zero or more traces based on
// Query.
type Rule struct {
	// ID is the id used to store this Rule in a Store. They should be unique.
	ID string
	// Name is the email of the user who created the rule.
	Name string
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
func NewRule(createdByUser string, expires time.Time, queryStr string, note string) *Rule {
	return &Rule{
		Name:      createdByUser,
		UpdatedBy: createdByUser,
		Expires:   expires,
		Query:     queryStr,
		Note:      note,
	}
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

// FilterIgnored returns a copy of the given tile with all traces removed
// that match the ignore rules in the given ignore store. It also returns the
// ignore rules for later matching.
func FilterIgnored(inputTile *tiling.Tile, ignores []*Rule) (*tiling.Tile, paramtools.ParamMatcher, error) {
	// Make a shallow copy with a new Traces map
	ret := &tiling.Tile{
		Traces:   map[tiling.TraceID]tiling.Trace{},
		ParamSet: inputTile.ParamSet,
		Commits:  inputTile.Commits,

		Scale:     inputTile.Scale,
		TileIndex: inputTile.TileIndex,
	}

	// Then, add any traces that don't match any ignore rules
	ignoreQueries, err := toQuery(ignores)
	if err != nil {
		return nil, nil, err
	}
nextTrace:
	for id, tr := range inputTile.Traces {
		for _, q := range ignoreQueries {
			if tiling.Matches(tr, q) {
				continue nextTrace
			}
		}
		ret.Traces[id] = tr
	}

	ignoreRules := make([]paramtools.ParamSet, len(ignoreQueries))
	for idx, q := range ignoreQueries {
		ignoreRules[idx] = paramtools.ParamSet(q)
	}
	return ret, ignoreRules, nil
}
