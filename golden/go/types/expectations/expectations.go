package expectations

import (
	"fmt"
	"sort"
	"strings"

	"go.skia.org/infra/golden/go/types"
)

// Expectations is a map[test_name][digest]Label that captures the expectations
// for a set of tests and digests as labels (Positive/Negative/Untriaged).
// Put another way, this data structure keeps track if a digest (image) is
// drawn correctly, incorrectly, or newly-seen for a given test.
type Expectations struct {
	labels map[types.TestName]map[types.Digest]Label
}

func WithLabels(exp map[types.TestName]map[types.Digest]Label) *Expectations {
	return &Expectations{
		labels: exp,
	}
}

// AddDigest is a convenience function to set the label for a test_name/digest pair. If the
// pair already exists it will be over written.
func (e *Expectations) AddDigest(testName types.TestName, digest types.Digest, label Label) {
	e.ensureInit()
	if testEntry, ok := e.labels[testName]; ok {
		testEntry[digest] = label
	} else {
		e.labels[testName] = map[types.Digest]Label{digest: label}
	}
}

// Empty returns true iff NumTests() == 0
func (e *Expectations) Empty() bool {
	return e.NumTests() == 0
}

// NumTests returns the number of tests that Expectations knows about.
func (e *Expectations) NumTests() int {
	if e == nil {
		return 0
	}
	return len(e.labels)
}

// Len returns the number of test/digest pairs stored.
func (e *Expectations) Len() int {
	if e == nil {
		return 0
	}
	n := 0
	for _, d := range e.labels {
		n += len(d)
	}
	return n
}

// ForAll will iterate through all entries in Expectations and call the callback with them.
// Iteration will stop if a non-nil error is returned (and will be forwarded to the caller).
func (e *Expectations) ForAll(fn func(types.TestName, types.Digest, Label) error) error {
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

// MergeExpectations adds the given expectations to the current expectations, letting
// the ones provided by the passed in parameter overwrite any existing data.
func (e *Expectations) MergeExpectations(other Expectations) {
	e.ensureInit()
	for testName, digests := range other.labels {
		e.addDigests(testName, digests)
	}
}

// addDigests is a convenience function to set the expectations of a set of digests for a
// given test_name.
func (e *Expectations) addDigests(testName types.TestName, digests map[types.Digest]Label) {
	testEntry, ok := e.labels[testName]
	if !ok {
		testEntry = make(map[types.Digest]Label, len(digests))
	}
	for digest, label := range digests {
		testEntry[digest] = label
	}
	e.labels[testName] = testEntry
}

// DeepCopy makes a deep copy of the current expectations/baseline.
func (e *Expectations) DeepCopy() Expectations {
	ret := Expectations{
		labels: make(map[types.TestName]map[types.Digest]Label, len(e.labels)),
	}
	ret.MergeExpectations(*e)
	return ret
}

// Classification returns the label for the given test/digest pair. By definition,
// this will return Untriaged if there isn't already a classification set.
func (e *Expectations) Classification(test types.TestName, digest types.Digest) Label {
	e.ensureInit()
	if label, ok := e.labels[test][digest]; ok {
		return label
	}
	return Untriaged
}

// String returns an alphabetically sorted string representation of this object.
func (e *Expectations) String() string {
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
			_, _ = fmt.Fprintf(&s, "\t%s : %s\n", d, digestMap[types.Digest(d)].String())
		}
	}
	return s.String()
}

// AsBaseline returns a copy that has all negative and untriaged digests removed.
func (e *Expectations) AsBaseline() map[types.TestName]map[types.Digest]Label {
	n := Expectations{
		labels: map[types.TestName]map[types.Digest]Label{},
	}
	for testName, digests := range e.labels {
		for d, c := range digests {
			if c == Positive {
				n.AddDigest(testName, d, Positive)
			}
		}
	}
	return n.labels
}

func (e *Expectations) ensureInit() {
	if e.labels == nil {
		e.labels = map[types.TestName]map[types.Digest]Label{}
	}
}
