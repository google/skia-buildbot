package expectations

import (
	"fmt"
	"sort"
	"strings"

	"go.skia.org/infra/golden/go/types"
)

// Expectations captures the expectations for a set of tests and digests as
// labels (Positive/Negative/Untriaged).
// Put another way, this data structure keeps track if a digest (image) is
// drawn correctly, incorrectly, or newly-seen for a given test.
type Expectations struct {
	// TODO(kjlubick) Consider storing this as
	//   map[digestGrouping]Label where
	//   type digestGrouping struct {
	//     digest types.Digest
	//     grouping types.TestName
	//   }
	labels map[types.TestName]map[types.Digest]Label
}

// Baseline is a simplified view of the Expectations, suitable for JSON encoding.
// A Baseline only has entries with positive labels.
type Baseline map[types.TestName]map[types.Digest]Label

// Set is a convenience function to set the label for a test_name/digest pair. If the
// pair already exists it will be over written.
func (e *Expectations) Set(testName types.TestName, digest types.Digest, label Label) {
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
// given test_name.
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
// the ones provided by the passed in parameter overwrite any existing data.
func (e *Expectations) MergeExpectations(other Expectations) {
	e.ensureInit()
	for testName, digests := range other.labels {
		e.setDigests(testName, digests)
	}
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
	if label, ok := e.labels[test][digest]; ok {
		return label
	}
	return Untriaged
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

// String returns an alphabetically sorted string representation
// of this object.
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
func (e *Expectations) AsBaseline() Baseline {
	n := Expectations{
		labels: map[types.TestName]map[types.Digest]Label{},
	}
	for testName, digests := range e.labels {
		for d, c := range digests {
			if c == Positive {
				n.Set(testName, d, Positive)
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
