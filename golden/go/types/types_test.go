package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
)

func TestGoldenTrace(t *testing.T) {
	unittest.SmallTest(t)
	N := 5
	// Test NewGoldenTrace.
	g := NewGoldenTraceN(N, nil)
	assert.Equal(t, N, g.Len(), "wrong digests size")
	assert.Equal(t, 0, len(g.Keys), "wrong keys initial size")
	g.Digests[0] = "a digest"

	assert.True(t, g.IsMissing(1), "values start missing")
	assert.False(t, g.IsMissing(0), "set values shouldn't be missing")

	// Test Merge.
	M := 7
	gm := NewGoldenTraceN(M, nil)
	gm.Digests[1] = "another digest"
	g2 := g.Merge(gm)
	assert.Equal(t, N+M, g2.Len(), "merge length wrong")
	assert.Equal(t, Digest("a digest"), g2.(*GoldenTrace).Digests[0])
	assert.Equal(t, Digest("another digest"), g2.(*GoldenTrace).Digests[6])

	// Test Grow.
	g = NewGoldenTraceN(N, nil)
	g.Digests[0] = "foo"
	g.Grow(2*N, tiling.FILL_BEFORE)
	assert.Equal(t, Digest("foo"), g.Digests[N], "Grow didn't FILL_BEFORE correctly")

	g = NewGoldenTraceN(N, nil)
	g.Digests[0] = "foo"
	g.Grow(2*N, tiling.FILL_AFTER)
	assert.Equal(t, Digest("foo"), g.Digests[0], "Grow didn't FILL_AFTER correctly")

	// Test Trim
	g = NewGoldenTraceN(N, nil)
	g.Digests[1] = "foo"
	require.NoError(t, g.Trim(1, 3))
	assert.Equal(t, Digest("foo"), g.Digests[0], "Trim didn't copy correctly")
	assert.Equal(t, 2, g.Len(), "Trim wrong length")

	assert.Error(t, g.Trim(-1, 1))
	assert.Error(t, g.Trim(1, 3))
	assert.Error(t, g.Trim(2, 1))

	require.NoError(t, g.Trim(1, 1))
	assert.Equal(t, 0, g.Len(), "final size wrong")
}

func TestSetAt(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		want Digest
	}{
		{
			want: "",
		},
		{
			want: "abcd",
		},
		{
			want: MISSING_DIGEST,
		},
	}
	tr := NewGoldenTraceN(len(testCases), nil)
	for i, tc := range testCases {
		require.NoError(t, tr.SetAt(i, []byte(tc.want)))
	}
	for i, tc := range testCases {
		assert.Equal(t, tc.want, tr.Digests[i], "Bad at test case %d", i)
	}
}
