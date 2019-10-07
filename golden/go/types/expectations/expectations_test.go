package expectations

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

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

	expectedOutput := Expectations{
		labels: map[types.TestName]map[types.Digest]Label{
			"beta": {
				"hash1": Positive,
				"hash4": Positive,
			},
		},
	}

	require.Equal(t, expectedOutput, input.AsBaseline())
}
