package types

import (
	"net/url"
	"time"
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
	return buildRuleMatcher(m)
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
