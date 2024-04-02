package stats

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRank_WithTieInInput_ReturnsTie(t *testing.T) {
	nums := []float64{10, 10, 30, 40, 50}
	expected := []float64{1.5, 1.5, 3, 4, 5}
	result, tie := rank(nums)
	assert.True(t, tie)
	assert.Equal(t, expected, result)
}

func TestRank_WithNoTieInInput_ReturnsNoTie(t *testing.T) {
	nums := []float64{10, 20, 30, 40, 50}
	expected := []float64{1, 2, 3, 4, 5}
	result, tie := rank(nums)
	assert.False(t, tie)
	assert.Equal(t, expected, result)
}

func TestWilcoxonSignedRankTest_GivenInvalidInputs_ReturnsError(t *testing.T) {
	test := func(name string, x, y []float64) {
		t.Run(name, func(t *testing.T) {
			result, err := WilcoxonSignedRankedTest(x, y, TwoSided)
			require.Error(t, err)
			assert.Empty(t, result)
		})
	}
	x := []float64{1.0}
	y := []float64{1.0, 2.0}
	test("x is missing", nil, y)
	test("x and y have unequal lengths", x, y)
}

func TestWilcoxonSignedRankTest_GivenValidInputs_ReturnsCorrectResult(t *testing.T) {
	test := func(name string, x, y []float64, alt Hypothesis, expected WilcoxonSignedRankedTestResult) {
		t.Run(name, func(t *testing.T) {
			result, err := WilcoxonSignedRankedTest(x, y, alt)
			require.NoError(t, err)
			assert.Equal(t, expected, result)
		})
	}
	x := []float64{447, 832, 640, 286, 501, 123}
	y := []float64{241, 608, 130, 951, 604, 690}
	expected := WilcoxonSignedRankedTestResult{Estimate: -77.5, LowerCi: -665, UpperCi: 510, PValue: 0.84375}
	test("Paired sample test with exact estimation and TwoSided hypothesis", x, y, TwoSided, expected)

	expected = WilcoxonSignedRankedTestResult{Estimate: -77.5, LowerCi: math.Inf(-1), UpperCi: 358, PValue: 0.421875}
	test("Paired sample test with exact estimation and Less hypothesis", x, y, Less, expected)

	expected = WilcoxonSignedRankedTestResult{Estimate: -77.5, LowerCi: -567, UpperCi: math.Inf(1), PValue: 0.65625}
	test("Paired sample test with exact estimation and Greater hypothesis", x, y, Greater, expected)

	x = []float64{206, 224, 510, -665, -103, -567}
	y = []float64{}
	expected = WilcoxonSignedRankedTestResult{Estimate: -77.5, LowerCi: -665, UpperCi: 510, PValue: 0.84375}
	test("One sample test with exact estimation and TwoSided hypothesis", x, y, TwoSided, expected)

	expected = WilcoxonSignedRankedTestResult{Estimate: -77.5, LowerCi: math.Inf(-1), UpperCi: 358, PValue: 0.421875}
	test("One sample test with exact estimation and Less hypothesis", x, y, Less, expected)

	expected = WilcoxonSignedRankedTestResult{Estimate: -77.5, LowerCi: -567, UpperCi: math.Inf(1), PValue: 0.65625}
	test("One sample test with exact estimation and Greater hypothesis", x, y, Greater, expected)

	x = []float64{206, 224, 510, -665, -103}
	y = []float64{}
	expected = WilcoxonSignedRankedTestResult{Estimate: 60.5, LowerCi: -665, UpperCi: 510, PValue: 0.8125}
	test("One sample (TwoSided) test with exact estimation with odd number of input length", x, y, TwoSided, expected)
	x = []float64{206, 206, 510, -665, -103}
	expected = WilcoxonSignedRankedTestResult{Estimate: 51.50002392602765, LowerCi: -665, UpperCi: 510, PValue: 0.7864570351373765}
	test("One sample (TwoSided) test with exact estimation with ties in the input", x, y, TwoSided, expected)

	x = []float64{206, 0, 510, -665, -103}
	expected = WilcoxonSignedRankedTestResult{Estimate: -16.99287685975377, LowerCi: -664.9999137353511, UpperCi: 509.9999137353511, PValue: 1}
	test("One sample (TwoSided) test with exact estimation with zeros in the input", x, y, TwoSided, expected)

	x = []float64{4737, 1582, 5352, 4606, 7701, 2267, 2247, 6200, 9248, 2297, 4152, 199, 1743, 8457, 2462, 7268, 7014, 4716, 4992, 3264, 3885, 160, 4495, 6600, 3249, 4187, 1167, 8918, 6826, 9391, 3164, 3459, 9559, 836, 6252, 9997, 7246, 8492, 9713, 7141, 2880, 1499, 5605, 5838, 1469, 6679, 9534, 125, 5544, 3365}
	y = []float64{8362, 3569, 5106, 98, 4711, 3640, 8634, 1815, 7558, 2354, 4629, 6486, 630, 3679, 7190, 163, 2545, 6947, 802, 6571, 4834, 688, 7618, 8954, 3200, 1801, 9162, 6049, 6298, 5785, 9655, 9630, 9504, 1850, 4927, 1653, 7669, 4331, 4616, 9526, 8724, 4015, 8545, 246, 9912, 5474, 5455, 4335, 7096, 4175}
	expected = WilcoxonSignedRankedTestResult{Estimate: -292.4999968196823, LowerCi: -1456.499966104948, UpperCi: 983.4999551510192, PValue: 0.6604972529810283}
	test("Paired sample test with normal approximation and TwoSided Hypothesis", x, y, TwoSided, expected)
	expected = WilcoxonSignedRankedTestResult{Estimate: -292.4999968196823, LowerCi: math.Inf(-1), UpperCi: 787.999996305465, PValue: 0.3302486264905142}
	test("Paired sample test with normal approximation and Less Hypothesis", x, y, Less, expected)
	expected = WilcoxonSignedRankedTestResult{Estimate: -292.4999968196823, LowerCi: -1230.4999712058636, UpperCi: math.Inf(1), PValue: 0.6732409140493585}
	test("Paired sample test with normal approximation and Greater Hypothesis", x, y, Greater, expected)
}

