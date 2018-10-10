package types

import (
	"encoding/json"
)

// TestExp is a map[test_name][digest]Label that captures the expectations or baselines
// for a set of tests and digests as labels (POSITIVE/NEGATIVE/UNTRIAGED).
// It is used throughout Gold to hold expectations/baselines.
type TestExp map[string]map[string]Label

// AddDigest is a convenience function to set the label for a test_name/digest pair. If the
// pair already exists it will be over written.
func (t TestExp) AddDigest(testName, digest string, label Label) {
	if testEntry, ok := t[testName]; ok {
		testEntry[digest] = label
	} else {
		t[testName] = map[string]Label{digest: label}
	}
}

// AddDigests is a convenience function to set the expectations of a set of digests for a
// given test_name.
func (t TestExp) AddDigests(testName string, digests map[string]Label) {
	testEntry, ok := t[testName]
	if !ok {
		testEntry = make(map[string]Label, len(digests))
	}
	for digest, label := range digests {
		testEntry[digest] = label
	}
	t[testName] = testEntry
}

// Update updates the current expectations with the expectations in 'right'. Any existing
// test_name/digest pair will be overwritten.
func (t TestExp) Update(right TestExp) {
	for testName, digests := range right {
		t.AddDigests(testName, digests)
	}
}

// DeepCopy makes a deep copy of the current expectations/baseline.
func (t TestExp) DeepCopy() TestExp {
	ret := make(TestExp, len(t))
	for testName, digests := range t {
		ret.AddDigests(testName, digests)
	}
	return ret
}

// Expectations is an interface to interact with expectations. It is mostly a wrapper around
// a TestExp instance, but does some intermediate processing if necessary and might filter results.
type Expectations interface {
	// TestExp returns the underlying expectations. Generally this should not be altered since any
	// alteration would also change the Expectations instance. If alterations are necessary the caller
	// should make a DeepCopy of the returned value and wrap it in a new instance of Expectations.
	TestExp() TestExp

	// AddTestExp adds the given expectations. It might do some filtering before it writes it to the
	// underlying datastructure.
	AddTestExp(testExp TestExp)

	// SetExpectations sets the given test/digest pair to the given label.
	SetExpectation(test, digest string, label Label)

	// Classification returns the label for the given test/digest pair.
	Classification(test, digest string) Label
}

// NewExpectations returns an Expecations instance that wraps around an instance of TestExp.
// If 'testExp' is not nil, then NewExpecations will take ownership of it and wrap around it.
func NewExpectations(testExp TestExp) Expectations {
	if testExp == nil {
		testExp = TestExp{}
	}

	return &basicExp{
		testExp: testExp,
	}
}

// basicExp is the canonical implementation of the Expectations interface. It wraps a
// instance of TestExp. It uses a pointer receiver to a struct to make the Marshaller/Unmarshaler
// interface work for the encoding/json package.
type basicExp struct {
	testExp TestExp
}

func (b *basicExp) TestExp() TestExp {
	return b.testExp
}

func (b *basicExp) Classification(test, digest string) Label {
	if label, ok := b.testExp[test][digest]; ok {
		return label
	}
	return UNTRIAGED
}

func (b *basicExp) AddTestExp(testExp TestExp) {
	for testName, digests := range testExp {
		if _, ok := b.testExp[testName]; !ok {
			b.testExp[testName] = map[string]Label{}
		}
		for digest, label := range digests {
			// UNTRIAGED is the default value and we don't need to store it
			if label == UNTRIAGED {
				delete(b.testExp[testName], digest)
			} else {
				b.testExp[testName][digest] = label
			}
		}
		// In case we had only assigned UNTRIAGED values
		if len(b.testExp[testName]) == 0 {
			delete(b.testExp, testName)
		}
	}
}

// SetExpectation implements the Expectations interface.
func (b *basicExp) SetExpectation(testName string, digest string, label Label) {
	b.testExp.AddDigest(testName, digest, label)
}

// MarshalJSON implements json.Marshaller interface
func (b *basicExp) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.testExp)
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (b *basicExp) UnmarshalJSON(data []byte) error {
	// TODO(stephana) once all test assets are converted the following code to handle the old
	// JSON serialization (from the expstorage package) can be removed.
	oldFormat := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &oldFormat); err != nil {
		return err
	}
	if oldBytes, ok := oldFormat["tests"]; ok {
		return json.Unmarshal([]byte(oldBytes), &b.testExp)
	}

	// Simply de-serialize it as a map to testExp.
	return json.Unmarshal(data, &b.testExp)
}
