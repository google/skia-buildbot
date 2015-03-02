// Some basic functions on slices of float64s.
package vec

import (
	"fmt"
	"math"

	"go.skia.org/infra/perf/go/config"
)

func MeanAndStdDev(a []float64) (float64, float64, error) {
	count := 0
	sum := 0.0
	for _, x := range a {
		if x != config.MISSING_DATA_SENTINEL {
			count += 1
			sum += x
		}
	}

	if count == 0 {
		return 0, 0, fmt.Errorf("Slice of length zero.")
	}
	mean := sum / float64(count)

	vr := 0.0
	for _, x := range a {
		if x != config.MISSING_DATA_SENTINEL {
			vr += (x - mean) * (x - mean)
		}
	}
	stddev := math.Sqrt(vr / float64(count))

	return mean, stddev, nil
}

// Norm normalizes the slice to a mean of 0 and a standard deviation of 1.0.
// The minStdDev is the minimum standard deviation that is normalized. Slices
// with a standard deviation less than that are not normalized for variance.
func Norm(a []float64, minStdDev float64) {
	mean, stddev, err := MeanAndStdDev(a)
	if err != nil {
		return
	}
	// Normalize the data to a mean of 0 and standard deviation of 1.0.
	for i, x := range a {
		if x != config.MISSING_DATA_SENTINEL {
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
// Sentinel values are filled with older points, except for the beginning of
// the array where we can't do that, so we fill those points in using the first
// non sentinel.
//
// So
//    [1e100, 1e100, 2, 3, 1e100, 5]
// becomes
//    [2    , 2    , 2, 3, 5    , 5]
//
//
// Note that a vector filled with all sentinels will be filled with 0s.
func Fill(a []float64) {
	// Find the first non-sentinel data point.
	last := 0.0
	for i := len(a) - 1; i >= 0; i-- {
		if a[i] != config.MISSING_DATA_SENTINEL {
			last = a[i]
			break
		}
	}
	// Now fill.
	for i := len(a) - 1; i >= 0; i-- {
		if a[i] == config.MISSING_DATA_SENTINEL {
			a[i] = last
		} else {
			last = a[i]
		}
	}
}

// FillAt returns the value at the given index of a vector, using non-sentinel
// values with nearby points if the original is config.MISSING_DATA_SENTINEL.
//
// Note that the input vector is unchanged.
//
// Returns non-nil error if the given index is out of bounds.
func FillAt(a []float64, i int) (float64, error) {
	l := len(a)
	if i < 0 || i >= l {
		return 0, fmt.Errorf("FillAt index %d out of bound %d.\n", i, l)
	}
	b := make([]float64, l, l)
	copy(b, a)
	Fill(b)
	return b[i], nil
}
