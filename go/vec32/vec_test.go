package vec32

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	e = MissingDataSentinel
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
	assert.Equal(t, MissingDataSentinel, v[0])

	v = New(2)
	assert.Len(t, v, 2)
	assert.Equal(t, MissingDataSentinel, v[0])
	assert.Equal(t, MissingDataSentinel, v[1])
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
	a := []float32{1, 2, MissingDataSentinel, 0}
	b := Dup(a)
	assert.Equal(t, a, b)
	b[0] = 2
	assert.NotEqual(t, a, b)

	a = []float32{}
	b = Dup(a)
	assert.Equal(t, a, b)
}

func TestMean_ReturnsZeroIfAllMissing(t *testing.T) {
	unittest.SmallTest(t)

	assert.Equal(t, float32(1), Mean([]float32{1, 2, e, 0}))
	assert.Equal(t, float32(0), Mean([]float32{}))
	assert.Equal(t, float32(0), Mean([]float32{e}))
	assert.Equal(t, float32(0), Mean([]float32{e, e}))
	assert.Equal(t, float32(3), Mean([]float32{1, 5}))
}

func TestMeanE_ReturnsMissingIfAllMissing(t *testing.T) {
	unittest.SmallTest(t)

	assert.Equal(t, float32(1), MeanE([]float32{1, 2, e, 0}))
	assert.Equal(t, float32(e), MeanE([]float32{}))
	assert.Equal(t, float32(e), MeanE([]float32{e}))
	assert.Equal(t, float32(e), MeanE([]float32{e, e}))
	assert.Equal(t, float32(3), MeanE([]float32{1, 5}))
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

func TestStdDev(t *testing.T) {
	unittest.SmallTest(t)
	type args struct {
		xs   []float32
		base float32
	}
	tests := []struct {
		name string
		args args
		want float32
	}{
		{
			name: "zero length",
			args: args{
				xs:   []float32{},
				base: 0,
			},
			want: 0,
		},
		{
			name: "length one",
			args: args{
				xs:   []float32{1},
				base: 1,
			},
			want: 0,
		},
		{
			name: "length two",
			args: args{
				xs:   []float32{1, 1},
				base: 1,
			},
			want: 0,
		},
		{
			name: "length two off base",
			args: args{
				xs:   []float32{1, 1},
				base: 0,
			},
			want: math.Sqrt2,
		},
		{
			name: "length two off base",
			args: args{
				xs:   []float32{1, 1},
				base: 0,
			},
			want: math.Sqrt2,
		},
		{
			name: "length two off base after removing sentinels",
			args: args{
				xs:   []float32{1, e, 1},
				base: 0,
			},
			want: math.Sqrt2,
		},
		{
			name: "length two with negatives off base after removing sentinels",
			args: args{
				xs:   []float32{1, e, -1},
				base: 0,
			},
			want: math.Sqrt2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StdDev(tt.args.xs, tt.args.base); got != tt.want {
				t.Errorf("StdDev() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTwoSidedStdDev(t *testing.T) {
	unittest.SmallTest(t)
	tests := []struct {
		name         string
		arr          []float32
		median       float32
		lower, upper float32
		hasError     bool
	}{
		{
			"EmptyArray_Error",
			[]float32{},
			0,
			0, 0,
			true,
		},
		{
			"InsufficientNonMissingData_Error",
			[]float32{e, 1, 1, 2},
			0,
			0, 0,
			true,
		},
		{
			"ConstantArray_ZeroStdDevs",
			[]float32{1, 1, 1, 1},
			1,
			0, 0,
			false,
		},
		{
			"ConfirmSorting_UpperHasNonZeroStdDev",
			[]float32{1, 2, 1, 1},
			1,
			0, 1,
			false,
		},
		{
			"MissingDataValuesAreIgnored",
			[]float32{1, 2, 1, 1, e, e, e},
			1,
			0, 1,
			false,
		},
		{
			"AnOddNumberOfValuesInArray",
			[]float32{2, 2, 1, 1, 1},
			1,
			0, 1,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			median, lower, upper, err := TwoSidedStdDev(tt.arr)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.median, median, "median")
			assert.Equal(t, tt.lower, lower, "lower")
			assert.Equal(t, tt.upper, upper, "upper")
		})
	}
}

func TestRemoveMissingDataSentinel_Success(t *testing.T) {
	unittest.SmallTest(t)

	assert.Equal(t, []float32{1, 2}, RemoveMissingDataSentinel([]float32{e, 1, e, 2, e}))
}

func TestRemoveMissingDataSentinel_Empty_Success(t *testing.T) {
	unittest.SmallTest(t)

	assert.Equal(t, []float32{}, RemoveMissingDataSentinel([]float32{}))
}

func TestStdDevRatio(t *testing.T) {
	unittest.SmallTest(t)

	tests := []struct {
		name          string
		arg           []float32
		stddevRatio   float32
		median        float32
		lower         float32
		upper         float32
		wantError     bool
		errorContains string
	}{
		{
			name:          "Too little data, needs at least 5 data points",
			arg:           []float32{0, 0, 0, 0},
			wantError:     true,
			errorContains: "Insufficient",
		},
		{
			name:          "Too little data after removing MissingDataSentinels",
			arg:           []float32{e, e, e, e, e, e, 1.0},
			wantError:     true,
			errorContains: "Insufficient",
		},
		{
			name:          "Catches NaN results",
			arg:           []float32{0, 0, 0, 0, 0},
			wantError:     true,
			errorContains: "NaN",
		},
		{
			name:          "Catches MissingDataSentinel as last point",
			arg:           []float32{0, 0, 0, 0, e},
			wantError:     true,
			errorContains: "MissingDataSentinel",
		},

		{
			name:        "-Inf result is forced to -maxStdDevRatio",
			arg:         []float32{1, 1, 1, 1, -1.1},
			stddevRatio: -maxStdDevRatio,
			median:      1,
			lower:       0,
			upper:       0,
		},
		{
			name:        "Inf result is forced to -maxStdDevRatio",
			arg:         []float32{1, 1, 1, 1, 1.1},
			stddevRatio: maxStdDevRatio,
			median:      1,
			lower:       0,
			upper:       0,
		},
		{
			name:        "Last point equals the median",
			arg:         []float32{1, 2, 4, 5, 3},
			stddevRatio: 0,
			median:      3,
			lower:       2.236068,
			upper:       2.236068,
		},
		{
			name:        "Last point equals the twice the median",
			arg:         []float32{11, 12, 13, 14, 15, 16, 17, 20},
			stddevRatio: 2.7774603,
			median:      14,
			lower:       2.6457512,
			upper:       2.1602468,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stddevRatio, median, lower, upper, err := StdDevRatio(tt.arg)
			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				return
			}
			assert.Equal(t, tt.stddevRatio, stddevRatio, "stddevRatio")
			assert.Equal(t, tt.median, median, "median")
			assert.Equal(t, tt.lower, lower, "lower")
			assert.Equal(t, tt.upper, upper, "upper")
		})
	}
}

func TestToFloat64(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, []float64{}, ToFloat64(nil))
	assert.Equal(t, []float64{}, ToFloat64([]float32{}))
	assert.Equal(t, []float64{1.0, 2.0}, ToFloat64([]float32{1.0, 2.0}))
}

