package expstorage

import (
	"sync"

	"skia.googlesource.com/buildbot.git/golden/go/types"
)

// Wraps the set of expectations and provides methods to manipulate them.
type Expectations struct {
	Tests      map[string]types.TestClassification `json:"tests"`
	Modifiable bool                                `json:"-"`
}

func NewExpectations(modifiable bool) *Expectations {
	return &Expectations{
		Modifiable: modifiable,
		Tests:      map[string]types.TestClassification{},
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

func (e *Expectations) RemoveDigests(digests []string) {
	e.checkModifiable()

	for testName, labeledDigests := range e.Tests {
		for _, d := range digests {
			delete(labeledDigests, d)
		}
		if 0 == len(labeledDigests) {
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
		Tests:      m,
		Modifiable: e.Modifiable,
	}
}

func (e *Expectations) checkModifiable() {
	if !e.Modifiable {
		panic("Cannot modify expectations. Marked as unmodifiable.")
	}
}

// ------------  Interface to store expectations

// Defines the storage interface for expectations.
type ExpectationsStore interface {
	// Get the current classifications for image digests. The keys of the
	// expectations map are the test names.
	Get(modifiable bool) (exp *Expectations, err error)

	// Write the given classified digests to the database and record the
	// user that made the change.
	Put(exp *Expectations, userId string) error
}

// Implements ExpectationsStore in memory for prototyping and testing.
type MemExpectationsStore struct {
	expectations *Expectations

	// Protects expectations.
	mutex sync.Mutex
}

// New instance of memory backed expecation storage.
func NewMemExpectationsStore() ExpectationsStore {
	return &MemExpectationsStore{
		expectations: NewExpectations(false),
	}
}

// ------------- In-memory implementation
// See ExpectationsStore interface.
func (m *MemExpectationsStore) Get(modifiable bool) (*Expectations, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !modifiable {
		return m.expectations, nil
	}

	result := m.expectations.DeepCopy()
	result.Modifiable = true
	return result, nil
}

func (m *MemExpectationsStore) Put(exps *Expectations, userId string) error {
	exps.checkModifiable()

	newExps := exps.DeepCopy()
	newExps.Modifiable = false

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.expectations = newExps
	return nil
}
