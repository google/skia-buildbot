package imgmatching

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestMatcherFactoryImpl_NoAlgorithmSpecified_ReturnsExactMatching(t *testing.T) {
	unittest.SmallTest(t)

	f := MatcherFactoryImpl{}
	algorithmName, matcher, err := f.Make(map[string]string{})

	assert.NoError(t, err)
	assert.Equal(t, algorithmName, ExactMatching)
	assert.Nil(t, matcher)
}
