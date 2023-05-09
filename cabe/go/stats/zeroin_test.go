package stats

import (
	"strings"
	"testing"
)

func TestZeroin(t *testing.T) {
	for _, test := range []struct {
		testName     string
		zq           float64
		a            float64
		b            float64
		tol          float64
		nums         []float64
		alt          Hypothesis
		correct      bool
		f            func([]float64, float64, Hypothesis, bool) float64
		result       float64
		expectError  bool
		expectErrMsg string
	}{
		{
			testName:    "fa returns math.Nan()",
			zq:          1.96,
			a:           0,
			b:           0,
			tol:         0.0001,
			nums:        []float64{0, 0, 0, 0, 0},
			alt:         TwoSided,
			correct:     true,
			f:           asymptoticW,
			result:      0,
			expectError: false,
		}, {
			testName:    "fb returns math.Nan()",
			zq:          1.96,
			a:           0.3,
			b:           1,
			tol:         0.0001,
			nums:        []float64{1, 1},
			alt:         TwoSided,
			correct:     true,
			f:           asymptoticW,
			result:      0.99991455078125,
			expectError: false,
		}, {
			testName:     "zeroin returns an error because of the same signs",
			zq:           1.96,
			a:            0,
			b:            0.5,
			tol:          0.0001,
			nums:         []float64{1, 1},
			alt:          TwoSided,
			correct:      true,
			f:            asymptoticW,
			expectError:  true,
			expectErrMsg: "opposite sign",
		}, {
			testName:    "math.Abs(fa) < math.Abs(fb)",
			zq:          1.9599639845400538,
			a:           -753,
			b:           696,
			tol:         0.0001,
			nums:        []float64{141, 15, -411, -753, -169, 696, -522, 98, -24, 696, -452},
			alt:         TwoSided,
			correct:     true,
			f:           asymptoticW,
			result:      -411.0000313465373,
			expectError: false,
		}, {
			testName:    "math.Abs(fa) >= math.Abs(fb)",
			zq:          1.9599639845400538,
			a:           696,
			b:           -753,
			tol:         0.0001,
			nums:        []float64{141, 15, -411, -753, -169, 696, -522, 98, -24, 696, -452},
			alt:         TwoSided,
			correct:     true,
			f:           asymptoticW,
			result:      -411.0000313465373,
			expectError: false,
		}, {
			testName:    "brent with alternative set to Less",
			zq:          1.9599639845400538,
			a:           696,
			b:           -753,
			tol:         0.0001,
			nums:        []float64{141, 15, -411, -753, -169, 696, -522, 98, -24, 696, -452},
			alt:         Less,
			correct:     true,
			f:           asymptoticW,
			result:      -388.50004776516573,
			expectError: false,
		}, {
			testName:    "brent with alternative set to Greater",
			zq:          1.6448536269514726,
			a:           -593,
			b:           296,
			tol:         0.0001,
			nums:        []float64{-555, 258, -193, -209, 18, -408, -593, -121, 296, 86, -246, -481, 23},
			alt:         Greater,
			correct:     true,
			f:           asymptoticW,
			result:      -327.0000065373828,
			expectError: false,
		}, {
			testName:    "brent with correct set to false",
			zq:          1.6448536269514726,
			a:           -593,
			b:           296,
			tol:         0.0001,
			nums:        []float64{-555, 258, -193, -209, 18, -408, -593, -121, 296, 86, -246, -481, 23},
			alt:         TwoSided,
			correct:     false,
			f:           asymptoticW,
			result:      -326.9999549455591,
			expectError: false,
		},
	} {
		t.Run(test.testName, func(t *testing.T) {
			res, err := zeroin(test.zq, test.a, test.b, test.tol, test.nums, test.alt, test.correct, test.f)
			if err != nil && !strings.Contains(err.Error(), test.expectErrMsg) {
				t.Errorf("Expected (%s) and got (%v) error message doesn't match", test.expectErrMsg, err)
			}
			if err == nil && test.expectError {
				t.Error("Expected error but not nil")
			}
			if err == nil && (res != test.result) {
				t.Errorf("Expected res (%v) and got (%v) does not match", test.result, res)
			}
		})
	}
}
