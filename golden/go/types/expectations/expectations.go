package expectations

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go.skia.org/infra/golden/go/types"
)

// Expectations is a map[test_name][digest]Label that captures the expectations
// for a set of tests and digests as labels (Positive/Negative/Untriaged).
// Put another way, this data structure keeps track if a digest (image) is
// drawn correctly, incorrectly, or newly-seen for a given test.
type Expectations struct {
	labels map[types.TestName]map[types.Digest]Label
}

// Comment represents any comments a test+digest has.
type Comment struct {
	Author string
	Body   string
	TS     time.Time
}

// AddDigest is a convenience function to set the label for a test_name/digest pair. If the
// pair already exists it will be over written.
func (e Expectations) AddDigest(testName types.TestName, digest types.Digest, label Label) {
	if testEntry, ok := e.labels[testName]; ok {
		testEntry[digest] = label
	} else {
		e.labels[testName] = map[types.Digest]Label{digest: label}
	}
}

// AddDigests is a convenience function to set the expectations of a set of digests for a
// given test_name.
func (e Expectations) AddDigests(testName types.TestName, digests map[types.Digest]Label) {
	testEntry, ok := e.labels[testName]
	if !ok {
		testEntry = make(map[types.Digest]Label, len(digests))
	}
	for digest, label := range digests {
		testEntry[digest] = label
	}
	e.labels[testName] = testEntry
}

// MergeExpectations adds the given expectations to the current expectations, letting
// the ones provided by the passed in parameter overwrite any existing data.
func (e Expectations) MergeExpectations(other Expectations) {
	for testName, digests := range other.labels {
		e.AddDigests(testName, digests)
	}
}

// DeepCopy makes a deep copy of the current expectations/baseline.
func (e Expectations) DeepCopy() Expectations {
	ret := Expectations{
		labels: make(map[types.TestName]map[types.Digest]Label, len(e.labels)),
	}
	ret.MergeExpectations(e)
	return ret
}

// Classification returns the label for the given test/digest pair. By definition,
// this will return Untriaged if there isn't already a classification set.
func (e Expectations) Classification(test types.TestName, digest types.Digest) Label {
	if label, ok := e.labels[test][digest]; ok {
		return label
	}
	return Untriaged
}

// Comments returns a slice of comments associated with this test+digest pair.
func (e Expectations) Comments(test types.TestName, digest types.Digest) []Comment {
	return nil
}

// String returns an alphabetically sorted string representation
// of this object.
func (e Expectations) String() string {
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
func (e Expectations) AsBaseline() Expectations {
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
	return n
}
