package expstorage

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/skia-dev/glog"

	"skia.googlesource.com/buildbot.git/golden/go/types"
)

// Wraps the set of expectations and provides methods to manipulate them.
type Expectations struct {
	Tests      map[string]types.TestClassification `json:"tests"`
	Modifiable bool                                `json:"-"`
}

// Classification returns the classification for a single digest, returning
// types.UNTRIAGED if that test or digest is unknown to Expectations.
func (e *Expectations) Classification(test, digest string) types.Label {
	if label, ok := e.Tests[test][digest]; ok {
		return label
	}
	return types.UNTRIAGED
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

	// Put writes the given classified digests to the database and records the
	// user that made the change.
	Put(exp *Expectations, userId string) error

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

// diff returns a list of names of tests that have different expectations
// between a and b.
func diff(a, b *Expectations) []string {
	ret := []string{}
	// Check for tests in a that are different in b.
	for name, ea := range a.Tests {
		if eb, ok := b.Tests[name]; ok {
			fmt.Printf("%#v %#v", ea, eb)
			if !reflect.DeepEqual(ea, eb) {
				ret = append(ret, name)
			}
		} else {
			ret = append(ret, name)
		}
	}
	// Check for tests that exist in b that aren't in a.
	for name, _ := range b.Tests {
		if _, ok := a.Tests[name]; !ok {
			ret = append(ret, name)
		}
	}
	return ret
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
		expectations: NewExpectations(false),
		changes:      changesSlice{},
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

	testNames := diff(m.expectations, exps)

	newExps := exps.DeepCopy()
	newExps.Modifiable = false

	m.mutex.Lock()
	m.expectations = newExps
	m.mutex.Unlock()

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
