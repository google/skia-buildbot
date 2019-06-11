package ignore

import (
	"fmt"
	"net/url"
	"time"
)

// RuleMatcher returns a list of rules in the IgnoreStore that match the given
// set of parameters.
type RuleMatcher func(map[string]string) ([]*IgnoreRule, bool)

// IgnoreStore stores and matches ignore rules.
// TODO(kjlubick): Add context to these methods such that we can
// pass in a context from the web request to the backend.
type IgnoreStore interface {
	// Create adds a new rule to the ignore store.
	Create(*IgnoreRule) error

	// List returns all ignore rules in the ignore store.
	List() ([]*IgnoreRule, error)

	// Updates an IgnoreRule.
	Update(id int64, rule *IgnoreRule) error

	// Removes an IgnoreRule from the store. The return value is the number of
	// records that were deleted (either 0 or 1).
	Delete(id int64) (int, error)

	// Revision returns a monotonically increasing int64 that goes up each time
	// the ignores have been changed. It will not persist nor will it be the same
	// between different instances of IgnoreStore. I.e. it will probably start at
	// zero each time an IngoreStore is instantiated.
	Revision() int64

	// BuildRuleMatcher returns a RuleMatcher based on the current content
	// of the ignore store.
	BuildRuleMatcher() (RuleMatcher, error)
}

// IgnoreRule is the GUI struct for dealing with Ignore rules.
type IgnoreRule struct {
	ID        int64     `json:"id,string"`
	Name      string    `json:"name"`
	UpdatedBy string    `json:"updatedBy"`
	Expires   time.Time `json:"expires"`
	Query     string    `json:"query"`
	Note      string    `json:"note"`
}

// ToQuery makes a slice of url.Values from the given slice of IngoreRules.
func ToQuery(ignores []*IgnoreRule) ([]url.Values, error) {
	ret := []url.Values{}
	for _, ignore := range ignores {
		v, err := url.ParseQuery(ignore.Query)
		if err != nil {
			return nil, fmt.Errorf("Found an invalid ignore rule %d %s: %s", ignore.ID, ignore.Query, err)
		}
		ret = append(ret, v)
	}
	return ret, nil
}

func NewIgnoreRule(createdByUser string, expires time.Time, queryStr string, note string) *IgnoreRule {
	return &IgnoreRule{
		Name:      createdByUser,
		UpdatedBy: createdByUser,
		Expires:   expires,
		Query:     queryStr,
		Note:      note,
	}
}

// TODO(stephana): Factor out QueryRule into the shared library and consolidate
// with the Matches function in the ./go/tiling package.

// QueryRule wraps around a web query and allows matching of
// parameter sets against it.
type QueryRule map[string]map[string]bool

func NewQueryRule(v url.Values) QueryRule {
	ret := make(map[string]map[string]bool, len(v))
	for k, paramValues := range v {
		ret[k] = make(map[string]bool, len(paramValues))
		for _, oneVal := range paramValues {
			ret[k][oneVal] = true
		}
	}
	return ret
}

// IsMatch returns true if the set of parameters in params matches this query.
func (q QueryRule) IsMatch(params map[string]string) bool {
	if len(q) > len(params) {
		return false
	}

	for ruleKey, ruleValues := range q {
		paramVal, ok := params[ruleKey]
		if !(ok && ruleValues[paramVal]) {
			return false
		}
	}
	return true
}

func noopRuleMatcher(p map[string]string) ([]*IgnoreRule, bool) {
	return nil, false
}
