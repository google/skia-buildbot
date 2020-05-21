package child

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestSemVerGCSCompareSemanticVersions(t *testing.T) {
	unittest.SmallTest(t)

	test := func(a, b []int, expect int) {
		require.Equal(t, expect, compareSemanticVersions(a, b))
	}
	test([]int{}, []int{}, 0)
	test([]int{}, []int{1}, 1)
	test([]int{1}, []int{}, -1)
	test([]int{1}, []int{1}, 0)
	test([]int{0}, []int{1}, 1)
	test([]int{1}, []int{0}, -1)
	test([]int{1, 1}, []int{1, 0}, -1)
	test([]int{1}, []int{1, 0}, 1)
	test([]int{1, 0}, []int{1}, -1)
}

func TestSemVerGCSShortRev(t *testing.T) {
	unittest.SmallTest(t)
	require.Equal(t, "123", semVerShortRev("\\d+", "abc123def"))
	require.Equal(t, "123", semVerShortRev("\\d+", "abc123def456"))
	require.Equal(t, "abc123def456", semVerShortRev("[a-z]+\\d+[a-z]+\\d+", "abc123def456"))
	require.Equal(t, "456", semVerShortRev("[a-z]+\\d+[a-z]+(\\d+)", "abc123def456"))
	require.Equal(t, "123", semVerShortRev("[a-z]+(\\d+)[a-z]+(\\d+)", "abc123def456"))
	require.Equal(t, "abcdef0123456789abcdef0123456789abcdef01", semVerShortRev(".{7}.{33}", "abcdef0123456789abcdef0123456789abcdef01"))
	require.Equal(t, "abcdef0", semVerShortRev("(.{7}).{33}", "abcdef0123456789abcdef0123456789abcdef01"))
}
