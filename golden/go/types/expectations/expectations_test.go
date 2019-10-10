package expectations

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

func TestAdd(t *testing.T) {
	unittest.SmallTest(t)

	var e Expectations
	e.AddDigest("a", "pos", Positive)
	e.AddDigest("b", "neg", Negative)
	e.AddDigest("c", "untr", Untriaged)

	require.Equal(t, e.Classification("a", "pos"), Positive)
	require.Equal(t, e.Classification("b", "neg"), Negative)
	require.Equal(t, e.Classification("c", "untr"), Untriaged)
	require.Equal(t, e.Classification("d", "also_untriaged"), Untriaged)
	require.Equal(t, e.Classification("a", "nope"), Untriaged)
	require.Equal(t, e.Classification("b", "pos"), Untriaged)

	e.AddDigest("c", "untr", Positive)
	require.Equal(t, e.Classification("c", "untr"), Positive)
	require.Equal(t, e.Classification("c", "nope"), Untriaged)
	require.Equal(t, e.Classification("a", "nope"), Untriaged)

	e.AddDigest("a", "oops", Negative)
	require.Equal(t, e.Classification("a", "oops"), Negative)
}

func TestMerge(t *testing.T) {
	unittest.SmallTest(t)

	var e Expectations
	e.AddDigest("a", "pos", Positive)
	e.AddDigest("b", "neg", Positive)
	e.AddDigest("c", "untr", Untriaged)

	f := Expectations{}               // test both ways of initialization
	f.AddDigest("a", "neg", Negative) // creates new in existing test
	f.AddDigest("b", "neg", Negative) // overwrites previous
	f.AddDigest("d", "neg", Negative) // creates new test

	e.MergeExpectations(f)

	require.Equal(t, Positive, e.Classification("a", "pos"))
	require.Equal(t, Negative, e.Classification("a", "neg"))
	require.Equal(t, Negative, e.Classification("b", "neg"))
	require.Equal(t, Untriaged, e.Classification("c", "unt"))
	require.Equal(t, Negative, e.Classification("d", "neg"))

	// f should be unchanged
	require.Equal(t, Untriaged, f.Classification("a", "pos"))
	require.Equal(t, Negative, f.Classification("a", "neg"))
	require.Equal(t, Negative, f.Classification("b", "neg"))
	require.Equal(t, Untriaged, f.Classification("c", "unt"))
	require.Equal(t, Negative, f.Classification("d", "neg"))
}

func TestForAll(t *testing.T) {
	unittest.SmallTest(t)

	var e Expectations
	e.AddDigest("a", "pos", Positive)
	e.AddDigest("b", "neg", Negative)
	e.AddDigest("c", "untr", Untriaged)
	e.AddDigest("c", "pos", Positive)

	labels := map[types.TestName]map[types.Digest]Label{}
	err := e.ForAll(func(testName types.TestName, d types.Digest, l Label) error {
		if digests, ok := labels[testName]; ok {
			digests[d] = l
		} else {
			labels[testName] = map[types.Digest]Label{d: l}
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, map[types.TestName]map[types.Digest]Label{
		"a": {
			"pos": Positive,
		},
		"b": {
			"neg": Negative,
		},
		"c": {
			"untr": Untriaged,
			"pos":  Positive,
		},
	}, labels)
}

// TestForAllError tests that we stop iterating through the entries when an error is returned.
func TestForAllError(t *testing.T) {
	unittest.SmallTest(t)

	var e Expectations
	e.AddDigest("a", "pos", Positive)
	e.AddDigest("b", "neg", Negative)
	e.AddDigest("c", "untr", Untriaged)
	e.AddDigest("c", "pos", Positive)

	counter := 0
	err := e.ForAll(func(testName types.TestName, d types.Digest, l Label) error {
		if counter == 2 {
			return errors.New("oops")
		}
		counter++
		return nil
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "oops")
	require.Equal(t, 2, counter)
}

func TestDeepCopy(t *testing.T) {
	unittest.SmallTest(t)

	var e Expectations
	e.AddDigest("a", "pos", Positive)

	f := e.DeepCopy()
	e.AddDigest("b", "neg", Negative)
	f.AddDigest("b", "neg", Positive)

	require.Equal(t, Positive, e.Classification("a", "pos"))
	require.Equal(t, Negative, e.Classification("b", "neg"))

	require.Equal(t, Positive, f.Classification("a", "pos"))
	require.Equal(t, Positive, f.Classification("b", "neg"))
}

func TestCounts(t *testing.T) {
	unittest.SmallTest(t)

	var e Expectations
	require.True(t, e.Empty())
	require.Equal(t, 0, e.NumTests())
	require.Equal(t, 0, e.Len())
	e.AddDigest("a", "pos", Positive)
	e.AddDigest("b", "neg", Negative)
	e.AddDigest("c", "untr", Untriaged)
	e.AddDigest("c", "pos", Positive)

	require.False(t, e.Empty())
	require.Equal(t, 3, e.NumTests())
	require.Equal(t, 4, e.Len())
}

func TestExpString(t *testing.T) {
	unittest.SmallTest(t)
	te := Expectations{
		labels: map[types.TestName]map[types.Digest]Label{
			"beta": {
				"hash1": Positive,
				"hash3": Negative,
				"hash2": Untriaged,
			},
			"alpha": {
				"hashB": Untriaged,
				"hashA": Negative,
				"hashC": Untriaged,
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
		labels: map[types.TestName]map[types.Digest]Label{
			"beta": {
				"hash1": Positive,
				"hash3": Negative,
				"hash2": Untriaged,
				"hash4": Positive,
			},
			"alpha": {
				"hashB": Untriaged,
				"hashA": Negative,
				"hashC": Untriaged,
			},
		},
	}

	expectedOutput := map[types.TestName]map[types.Digest]Label{
		"beta": {
			"hash1": Positive,
			"hash4": Positive,
		},
	}
	require.Equal(t, expectedOutput, input.AsBaseline())
}
