// Package vec32 has some basic functions on slices of float32.
package vec32

import (
	"fmt"
	"math"
	"sort"
)

const (
	// MissingDataSentinel signifies a missing sample value.
	//
	// JSON doesn't support NaN or +/- Inf, so we need a valid float32 to signal
	// missing data that also has a compact JSON representation.
	MissingDataSentinel float32 = 1e32
)

// New creates a new []float32 of the given size pre-populated
// with MISSING_DATA_SENTINEL.
func New(size int) []float32 {
	ret := make([]float32, size)
	for i := range ret {
		ret[i] = MissingDataSentinel
	}
	return ret
}

// MeanAndStdDev returns the mean, stddev, and if an error occurred while doing
// the calculation. MISSING_DATA_SENTINELs are ignored.
func MeanAndStdDev(a []float32) (float32, float32, error) {
	count := 0
	sum := float32(0.0)
	for _, x := range a {
		if x != MissingDataSentinel {
			count += 1
			sum += x
		}
	}

	if count == 0 {
		return 0, 0, fmt.Errorf("Slice of length zero.")
	}
	mean := sum / float32(count)

	vr := float32(0.0)
	for _, x := range a {
		if x != MissingDataSentinel {
			vr += (x - mean) * (x - mean)
		}
	}
	stddev := float32(math.Sqrt(float64(vr / float32(count))))

	return mean, stddev, nil
}

// stddev only works on arrays that have no MISSING_DATA_SENTINEL values.
func stddev(arr []float32, middle float32) float32 {
	sum := float32(0)
	for _, x := range arr {
		sum += (x - middle) * (x - middle)
	}
	return float32(math.Sqrt(float64(sum) / float64(len(arr)-1)))
}

type float32Slice []float32

