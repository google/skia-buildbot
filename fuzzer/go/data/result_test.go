package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestIsGrey(t *testing.T) {
	unittest.SmallTest(t)
	assert.True(t, isGrey(0))
	assert.True(t, isGrey(TerminatedGracefully))
	assert.True(t, isGrey(TimedOut))
	assert.True(t, isGrey(TimedOut|TerminatedGracefully))
	assert.True(t, isGrey(BadAlloc))

	assert.False(t, isGrey(SKAbortHit))
	assert.False(t, isGrey(AssertionViolated))
	assert.False(t, isGrey(ClangCrashed))
}

func TestToHumanReadableFlags(t *testing.T) {
	unittest.SmallTest(t)
	expected := []string{"ASANCrashed", "ASAN_heap-use-after-free"}
	flag := ASANCrashed | ASAN_HeapUseAfterFree

	assert.Equal(t, expected, flag.ToHumanReadableFlags())
}
