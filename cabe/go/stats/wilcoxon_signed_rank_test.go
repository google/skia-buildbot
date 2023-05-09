package stats

import (
	"math"
	"strings"
	"testing"
)

func TestRank(t *testing.T) {
	for _, test := range []struct {
		testName string
		nums     []float64
		result   []float64
		hasTie   bool
	}{
		{
			testName: "Input nums has ties",
			nums:     []float64{10, 10, 30, 40, 50},
			result:   []float64{1.5, 1.5, 3, 4, 5},
			hasTie:   true,
		}, {
			testName: "Input nums does not have ties",
			nums:     []float64{10, 20, 30, 40, 50},
			result:   []float64{1, 2, 3, 4, 5},
			hasTie:   false,
		},
	} {
		t.Run(test.testName, func(t *testing.T) {
			gotRes, gotTie := rank(test.nums)
			if gotTie != test.hasTie {
				t.Errorf("Expected (%v) and got (%v) tie doesn't match", test.hasTie, gotTie)
			}
			for i, ele := range test.result {
				if gotRes[i] != ele {
					t.Errorf("Expected (%v) and got (%v) res doesn't match", ele, gotRes[i])
				}
			}
		})
	}
}

func TestWilcoxonSignedRankedTest(t *testing.T) {
	for _, test := range []struct {
		testName     string
		x            []float64
		y            []float64
		alt          Hypothesis
		result       *WilcoxonSignedRankedTestResult
		expectError  bool
		expectErrMsg string
	}{
		{
			testName:     "Input x is missing",
			x:            nil,
			y:            nil,
			alt:          TwoSided,
			result:       nil,
			expectError:  true,
			expectErrMsg: "x is missing",
		}, {
			testName:     "Length of input x and y do not match",
			x:            []float64{1.0},
			y:            []float64{1.0, 2.0},
			alt:          TwoSided,
			result:       nil,
			expectError:  true,
			expectErrMsg: "same length",
		}, {
			testName:    "Paired sample (TwoSided) test with exact estimation",
			x:           []float64{447, 832, 640, 286, 501, 123},
			y:           []float64{241, 608, 130, 951, 604, 690},
			alt:         TwoSided,
			result:      &WilcoxonSignedRankedTestResult{Estimate: -77.5, LowerCi: -665, UpperCi: 510, PValue: 0.84375},
			expectError: false,
		}, {
			testName:    "Paired sample (Less) test with exact estimation",
			x:           []float64{447, 832, 640, 286, 501, 123},
			y:           []float64{241, 608, 130, 951, 604, 690},
			alt:         Less,
			result:      &WilcoxonSignedRankedTestResult{Estimate: -77.5, LowerCi: math.Inf(-1), UpperCi: 358, PValue: 0.421875},
			expectError: false,
		}, {
			testName:    "Paired sample (Greater) test with exact estimation",
			x:           []float64{447, 832, 640, 286, 501, 123},
			y:           []float64{241, 608, 130, 951, 604, 690},
			alt:         Greater,
			result:      &WilcoxonSignedRankedTestResult{Estimate: -77.5, LowerCi: -567, UpperCi: math.Inf(1), PValue: 0.65625},
			expectError: false,
		}, {
			testName:    "One sample (TwoSided) test with exact estimation",
			x:           []float64{206, 224, 510, -665, -103, -567},
			y:           []float64{},
			alt:         TwoSided,
			result:      &WilcoxonSignedRankedTestResult{Estimate: -77.5, LowerCi: -665, UpperCi: 510, PValue: 0.84375},
			expectError: false,
		}, {
			testName:    "One sample (Less) test with exact estimation",
			x:           []float64{206, 224, 510, -665, -103, -567},
			y:           []float64{},
			alt:         Less,
			result:      &WilcoxonSignedRankedTestResult{Estimate: -77.5, LowerCi: math.Inf(-1), UpperCi: 358, PValue: 0.421875},
			expectError: false,
		}, {
			testName:    "One sample (Greater) test with exact estimation",
			x:           []float64{206, 224, 510, -665, -103, -567},
			y:           []float64{},
			alt:         Greater,
			result:      &WilcoxonSignedRankedTestResult{Estimate: -77.5, LowerCi: -567, UpperCi: math.Inf(1), PValue: 0.65625},
			expectError: false,
		}, {
			testName:    "One sample (TwoSided) test with exact estimation with odd number of input length",
			x:           []float64{206, 224, 510, -665, -103},
			y:           []float64{},
			alt:         TwoSided,
			result:      &WilcoxonSignedRankedTestResult{Estimate: 60.5, LowerCi: -665, UpperCi: 510, PValue: 0.8125},
			expectError: false,
		}, {
			testName:    "One sample (TwoSided) test with exact estimation with ties in the input",
			x:           []float64{206, 206, 510, -665, -103},
			y:           []float64{},
			alt:         TwoSided,
			result:      &WilcoxonSignedRankedTestResult{Estimate: 51.50002392602765, LowerCi: -665, UpperCi: 510, PValue: 0.7864570351373765},
			expectError: false,
		}, {
			testName:    "One sample (TwoSided) test with exact estimation with zeros in the input",
			x:           []float64{206, 0, 510, -665, -103},
			y:           []float64{},
			alt:         TwoSided,
			result:      &WilcoxonSignedRankedTestResult{Estimate: -16.99287685975377, LowerCi: -664.9999137353511, UpperCi: 509.9999137353511, PValue: 1},
			expectError: false,
		}, {
			testName:    "Paired sample (TwoSided) test with normal approximation",
			x:           []float64{4737, 1582, 5352, 4606, 7701, 2267, 2247, 6200, 9248, 2297, 4152, 199, 1743, 8457, 2462, 7268, 7014, 4716, 4992, 3264, 3885, 160, 4495, 6600, 3249, 4187, 1167, 8918, 6826, 9391, 3164, 3459, 9559, 836, 6252, 9997, 7246, 8492, 9713, 7141, 2880, 1499, 5605, 5838, 1469, 6679, 9534, 125, 5544, 3365},
			y:           []float64{8362, 3569, 5106, 98, 4711, 3640, 8634, 1815, 7558, 2354, 4629, 6486, 630, 3679, 7190, 163, 2545, 6947, 802, 6571, 4834, 688, 7618, 8954, 3200, 1801, 9162, 6049, 6298, 5785, 9655, 9630, 9504, 1850, 4927, 1653, 7669, 4331, 4616, 9526, 8724, 4015, 8545, 246, 9912, 5474, 5455, 4335, 7096, 4175},
			alt:         TwoSided,
			result:      &WilcoxonSignedRankedTestResult{Estimate: -292.4999968196823, LowerCi: -1456.499966104948, UpperCi: 983.4999551510192, PValue: 0.6604972529810283},
			expectError: false,
		}, {
			testName:    "Paired sample (Less) test with normal approximation",
			x:           []float64{4737, 1582, 5352, 4606, 7701, 2267, 2247, 6200, 9248, 2297, 4152, 199, 1743, 8457, 2462, 7268, 7014, 4716, 4992, 3264, 3885, 160, 4495, 6600, 3249, 4187, 1167, 8918, 6826, 9391, 3164, 3459, 9559, 836, 6252, 9997, 7246, 8492, 9713, 7141, 2880, 1499, 5605, 5838, 1469, 6679, 9534, 125, 5544, 3365},
			y:           []float64{8362, 3569, 5106, 98, 4711, 3640, 8634, 1815, 7558, 2354, 4629, 6486, 630, 3679, 7190, 163, 2545, 6947, 802, 6571, 4834, 688, 7618, 8954, 3200, 1801, 9162, 6049, 6298, 5785, 9655, 9630, 9504, 1850, 4927, 1653, 7669, 4331, 4616, 9526, 8724, 4015, 8545, 246, 9912, 5474, 5455, 4335, 7096, 4175},
			alt:         Less,
			result:      &WilcoxonSignedRankedTestResult{Estimate: -292.4999968196823, LowerCi: math.Inf(-1), UpperCi: 787.999996305465, PValue: 0.3302486264905142},
			expectError: false,
		}, {
			testName:    "Paired sample (Greater) test with normal approximation",
			x:           []float64{4737, 1582, 5352, 4606, 7701, 2267, 2247, 6200, 9248, 2297, 4152, 199, 1743, 8457, 2462, 7268, 7014, 4716, 4992, 3264, 3885, 160, 4495, 6600, 3249, 4187, 1167, 8918, 6826, 9391, 3164, 3459, 9559, 836, 6252, 9997, 7246, 8492, 9713, 7141, 2880, 1499, 5605, 5838, 1469, 6679, 9534, 125, 5544, 3365},
			y:           []float64{8362, 3569, 5106, 98, 4711, 3640, 8634, 1815, 7558, 2354, 4629, 6486, 630, 3679, 7190, 163, 2545, 6947, 802, 6571, 4834, 688, 7618, 8954, 3200, 1801, 9162, 6049, 6298, 5785, 9655, 9630, 9504, 1850, 4927, 1653, 7669, 4331, 4616, 9526, 8724, 4015, 8545, 246, 9912, 5474, 5455, 4335, 7096, 4175},
			alt:         Greater,
			result:      &WilcoxonSignedRankedTestResult{Estimate: -292.4999968196823, LowerCi: -1230.4999712058636, UpperCi: math.Inf(1), PValue: 0.6732409140493585},
			expectError: false,
		}, {
			testName:    "Paired sample (TwoSided) test with all ties",
			x:           []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			y:           []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			alt:         TwoSided,
			result:      &WilcoxonSignedRankedTestResult{Estimate: math.NaN(), LowerCi: math.NaN(), UpperCi: math.NaN(), PValue: math.NaN()},
			expectError: false,
		}, {
			testName:    "Paired sample (Less) test with all ties",
			x:           []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			y:           []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			alt:         Less,
			result:      &WilcoxonSignedRankedTestResult{Estimate: math.NaN(), LowerCi: math.Inf(-1), UpperCi: math.NaN(), PValue: 1},
			expectError: false,
		}, {
			testName:    "Paired sample (Greater) test with all ties",
			x:           []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			y:           []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			alt:         Greater,
			result:      &WilcoxonSignedRankedTestResult{Estimate: math.NaN(), LowerCi: math.NaN(), UpperCi: math.Inf(1), PValue: 1},
			expectError: false,
		},
	} {
		t.Run(test.testName, func(t *testing.T) {
			res, err := WilcoxonSignedRankedTest(test.x, test.y, test.alt)
			if err != nil && !strings.Contains(err.Error(), test.expectErrMsg) {
				t.Errorf("Expected (%s) and got (%v) error message doesn't match", test.expectErrMsg, err)
			}
			if err == nil && test.expectError {
				t.Error("Expected error but not nil")
			}
			if err == nil && !math.IsNaN(res.Estimate) && (res.PValue != test.result.PValue || res.Estimate != test.result.Estimate ||
				res.UpperCi != test.result.UpperCi || res.LowerCi != test.result.LowerCi) {
				t.Errorf("Expected res (%v) and got (%v) does not match", test.result, res)
			}

			if err == nil && math.IsNaN(res.Estimate) {
				if !math.IsNaN(test.result.Estimate) {
					t.Errorf("Expected res (%v) and got (%v) does not match", test.result, res)
				}

				if math.IsNaN(res.LowerCi) {
					if !math.IsNaN(test.result.LowerCi) {
						t.Errorf("Expected res (%v) and got (%v) does not match", test.result, res)
					}
				}

				if math.IsNaN(res.UpperCi) {
					if !math.IsNaN(test.result.UpperCi) {
						t.Errorf("Expected res (%v) and got (%v) does not match", test.result, res)
					}
				}

				if math.IsInf(res.UpperCi, 1) {
					if !math.IsInf(test.result.UpperCi, 1) {
						t.Errorf("Expected res (%v) and got (%v) does not match", test.result, res)
					}
				}

				if math.IsInf(res.LowerCi, -1) {
					if !math.IsInf(test.result.LowerCi, -1) {
						t.Errorf("Expected res (%v) and got (%v) does not match", test.result, res)
					}
				}
			}
		})
	}
}