func TestWilcoxonSignedRankTest_GivenTies_ReturnsMathNan(t *testing.T) {
	x := []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	y := []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	test := func(name string, alt Hypothesis, expected WilcoxonSignedRankedTestResult) {
		t.Run(name, func(t *testing.T) {
			result, err := WilcoxonSignedRankedTest(x, y, alt)
			require.NoError(t, err)
			assert.True(t, math.IsNaN(result.Estimate))
			if math.IsNaN(expected.LowerCi) {
				assert.True(t, math.IsNaN(result.LowerCi))
			} else {
				assert.True(t, math.IsInf(result.LowerCi, -1))
			}
			if math.IsNaN(expected.UpperCi) {
				assert.True(t, math.IsNaN(result.UpperCi))
			} else {
				assert.True(t, math.IsInf(result.UpperCi, 1))
			}
			if math.IsNaN(expected.PValue) {
				assert.True(t, math.IsNaN(result.UpperCi))
			} else {
				assert.Equal(t, expected.PValue, result.PValue)
			}
		})
	}
	expected := WilcoxonSignedRankedTestResult{Estimate: math.NaN(), LowerCi: math.NaN(), UpperCi: math.NaN(), PValue: math.NaN()}
	test("TwoSided Hypothesis", TwoSided, expected)
	expected = WilcoxonSignedRankedTestResult{Estimate: math.NaN(), LowerCi: math.Inf(-1), UpperCi: math.NaN(), PValue: 1}
	test("Less Hypothesis", Less, expected)
	expected = WilcoxonSignedRankedTestResult{Estimate: math.NaN(), LowerCi: math.NaN(), UpperCi: math.Inf(1), PValue: 1}
	test("Greater Hypothesis", Greater, expected)
}

func TestPairwiseWilcoxonSignedRankedTest_GivenMismatchLength_ReturnsError(t *testing.T) {
	result, err := PairwiseWilcoxonSignedRankedTest(
		[]float64{1.0},
		[]float64{1.0, 2.0},
		TwoSided,
		LogTransform)
	require.Error(t, err)
	assert.Empty(t, result)
}

func TestPairwiseWilcoxonSignedRankedTest_GivenDataTransform_ReturnsCorrectResult(t *testing.T) {
	test := func(name string, x, y []float64, transform DataTransform, expected PairwiseWilcoxonSignedRankedTestResult) {
		t.Run(name, func(t *testing.T) {
			result, err := PairwiseWilcoxonSignedRankedTest(x, y, TwoSided, transform)
			require.NoError(t, err)
			assert.Equal(t, expected, result)
		})
	}
	x := []float64{0.1287916, 0.1551960, 0.3128008, 0.4482681, 0.2929843, 0.4259846}
	y := []float64{0.20628058, 0.33817586, 0.22281578, 0.28119775, 0.07023901, 0.23528268}
	expected := PairwiseWilcoxonSignedRankedTestResult{Estimate: 40.385389221535384, LowerCi: -54.10790113759154, UpperCi: 317.1247573107878, PValue: 0.6875, XMedian: 0.30289255, YMedian: 0.22904923}
	test("Paired sample test with LogTransform", x, y, LogTransform, expected)

	x = []float64{0.4476217, 0.6152619}
	y = []float64{0.2416391, 0.3708087}
	expected = PairwiseWilcoxonSignedRankedTestResult{Estimate: 73.54680676459284, LowerCi: 67.2653571455396, UpperCi: 79.8282563836461, PValue: 0.5, XMedian: 0.5314418000000001, YMedian: 0.3062239}
	test("Paired sample test with Normalized transform", x, y, NormalizeResult, expected)

	x = []float64{0.3583569, 0.3603624, 0.5738001, 0.3583569}
	y = []float64{0.4493686, 0.8384975, 0.7492481, 0.4493686}
	expected = PairwiseWilcoxonSignedRankedTestResult{Estimate: -0.15951242203815116, LowerCi: -0.2845975069273507, UpperCi: -0.09101170000000003, PValue: 0.09751253817810965, XMedian: 0.35935965000000003, YMedian: 0.59930835}
	test("Paired sample test with no transform", x, y, OriginalResult, expected)
}