func (p float32Slice) Len() int           { return len(p) }
func (p float32Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p float32Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// TwoSidedStdDev returns the median, and the stddev of all the points below and
// above the median respectively.
//
// That is, the vector is sorted, the median found, and then the stddev of all
// the points below the median are returned, along with the stddev of all the
// points above the median.
//
// This is useful because performance measurements are inherintly asymmetric. A
// benchmark can always run 2x slower, or 10x slower, there's no upper bound. On
// the other hand a performance metric can only run 100% faster, i.e. have a
// value of 0. This implies that the distribution of samples from a benchmark
// are skewed.
//
// MISSING_DATA_SENTINELs are ignored.
//
// The median is chosen as the midpoint instead of the mean because that ensures
// that both sides have the same number of points (+/- 1).
func TwoSidedStdDev(arr []float32) (float32, float32, float32, error) {
	// Allocate a new slice since we need to sort the incoming values in the
	// array.
	values := make([]float32, 0, len(arr))
	count := 0
	for _, x := range arr {
		if x != MissingDataSentinel {
			count += 1
			values = append(values, x)
		}
	}
	if count < 4 {
		return 0, 0, 0, fmt.Errorf("Insufficient number of points, at least 4 are needed: %d", len(values))
	}
	sort.Sort(float32Slice(values))

	mid := len(values) / 2
	var median float32
	if len(values)%2 == 1 {
		median = values[mid]
	} else {
		median = (values[mid-1] + values[mid]) / 2
	}
	return median, stddev(values[:mid], median), stddev(values[mid:], median), nil
}

// ScaleBy divides each non-sentinel value in the slice by 'b', converting
// resulting NaNs and Infs into sentinel values.
func ScaleBy(a []float32, b float32) {
	for i, x := range a {
		if x != MissingDataSentinel {
			scaled := a[i] / b
			if math.IsNaN(float64(scaled)) || math.IsInf(float64(scaled), 0) {
				a[i] = MissingDataSentinel
			} else {
				a[i] = scaled
			}
		}
	}
}

// Norm normalizes the slice to a mean of 0 and a standard deviation of 1.0.
// The minStdDev is the minimum standard deviation that is normalized. Slices
// with a standard deviation less than that are not normalized for variance.
func Norm(a []float32, minStdDev float32) {
	mean, stddev, err := MeanAndStdDev(a)
	if err != nil {
		return
	}
	// Normalize the data to a mean of 0 and standard deviation of 1.0.
	for i, x := range a {
		if x != MissingDataSentinel {
			newX := x - mean
			if stddev > minStdDev {
				newX = newX / stddev
			}
			a[i] = newX
		}
	}
}

// Fill in non-sentinel values with nearby points.
//
// Sentinel values are filled with points later in the array, except for the
// end of the array where we can't do that, so we fill those points in
// using the first non sentinel found when searching backwards from the end.
//
// So
//    [1e32, 1e32,   2, 3, 1e32, 5]
// becomes
//    [2,    2,      2, 3, 5,    5]
//
// and
//    [3, 1e32, 5, 1e32, 1e32]
// becomes
//    [3, 5,    5, 5,    5]
//
//
// Note that a vector filled with all sentinels will be filled with 0s.
func Fill(a []float32) {
	// Find the first non-sentinel data point.
	last := float32(0.0)
	for i := len(a) - 1; i >= 0; i-- {
		if a[i] != MissingDataSentinel {
			last = a[i]
			break
		}
	}
	// Now fill.
	for i := len(a) - 1; i >= 0; i-- {
		if a[i] == MissingDataSentinel {
			a[i] = last
		} else {
			last = a[i]
		}
	}
}

// FillAt returns the value at the given index of a vector, using non-sentinel
// values with nearby points if the original is MISSING_DATA_SENTINEL.
//
// Note that the input vector is unchanged.
//
// Returns non-nil error if the given index is out of bounds.
func FillAt(a []float32, i int) (float32, error) {
	l := len(a)
	if i < 0 || i >= l {
		return 0, fmt.Errorf("FillAt index %d out of bound %d.\n", i, l)
	}
	b := make([]float32, l, l)
	copy(b, a)
	Fill(b)
	return b[i], nil
}

// Dup a slice of float32.
func Dup(a []float32) []float32 {
	ret := make([]float32, len(a), len(a))
	copy(ret, a)
	return ret
}

// Mean calculates and returns the Mean value of the given []float32.
//
// Returns 0 for an array with no non-MISSING_DATA_SENTINEL values.
func Mean(xs []float32) float32 {
	total := float32(0.0)
	n := 0
	for _, v := range xs {
		if v != MissingDataSentinel {
			total += v
			n++
		}
	}
	if n == 0 {
		return total
	}
	return total / float32(n)
}

// MeanMissing calculates and returns the Mean value of the given []float32.
//
// Returns MISSING_DATA_SENTINEL for an array with all MISSING_DATA_SENTINEL values.
func MeanMissing(xs []float32) float32 {
	total := float32(0.0)
	n := 0
	for _, v := range xs {
		if v != MissingDataSentinel {
			total += v
			n++
		}
	}
	if n == 0 {
		return MissingDataSentinel
	}
	return total / float32(n)
}

// FillMeanMissing fills the slice with the mean of all the values in the slice
// using MeanMissing.
func FillMeanMissing(a []float32) {
	value := MeanMissing(a)
	// Now fill.
	for i := range a {
		a[i] = value
	}
}

// FillStdDev fills the slice with the Standard Deviation of the values in the slice.
//
// If slice is filled with only MISSING_DATA_SENTINEL then the slice will be
// filled with MISSING_DATA_SENTINEL.
func FillStdDev(a []float32) {
	_, stddev, err := MeanAndStdDev(a)
	if err != nil {
		stddev = MissingDataSentinel
	}
	// Now fill.
	for i := range a {
		a[i] = stddev
	}
}

// FillCov fills the slice with the Coefficient of Variation of the values in the slice.
//
// If the mean is 0 or the slice is filled with only MISSING_DATA_SENTINEL then
// the slice will be filled with MISSING_DATA_SENTINEL.
func FillCov(a []float32) {
	mean, stddev, err := MeanAndStdDev(a)
	cov := MissingDataSentinel
	if err == nil {
		cov = stddev / mean
	}
	if math.IsNaN(float64(cov)) || math.IsInf(float64(cov), 0) {
		cov = MissingDataSentinel
	}
	// Now fill.
	for i := range a {
		a[i] = cov
	}
}

// ssen calculates and returns the sum squared error from the given base of []float32.
//
// Returns 0 for an array with no non-MISSING_DATA_SENTINEL values.
func ssen(xs []float32, base float32) (float32, int) {
	total := float32(0.0)
	n := 0
	for _, v := range xs {
		if v != MissingDataSentinel {
			n++
			total += (v - base) * (v - base)
		}
	}
	return total, n
}

// SSE calculates and returns the sum squared error from the given base of []float32.
//
// Returns 0 for an array with no non-MISSING_DATA_SENTINEL values.
func SSE(xs []float32, base float32) float32 {
	total, _ := ssen(xs, base)
	return total
}

// StdDev returns the sample standard deviation.
func StdDev(xs []float32, base float32) float32 {
	n := len(xs)
	if n < 2 {
		return 0
	}
	sse, n := ssen(xs, base)
	return float32(math.Sqrt(float64(sse / float32(n-1))))
}

// FillStep fills the slice with the step function value, i.e.  the ratio of
// the ave of the first half of the trace values divided by the ave of the
// second half of the trace values.
//
// If the second mean is 0 or the slice is filled with only MISSING_DATA_SENTINEL then
// the slice will be filled with MISSING_DATA_SENTINEL.
func FillStep(a []float32) {
	mid := len(a) / 2

	step := MissingDataSentinel
	meanFirst := MeanMissing(a[:mid])
	meanLast := MeanMissing(a[mid:])
	if meanLast != MissingDataSentinel && meanFirst != MissingDataSentinel {
		step = meanFirst / meanLast
	}
	if math.IsNaN(float64(step)) || math.IsInf(float64(step), 0) {
		step = MissingDataSentinel
	}
	// Now fill.
	for i := range a {
		a[i] = step
	}
}
