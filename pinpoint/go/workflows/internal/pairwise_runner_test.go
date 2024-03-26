package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGeneratePairIndices_GenerateRandomPair(t *testing.T) {
	generate_even := func(count int) []int {
		lt := make([]int, count)
		for i := range lt {
			lt[i] = i % 2
		}
		return lt
	}
	verify := func(name string, generated []int, even []int) {
		t.Run(name, func(t *testing.T) {
			// This can still happen because this is one of the random cases, then we should change to
			// a different seed.
			assert.NotEqualValues(t, generated, even, "shuffled pairs are still evenly distributed.")
			ct := 0
			for i := range generated {
				ct = ct + generated[i]
			}
			assert.EqualValues(t, len(generated)/2, ct, "pairs don't have equal 0's and 1's.")
		})
	}

	even10 := generate_even(10)
	verify("10 pairs with seed 0", generatePairIndices(0, 10), even10)
	verify("10 pairs with seed 100", generatePairIndices(100, 10), even10)
	verify("20 (even) pairs with seed 200", generatePairIndices(200, 20), generate_even(20))
	verify("21 (odd) pairs with seed 210", generatePairIndices(210, 21), generate_even(21))

	for i := 1; i < 10; i++ {
		pairs := i * 17 // 17 and 10169 are arbitrary prime numbers.
		verify(fmt.Sprintf("%v pairs", pairs), generatePairIndices(int64(pairs*10169), pairs), generate_even(pairs))
	}
}

func TestPairwiseCommitRunner_GivenValidInput_ShouldReturnValues(t *testing.T) {
	// TODO(viditchitkara@): Fill the simple case for the happy path
}
