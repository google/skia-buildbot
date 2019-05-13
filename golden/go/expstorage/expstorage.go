package expstorage

import (
	"sync"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gevent"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

// Events emitted by this package.
const (
	// Event emitted when expectations change.
	// Callback argument: []string with the names of changed tests.
	EV_EXPSTORAGE_CHANGED = "expstorage:changed"
)

func init() {
	// Register the codec for EV_EXPSTORAGE_CHANGED so we can have distributed events.
	gevent.RegisterCodec(EV_EXPSTORAGE_CHANGED, util.JSONCodec(&EventExpectationChange{}))
}

// Defines the storage interface for expectations.
type ExpectationsStore interface {
	// Get the current classifications for image digests. The keys of the
	// expectations map are the test names.
	Get() (exp types.Expectations, err error)

	// AddChange writes the given classified digests to the database and records the
	// user that made the change.
	AddChange(changes types.Expectations, userId string) error

	// QueryLog allows to paginate through the changes in the expectations.
	// If details is true the result will include a list of triage operations
	// that were part a change.
	QueryLog(offset, size int, details bool) ([]*TriageLogEntry, int, error)

	// UndoChange reverts a change by setting all testname/digest pairs of the
	// original change to the label they had before the change was applied.
	// A new entry is added to the log with a reference to the change that was
	// undone.
	UndoChange(changeID int64, userID string) (types.Expectations, error)

	// Clear deletes all expectations in this ExpectationsStore. This is mostly
	// used for testing, but also to delete the expectations for a Gerrit issue.
	// See the tryjobstore package.
	Clear() error
}

type DEPRECATED_ExpectationsStore interface {
	ExpectationsStore
	// RemoveChange removes the given digests from the expectations store.
	// The key in changes is the test name which maps to a list of digests
	// to remove. Used for testing only.
	// TODO(kjlubick): The removeChange function is obsolete and should be removed.
	// It was used for testing before the UndoChange function was added. It is simply
	// wrong to change the expectations without a change record being added.
	RemoveChange(changes types.Expectations) error
}

// TriageDetails represents one changed digest and the label that was
// assigned as part of the triage operation.
type TriageDetail struct {
	TestName types.TestName `json:"test_name"`
	Digest   types.Digest   `json:"digest"`
	Label    string         `json:"label"`
}

// TriageLogEntry represents one change in the expectation store.
type TriageLogEntry struct {
	// Note: The ID is a string because an int64 cannot be passed back and
	// forth to the JS frontend.
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	TS           int64           `json:"ts"`
	ChangeCount  int             `json:"changeCount"`
	Details      []*TriageDetail `json:"details"`
	UndoChangeID int64           `json:"undoChangeId"`
}

func (t *TriageLogEntry) GetChanges() types.Expectations {
	ret := types.Expectations{}
	for _, d := range t.Details {
		label := types.LabelFromString(d.Label)
		if found, ok := ret[d.TestName]; !ok {
			ret[d.TestName] = types.TestClassification{d.Digest: label}
		} else {
			found[d.Digest] = label
		}
	}
	return ret
}

// Implements ExpectationsStore in memory for prototyping and testing.
type MemExpectationsStore struct {
	expectations types.Expectations
	readCopy     types.Expectations
	eventBus     eventbus.EventBus

	// Protects expectations.
	mutex sync.RWMutex
}

// New instance of memory backed expectation storage.
func NewMemExpectationsStore(eventBus eventbus.EventBus) *MemExpectationsStore {
	ret := &MemExpectationsStore{
		eventBus: eventBus,
	}
	_ = ret.Clear()
	return ret
}

// Get fulfills the ExpectationsStore interface.
func (m *MemExpectationsStore) Get() (types.Expectations, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.readCopy, nil
}

// AddChange fulfills the ExpectationsStore interface.
func (m *MemExpectationsStore) AddChange(changedTests types.Expectations, userId string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.expectations.MergeExpectations(changedTests)
	if m.eventBus != nil {
		m.eventBus.Publish(EV_EXPSTORAGE_CHANGED, evExpChange(changedTests, masterIssueID, nil), true)
	}

	m.readCopy = m.expectations.DeepCopy()
	return nil
}

// RemoveChange fulfills the DEPRECATED_ExpectationsStore interface.
func (m *MemExpectationsStore) RemoveChange(changedDigests types.Expectations) error {
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
		m.eventBus.Publish(EV_EXPSTORAGE_CHANGED, evExpChange(changedDigests, masterIssueID, nil), true)
	}

	m.readCopy = m.expectations.DeepCopy()
	return nil
}

// See ExpectationsStore interface.
func (m *MemExpectationsStore) QueryLog(offset, size int, details bool) ([]*TriageLogEntry, int, error) {
	sklog.Fatal("MemExpectation store does not support querying the logs.")
	return nil, 0, nil
}

// See ExpectationsStore interface.
func (m *MemExpectationsStore) UndoChange(changeID int64, userID string) (types.Expectations, error) {
	sklog.Fatal("MemExpectation store does not support undo.")
	return nil, nil
}

// See  ExpectationsStore interface.
func (m *MemExpectationsStore) Clear() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.expectations = types.Expectations{}
	m.readCopy = types.Expectations{}
	return nil
}

// Make sure MemExpectationsStore fulfills the ExpectationsStore interface
var _ ExpectationsStore = (*MemExpectationsStore)(nil)

// Make sure MemExpectationsStore fulfills the DEPRECATED_ExpectationsStore interface
var _ DEPRECATED_ExpectationsStore = (*MemExpectationsStore)(nil)
