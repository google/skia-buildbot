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

func (e *Expectations) DeepCopy() *Expectations {
	m := make(map[string]types.TestClassification, len(e.Tests))
	for k, v := range e.Tests {
		m[k] = v.DeepCopy()
	}
	return &Expectations{
		Tests: m,
	}
}

// ------------  Interface to store expectations

// TODO(stephana): Add RemoveChange to allow the removal of
// tests/digests from the expectations.

// Defines the storage interface for expectations.
type ExpectationsStore interface {
	// Get the current classifications for image digests. The keys of the
	// expectations map are the test names.
	Get() (exp *Expectations, err error)

	// AddChange writes the given classified digests to the database and records the
	// user that made the change.
	AddChange(changes map[string]types.TestClassification, userId string) error

	// Changes returns a receive-only channel that will provide a list of test
	// names every time expectations are updated.
	Changes() <-chan []string
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
	changes      changesSlice

	// Protects expectations.
	mutex sync.Mutex
}

// New instance of memory backed expecation storage.
func NewMemExpectationsStore() ExpectationsStore {
	return &MemExpectationsStore{
		expectations: NewExpectations(),
		changes:      changesSlice{},
	}
}

// ------------- In-memory implementation
// See ExpectationsStore interface.
func (m *MemExpectationsStore) Get() (*Expectations, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.expectations, nil
}

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

	m.changes.send(testNames)
	return nil
}

func (m *MemExpectationsStore) Changes() <-chan []string {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	ch := make(chan []string)
	m.changes = append(m.changes, ch)
	return ch
}
