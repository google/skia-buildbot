package expectations

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

func TestSet(t *testing.T) {
	unittest.SmallTest(t)

	var e Expectations
	e.Set("a", "pos", PositiveStr)
	e.Set("b", "neg", NegativeStr)
	e.Set("c", "untr", PositiveStr)
	e.Set("c", "untr", UntriagedStr)

	assert.Equal(t, e.Classification("a", "pos"), PositiveStr)
	assert.Equal(t, e.Classification("b", "neg"), NegativeStr)
	assert.Equal(t, e.Classification("c", "untr"), UntriagedStr)
	assert.Equal(t, e.Classification("d", "also_untriaged"), UntriagedStr)
	assert.Equal(t, e.Classification("a", "nope"), UntriagedStr)
	assert.Equal(t, e.Classification("b", "pos"), UntriagedStr)

	assert.Equal(t, 2, e.Len())
	assert.Equal(t, 3, e.NumTests()) // c was seen, but has all untriaged entries

	e.Set("c", "untr", PositiveStr)
	assert.Equal(t, e.Classification("c", "untr"), PositiveStr)
	assert.Equal(t, e.Classification("c", "nope"), UntriagedStr)
	assert.Equal(t, e.Classification("a", "nope"), UntriagedStr)

	assert.Equal(t, 3, e.Len())
	assert.Equal(t, 3, e.NumTests())

	e.Set("a", "oops", NegativeStr)
	assert.Equal(t, e.Classification("a", "oops"), NegativeStr)
	assert.Equal(t, 4, e.Len())
	assert.Equal(t, 3, e.NumTests())
}

func TestMerge(t *testing.T) {
	unittest.SmallTest(t)

	var e Expectations
	e.Set("a", "pos", PositiveStr)
	e.Set("b", "neg", PositiveStr)
	e.Set("c", "untr", UntriagedStr)

	f := Expectations{}            // test both ways of initialization
	f.Set("a", "neg", NegativeStr) // creates new in existing test
	f.Set("b", "neg", NegativeStr) // overwrites previous
	f.Set("d", "neg", NegativeStr) // creates new test

	e.MergeExpectations(&f)
	e.MergeExpectations(nil)

	assert.Equal(t, PositiveStr, e.Classification("a", "pos"))
	assert.Equal(t, NegativeStr, e.Classification("a", "neg"))
	assert.Equal(t, NegativeStr, e.Classification("b", "neg"))
	assert.Equal(t, UntriagedStr, e.Classification("c", "untr"))
	assert.Equal(t, NegativeStr, e.Classification("d", "neg"))

	assert.Equal(t, 4, e.Len())

	// f should be unchanged
	assert.Equal(t, UntriagedStr, f.Classification("a", "pos"))
	assert.Equal(t, NegativeStr, f.Classification("a", "neg"))
	assert.Equal(t, NegativeStr, f.Classification("b", "neg"))
	assert.Equal(t, UntriagedStr, f.Classification("c", "untr"))
	assert.Equal(t, NegativeStr, f.Classification("d", "neg"))
	assert.Equal(t, 3, f.Len())
}