func TestIQRR(t *testing.T) {
	unittest.SmallTest(t)
	tests := []struct {
		name     string
		args     []float32
		expected []float32
	}{
		{
			name:     "Handles empty arrays.",
			args:     []float32{},
			expected: []float32{},
		},
		{
			name:     "Handles arrays with NaN for quartile values.",
			args:     []float32{1},
			expected: []float32{1},
		},
		{
			name:     "Handles missing data sentinels.",
			args:     []float32{e},
			expected: []float32{e},
		},
		{
			name:     "Handles arrays with Inf values.",
			args:     []float32{float32(math.Inf(0))},
			expected: []float32{float32(math.Inf(0))},
		},
		{
			name:     "Outliers are removed",
			args:     []float32{5, 7, 10, 15, 19, 21, 21, 22, 22, 23, 23, 23, 23, 23, 24, 24, 24, 24, 25},
			expected: []float32{e, e, e, 15, 19, 21, 21, 22, 22, 23, 23, 23, 23, 23, 24, 24, 24, 24, 25},
		},
		{
			name:     "Outliers are removed even in the presence of missing data sentinels",
			args:     []float32{5, 7, 10, 15, 19, 21, 21, 22, 22, e, 23, 23, 23, 23, 23, 24, 24, 24, 24, e, 25},
			expected: []float32{e, e, e, 15, 19, 21, 21, 22, 22, e, 23, 23, 23, 23, 23, 24, 24, 24, 24, e, 25},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			IQRR(tt.args)
			assert.Equal(t, tt.expected, tt.args)
		})
	}
}

func TestSum(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, float32(0), Sum(nil))
	assert.Equal(t, float32(0), Sum([]float32{e}))
	assert.Equal(t, float32(3), Sum([]float32{e, 1, 2}))
}

func TestGeo_ReturnsZeroIfAllMissing(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, float32(4), Geo([]float32{2, 8}), "4 = sqrt(2*8)")
	assert.Equal(t, float32(0), Geo([]float32{-2, -8}), "Return 0 on all ignored.")
	assert.Equal(t, float32(8), Geo([]float32{-2, 8}), "Ignore negative numbers.")
	assert.Equal(t, float32(2), Geo([]float32{2, e}), "Ingore MissingDataSentinels.")
	assert.Equal(t, float32(0), Geo([]float32{}), "Return 0 on empty vector.")
}

func TestGeoE_ReturnsMissingIfAllMissing(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, float32(4), GeoE([]float32{2, 8}), "4 = sqrt(2*8)")
	assert.Equal(t, float32(e), GeoE([]float32{-2, -8}), "Return 0 on all ignored.")
	assert.Equal(t, float32(8), GeoE([]float32{-2, 8}), "Ignore negative numbers.")
	assert.Equal(t, float32(2), GeoE([]float32{2, e}), "Ingore MissingDataSentinels.")
	assert.Equal(t, float32(e), GeoE([]float32{}), "Return 0 on empty vector.")
}

func TestCount(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, float32(0), Count([]float32{}), "Empty returns 0.")
	assert.Equal(t, float32(0), Count([]float32{e}), "MissingDataSentinels are ignored")
	assert.Equal(t, float32(2), Count([]float32{1, 3}), "Counts all non-MissingDataSentinels values")
	assert.Equal(t, float32(2), Count([]float32{1, e, 3}), "Ingores MissingDataSentinels")
}

func TestMin(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, float32(math.MaxFloat32), Min([]float32{}))
	assert.Equal(t, float32(math.MaxFloat32), Min([]float32{e}))
	assert.Equal(t, float32(2), Min([]float32{2}))
	assert.Equal(t, float32(3), Min([]float32{5, e, 3}))
}

func TestMax(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, float32(-math.MaxFloat32), Max([]float32{}))
	assert.Equal(t, float32(-math.MaxFloat32), Max([]float32{e}))
	assert.Equal(t, float32(2), Max([]float32{2}))
	assert.Equal(t, float32(5), Max([]float32{5, e, 3}))
}
