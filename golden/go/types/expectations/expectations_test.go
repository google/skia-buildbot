package expectations

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

func TestExpString(t *testing.T) {
	unittest.SmallTest(t)

	te := Expectations{
		"beta": map[types.Digest]Label{
			"hash1": Positive,
			"hash3": Negative,
			"hash2": Untriaged,
		},
		"alpha": map[types.Digest]Label{
			"hashB": Untriaged,
			"hashA": Negative,
			"hashC": Untriaged,
		},
	}

	assert.Equal(t, `alpha:
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
			"hash1": Positive,
			"hash3": Negative,
			"hash2": Untriaged,
			"hash4": Positive,
		},
		"alpha": map[types.Digest]Label{
			"hashB": Untriaged,
			"hashA": Negative,
			"hashC": Untriaged,
		},
	}

	expectedOutput := Expectations{
		"beta": map[types.Digest]Label{
			"hash1": Positive,
			"hash4": Positive,
		},
	}

	assert.Equal(t, expectedOutput, input.AsBaseline())
}
