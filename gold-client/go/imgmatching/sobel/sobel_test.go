package sobel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/gold-client/go/imgmatching"
)

func TestSobelFuzzyMatcher_IdenticalImages_ReturnsTrue(t *testing.T) {
	unittest.SmallTest(t)

	// TODO(lovisolo): Implement.
}

func TestSobelFuzzyMatcher_ImplementsMatcherInterface(t *testing.T) {
	unittest.SmallTest(t)
	assert.Implements(t, (*imgmatching.Matcher)(nil), &SobelFuzzyMatcher{})
}
