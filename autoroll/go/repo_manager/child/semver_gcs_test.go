package child

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestCompareSemanticVersions(t *testing.T) {
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
