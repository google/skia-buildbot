package expstorage

import (
	"sync"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/golden/go/types"
)

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
			e.Tests[testName][digest] = label
		}
	}
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
// AddChange and RemoveChange functions of the ExpectationsStore.
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

	// RemoveChange removes the given digests from the expectations store.
	// The key in changes is the test name which maps to a list of digests
	// to remove.
	RemoveChange(changes map[string][]string) error

	// Changes returns a receive-only channel that will provide a list of test
	// names every time expectations are updated.
	Changes() <-chan []string

	// QueryLog allows to paginate through the changes in the expecations.
	QueryLog(offset, size int) ([]*TriageLogEntry, int, error)
}

// TODO(stephana): Expand the TriageLogEntry with change id and the
// map of changed digests and their labels to support undo functionality.

// TriageLogEntry represents one change in the expectation store.
type TriageLogEntry struct {
	Name        string `json:"name"`
	TS          int64  `json:"ts"`
	ChangeCount int    `json:"changeCount"`
}

// changesSlice is a slice of channels.
type changesSlice [](chan []string)

// send will send all the string slices down each channel.
//
// Each send is done in its own go routine so this call will
// not block.
func (c changesSlice) send(s []string) {
	for _, ch := range c {
		go func(ch chan []string) {
			ticker := time.Tick(time.Second * 10)
			select {
			case ch <- s:
				break
			case <-ticker:
				glog.Errorf("Failed to send a change event.")
			}
		}(ch)
	}
}

// Implements ExpectationsStore in memory for prototyping and testing.
type MemExpectationsStore struct {
	expectations *Expectations
	readCopy     *Expectations
	changes      changesSlice

	// Protects expectations.
	mutex sync.Mutex
}

// New instance of memory backed expecation storage.
func NewMemExpectationsStore() ExpectationsStore {
	return &MemExpectationsStore{
		expectations: NewExpectations(),
		readCopy:     NewExpectations(),
		changes:      changesSlice{},
	}
}

// ------------- In-memory implementation
// See ExpectationsStore interface.
func (m *MemExpectationsStore) Get() (*Expectations, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.readCopy, nil
}

func (m *MemExpectationsStore) dataChanged(testNames []string) {
	m.readCopy = m.expectations.DeepCopy()
	m.changes.send(testNames)
}

// See ExpectationsStore interface.
func (m *MemExpectationsStore) AddChange(changedTests map[string]types.TestClassification, userId string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	testNames := make([]string, 0, len(changedTests))
	for testName, digests := range changedTests {
		if _, ok := m.expectations.Tests[testName]; !ok {
			m.expectations.Tests[testName] = map[string]types.Label{}
		}
		for d, label := range digests {
			m.expectations.Tests[testName][d] = label
		}
		testNames = append(testNames, testName)
	}

	m.dataChanged(testNames)
	return nil
}

// RemoveChange, see ExpectationsStore interface.
func (m *MemExpectationsStore) RemoveChange(changedDigests map[string][]string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	testNames := make([]string, 0, len(changedDigests))

	for testName, digests := range changedDigests {
		for _, digest := range digests {
			delete(m.expectations.Tests[testName], digest)
			if len(m.expectations.Tests[testName]) == 0 {
				delete(m.expectations.Tests, testName)
			}
		}
		testNames = append(testNames, testName)
	}

	m.dataChanged(testNames)
	return nil
}

// See ExpectationsStore interface.
func (m *MemExpectationsStore) Changes() <-chan []string {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	ch := make(chan []string)
	m.changes = append(m.changes, ch)
	return ch
}

// See ExpectationsStore interface.
func (m *MemExpectationsStore) QueryLog(offset, size int) ([]*TriageLogEntry, int, error) {
	glog.Fatal("MemExpectation store does not support querying the logs.")
	return nil, 0, nil
}
