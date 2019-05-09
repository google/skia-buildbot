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
// TODO(kjlubick): Add context to these methods such that we can
// pass in a context from the web request to the backend.
type IgnoreStore interface {
	// Create adds a new rule to the ignore store.
	Create(*IgnoreRule) error

	// List returns all ignore rules in the ignore store.
	// 'addCounts' indicates whether to include counts how often an ignore
	// rule appears in the current tile.
	// TODO(kjlubick): Remove the addCounts flag in the signature of the List function
	// and expose the AddIgnoreCounts function. This would remove the expsStore and
	// tileStream members of the cloudIgnoreStore struct and simplify the interface.
	List(addCounts bool) ([]*IgnoreRule, error)

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
	ID             int64     `json:"id,string"`
	Name           string    `json:"name"`
	UpdatedBy      string    `json:"updatedBy"`
	Expires        time.Time `json:"expires"`
	Query          string    `json:"query"`
	Note           string    `json:"note"`
	Count          int       `json:"count"          datastore:"-"`
	ExclusiveCount int       `json:"exclusiveCount" datastore:"-"`
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

// MemIgnoreStore is an in-memory implementation of IgnoreStore.
type MemIgnoreStore struct {
	rules    []*IgnoreRule
	mutex    sync.Mutex
	nextId   int64
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
func (m *MemIgnoreStore) List(addCounts bool) ([]*IgnoreRule, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.expire()
	result := make([]*IgnoreRule, len(m.rules))
	copy(result, m.rules)
	return result, nil
}

// Update, see IgnoreStore interface.
func (m *MemIgnoreStore) Update(id int64, updated *IgnoreRule) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for i := range m.rules {
		if updated.ID == id {
			m.rules[i] = updated
			m.inc()
			return nil
		}
	}

	return fmt.Errorf("Did not find an IgnoreRule with id: %d", id)
}

// Delete, see IgnoreStore interface.
func (m *MemIgnoreStore) Delete(id int64) (int, error) {
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
