package mem_expstore

import (
	"context"
	"sync"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
)

// Implements ExpectationsStore in memory for prototyping and testing.
type MemExpectationsStore struct {
	expectations types.Expectations
	readCopy     types.Expectations
	eventBus     eventbus.EventBus

	// Protects expectations.
	mutex sync.RWMutex
}

// New creates an in-memory implementation of ExpectationsStore.
func New(eventBus eventbus.EventBus) *MemExpectationsStore {
	ret := &MemExpectationsStore{
		eventBus:     eventBus,
		expectations: types.Expectations{},
		readCopy:     types.Expectations{},
	}
	return ret
}

// Get fulfills the ExpectationsStore interface.
func (m *MemExpectationsStore) Get() (types.Expectations, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.readCopy, nil
}

// AddChange fulfills the ExpectationsStore interface.
func (m *MemExpectationsStore) AddChange(c context.Context, changedTests types.Expectations, userId string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.expectations.MergeExpectations(changedTests)
	if m.eventBus != nil {
		m.eventBus.Publish(expstorage.EV_EXPSTORAGE_CHANGED, &expstorage.EventExpectationChange{
			TestChanges: changedTests,
			IssueID:     expstorage.MasterIssueID,
		}, true)
	}

	m.readCopy = m.expectations.DeepCopy()
	return nil
}

// TESTING_ONLY_RemoveChange removes the expectations from the store. For tests only.
func (m *MemExpectationsStore) TESTING_ONLY_RemoveChange(changedDigests types.Expectations) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	exp := m.expectations.DeepCopy()
	for testName, digests := range changedDigests {
		for digest := range digests {
			delete(exp[testName], digest)
			if len(exp[testName]) == 0 {
				delete(exp, testName)
			}
		}
	}
	// Replace the current expectations.
	m.expectations = exp

	if m.eventBus != nil {
		m.eventBus.Publish(expstorage.EV_EXPSTORAGE_CHANGED, &expstorage.EventExpectationChange{
			TestChanges: changedDigests,
			IssueID:     expstorage.MasterIssueID,
		}, true)
	}

	m.readCopy = m.expectations.DeepCopy()
	return nil
}

// See ExpectationsStore interface.
func (m *MemExpectationsStore) QueryLog(c context.Context, offset, size int, details bool) ([]*expstorage.TriageLogEntry, int, error) {
	sklog.Fatal("MemExpectation store does not support querying the logs.")
	return nil, 0, nil
}

// See ExpectationsStore interface.
func (m *MemExpectationsStore) UndoChange(c context.Context, changeID int64, userID string) (types.Expectations, error) {
	sklog.Fatal("MemExpectation store does not support undo.")
	return nil, nil
}

// See  ExpectationsStore interface.
func (m *MemExpectationsStore) Clear(c context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.expectations = types.Expectations{}
	m.readCopy = types.Expectations{}
	return nil
}

// Make sure MemExpectationsStore fulfills the ExpectationsStore interface
var _ expstorage.ExpectationsStore = (*MemExpectationsStore)(nil)