func TestForAll(t *testing.T) {
	unittest.SmallTest(t)

	var e Expectations
	e.Set("a", "pos", PositiveStr)
	e.Set("b", "neg", NegativeStr)
	e.Set("c", "pos", PositiveStr)
	e.Set("c", "untr", UntriagedStr)

	labels := map[types.TestName]map[types.Digest]LabelStr{}
	err := e.ForAll(func(testName types.TestName, d types.Digest, l LabelStr) error {
		if digests, ok := labels[testName]; ok {
			digests[d] = l
		} else {
			labels[testName] = map[types.Digest]LabelStr{d: l}
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, map[types.TestName]map[types.Digest]LabelStr{
		"a": {
			"pos": PositiveStr,
		},
		"b": {
			"neg": NegativeStr,
		},
		"c": {
			"pos": PositiveStr,
		},
	}, labels)
}

// TestForAllError tests that we stop iterating through the entries when an error is returned.
func TestForAllError(t *testing.T) {
	unittest.SmallTest(t)

	var e Expectations
	e.Set("a", "pos", PositiveStr)
	e.Set("b", "neg", NegativeStr)
	e.Set("c", "pos", PositiveStr)
	e.Set("c", "untr", UntriagedStr)

	counter := 0
	err := e.ForAll(func(testName types.TestName, d types.Digest, l LabelStr) error {
		if counter == 2 {
			return errors.New("oops")
		}
		counter++
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "oops")
	assert.Equal(t, 2, counter)
}

func TestDeepCopy(t *testing.T) {
	unittest.SmallTest(t)

	var e Expectations
	e.Set("a", "pos", PositiveStr)

	f := e.DeepCopy()
	e.Set("b", "neg", NegativeStr)
	f.Set("b", "neg", PositiveStr)

	require.Equal(t, PositiveStr, e.Classification("a", "pos"))
	require.Equal(t, NegativeStr, e.Classification("b", "neg"))

	require.Equal(t, PositiveStr, f.Classification("a", "pos"))
	require.Equal(t, PositiveStr, f.Classification("b", "neg"))
}

func TestCounts(t *testing.T) {
	unittest.SmallTest(t)

	var e Expectations
	require.True(t, e.Empty())
	require.Equal(t, 0, e.NumTests())
	require.Equal(t, 0, e.Len())
	e.Set("a", "pos", PositiveStr)
	e.Set("b", "neg", NegativeStr)
	e.Set("c", "untr", UntriagedStr)
	e.Set("c", "pos", PositiveStr)
	e.Set("c", "neg", NegativeStr)

	require.False(t, e.Empty())
	assert.Equal(t, 3, e.NumTests())
	assert.Equal(t, 4, e.Len())

	// Make sure we are somewhat defensive and can handle nils gracefully
	var en *Expectations = nil
	assert.True(t, en.Empty())
	assert.Equal(t, 0, en.Len())
	assert.Equal(t, 0, en.NumTests())
}

func TestExpString(t *testing.T) {
	unittest.SmallTest(t)
	te := Expectations{
		labels: map[types.TestName]map[types.Digest]LabelStr{
			"beta": {
				"hash1": PositiveStr,
				"hash3": NegativeStr,
				"hash2": UntriagedStr,
			},
			"alpha": {
				"hashB": UntriagedStr,
				"hashA": NegativeStr,
				"hashC": UntriagedStr,
			},
		},
	}

	require.Equal(t, `alpha:
	hashA : negative
	hashB : untriaged
	hashC : untriaged
beta:
	hash1 : positive
	hash2 : untriaged
	hash3 : negative
`, te.String())
}

func TestAsBaseline(t *testing.T) {
	unittest.SmallTest(t)
	input := Expectations{
		labels: map[types.TestName]map[types.Digest]LabelStr{
			"gamma": {
				"hashX": UntriagedStr,
				"hashY": UntriagedStr,
				"hashZ": UntriagedStr,
			},
			"beta": {
				"hash1": PositiveStr,
				"hash3": NegativeStr,
				"hash2": UntriagedStr,
				"hash4": PositiveStr,
			},
			"alpha": {
				"hashB": UntriagedStr,
				"hashA": NegativeStr,
				"hashC": UntriagedStr,
			},
		},
	}

	expectedOutput := Baseline{
		"beta": {
			"hash1": PositiveInt,
			"hash3": NegativeInt,
			"hash4": PositiveInt,
		},
		"alpha": {
			"hashA": NegativeInt,
		},
	}
	require.Equal(t, expectedOutput, input.AsBaseline())
}

// All this test data is valid, but arbitrary.
const (
	alphaPositiveDigest = types.Digest("aaa884cd5ac3d6785c35cff8f26d2da5")
	betaNegativeDigest  = types.Digest("bbb8d94852dfde3f3bebcc000be60153")
	gammaPositiveDigest = types.Digest("ccc84ad6f1a0c628d5f27180e497309e")
	untriagedDigest     = types.Digest("7bf4d4e913605c0781697df4004191c5")
	testName            = types.TestName("some_test")
)

func TestJoin(t *testing.T) {
	unittest.SmallTest(t)

	var masterE Expectations
	masterE.Set(testName, alphaPositiveDigest, PositiveStr)
	masterE.Set(testName, betaNegativeDigest, PositiveStr)

	var changeListE Expectations
	changeListE.Set(testName, gammaPositiveDigest, PositiveStr)
	changeListE.Set(testName, betaNegativeDigest, NegativeStr) // this should win

	e := Join(&changeListE, &masterE)

	assert.Equal(t, PositiveStr, e.Classification(testName, alphaPositiveDigest))
	assert.Equal(t, PositiveStr, e.Classification(testName, gammaPositiveDigest))
	assert.Equal(t, NegativeStr, e.Classification(testName, betaNegativeDigest))
	assert.Equal(t, UntriagedStr, e.Classification(testName, untriagedDigest))
}

func TestEmptyClassifier(t *testing.T) {
	unittest.SmallTest(t)

	e := EmptyClassifier()
	assert.Equal(t, UntriagedStr, e.Classification(testName, alphaPositiveDigest))
	assert.Equal(t, UntriagedStr, e.Classification(testName, gammaPositiveDigest))
	assert.Equal(t, UntriagedStr, e.Classification(testName, betaNegativeDigest))
	assert.Equal(t, UntriagedStr, e.Classification(testName, untriagedDigest))
}
