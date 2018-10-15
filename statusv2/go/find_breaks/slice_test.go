package find_breaks

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
)

func TestSliceValid(t *testing.T) {
	testutils.SmallTest(t)
	assert.False(t, newSlice(-1, 0).Valid())
	assert.True(t, newSlice(-1, -1).Valid())
	assert.True(t, newSlice(10, 10).Valid())
	assert.False(t, newSlice(11, 10).Valid())
	assert.False(t, newSlice(-1, 0).Valid())
}

func TestSliceLen(t *testing.T) {
	testutils.SmallTest(t)
	assert.Equal(t, 0, newSlice(-1, -1).Len())
	assert.Equal(t, 0, newSlice(15, 3).Len())
	assert.Equal(t, 0, newSlice(-1, 0).Len())
	assert.Equal(t, 0, newSlice(0, 0).Len())
	assert.Equal(t, 1, newSlice(0, 1).Len())
	assert.Equal(t, 10, newSlice(0, 10).Len())
}

func TestSliceEmpty(t *testing.T) {
	testutils.SmallTest(t)
	assert.True(t, newSlice(-1, -1).Empty())
	assert.True(t, newSlice(0, 0).Empty())
	assert.False(t, newSlice(0, 1).Empty())
	assert.False(t, newSlice(0, 10).Empty())
}

func TestSliceCopy(t *testing.T) {
	testutils.SmallTest(t)
	test := func(s slice) {
		deepequal.AssertDeepEqual(t, s, s.Copy())
	}
	test(newSlice(-1, -1))
	test(newSlice(0, 0))
	test(newSlice(0, 1))
	test(newSlice(0, 10))
}

func TestSliceOverlap(t *testing.T) {
	testutils.SmallTest(t)
	test := func(a, b, expect slice) {
		deepequal.AssertDeepEqual(t, expect, a.Overlap(b))
		deepequal.AssertDeepEqual(t, expect, b.Overlap(a))
	}
	test(newSlice(-1, -1), newSlice(-1, -1), newSlice(-1, -1))
	test(newSlice(-5, -7), newSlice(-4, -8), newSlice(-1, -1))
	test(newSlice(0, 1), newSlice(0, 1), newSlice(0, 1))
	test(newSlice(0, 2), newSlice(1, 3), newSlice(1, 2))
	test(newSlice(4, 5), newSlice(0, 35), newSlice(4, 5))
	test(newSlice(-1, 10), newSlice(4, 5), newSlice(-1, -1))
}

func TestMakeSlice(t *testing.T) {
	testutils.SmallTest(t)
	test := func(sub, super []string, start, end int) {
		s := makeSlice(sub, super)
		deepequal.AssertDeepEqual(t, newSlice(start, end), s)
	}

	// Actual subslices.
	test([]string{"a", "b"}, []string{"a", "b", "c"}, 0, 2)
	test([]string{"b"}, []string{"a", "b", "c"}, 1, 2)
	test([]string{"b"}, []string{"a", "b", "c", "d", "e", "f", "g"}, 1, 2)
	test([]string{"d"}, []string{"b", "c", "d"}, 2, 3)

	// Subslice not in parent slice.
	test([]string{"q"}, []string{"a", "b", "c"}, -1, -1)

	// Subslice extends outside of parent slice. It should get trimmed to
	// the parent slice.
	test([]string{"a", "b"}, []string{"b", "c"}, 0, 1)
	test([]string{"b", "c"}, []string{"a", "b"}, 1, 2)
}
