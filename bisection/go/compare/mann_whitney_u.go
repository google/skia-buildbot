// Golang implementation of the Mann-Whitney U test.
//
// This code is adapted from [SciPy].
// Which is provided under a BSD-style license.
//
// There is also a Python version in Catapult.
//
// [SciPy]: https://github.com/scipy/scipy/blob/master/scipy/stats/stats.py
// [Catapult]: https://chromium.googlesource.com/catapult.git/+/HEAD/dashboard/dashboard/pinpoint/models/compare/mann_whitney_u.py

package compare

import (
	"math"
	"slices"
	"sort"
)

// MannWhitneyU computes the Mann-Whitney rank test on samples x and y.
func MannWhitneyU(x []float64, y []float64) float64 {
	// The distribution of U is approximately normal for large samples. This
	// implementation uses the normal approximation, so it's recommended to have
	// sample sizes > 20. This version assumes the method = 'asymptotic'.
	// https://docs.scipy.org/doc/scipy/reference/generated/scipy.stats.mannwhitneyu.html

	// Note that there is a golang interpretation of MWU, but it handles ties differently,
	// so it is possible for no p-value to be generated when x and y values are identical. This
	// is possible given that there are benchmarks that measure bimodal values or in the case
	// of bisection error analysis. The scipy equivalent is method = 'exact'
	// https://pkg.go.dev/github.com/aclements/go-moremath/stats#MannWhitneyUTest

	n1 := float64(len(x))
	n2 := float64(len(y))
	ranked := rankData(append(x, y...))
	rankx := ranked[:int(n1)] // get the x-ranks
	s := 0.0
	for _, val := range rankx {
		s += val
	}
	u1 := n1*n2 + n1*(n1+1)/2.0 - s // calc U for x
	u2 := n1*n2 - u1                // remainder is U for y
	t := tieCorrectionFactor(ranked)
	if t == 0 {
		return 1.0
	}
	sd := math.Sqrt(t * n1 * n2 * (n1 + n2 + 1) / 12.0)
	mean_rank := n1*n2/2.0 + 0.5
	big_u := math.Max(u1, u2)
	z := (big_u - mean_rank) / sd
	return 2 * normSf(math.Abs(z))
}

func rankData(a []float64) []float64 {
	// Assigns ranks to data. Ties are given the mean of the ranks of the items.
	//
	// This is called "fractional ranking":
	// 	https://en.wikipedia.org/wiki/Ranking

	sorter := argSortReverse(a)
	rankedMin := make([]float64, len(sorter))
	for i, j := range sorter {
		rankedMin[j] = float64(i)
	}

	sorter = argSort(a)
	rankedMax := make([]float64, len(sorter))
	for i, j := range sorter {
		rankedMax[j] = float64(i)
	}
	result := make([]float64, len(rankedMin))
	for i := 0; i < len(result); i++ {
		result[i] = 1 + (rankedMin[i]+rankedMax[i])/2.0
	}
	return result
}

func argSort(a []float64) []int {
	// Returns the indices that would sort an array.
	// Ties are given indices in ordinal order.

	indexes := make([]int, len(a))
	for i := range indexes {
		indexes[i] = i
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		return a[indexes[i]] < a[indexes[j]]
	})
	return indexes
}

func argSortReverse(a []float64) []int {
	// Returns the indices that would sort an array.
	// Ties are given indices in reverse ordinal order.

	indexes := make([]int, len(a))
	for i := range indexes {
		indexes[i] = i
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		return a[indexes[i]] > a[indexes[j]]
	})
	slices.Reverse(indexes)
	return indexes
}

func tieCorrectionFactor(rankvals []float64) float64 {
	arr := make([]float64, len(rankvals))
	copy(arr, rankvals)
	sort.Float64s(arr)
	cnt := make([]float64, 0)
	for i := 0; i < len(arr); i++ {
		count := 1.0
		for i+1 < len(arr) && arr[i] == arr[i+1] {
			count++
			i++
		}
		cnt = append(cnt, count)
	}
	size := len(arr)
	if size < 2 {
		return 1.0
	}
	sum := 0.0
	for _, x := range cnt {
		sum += math.Pow(x, 3) - x
	}
	return 1.0 - sum/float64(size*size*size-size)
}

func normSf(x float64) float64 {
	return (1 - math.Erf(x/math.Sqrt(2))) / 2
}
