package mem_ignorestore

import (
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/golden/go/ignore"
)

// MemIgnoreStore is an in-memory implementation of IgnoreStore.
type MemIgnoreStore struct {
	rules    []*ignore.IgnoreRule
	mutex    sync.Mutex
	nextId   int64
	revision int64
}

func New() *MemIgnoreStore {
	return &MemIgnoreStore{
		rules: []*ignore.IgnoreRule{},
	}
}

func (m *MemIgnoreStore) inc() {
	m.revision += 1
}

// Create, see IgnoreStore interface.
func (m *MemIgnoreStore) Create(rule *ignore.IgnoreRule) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	rule.ID = m.nextId
	m.nextId++
	m.rules = append(m.rules, rule)
	m.inc()
	return nil
}

// List, see IgnoreStore interface.
func (m *MemIgnoreStore) List() ([]*ignore.IgnoreRule, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.expire()
	result := make([]*ignore.IgnoreRule, len(m.rules))
	copy(result, m.rules)
	return result, nil
}

// Update, see IgnoreStore interface.
func (m *MemIgnoreStore) Update(id int64, updated *ignore.IgnoreRule) error {
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
	newrules := make([]*ignore.IgnoreRule, 0, len(m.rules))
	now := time.Now()
	for _, rule := range m.rules {
		if rule.Expires.After(now) {
			newrules = append(newrules, rule)
		}
	}
	m.rules = newrules
}

// BuildRuleMatcher, see IgnoreStore interface.
func (m *MemIgnoreStore) BuildRuleMatcher() (ignore.RuleMatcher, error) {
	return ignore.BuildRuleMatcher(m)
}
