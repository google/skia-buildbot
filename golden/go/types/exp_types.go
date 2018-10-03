package types

type TestExp map[string]map[string]Label

func (t TestExp) DeepCopy() TestExp {
	ret := make(TestExp, len(t))
	for testName, digests := range t {
		newDigests := make(map[string]Label, len(digests))
		for digest, label := range digests {
			newDigests[digest] = label
		}
		ret[testName] = newDigests
	}
	return ret
}

type Expectations interface {
	TestExp() TestExp
	Classification(test, digest string) Label
}

func NewExpectations() Expectations {
	return &basicExp{
		testExp: TestExp{},
	}
}

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

func (b *basicExp) AddDigests(testExp TestExp) {
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

// SetTestExpectation sets the label (expectation) for a single test/digest pair.
func (b *basicExp) SetTestExpectation(testName string, digest string, label Label) {
	if _, ok := b.testExp[testName]; !ok {
		b.testExp[testName] = map[string]Label{}
	}
	b.testExp[testName][digest] = label
}

func (b *basicExp) DeepCopy() Expectations {
	return &basicExp{
		testExp: b.testExp.DeepCopy(),
	}
}
