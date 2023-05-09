package stats

import (
	"strings"
	"testing"
)

func TestPSignRank(t *testing.T) {
	for _, test := range []struct {
		testName     string
		x            float64
		n            int
		lowerTail    bool
		result       float64
		expectError  bool
		expectErrMsg string
	}{
		{
			testName:     "Input n less than 0",
			x:            2.0,
			n:            -1,
			lowerTail:    true,
			result:       0,
			expectError:  true,
			expectErrMsg: "non zero",
		}, {
			testName:    "Input x less than 0 and lowerTail",
			x:           -1.0,
			n:           2,
			lowerTail:   true,
			result:      0,
			expectError: false,
		}, {
			testName:    "Input x less than 0 and not lowerTail",
			x:           -1.0,
			n:           2,
			lowerTail:   false,
			result:      1,
			expectError: false,
		}, {
			testName:    "Input x large number and lowerTail",
			x:           55.0,
			n:           5,
			lowerTail:   false,
			result:      0,
			expectError: false,
		}, {
			testName:    "Input x large number and not lowerTail",
			x:           55.0,
			n:           5,
			lowerTail:   true,
			result:      1,
			expectError: false,
		}, {
			testName:    "Input x between 1 and n * (n + 1) / 2 and lowerTail",
			x:           5.0,
			n:           5,
			lowerTail:   true,
			result:      0.3125,
			expectError: false,
		}, {
			testName:    "Input x between 1 and n * (n + 1) / 2 and not lowerTail",
			x:           8.0,
			n:           4,
			lowerTail:   false,
			result:      0.125,
			expectError: false,
		},
	} {
		t.Run(test.testName, func(t *testing.T) {
			dist, err := newWilcoxonDistribution(test.n)
			if err != nil && !strings.Contains(err.Error(), test.expectErrMsg) {
				t.Errorf("Expected (%s) and got (%v) error message doesn't match", test.expectErrMsg, err)
			}
			if err == nil {
				p := dist.pSignRank(test.x, test.lowerTail)
				if p != test.result {
					t.Errorf("Expected p value (%v) and got (%v) does not match", test.result, p)
				}
			}
		})
	}
}

func TestQSignRank(t *testing.T) {
	for _, test := range []struct {
		testName     string
		x            float64
		n            int
		lowerTail    bool
		result       float64
		expectError  bool
		expectErrMsg string
	}{
		{
			testName:     "Input p less than 0 and lowerTail",
			x:            -1.0,
			n:            5,
			lowerTail:    true,
			result:       0,
			expectError:  true,
			expectErrMsg: "probability",
		}, {
			testName:     "Input p less than 0 and not lowerTail",
			x:            -1.0,
			n:            2,
			lowerTail:    false,
			result:       0,
			expectError:  true,
			expectErrMsg: "probability",
		}, {
			testName:     "Input x greater than 1 and lowerTail",
			x:            2.0,
			n:            2,
			lowerTail:    true,
			result:       0,
			expectError:  true,
			expectErrMsg: "probability",
		}, {
			testName:     "Input x greater than 1 and not lowerTail",
			x:            55.0,
			n:            5,
			lowerTail:    false,
			result:       0,
			expectError:  true,
			expectErrMsg: "probability",
		}, {
			testName:     "Input n less than 0 and lowerTail",
			x:            0.5,
			n:            -2,
			lowerTail:    true,
			result:       0,
			expectError:  true,
			expectErrMsg: "n",
		}, {
			testName:     "Input n less than 0 and not lowerTail",
			x:            0.5,
			n:            -2,
			lowerTail:    false,
			result:       0,
			expectError:  true,
			expectErrMsg: "n",
		}, {
			testName:    "Input x equals to 0 and lowerTail",
			x:           0.0,
			n:           4,
			lowerTail:   true,
			result:      0,
			expectError: false,
		}, {
			testName:    "Input x equals to 0 and not lowerTail",
			x:           0.0,
			n:           4,
			lowerTail:   false,
			result:      10,
			expectError: false,
		}, {
			testName:    "Input x equals to 1 and lowerTail",
			x:           1.0,
			n:           4,
			lowerTail:   true,
			result:      10,
			expectError: false,
		}, {
			testName:    "Input x equals to 0 and not lowerTail with odd n",
			x:           0.0,
			n:           5,
			lowerTail:   false,
			result:      15,
			expectError: false,
		}, {
			testName:    "Input x equals to 1 and not lowerTail",
			x:           1.0,
			n:           4,
			lowerTail:   false,
			result:      0,
			expectError: false,
		}, {
			testName:    "Input x between 0 and 1 and lowerTail",
			x:           0.52158,
			n:           3,
			lowerTail:   true,
			result:      3,
			expectError: false,
		}, {
			testName:    "Input x between 0 and 1 and not lowerTail",
			x:           0.798382,
			n:           5,
			lowerTail:   false,
			result:      4,
			expectError: false,
		},
	} {
		t.Run(test.testName, func(t *testing.T) {
			dist, err := newWilcoxonDistribution(test.n)
			if err != nil && !strings.Contains(err.Error(), test.expectErrMsg) {
				t.Errorf("Expected (%s) and got (%v) error message doesn't match", test.expectErrMsg, err)
			}
			if err == nil {
				q, err := dist.qSignRank(test.x, test.lowerTail)
				if err != nil && !strings.Contains(err.Error(), test.expectErrMsg) {
					t.Errorf("Expected (%s) and got (%v) error message doesn't match", test.expectErrMsg, err)
				}
				if err == nil && test.expectError {
					t.Error("Expected error but not nil")
				}
				if err == nil && q != test.result {
					t.Errorf("Expected q value (%v) and got (%v) does not match", test.result, q)
				}
			}
		})
	}
}
