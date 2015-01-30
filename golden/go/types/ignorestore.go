package types

import (
	"net/url"
	"time"

	"skia.googlesource.com/buildbot.git/go/database"
)

// RuleMatcher returns a list of rules in the IgnoreStore that match the given
// set of parameters.
type RuleMatcher func(map[string]string) ([]*IgnoreRule, bool)

// IgnoreStore stores and matches ignore rules.
type IgnoreStore interface {
	// Create adds a new rule to the ignore store.
	Create(*IgnoreRule) error

	// List returns all ignore rules in the ignore store.
	List() ([]*IgnoreRule, error)

	// Removes an IgnoreRule from the store.
	Delete(id int, userId string) (int, error)

	// BuildRuleMatcher returns a RuleMatcher based on the current content
	// of the ignore store.
	BuildRuleMatcher() (RuleMatcher, error)
}

// IgnoreRule is the GUI struct for dealing with Ignore rules.
type IgnoreRule struct {
	ID      int       `json:"id"`
	Name    string    `json:"name"`
	Expires time.Time `json:"expires"`
	Query   string    `json:"query"`
	Note    string    `json:"note"`
	Count   int       `json:"count"`
}

func NewIgnoreRule(name string, expires time.Time, queryStr string, note string) *IgnoreRule {
	return &IgnoreRule{
		Name:    name,
		Expires: expires,
		Query:   queryStr,
		Note:    note,
	}
}

// MemIgnoreStore is an in-memory implementation of IgnoreStore.
type MemIgnoreStore []*IgnoreRule

func NewMemIgnoreStore() IgnoreStore {
	return new(MemIgnoreStore)
}

// Create, see IgnoreStore interface.
func (m *MemIgnoreStore) Create(rule *IgnoreRule) error {
	rule.ID = len(*m)
	*m = append(*m, rule)
	return nil
}

// List, see IgnoreStore interface.
func (m *MemIgnoreStore) List() ([]*IgnoreRule, error) {
	result := make([]*IgnoreRule, 0, len(*m))
	for _, r := range *m {
		if r != nil {
			result = append(result, r)
		}
	}
	return result, nil
}

// Delete, see IgnoreStore interface.
func (m *MemIgnoreStore) Delete(id int, userId string) (int, error) {
	for idx := range *m {
		if ((*m)[idx] != nil) && ((*m)[idx].ID == id) {
			(*m)[idx] = nil
			return 1, nil
		}
	}
	return 0, nil
}

// BuildRuleMatcher, see IgnoreStore interface.
func (m *MemIgnoreStore) BuildRuleMatcher() (RuleMatcher, error) {
	rulesList, err := m.List()
	if err != nil {
		return noopRuleMatcher, err
	}

	ignoreRules := make([]map[string]map[string]bool, len(rulesList))
	for idx, rawRule := range rulesList {
		ignoreRules[idx], err = compileRule(rawRule.Query)
		if err != nil {
			return noopRuleMatcher, err
		}
	}

	return func(params map[string]string) ([]*IgnoreRule, bool) {
		result := []*IgnoreRule{}

	Loop:
		for ruleIdx, rule := range ignoreRules {
			// All elements in the rules are AND connected. If the list is
			// longer than available parameters the result will be false.
			if len(rule) > len(params) {
				continue Loop
			}

			// Check if the parameters match the rule.
			for ruleKey, ruleValues := range rule {
				if _, ok := params[ruleKey]; !ok {
					continue Loop
				}
				if !ruleValues[params[ruleKey]] {
					continue Loop
				}
			}
			result = append(result, rulesList[ruleIdx])
		}

		return result, len(result) > 0
	}, nil
}

func compileRule(query string) (map[string]map[string]bool, error) {
	v, err := url.ParseQuery(query)
	if err != nil {
		return nil, err
	}

	ret := make(map[string]map[string]bool, len(v))
	for k, paramValues := range v {
		ret[k] = make(map[string]bool, len(paramValues))
		for _, oneVal := range paramValues {
			ret[k][oneVal] = true
		}
	}
	return ret, nil
}

func noopRuleMatcher(p map[string]string) ([]*IgnoreRule, bool) {
	return nil, false
}

func NewSQLIgnoreStore(vdb *database.VersionedDB) IgnoreStore {
	return NewMemIgnoreStore()
}
