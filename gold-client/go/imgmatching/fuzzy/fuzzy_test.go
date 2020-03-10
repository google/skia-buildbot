package fuzzy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/gold-client/go/imgmatching"
)

func TestFuzzyMatcher_IdenticalImages_ReturnsTrue(t *testing.T) {
	unittest.SmallTest(t)

	// TODO(lovisolo): Implement.
}

func TestFuzzyMatcher_ImplementsMatcherInterface(t *testing.T) {
	unittest.SmallTest(t)
	assert.Implements(t, (*imgmatching.Matcher)(nil), &FuzzyMatcher{})
}
