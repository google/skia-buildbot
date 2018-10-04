package types

type TestExp map[string]map[string]Label

func (t TestExp) AddDigest(testName, digest string, label Label) {
	if testEntry, ok := t[testName]; ok {
		testEntry[digest] = label
	} else {
		t[testName] = map[string]Label{digest: label}
	}
}

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

func (t TestExp) DeepCopy() TestExp {
	ret := make(TestExp, len(t))
	for testName, digests := range t {
		ret.AddDigests(testName, digests)
	}
	return ret
}

type Expectations interface {
	TestExp() TestExp
	AddTestExp(testExp TestExp)
	Classification(test, digest string) Label
}

func NewExpectations(testExp TestExp) Expectations {
	if testExp == nil {
		testExp = TestExp{}
	}

	return &basicExp{
		testExp: testExp,
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
