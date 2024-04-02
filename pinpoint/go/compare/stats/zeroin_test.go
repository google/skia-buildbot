package stats

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tol = 0.0001

func TestZeroIn_GivenFuncInputsWithoutZero_ReturnsError(t *testing.T) {
	// Cannot find root
	root, err := zeroin(1.96, 0, 0.5, tol, []float64{1, 2}, TwoSided, true, asymptoticW)
	require.Error(t, err)
	assert.True(t, math.IsNaN(root))
}

func TestZeroIn_GivenFunctionNaN_ContinuesCalculation(t *testing.T) {
	// f(a) = 0
	expected := 0.0
	root, err := zeroin(1.96, 0, 0, tol, []float64{0, 0, 0, 0, 0}, TwoSided, true, asymptoticW)
	require.NoError(t, err)
	assert.InDelta(t, expected, root, tol)

	// f(b) = 0
	expected = 0.9999
	root, err = zeroin(1.96, 0.3, 1.0, tol, []float64{1, 1}, TwoSided, true, asymptoticW)
	require.NoError(t, err)
	assert.InDelta(t, expected, root, tol)
}

func TestZeroIn_GivenFaVsFb_ReturnsCorrectResult(t *testing.T) {
	zq := 1.9599
	nums := []float64{141, 15, -411, -753, -169, 696, -522, 98, -24, 696, -452}
	expected := -411.0000

	test := func(name string, a, b float64, correct bool) {
		t.Run(name, func(t *testing.T) {
			result, err := zeroin(zq, a, b, tol, nums, TwoSided, correct, asymptoticW)
			require.NoError(t, err)
			assert.InDelta(t, expected, result, tol)
		})
	}
	test("verify result for math.Abs(fa) < math.Abs(fb)", -753, 696, true)
	test("verify result for math.Abs(fa) >= math.Abs(fb)", 696, -753, true)
}

func TestZeroIn_GivenDifferentHypothesis_ReturnsCorrectResult(t *testing.T) {
	test := func(name string, zq, a, b float64, nums []float64, alt Hypothesis, correct bool, expected float64) {
		t.Run(name, func(t *testing.T) {
			result, err := zeroin(zq, a, b, tol, nums, alt, correct, asymptoticW)
			require.NoError(t, err)
			assert.InDelta(t, expected, result, tol)
		})
	}
	nums := []float64{141, 15, -411, -753, -169, 696, -522, 98, -24, 696, -452}
	test("alternative hypothesis = Less", 1.9599, 696, -753, nums, Less, true, -388.500067)

	nums = []float64{-555, 258, -193, -209, 18, -408, -593, -121, 296, 86, -246, -481, 23}
	test("alternative hypothesis = TwoSided", 1.6448, -593, 296, nums, TwoSided, true, -327.0000)
	test("alternative hypothesis = Greater", 1.6448, -593, 296, nums, Greater, true, -327.0000)
}

func TestZeroIn_GivenFalseCorrection_ReturnsCorrectResult(t *testing.T) {
	nums := []float64{-555, 258, -193, -209, 18, -408, -593, -121, 296, 86, -246, -481, 23}
	result, err := zeroin(1.6448, -593, 296, tol, nums, TwoSided, false, asymptoticW)
	require.NoError(t, err)
	assert.InDelta(t, -326.9999, result, tol)
}
