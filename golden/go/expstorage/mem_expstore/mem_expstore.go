package mem_expstore

import (
	"context"
	"sync"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types/expectations"
)

// Implements ExpectationsStore in memory for prototyping and testing.
type MemExpectationsStore struct {
	expectations expectations.Expectations
	readCopy     expectations.Expectations
	eventBus     eventbus.EventBus

	// Protects expectations.
	mutex sync.RWMutex
}

// New creates an in-memory implementation of ExpectationsStore.
func New(eventBus eventbus.EventBus) *MemExpectationsStore {
	ret := &MemExpectationsStore{
		eventBus:     eventBus,
		expectations: expectations.Expectations{},
		readCopy:     expectations.Expectations{},
	}
	return ret
}

// See ExpectationsStore interface.
func (m *MemExpectationsStore) ForChangeList(id, crs string) expstorage.ExpectationsStore {
	sklog.Fatal("MemExpectation store does not support ForChangeList.")
	return nil
}

// Get fulfills the ExpectationsStore interface.
func (m *MemExpectationsStore) Get() (expectations.Expectations, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.readCopy, nil
}

// AddChange fulfills the ExpectationsStore interface.
func (m *MemExpectationsStore) AddChange(c context.Context, changedTests expectations.Expectations, userId string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.expectations.MergeExpectations(changedTests)
	if m.eventBus != nil {
		m.eventBus.Publish(expstorage.EV_EXPSTORAGE_CHANGED, &expstorage.EventExpectationChange{
			ExpectationDelta: changedTests,
			CRSAndCLID:       "",
		}, true)
	}

	m.readCopy = m.expectations.DeepCopy()
	return nil
}

// TESTING_ONLY_RemoveChange removes the expectations from the store. For tests only.
func (m *MemExpectationsStore) TESTING_ONLY_RemoveChange(changedDigests expectations.Expectations) error {
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
			ExpectationDelta: changedDigests,
			CRSAndCLID:       "",
		}, true)
	}

	m.readCopy = m.expectations.DeepCopy()
	return nil
}

// See ExpectationsStore interface.
func (m *MemExpectationsStore) QueryLog(c context.Context, offset, size int, details bool) ([]expstorage.TriageLogEntry, int, error) {
	sklog.Fatal("MemExpectation store does not support querying the logs.")
	return nil, 0, nil
}

// See ExpectationsStore interface.
func (m *MemExpectationsStore) UndoChange(ctx context.Context, changeID, userID string) error {
	sklog.Fatal("MemExpectation store does not support undo.")
	return nil
}

// See  ExpectationsStore interface.
func (m *MemExpectationsStore) Clear(c context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.expectations = expectations.Expectations{}
	m.readCopy = expectations.Expectations{}
	return nil
}

// Make sure MemExpectationsStore fulfills the ExpectationsStore interface
var _ expstorage.ExpectationsStore = (*MemExpectationsStore)(nil)
