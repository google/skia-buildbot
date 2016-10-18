// Some basic functions on slices of float32.
package vec32

import (
	"fmt"
	"math"
)

const (
	// MISSING_DATA_SENTINEL signifies a missing sample value.
	//
	// JSON doesn't support NaN or +/- Inf, so we need a valid float32 to signal
	// missing data that also has a compact JSON representation.
	MISSING_DATA_SENTINEL float32 = 1e32
)

func MeanAndStdDev(a []float32) (float32, float32, error) {
	count := 0
	sum := float32(0.0)
	for _, x := range a {
		if x != MISSING_DATA_SENTINEL {
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
		if x != MISSING_DATA_SENTINEL {
			vr += (x - mean) * (x - mean)
		}
	}
	stddev := float32(math.Sqrt(float64(vr / float32(count))))

	return mean, stddev, nil
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
		if x != MISSING_DATA_SENTINEL {
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
		if a[i] != MISSING_DATA_SENTINEL {
			last = a[i]
			break
		}
	}
	// Now fill.
	for i := len(a) - 1; i >= 0; i-- {
		if a[i] == MISSING_DATA_SENTINEL {
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

func Dup(a []float32) []float32 {
	ret := make([]float32, len(a), len(a))
	copy(ret, a)
	return ret
}