func TestBerfWilcoxonSignedRankedTest(t *testing.T) {
	for _, test := range []struct {
		testName     string
		x            []float64
		y            []float64
		alt          Hypothesis
		transform    DataTransform
		result       *BerfWilcoxonSignedRankedTestResult
		expectError  bool
		expectErrMsg string
	}{
		{
			testName:     "Length of input x and y does not match",
			x:            []float64{1.0},
			y:            []float64{1.0, 2.0},
			alt:          TwoSided,
			transform:    LogTransform,
			result:       nil,
			expectError:  true,
			expectErrMsg: "must have the same length",
		}, {
			testName:    "Paired sample (TwoSided) test using LogTransform",
			x:           []float64{0.1287916, 0.1551960, 0.3128008, 0.4482681, 0.2929843, 0.4259846},
			y:           []float64{0.20628058, 0.33817586, 0.22281578, 0.28119775, 0.07023901, 0.23528268},
			alt:         TwoSided,
			transform:   LogTransform,
			result:      &BerfWilcoxonSignedRankedTestResult{Estimate: 40.385389221535384, LowerCi: -54.10790113759154, UpperCi: 317.1247573107878, PValue: 0.6875, XMedian: 0.30289255, YMedian: 0.22904923},
			expectError: false,
		}, {
			testName:    "Paired sample (TwoSided) test using NormalizeResult",
			x:           []float64{0.4476217, 0.6152619},
			y:           []float64{0.2416391, 0.3708087},
			alt:         TwoSided,
			transform:   NormalizeResult,
			result:      &BerfWilcoxonSignedRankedTestResult{Estimate: 73.54680676459284, LowerCi: 67.2653571455396, UpperCi: 79.8282563836461, PValue: 0.5, XMedian: 0.5314418000000001, YMedian: 0.3062239},
			expectError: false,
		}, {
			testName:    "Paired sample (TwoSided) test using OriginalResult",
			x:           []float64{0.3583569, 0.3603624, 0.5738001, 0.3583569},
			y:           []float64{0.4493686, 0.8384975, 0.7492481, 0.4493686},
			alt:         TwoSided,
			transform:   OriginalResult,
			result:      &BerfWilcoxonSignedRankedTestResult{Estimate: -0.15951242203815116, LowerCi: -0.2845975069273507, UpperCi: -0.09101170000000003, PValue: 0.09751253817810965, XMedian: 0.35935965000000003, YMedian: 0.59930835},
			expectError: false,
		},
	} {
		t.Run(test.testName, func(t *testing.T) {
			res, err := BerfWilcoxonSignedRankedTest(test.x, test.y, test.alt, test.transform)
			if err != nil && !strings.Contains(err.Error(), test.expectErrMsg) {
				t.Errorf("Expected (%s) and got (%v) error message doesn't match", test.expectErrMsg, err)
			}
			if err == nil && test.expectError {
				t.Error("Expected error but not nil")
			}
			if err == nil && !math.IsNaN(res.Estimate) && (res.PValue != test.result.PValue || res.Estimate != test.result.Estimate ||
				res.UpperCi != test.result.UpperCi || res.LowerCi != test.result.LowerCi || res.XMedian != test.result.XMedian || res.YMedian != test.result.YMedian) {
				t.Errorf("Expected res (%v) and got (%v) does not match", test.result, res)
			}
		})
	}
}
