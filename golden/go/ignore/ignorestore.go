package ignore

import (
	"net/url"
	"time"

	"go.skia.org/infra/go/skerr"
)

// RuleMatcher returns a list of rules that match the given set of parameters.
type RuleMatcher func(map[string]string) ([]*Rule, bool)

// Store is an interface for a database that saves ignore rules.
// TODO(kjlubick): Add context to these methods such that we can
// pass in a context from the web request to the backend.
type Store interface {
	// Create adds a new rule to the ignore store.
	Create(*Rule) error

	// List returns all ignore rules in the ignore store.
	List() ([]*Rule, error)

	// Updates an Rule.
	Update(id int64, rule *Rule) error

	// Removes an Rule from the store. The return value is the number of
	// records that were deleted (either 0 or 1).
	Delete(id int64) (int, error)

	// Revision returns a monotonically increasing int64 that goes up each time
	// the ignores have been changed. It will not persist nor will it be the same
	// between different instances of Store. I.e. it will probably start at
	// zero each time a Store is instantiated.
	Revision() int64

	// BuildRuleMatcher returns a RuleMatcher based on the current content
	// of the ignore store.
	BuildRuleMatcher() (RuleMatcher, error)
}

// Rule is the GUI struct for dealing with Ignore rules.
type Rule struct {
	ID        int64     `json:"id,string"`
	Name      string    `json:"name"`
	UpdatedBy string    `json:"updatedBy"`
	Expires   time.Time `json:"expires"`
	Query     string    `json:"query"`
	Note      string    `json:"note"`
}

// ToQuery makes a slice of url.Values from the given slice of Rules.
func ToQuery(ignores []*Rule) ([]url.Values, error) {
	var ret []url.Values
	for _, ignore := range ignores {
		v, err := url.ParseQuery(ignore.Query)
		if err != nil {
			return nil, skerr.Wrapf(err, "invalid ignore rule %d %s", ignore.ID, ignore.Query)
		}
		ret = append(ret, v)
	}
	return ret, nil
}

func NewRule(createdByUser string, expires time.Time, queryStr string, note string) *Rule {
	return &Rule{
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

func noopRuleMatcher(_ map[string]string) ([]*Rule, bool) {
	return nil, false
}
