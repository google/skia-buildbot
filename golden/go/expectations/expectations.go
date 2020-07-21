package expectations

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"go.skia.org/infra/golden/go/types"
)

// Expectations captures the expectations for a set of tests and digests as
// labels (Positive/Negative/Untriaged).
// Put another way, this data structure keeps track if a digest (image) is
// drawn correctly, incorrectly, or newly-seen for a given test.
// Expectations is thread safe.
type Expectations struct {
	mutex sync.RWMutex
	// TODO(kjlubick) Consider storing this as
	//   map[digestGrouping]Label where
	//   type digestGrouping struct {
	//     digest types.Digest
	//     grouping types.TestName
	//   }
	labels map[types.TestName]map[types.Digest]Label
}

// BaselineInt is a simplified view of the Expectations, suitable for JSON encoding. A BaselineInt only
// has entries with positive and negative labels (i.e. no untriaged entries).
type BaselineInt map[types.TestName]map[types.Digest]LabelInt

// ReadOnly is an interface with the non-mutating functions of Expectations.
// By using this instead of Expectations, we can make fewer copies, helping performance.
type ReadOnly interface {
	Classifier
	// ForAll will iterate through all entries in Expectations and call the callback with them.
	// Iteration will stop if a non-nil error is returned (and will be forwarded to the caller).
	ForAll(fn func(types.TestName, types.Digest, Label) error) error

	// Empty returns true iff NumTests() == 0
	Empty() bool

	// NumTests returns the number of tests that Expectations knows about.
	NumTests() int

	// Len returns the number of test/digest pairs stored.
	Len() int
}

// Classifier is a simple interface for querying expectations.
type Classifier interface {
	// Classification returns the label for the given test/digest pair. By definition,
	// this will return Untriaged if there isn't already a classification set.
	Classification(test types.TestName, digest types.Digest) Label
}

// Set sets the label for a test_name/digest pair. If the pair already exists,
// it will be over written.
func (e *Expectations) Set(testName types.TestName, digest types.Digest, label Label) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.ensureInit()
	if digests, ok := e.labels[testName]; ok {
		if label == Untriaged {
			delete(digests, digest)
		} else {
			digests[digest] = label
		}
	} else {
		if label != Untriaged {
			e.labels[testName] = map[types.Digest]Label{digest: label}
		}
	}
}

// setDigests is a convenience function to set the expectations of a set of digests for a
// given test_name. Callers should have the write mutex locked.
func (e *Expectations) setDigests(testName types.TestName, labels map[types.Digest]Label) {
	digests, ok := e.labels[testName]
	if !ok {
		digests = make(map[types.Digest]Label, len(labels))
	}
	for digest, label := range labels {
		if label != Untriaged {
			digests[digest] = label
		}
	}
	e.labels[testName] = digests
}

// MergeExpectations adds the given expectations to the current expectations, letting
// the ones provided by the passed in parameter overwrite any existing data. Trying to merge
// two expectations into each other simultaneously may result in a dead-lock.
func (e *Expectations) MergeExpectations(other *Expectations) {
	if other == nil {
		return
	}
	other.mutex.RLock()
	defer other.mutex.RUnlock()
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.ensureInit()
	for testName, digests := range other.labels {
		e.setDigests(testName, digests)
	}
}

// ForAll implements the ReadOnly interface.
func (e *Expectations) ForAll(fn func(types.TestName, types.Digest, Label) error) error {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	for test, digests := range e.labels {
		for digest, label := range digests {
			err := fn(test, digest, label)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// DeepCopy makes a deep copy of the current expectations/baseline.
func (e *Expectations) DeepCopy() *Expectations {
	ret := Expectations{
		labels: make(map[types.TestName]map[types.Digest]Label, len(e.labels)),
	}
	ret.MergeExpectations(e)
	return &ret
}

// Classification implements the ReadOnly interface.
func (e *Expectations) Classification(test types.TestName, digest types.Digest) Label {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	if label, ok := e.labels[test][digest]; ok {
		return label
	}
	return Untriaged
}

// Empty implements the ReadOnly interface.
func (e *Expectations) Empty() bool {
	return e.NumTests() == 0
}

// NumTests implements the ReadOnly interface.
func (e *Expectations) NumTests() int {
	if e == nil {
		return 0
	}
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return len(e.labels)
}

// Len implements the ReadOnly interface.
func (e *Expectations) Len() int {
	if e == nil {
		return 0
	}
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	n := 0
	for _, d := range e.labels {
		n += len(d)
	}
	return n
}

// String returns an alphabetically sorted string representation
// of this object.
func (e *Expectations) String() string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	names := make([]string, 0, len(e.labels))
	for testName := range e.labels {
		names = append(names, string(testName))
	}
	sort.Strings(names)
	s := strings.Builder{}
	for _, testName := range names {
		digestMap := e.labels[types.TestName(testName)]
		digests := make([]string, 0, len(digestMap))
		for d := range digestMap {
			digests = append(digests, string(d))
		}
		sort.Strings(digests)
		_, _ = fmt.Fprintf(&s, "%s:\n", testName)
		for _, d := range digests {
			_, _ = fmt.Fprintf(&s, "\t%s : %s\n", d, digestMap[types.Digest(d)])
		}
	}
	return s.String()
}

// AsBaselineInt returns a copy that has all untriaged digests removed.
func (e *Expectations) AsBaselineInt() BaselineInt {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	n := Expectations{
		labels: map[types.TestName]map[types.Digest]Label{},
	}
	for testName, digests := range e.labels {
		for d, c := range digests {
			if c != Untriaged {
				n.Set(testName, d, c)
			}
		}
	}

	// Convert the above into a BaselineInt.
	// TODO(skbug.com/10522): Remove once the expectations.LabelStr -> expectations.Label refactoring
	//                        is complete.
	baseline := BaselineInt{}
	for testName, digestToLabelStrMap := range n.labels {
		digestToLabelMap := map[types.Digest]LabelInt{}
		for digest, labelStr := range digestToLabelStrMap {
			digestToLabelMap[digest] = LabelIntFromString(labelStr)
		}
		baseline[testName] = digestToLabelMap
	}

	return baseline
}

// ensureInit expects that the write mutex is held prior to entry.
func (e *Expectations) ensureInit() {
	if e.labels == nil {
		e.labels = map[types.TestName]map[types.Digest]Label{}
	}
}

// JoinedExp represents a chain of ReadOnly that could contain Labels.
// The Expectations at the beginning of the list override those that follow.
type JoinedExp []ReadOnly

// Join returns a Classifier that combines the given ReadOnly. If multiple ReadOnly have a
// Label for a given Grouping (Test+Digest), the order of the ReadOnly will break the tie by
// using the ReadOnly which was provided first.
func Join(first, second ReadOnly, others ...ReadOnly) JoinedExp {
	rv := []ReadOnly{first, second}
	rv = append(rv, others...)
	return rv
}

// Classification returns the first non-untriaged label for the given
// test and digest. If none of the given ReadOnly have a match, Untriaged is returned.
func (e JoinedExp) Classification(test types.TestName, digest types.Digest) Label {
	for _, exp := range e {
		if label := exp.Classification(test, digest); label != Untriaged {
			return label
		}
	}
	return Untriaged
}

// EmptyClassifier returns a Classifier which returns Untriaged for given input.
// Mostly used for testing.
func EmptyClassifier() Classifier {
	return JoinedExp{}
}
