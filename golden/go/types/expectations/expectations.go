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
type Expectations map[types.TestName]map[types.Digest]Label

// AddDigest is a convenience function to set the label for a test_name/digest pair. If the
// pair already exists it will be over written.
func (e Expectations) AddDigest(testName types.TestName, digest types.Digest, label Label) {
	if testEntry, ok := e[testName]; ok {
		testEntry[digest] = label
	} else {
		e[testName] = map[types.Digest]Label{digest: label}
	}
}

// AddDigests is a convenience function to set the expectations of a set of digests for a
// given test_name.
func (e Expectations) AddDigests(testName types.TestName, digests map[types.Digest]Label) {
	testEntry, ok := e[testName]
	if !ok {
		testEntry = make(map[types.Digest]Label, len(digests))
	}
	for digest, label := range digests {
		testEntry[digest] = label
	}
	e[testName] = testEntry
}

// MergeExpectations adds the given expectations to the current expectations, letting
// the ones provided by the passed in parameter overwrite any existing data.
func (e Expectations) MergeExpectations(other Expectations) {
	for testName, digests := range other {
		e.AddDigests(testName, digests)
	}
}

// ForAll will iterate through all entries in Expectations and call the callback with them.
// Iteration will stop if a non-nil error is returned (and will be forwarded to the caller).
func (e Expectations) ForAll(fn func(types.TestName, types.Digest, Label) error) error {
	for test, digests := range e {
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
func (e Expectations) DeepCopy() Expectations {
	ret := make(Expectations, len(e))
	ret.MergeExpectations(e)
	return ret
}

// Classification returns the label for the given test/digest pair. By definition,
// this will return Untriaged if there isn't already a classification set.
func (e Expectations) Classification(test types.TestName, digest types.Digest) Label {
	if label, ok := e[test][digest]; ok {
		return label
	}
	return Untriaged
}

// String returns an alphabetically sorted string representation
// of this object.
func (e Expectations) String() string {
	names := make([]string, 0, len(e))
	for testName := range e {
		names = append(names, string(testName))
	}
	sort.Strings(names)
	s := strings.Builder{}
	for _, testName := range names {
		digestMap := e[types.TestName(testName)]
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
func (e Expectations) AsBaseline() Expectations {
	n := Expectations{}
	for testName, digests := range e {
		for d, c := range digests {
			if c == Positive {
				n.AddDigest(testName, d, Positive)
			}
		}
	}
	return n
}
