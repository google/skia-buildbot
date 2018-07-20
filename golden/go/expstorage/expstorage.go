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

// Wraps the set of expectations and provides methods to manipulate them.
type Expectations struct {
	Tests map[string]types.TestClassification `json:"tests"`
}

// Classification returns the classification for a single digest, returning
// types.UNTRIAGED if that test or digest is unknown to Expectations.
func (e *Expectations) Classification(test, digest string) types.Label {
	if label, ok := e.Tests[test][digest]; ok {
		return label
	}
	return types.UNTRIAGED
}

func NewExpectations() *Expectations {
	return &Expectations{
		Tests: map[string]types.TestClassification{},
	}
}

// Add tests and their labeled digests.
func (e *Expectations) AddDigests(testDigests map[string]types.TestClassification) {
	for testName, digests := range testDigests {
		if _, ok := e.Tests[testName]; !ok {
			e.Tests[testName] = map[string]types.Label{}
		}
		for digest, label := range digests {
			// UNTRIAGED is the default value and we don't need to store it
			if label == types.UNTRIAGED {
				delete(e.Tests[testName], digest)
			} else {
				e.Tests[testName][digest] = label
			}
		}
		// In case we had only assigned UNTRIAGED values
		if len(e.Tests[testName]) == 0 {
			delete(e.Tests, testName)
		}
	}
}

// SetTestExpectation sets the label (expectation) for a single test/digest pair.
func (e *Expectations) SetTestExpectation(testName string, digest string, label types.Label) {
	if _, ok := e.Tests[testName]; !ok {
		e.Tests[testName] = map[string]types.Label{}
	}
	e.Tests[testName][digest] = label
}

// RemoveDigests removes the given digests from the expectations.
// The key in the input is the test name.
func (e *Expectations) RemoveDigests(digests map[string][]string) {
	for testName, digests := range digests {
		for _, digest := range digests {
			delete(e.Tests[testName], digest)
		}

		if len(e.Tests[testName]) == 0 {
			delete(e.Tests, testName)
		}
	}
}

func (e *Expectations) DeepCopy() *Expectations {
	m := make(map[string]types.TestClassification, len(e.Tests))
	for k, v := range e.Tests {
		m[k] = v.DeepCopy()
	}
	return &Expectations{
		Tests: m,
	}
}

// Delta returns the additions and removals that are necessary to
// get from e to right. The results can be passed directly to the
// AddChange and removeChange functions of the ExpectationsStore.
func (e *Expectations) Delta(right *Expectations) (*Expectations, map[string][]string) {
	addExp := subtract(right, e, nil)
	removeExp := subtract(e, right, addExp.Tests)

	// Copy the testnames and digests into the output.
	ret := make(map[string][]string, len(removeExp.Tests))
	for testName, digests := range removeExp.Tests {
		temp := make([]string, 0, len(digests))
		for digest := range digests {
			temp = append(temp, digest)
		}
		ret[testName] = temp
	}

	return addExp, ret
}

// Returns a copy of expA with all values removed that also appear in expB.
func subtract(expA, expB *Expectations, exclude map[string]types.TestClassification) *Expectations {
	ret := make(map[string]types.TestClassification, len(expA.Tests))
	for testName, digests := range expA.Tests {
		for digest, labelA := range digests {
			if _, ok := exclude[testName][digest]; !ok {
				if labelB, ok := expB.Tests[testName][digest]; !ok || (labelB != labelA) {
					if found, ok := ret[testName]; !ok {
						ret[testName] = map[string]types.Label{digest: labelA}
					} else {
						found[digest] = labelA
					}
				}
			}
		}
	}
	return &Expectations{
		Tests: ret,
	}
}

// Defines the storage interface for expectations.
type ExpectationsStore interface {
	// Get the current classifications for image digests. The keys of the
	// expectations map are the test names.
	Get() (exp *Expectations, err error)

	// AddChange writes the given classified digests to the database and records the
	// user that made the change.
	AddChange(changes map[string]types.TestClassification, userId string) error

	// QueryLog allows to paginate through the changes in the expectations.
	// If details is true the result will include a list of triage operations
	// that were part a change.
	QueryLog(offset, size int, details bool) ([]*TriageLogEntry, int, error)

	// UndoChange reverts a change by setting all testname/digest pairs of the
	// original change to the label they had before the change was applied.
	// A new entry is added to the log with a reference to the change that was
	// undone.
	UndoChange(changeID int64, userID string) (map[string]types.TestClassification, error)

	// Clear deletes all expectations in this ExpectationsStore. This is mostly
	// used for testing, but also to delete the expectations for a Gerrit issue.
	// See the tryjobstore package.
	Clear() error

	// removeChange removes the given digests from the expectations store.
	// The key in changes is the test name which maps to a list of digests
	// to remove. Used for testing only.
	removeChange(changes map[string]types.TestClassification) error
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

func (t *TriageLogEntry) GetChanges() map[string]types.TestClassification {
	ret := map[string]types.TestClassification{}
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
	expectations *Expectations
	readCopy     *Expectations
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
func (m *MemExpectationsStore) Get() (*Expectations, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.readCopy, nil
}

// See ExpectationsStore interface.
func (m *MemExpectationsStore) AddChange(changedTests map[string]types.TestClassification, userId string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for testName, digests := range changedTests {
		if _, ok := m.expectations.Tests[testName]; !ok {
			m.expectations.Tests[testName] = map[string]types.Label{}
		}
		for d, label := range digests {
			m.expectations.Tests[testName][d] = label
		}
	}

	if m.eventBus != nil {
		m.eventBus.Publish(EV_EXPSTORAGE_CHANGED, evExpChange(changedTests, masterIssueID), true)
	}

	m.readCopy = m.expectations.DeepCopy()
	return nil
}

// removeChange, see ExpectationsStore interface.
func (m *MemExpectationsStore) removeChange(changedDigests map[string]types.TestClassification) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for testName, digests := range changedDigests {
		for digest := range digests {
			delete(m.expectations.Tests[testName], digest)
			if len(m.expectations.Tests[testName]) == 0 {
				delete(m.expectations.Tests, testName)
			}
		}
	}

	if m.eventBus != nil {
		m.eventBus.Publish(EV_EXPSTORAGE_CHANGED, evExpChange(changedDigests, masterIssueID), true)
	}

	m.readCopy = m.expectations.DeepCopy()
	return nil
}

// See ExpectationsStore interface.
func (m *MemExpectationsStore) QueryLog(offset, size int, details bool) ([]*TriageLogEntry, int, error) {
	sklog.Fatal("MemExpectation store does not support querying the logs.")
	return nil, 0, nil
}

// See  ExpectationsStore interface.
func (m *MemExpectationsStore) UndoChange(changeID int64, userID string) (map[string]types.TestClassification, error) {
	sklog.Fatal("MemExpectation store does not support undo.")
	return nil, nil
}

// See  ExpectationsStore interface.
func (m *MemExpectationsStore) Clear() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.expectations = NewExpectations()
	m.readCopy = NewExpectations()
	return nil
}
