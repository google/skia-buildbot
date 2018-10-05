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
	AddChange(changes types.TestExp, userId string) error

	// QueryLog allows to paginate through the changes in the expectations.
	// If details is true the result will include a list of triage operations
	// that were part a change.
	QueryLog(offset, size int, details bool) ([]*TriageLogEntry, int, error)

	// UndoChange reverts a change by setting all testname/digest pairs of the
	// original change to the label they had before the change was applied.
	// A new entry is added to the log with a reference to the change that was
	// undone.
	UndoChange(changeID int64, userID string) (types.TestExp, error)

	// Clear deletes all expectations in this ExpectationsStore. This is mostly
	// used for testing, but also to delete the expectations for a Gerrit issue.
	// See the tryjobstore package.
	Clear() error

	// removeChange removes the given digests from the expectations store.
	// The key in changes is the test name which maps to a list of digests
	// to remove. Used for testing only.
	removeChange(changes types.TestExp) error
}

// TriageDetails represents one changed digest and the label that was
// assigned as part of the triage operation.
type TriageDetail struct {
	TestName string `json:"test_name"`
	Digest   string `json:"digest"`
	Label    string `json:"label"`
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

func (t *TriageLogEntry) GetChanges() types.TestExp {
	ret := types.TestExp{}
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
func NewMemExpectationsStore(eventBus eventbus.EventBus) ExpectationsStore {
	ret := &MemExpectationsStore{
		eventBus: eventBus,
	}
	_ = ret.Clear()
	return ret
}

// ------------- In-memory implementation
// See ExpectationsStore interface.
func (m *MemExpectationsStore) Get() (types.Expectations, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.readCopy, nil
}

// See ExpectationsStore interface.
func (m *MemExpectationsStore) AddChange(changedTests types.TestExp, userId string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.expectations.AddTestExp(changedTests)
	if m.eventBus != nil {
		m.eventBus.Publish(EV_EXPSTORAGE_CHANGED, evExpChange(changedTests, masterIssueID), true)
	}

	m.readCopy = types.NewExpectations(m.expectations.TestExp())
	return nil
}

// removeChange, see ExpectationsStore interface.
func (m *MemExpectationsStore) removeChange(changedDigests types.TestExp) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	testExp := m.expectations.TestExp()
	for testName, digests := range changedDigests {
		for digest := range digests {
			delete(testExp[testName], digest)
			if len(testExp[testName]) == 0 {
				delete(testExp, testName)
			}
		}
	}
	// Replace the current expectations.
	m.expectations = types.NewExpectations(testExp)

	if m.eventBus != nil {
		m.eventBus.Publish(EV_EXPSTORAGE_CHANGED, evExpChange(changedDigests, masterIssueID), true)
	}

	m.readCopy = types.NewExpectations(m.expectations.TestExp())
	return nil
}

// See ExpectationsStore interface.
func (m *MemExpectationsStore) QueryLog(offset, size int, details bool) ([]*TriageLogEntry, int, error) {
	sklog.Fatal("MemExpectation store does not support querying the logs.")
	return nil, 0, nil
}

// See  ExpectationsStore interface.
func (m *MemExpectationsStore) UndoChange(changeID int64, userID string) (types.TestExp, error) {
	sklog.Fatal("MemExpectation store does not support undo.")
	return nil, nil
}

// See  ExpectationsStore interface.
func (m *MemExpectationsStore) Clear() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.expectations = types.NewExpectations(nil)
	m.readCopy = types.NewExpectations(nil)
	return nil
}
