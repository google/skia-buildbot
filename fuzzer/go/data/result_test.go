package data

import (
	"testing"

	"go.skia.org/infra/go/testutils"

	"github.com/stretchr/testify/assert"
)

func TestIsGrey(t *testing.T) {
	testutils.SmallTest(t)
	assert.True(t, isGrey(0, 0))
	assert.True(t, isGrey(TerminatedGracefully, TerminatedGracefully))
	assert.True(t, isGrey(TerminatedGracefully, TimedOut))
	assert.True(t, isGrey(TimedOut, TerminatedGracefully))
	assert.True(t, isGrey(TimedOut, TimedOut))
	assert.True(t, isGrey(TimedOut|TerminatedGracefully, TimedOut|TerminatedGracefully))

	assert.False(t, isGrey(SKAbortHit, TimedOut))
	assert.False(t, isGrey(TerminatedGracefully, SKPICTURE_DuringRendering|AssertionViolated))
	assert.False(t, isGrey(ClangCrashed, ClangCrashed))
}

func TestToHumanReadableFlags(t *testing.T) {
	testutils.SmallTest(t)
	expected := []string{"ASANCrashed", "ASAN_heap-use-after-free", "SKPICTURE_DuringRendering"}
	flag := ASANCrashed | ASAN_HeapUseAfterFree | SKPICTURE_DuringRendering

	assert.Equal(t, expected, flag.ToHumanReadableFlags())
}
