package stats

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const almostEqualDelta = 1e-6

func TestNewWilcoxonDistribution_GivenNLessThan0_ReturnsError(t *testing.T) {
	dist, err := newWilcoxonDistribution(-1)
	assert.Error(t, err)
	assert.Nil(t, dist)
}

func TestPSignRank_GivenValidInputs_ReturnsCorrectResult(t *testing.T) {
	test := func(name string, x float64, n int, lowerTail bool, expected float64) {
		t.Run(name, func(t *testing.T) {
			dist, err := newWilcoxonDistribution(n)
			require.NoError(t, err)

			p := dist.pSignRank(x, lowerTail)
			assert.InDelta(t, expected, p, almostEqualDelta)
		})
	}
	test("x < 0 and lowerTail = true, returns 0", -1.0, 2, true, 0)
	test("x < 0 and lowerTail = false, returns 1", -1.0, 2, false, 1)
	test("x is large and lowerTail = true, returns 1", 55.0, 5, true, 1)
	test("x is large and lowerTail = false, returns 0", 55.0, 5, false, 0)
	test("1 <= x <= n*(n+1)/2 and lowerTail = true, returns 1", 5.0, 5, true, 0.3125)
	test("1 <= x <= n*(n+1)/2 and lowerTail = false, returns 1", 8.0, 4, false, 0.125)
}

func TestQSignRank_GivenNegativeX_ReturnsError(t *testing.T) {
	const x = -0.0001
	test := func(name string, n int) {
		t.Run(name, func(t *testing.T) {
			dist, err := newWilcoxonDistribution(n)
			require.NoError(t, err)

			q, err := dist.qSignRank(x, false)
			require.Error(t, err)
			assert.Zero(t, q)

			q, err = dist.qSignRank(x, true)
			require.Error(t, err)
			assert.Zero(t, q)
		})
	}
	test("n = 5, returns error", 5)
	test("n = 10, returns error", 10)
	test("n = 30, returns error", 30)
}

func TestQSignRank_GivenXGreaterThan1_ReturnsError(t *testing.T) {
	const x = 1.00001
	test := func(name string, n int) {
		t.Run(name, func(t *testing.T) {
			dist, err := newWilcoxonDistribution(n)
			require.NoError(t, err)

			q, err := dist.qSignRank(x, false)
			require.Error(t, err)
			assert.Zero(t, q)

			q, err = dist.qSignRank(x, true)
			require.Error(t, err)
			assert.Zero(t, q)
		})
	}
	test("n = 5, returns error", 5)
	test("n = 10, returns error", 10)
	test("n = 30, returns error", 30)
}

func TestQSignRank_GivenValidInputs_ReturnsCorrectResult(t *testing.T) {
	test := func(name string, x float64, n int, lowerTail bool, expected float64) {
		t.Run(name, func(t *testing.T) {
			dist, err := newWilcoxonDistribution(n)
			require.NoError(t, err)

			q, err := dist.qSignRank(x, lowerTail)
			require.NoError(t, err)
			assert.InDelta(t, expected, q, almostEqualDelta)
		})
	}
	test("x = 0, n is even, and lowerTail = true, returns 0", 0, 4, true, 0)
	test("x = 0, n is even, and lowerTail = false, returns 10", 0, 4, false, 10)
	test("x = 1, n is even, and lowerTail = true, returns 10", 1, 4, true, 10)
	test("x = 1, n is even, and lowerTail = false, returns 0", 1, 4, false, 0)
	test("x = 1, n is odd, and lowerTail = false, returns 15", 0, 5, false, 15)
	test("0 < x < 1, n is odd, lowerTail = true, returns 3", 0.52158, 3, true, 3)
	test("0 < x < 1, n is odd, lowerTail = false, returns 4", 0.798382, 5, false, 4)
}
