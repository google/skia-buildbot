package vec32

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	e = MISSING_DATA_SENTINEL
)

func near(a, b float32) bool {
	return math.Abs(float64(a-b)) < 0.001
}

func vecNear(a, b []float32) bool {
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

func TestNew(t *testing.T) {
	unittest.SmallTest(t)
	v := New(0)
	assert.Len(t, v, 0)

	v = New(1)
	assert.Len(t, v, 1)
	assert.Equal(t, MISSING_DATA_SENTINEL, v[0])

	v = New(2)
	assert.Len(t, v, 2)
	assert.Equal(t, MISSING_DATA_SENTINEL, v[0])
	assert.Equal(t, MISSING_DATA_SENTINEL, v[1])
}

func TestNorm(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		In  []float32
		Out []float32
	}{
		{
			In:  []float32{1.0, -1.0, e},
			Out: []float32{1.0, -1.0, e},
		},
		{
			In:  []float32{e, 2.0, -2.0},
			Out: []float32{e, 1.0, -1.0},
		},
		{
			In:  []float32{e},
			Out: []float32{e},
		},
		{
			In:  []float32{},
			Out: []float32{},
		},
		{
			In:  []float32{2.0},
			Out: []float32{0.0},
		},
		{
			In:  []float32{0.0, 0.1},
			Out: []float32{-0.05, 0.05},
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
	unittest.SmallTest(t)
	testCases := []struct {
		In  []float32
		Out []float32
	}{
		{
			In:  []float32{e, e, 2, 3, e, 5},
			Out: []float32{2, 2, 2, 3, 5, 5},
		},
		{
			In:  []float32{e, 3, e},
			Out: []float32{3, 3, 3},
		},
		{
			In:  []float32{e, e},
			Out: []float32{0, 0},
		},
		{
			In:  []float32{e},
			Out: []float32{0},
		},
		{
			In:  []float32{},
			Out: []float32{},
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
	unittest.SmallTest(t)
	testCases := []struct {
		Slice []float32
		Idx   int
	}{
		{
			Slice: []float32{e, e, 2, 3, e, 5},
			Idx:   6,
		},
		{
			Slice: []float32{},
			Idx:   0,
		},
		{
			Slice: []float32{4},
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

func TestDup(t *testing.T) {
	unittest.SmallTest(t)
	a := []float32{1, 2, MISSING_DATA_SENTINEL, 0}
	b := Dup(a)
	assert.Equal(t, a, b)
	b[0] = 2
	assert.NotEqual(t, a, b)

	a = []float32{}
	b = Dup(a)
	assert.Equal(t, a, b)
}

func TestMean(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		Slice []float32
		Mean  float32
	}{
		{
			Slice: []float32{1, 2, e, 0},
			Mean:  1.0,
		},
		{
			Slice: []float32{},
			Mean:  0.0,
		},
		{
			Slice: []float32{e},
			Mean:  0.0,
		},
		{
			Slice: []float32{e, e},
			Mean:  0.0,
		},
		{
			Slice: []float32{1, 5},
			Mean:  3.0,
		},
	}
	for _, tc := range testCases {
		if got, want := Mean(tc.Slice), tc.Mean; !near(got, want) {
			t.Errorf("Mean(%v) Got %v Want %v", tc.Slice, got, want)
		}
	}
}

func TestSSE(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		Slice []float32
		Base  float32
		SSE   float32
	}{
		{
			Slice: []float32{1, 1, e, 0},
			Base:  0.0,
			SSE:   2.0,
		},
		{
			Slice: []float32{1, 1, e, 0},
			Base:  1.0,
			SSE:   1.0,
		},
		{
			Slice: []float32{},
			Base:  1.0,
			SSE:   0.0,
		},
		{
			Slice: []float32{e},
			Base:  3.0,
			SSE:   0.0,
		},
	}
	for _, tc := range testCases {
		if got, want := SSE(tc.Slice, tc.Base), tc.SSE; !near(got, want) {
			t.Errorf("SSE(%v, %f) Got %v Want %v", tc.Slice, tc.Base, got, want)
		}
	}
}

func TestFillMeanMissing(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		Slice []float32
		Mean  []float32
	}{
		{
			Slice: []float32{1, 2, e, 0},
			Mean:  []float32{1.0, 1.0, 1.0, 1.0},
		},
		{
			Slice: []float32{e, e, e, e},
			Mean:  []float32{e, e, e, e},
		},
		{
			Slice: []float32{e},
			Mean:  []float32{e},
		},
		{
			Slice: []float32{},
			Mean:  []float32{},
		},
		{
			Slice: []float32{2.0},
			Mean:  []float32{2.0},
		},
	}
	for _, tc := range testCases {
		v := Dup(tc.Slice)
		FillMeanMissing(v)
		if got, want := v, tc.Mean; !vecNear(got, want) {
			t.Errorf("Mean(%v) Got %v Want %v", tc.Slice, got, want)
		}
	}
}

func TestFillStdDev(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		Slice []float32
		Mean  []float32
	}{
		{
			Slice: []float32{0, 1, 4, 9},
			Mean:  []float32{3.5, 3.5, 3.5, 3.5},
		},
		{
			Slice: []float32{e, e, e, e},
			Mean:  []float32{e, e, e, e},
		},
		{
			Slice: []float32{e},
			Mean:  []float32{e},
		},
		{
			Slice: []float32{},
			Mean:  []float32{},
		},
		{
			Slice: []float32{2.0},
			Mean:  []float32{0.0},
		},
	}
	for _, tc := range testCases {
		v := Dup(tc.Slice)
		FillStdDev(v)
		if got, want := v, tc.Mean; !vecNear(got, want) {
			t.Errorf("Mean(%v) Got %v Want %v", tc.Slice, got, want)
		}
	}
}

func TestFillCov(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		Slice []float32
		Mean  []float32
	}{
		{
			Slice: []float32{0, 1, 4, 9},
			Mean:  []float32{1, 1, 1, 1},
		},
		{
			Slice: []float32{e, e, e, e},
			Mean:  []float32{e, e, e, e},
		},
		{
			Slice: []float32{e},
			Mean:  []float32{e},
		},
		{
			Slice: []float32{},
			Mean:  []float32{},
		},
		{
			Slice: []float32{2.0},
			Mean:  []float32{0.0},
		},
	}
	for _, tc := range testCases {
		v := Dup(tc.Slice)
		FillCov(v)
		if got, want := v, tc.Mean; !vecNear(got, want) {
			t.Errorf("Mean(%v) Got %v Want %v", tc.Slice, got, want)
		}
	}
}

func TestScaleBy(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		Slice    []float32
		Scale    float32
		Expected []float32
	}{
		{
			Slice:    []float32{e, 0, 2, 3},
			Scale:    math.SmallestNonzeroFloat32,
			Expected: []float32{e, 0, e, e},
		},
		{
			Slice:    []float32{e, 0, -1, 2},
			Scale:    0,
			Expected: []float32{e, e, e, e},
		},
		{
			Slice:    []float32{e, 0, -2, 2},
			Scale:    2,
			Expected: []float32{e, 0, -1, 1},
		},
	}
	for _, tc := range testCases {
		v := Dup(tc.Slice)
		ScaleBy(v, tc.Scale)
		if got, want := v, tc.Expected; !vecNear(got, want) {
			t.Errorf("Mean(%v) Got %v Want %v", tc.Slice, got, want)
		}
	}
}

func TestFillStep(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		Slice []float32
		Step  []float32
	}{
		{
			Slice: []float32{1, 1, 2, 2, 2},
			Step:  []float32{0.5, 0.5, 0.5, 0.5, 0.5},
		},
		{
			Slice: []float32{1, 1, 0, 0, 0},
			Step:  []float32{e, e, e, e, e},
		},
		{
			Slice: []float32{3, 5, 2, 2, 2},
			Step:  []float32{2, 2, 2, 2, 2},
		},
		{
			Slice: []float32{3, 5, e, 2, 2},
			Step:  []float32{2, 2, 2, 2, 2},
		},
		{
			Slice: []float32{3, 5, e, e, 2},
			Step:  []float32{2, 2, 2, 2, 2},
		},
		{
			Slice: []float32{4, e, e, e, 2},
			Step:  []float32{2, 2, 2, 2, 2},
		},
		{
			Slice: []float32{3, 5, e, e, e},
			Step:  []float32{e, e, e, e, e},
		},
		{
			Slice: []float32{e, e, e, e},
			Step:  []float32{e, e, e, e},
		},
		{
			Slice: []float32{e},
			Step:  []float32{e},
		},
		{
			Slice: []float32{},
			Step:  []float32{},
		},
		{
			Slice: []float32{1.0},
			Step:  []float32{e},
		},
	}
	for _, tc := range testCases {
		v := Dup(tc.Slice)
		FillStep(v)
		if got, want := v, tc.Step; !vecNear(got, want) {
			t.Errorf("Mean(%v) Got %v Want %v", tc.Slice, got, want)
		}
	}
}
