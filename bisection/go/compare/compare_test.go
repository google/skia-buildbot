package compare

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	bpb "go.skia.org/infra/bisection/go/proto"
)

func TestHighThreshold(t *testing.T) {
	for i, test := range []struct {
		name        string
		performance bool
		expected    float64
	}{
		{
			name:        "performance mode",
			performance: true,
			expected:    0.99,
		},
		{
			name:        "functional mode",
			performance: false,
			expected:    0.66,
		},
	} {
		got := getHighThreshold(test.performance)
		diff := cmp.Diff(got, test.expected)
		if diff != "" {
			t.Errorf("[%d] %s:\nexpected %+v got %+v\ndiff:%v", i, test.name, test.expected, got, diff)
		}
	}
}

func TestCompareSamples(t *testing.T) {
	for i, test := range []struct {
		name         string
		a            []float64
		b            []float64
		mag          float64
		expected     bpb.State
		expectError  bool
		expectErrMsg string
	}{
		{
			name:        "basic performance test",
			a:           []float64{1.0, 4.0},
			b:           []float64{2.0, 3.0},
			expected:    bpb.State_UNKNOWN,
			expectError: false,
		},
		{
			name:         "length of Sample A = 0",
			a:            []float64{},
			b:            []float64{2.0, 3.0},
			expectError:  true,
			expectErrMsg: "Commit(s) has sample size of 0",
		},
	} {
		req := bpb.GetPerformanceDifferenceRequest{
			SamplesA: test.a,
			SamplesB: test.b,
			Difference: &bpb.RequestedDifference{
				ComparisonMagnitude: test.mag,
			},
		}
		got, err := CompareSamples(&req)
		if test.expectError {
			if err == nil {
				t.Error("Expected error but not nil")
			}
			if !strings.Contains(err.Error(), test.expectErrMsg) {
				t.Errorf("[%d] %s:\nexpected error msg (%s) and received error msg (%v) did not match", i, test.name, test.expectErrMsg, err)
			}
		} else {
			diff := cmp.Diff(got.State, test.expected)
			if diff != "" {
				t.Errorf("[%d] %s:\nexpected %+v got %+v\ndiff:%v", i, test.name, test.expected, got, diff)
			}
		}
	}
}
