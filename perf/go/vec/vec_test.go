package vec

import (
	"math"
	"testing"
)

func near(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

func vecNear(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i, x := range a {
		if !near(x, b[i]) {
			return false
		}
	}
	return true
}

func TestNorm(t *testing.T) {
	testCases := []struct {
		In  []float64
		Out []float64
	}{
		{
			In:  []float64{1.0, -1.0, 1e100},
			Out: []float64{1.0, -1.0, 1e100},
		},
		{
			In:  []float64{1e100, 2.0, -2.0},
			Out: []float64{1e100, 1.0, -1.0},
		},
		{
			In:  []float64{1e100},
			Out: []float64{1e100},
		},
		{
			In:  []float64{},
			Out: []float64{},
		},
		{
			In:  []float64{2.0},
			Out: []float64{0.0},
		},
		{
			In:  []float64{0.0, 0.1},
			Out: []float64{-0.05, 0.05},
		},
	}
	for _, tc := range testCases {
		Norm(tc.In, 0.1)
		if got, want := tc.In, tc.Out; !vecNear(tc.Out, tc.In) {
			t.Errorf("Norm: Got %#v Want %#v", got, want)
		}
	}
}

func TestFill(t *testing.T) {
	testCases := []struct {
		In  []float64
		Out []float64
	}{
		{
			In:  []float64{1e100, 1e100, 2, 3, 1e100, 5},
			Out: []float64{2, 2, 2, 3, 5, 5},
		},
		{
			In:  []float64{1e100, 3, 1e100},
			Out: []float64{3, 3, 3},
		},
		{
			In:  []float64{1e100, 1e100},
			Out: []float64{0, 0},
		},
		{
			In:  []float64{1e100},
			Out: []float64{0},
		},
		{
			In:  []float64{},
			Out: []float64{},
		},
	}
	for _, tc := range testCases {
		Fill(tc.In)
		if got, want := tc.In, tc.Out; !vecNear(tc.Out, tc.In) {
			t.Errorf("Fill: Got %#v Want %#v", got, want)
		}
	}

}

func TestFillAtErrors(t *testing.T) {
	testCases := []struct {
		Slice []float64
		Idx   int
	}{
		{
			Slice: []float64{1e100, 1e100, 2, 3, 1e100, 5},
			Idx:   6,
		},
		{
			Slice: []float64{},
			Idx:   0,
		},
		{
			Slice: []float64{4},
			Idx:   -1,
		},
	}
	for _, tc := range testCases {
		_, err := FillAt(tc.Slice, tc.Idx)
		if err == nil {
			t.Fatalf("Expected \"%v\" to fail FillAt.", tc)
		}
	}
}
