package ignore

import (
	"fmt"
	"net/url"
	"sync"
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

	// Updates an IgnoreRule.
	Update(id int, rule *IgnoreRule) error

	// Removes an IgnoreRule from the store.
	Delete(id int, userId string) (int, error)

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
	ID      int       `json:"id"`
	Name    string    `json:"name"`
	Expires time.Time `json:"expires"`
	Query   string    `json:"query"`
	Note    string    `json:"note"`
	Count   int       `json:"count"`
}

// ToQuery makes a slice of url.Values from the given slice of IngoreRules.
func ToQuery(ignores []*IgnoreRule) ([]url.Values, error) {
	ret := []url.Values{}
	for _, ignore := range ignores {
		v, err := url.ParseQuery(ignore.Query)
		if err != nil {
			return nil, fmt.Errorf("Found an invaild ignore rule %d %s: %s", ignore.ID, ignore.Query, err)
		}
		ret = append(ret, v)
	}
	return ret, nil
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
type MemIgnoreStore struct {
	rules    []*IgnoreRule
	mutex    sync.Mutex
	nextId   int
	revision int64
}

func NewMemIgnoreStore() IgnoreStore {
	return &MemIgnoreStore{
		rules: []*IgnoreRule{},
	}
}

func (m *MemIgnoreStore) inc() {
	m.revision += 1
}

// Create, see IgnoreStore interface.
func (m *MemIgnoreStore) Create(rule *IgnoreRule) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	rule.ID = m.nextId
	m.nextId++
	m.rules = append(m.rules, rule)
	m.inc()
	return nil
}

// List, see IgnoreStore interface.
func (m *MemIgnoreStore) List() ([]*IgnoreRule, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.expire()
	result := make([]*IgnoreRule, len(m.rules))
	copy(result, m.rules)
	return result, nil
}

// Update, see IgnoreStore interface.
func (m *MemIgnoreStore) Update(id int, updated *IgnoreRule) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for i, _ := range m.rules {
		if updated.ID == id {
			m.rules[i] = updated
			m.inc()
			return nil
		}
	}

	return fmt.Errorf("Did not find an IgnoreRule with id: %d", id)
}

// Delete, see IgnoreStore interface.
func (m *MemIgnoreStore) Delete(id int, userId string) (int, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for idx, rule := range m.rules {
		if rule.ID == id {
			m.rules = append(m.rules[:idx], m.rules[idx+1:]...)
			m.inc()
			return 1, nil
		}
	}

	return 0, nil
}

func (m *MemIgnoreStore) Revision() int64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.revision
}

func (m *MemIgnoreStore) expire() {
	newrules := make([]*IgnoreRule, 0, len(m.rules))
	now := time.Now()
	for _, rule := range m.rules {
		if rule.Expires.After(now) {
			newrules = append(newrules, rule)
		}
	}
	m.rules = newrules
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
