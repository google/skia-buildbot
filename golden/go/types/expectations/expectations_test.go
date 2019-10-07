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
		"beta": map[types.Digest]Label{
			"hash1": POSITIVE,
			"hash3": NEGATIVE,
			"hash2": UNTRIAGED,
		},
		"alpha": map[types.Digest]Label{
			"hashB": UNTRIAGED,
			"hashA": NEGATIVE,
			"hashC": UNTRIAGED,
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
		"beta": map[types.Digest]Label{
			"hash1": POSITIVE,
			"hash3": NEGATIVE,
			"hash2": UNTRIAGED,
			"hash4": POSITIVE,
		},
		"alpha": map[types.Digest]Label{
			"hashB": UNTRIAGED,
			"hashA": NEGATIVE,
			"hashC": UNTRIAGED,
		},
	}

	expectedOutput := Expectations{
		"beta": map[types.Digest]Label{
			"hash1": POSITIVE,
			"hash4": POSITIVE,
		},
	}

	require.Equal(t, expectedOutput, input.AsBaseline())
}
