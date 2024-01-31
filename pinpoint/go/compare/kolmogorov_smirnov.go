// Golang interpretation of the Kolmogorov Smirnov test
// This code is adapted from [SciPy].
// Which is provided under a BSD-style license.
//
// There is also a Python version in Catapult.
//
// [Scipy]: https://github.com/scipy/scipy/blob/master/scipy/stats/stats.py
// [Catapult]: https://chromium.googlesource.com/catapult.git/+/HEAD/dashboard/dashboard/pinpoint/models/compare/kolmogorov_smirnov.py

package compare

import (
	"fmt"
	"math"
	"sort"
)

// KolmogorovSmirnov computes the 2-sample Kolmogorov-Smirnov test on
// samples x and y.
func KolmogorovSmirnov(x []float64, y []float64) (float64, error) {
	// This is a two-sided test for the null hypothesis that 2 independent samples
	// are drawn from the same continuous distribution.

	n1 := len(x)
	n2 := len(y)

	if n1 == 0 {
		return -1.0, fmt.Errorf("x is an empty array")
	} else if n2 == 0 {
		return -1.0, fmt.Errorf("y is an empty array")
	}

	sort.Float64s(x)
	sort.Float64s(y)
	dataAll := append(x, y...)

	cdf1 := make([]float64, len(dataAll))
	for i, value := range dataAll {
		cdf1[i] = float64(sort.SearchFloat64s(x, value)) / float64(n1)
	}

	cdf2 := make([]float64, len(dataAll))
	for i, value := range dataAll {
		cdf2[i] = float64(sort.SearchFloat64s(y, value)) / float64(n2)
	}

	d := 0.0
	for i := 0; i < len(cdf1); i++ {
		diff := math.Abs(cdf1[i] - cdf2[i])
		if diff > d {
			d = diff
		}
	}

	en := math.Sqrt(float64(n1*n2) / float64(n1+n2))

	p_value := survival((en + 0.12 + 0.11/en) * d)
	return p_value, nil
}

func survival(y float64) float64 {
	// Survival function of the Kolmogorov-Smirnov two-sided test for large N.

	// https://github.com/scipy/scipy/blob/master/scipy/special/cephes/kolmogorov.c

	if y < 1.1e-16 {
		return 1.0
	}

	x := -2.0 * y * y
	sign := 1.0
	p := 0.0
	r := 1.0

	for true {
		t := math.Exp(x * r * r)
		p += sign * t
		if t == 0.0 {
			break
		}
		r += 1.0
		sign = -sign
		if t/p <= 1.1e-16 {
			break
		}
	}

	return p + p
}
