package types

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// TestExp is a map[test_name][digest]Label that captures the expectations or baselines
// for a set of tests and digests as labels (POSITIVE/NEGATIVE/UNTRIAGED).
// It is used throughout Gold to hold expectations/baselines.
type Expectations map[TestName]map[Digest]Label

// AddDigest is a convenience function to set the label for a test_name/digest pair. If the
// pair already exists it will be over written.
func (e Expectations) AddDigest(testName TestName, digest Digest, label Label) {
	if testEntry, ok := e[testName]; ok {
		testEntry[digest] = label
	} else {
		e[testName] = map[Digest]Label{digest: label}
	}
}

// AddDigests is a convenience function to set the expectations of a set of digests for a
// given test_name.
func (e Expectations) AddDigests(testName TestName, digests map[Digest]Label) {
	testEntry, ok := e[testName]
	if !ok {
		testEntry = make(map[Digest]Label, len(digests))
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
		if _, ok := e[testName]; !ok {
			e[testName] = map[Digest]Label{}
		}
		for digest, label := range digests {
			// UNTRIAGED is the default value, so if the passed in version
			// is explicitly setting a label to UNTRIAGED, we delete what
			// was already there.
			if label == UNTRIAGED {
				delete(e[testName], digest)
			} else {
				e[testName][digest] = label
			}
		}
		// In case we had only assigned UNTRIAGED values
		if len(e[testName]) == 0 {
			delete(e, testName)
		}
	}
}

// Update updates the current expectations with the expectations in 'right'. Any existing
// test_name/digest pair will be overwritten.
func (e Expectations) Update(right Expectations) {
	for testName, digests := range right {
		e.AddDigests(testName, digests)
	}
}

// DeepCopy makes a deep copy of the current expectations/baseline.
func (e Expectations) DeepCopy() Expectations {
	ret := make(Expectations, len(e))
	for testName, digests := range e {
		ret.AddDigests(testName, digests)
	}
	return ret
}

// Classification returns the label for the given test/digest pair. By definition,
// this will return UNTRIAGED if there isn't already a classification set.
func (e Expectations) Classification(test TestName, digest Digest) Label {
	if label, ok := e[test][digest]; ok {
		return label
	}
	return UNTRIAGED
}

// String returns an alphabetically sorted string representation
// of this object.
func (t Expectations) String() string {
	names := make([]string, 0, len(t))
	for testName := range t {
		names = append(names, string(testName))
	}
	sort.Strings(names)
	s := strings.Builder{}
	for _, testName := range names {
		digestMap := t[TestName(testName)]
		digests := make([]string, 0, len(digestMap))
		for d := range digestMap {
			digests = append(digests, string(d))
		}
		sort.Strings(digests)
		_, _ = fmt.Fprintf(&s, "%s:\n", testName)
		for _, d := range digests {
			_, _ = fmt.Fprintf(&s, "\t%s : %s\n", d, digestMap[Digest(d)].String())
		}
	}
	return s.String()
}

// // TestExpBuilder is an interface to interact with expectations. It is mostly a wrapper around
// // a TestExp instance, but does some intermediate processing if necessary and might filter results.
// type TestExpBuilder interface {
// 	// TestExp returns the underlying expectations. Generally this should not be altered since any
// 	// alteration would also change the TestExpBuilder instance. If alterations are necessary
// 	// the caller should make a DeepCopy of the returned value and wrap it in a new instance of
// 	// TestExpBuilder.
// 	Expectations() Expectations

// 	// MergeExpectations adds the given expectations. It might do some filtering before it writes it to the
// 	// underlying datastructure.
// 	MergeExpectations(testExp Expectations)

// 	// SetExpectation sets the given test/digest pair to the given label.
// 	SetExpectation(test TestName, digest Digest, label Label)

// 	Classification(test TestName, digest Digest) Label
// }

// // NewTestExpBuilder returns an TestExpBuilder instance that wraps around an instance
// // of TestExp. If 'testExp' is not nil, then NewTestExpBuilder will take ownership of it and
// // wrap around it.
// func NewTestExpBuilder(testExp Expectations) *BuilderImpl {
// 	if testExp == nil {
// 		testExp = Expectations{}
// 	}

// 	return &BuilderImpl{
// 		testExp: testExp,
// 	}
// }

// // BuilderImpl is the canonical implementation of the Expectations interface. It wraps a
// // instance of TestExp. It uses a pointer receiver to a struct to make the Marshaller/Unmarshaler
// // interface work for the encoding/json package.
// type BuilderImpl struct {
// 	testExp Expectations
// }

// // TestExp implements the TestExpBuilder interface.
// func (b *BuilderImpl) Expectations() Expectations {
// 	return b.testExp
// }

// // SetExpectation implements the TestExpBuilder interface.
// func (b *BuilderImpl) SetExpectation(testName TestName, digest Digest, label Label) {
// 	b.testExp.AddDigest(testName, digest, label)
// }

// MarshalJSON implements json.Marshaller interface
func (b *Expectations) MarshalJSON() ([]byte, error) {
	return json.Marshal(b)
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (b *Expectations) UnmarshalJSON(data []byte) error {
	// TODO(stephana) once all test assets are converted the following code to handle the old
	// JSON serialization (from the expstorage package) can be removed.
	oldFormat := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &oldFormat); err != nil {
		return err
	}
	if oldBytes, ok := oldFormat["tests"]; ok {
		return json.Unmarshal([]byte(oldBytes), b)
	}

	// Simply de-serialize it as a map to testExp.
	return json.Unmarshal(data, b)
}
