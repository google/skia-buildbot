package stats

import (
	"fmt"
	"math"
	"sort"

	moreStat "github.com/aclements/go-moremath/stats"
	"go.skia.org/infra/go/sklog"
)

// A Hypothesis specifies the alternative hypothesis of a
// location test such as a WilcoxonSignedRankedTest. The
// default (zero) value is to test against the alternative hypothesis
// that they differ.
type Hypothesis int

// A DataTransform specifies transform of the input/output of WilcoxonSignedRankedTest. The
// default (zero) value is to use the log transform.
type DataTransform int

const (
	// Less specifies the alternative hypothesis that the
	// location of the first sample is less than the second. This
	// is a one-tailed test.
	Less Hypothesis = -1

	// TwoSided specifies the alternative hypothesis that
	// the locations of the two samples are not equal. This is a
	// two-tailed test.
	TwoSided Hypothesis = 0

	// Greater specifies the alternative hypothesis that
	// the location of the first sample is greater than the
	// second. This is a one-tailed test.
	Greater Hypothesis = 1

	confLevel float64 = 0.95

	// LogTransform specifies the input of the wilcox test should be log transformed.
	// The resulting point estimate and the boundaries are delta percentage.
	LogTransform DataTransform = 0

	// NormalizeResult specifies the output of the wilcox test should be normalized by
	// dividing the median of the second input of the wilcox test. The point estimate and
	// the boundaries are delta percentage.
	NormalizeResult DataTransform = 1

	// OriginalResult specifies that we don't need to do additional processing of the
	// output of the wilcox test.
	OriginalResult DataTransform = 2
)

// WilcoxonSignedRankedTestResult is the result of a WilcoxonSignedRankedTest.
type WilcoxonSignedRankedTestResult struct {
	// An estimate of the location parameter.
	Estimate float64

	// Lower boundary of the confidence interval.
	LowerCi float64

	// Upper boundary of the confidence interval.
	UpperCi float64

	// The p-value for the test
	PValue float64
}

// BerfWilcoxonSignedRankedTestResult is the result of a BerfWilcoxonSignedRankedTest.
type BerfWilcoxonSignedRankedTestResult struct {
	// An estimate of the location parameter.
	Estimate float64

	// Lower boundary of the confidence interval.
	LowerCi float64

	// Upper boundary of the confidence interval.
	UpperCi float64

	// The p-value for the test
	PValue float64

	// The median of the first input
	XMedian float64

	// The median of the second input
	YMedian float64
}

type valueIndex struct {
	value float64
	index int
}

// BerfWilcoxonSignedRankedTest conducts WilcoxonSignedRankedTest based on berf's use case.
// It performs a Wilcoxon signed rank test of the null that the distribution of x - y is symmetric
// about mu. y acts as baseline (control) and x acts as treatment. The resulting estimate/CI
// indicates % change relative to y when using LogTransform and NormalizeResult, and difference
// relative to y when using OriginalResult.
func BerfWilcoxonSignedRankedTest(x, y []float64, alt Hypothesis, transform DataTransform) (*BerfWilcoxonSignedRankedTestResult, error) {
	if len(y) != 0 && len(x) != len(y) {
		return nil, fmt.Errorf("x and y must have the same length for the WilcoxonSignedRankedTest")
	}

	xCopy := make([]float64, len(x))
	yCopy := make([]float64, len(y))
	copy(xCopy, x)
	copy(yCopy, y)
	sort.Float64s(xCopy)
	sort.Float64s(yCopy)
	xMedian := getMedianFromSortedArray(xCopy)
	yMedian := getMedianFromSortedArray(yCopy)

	if transform == LogTransform {
		var transformX, transformY []float64
		for i := range x {
			transformX = append(transformX, math.Log(x[i]))
			transformY = append(transformY, math.Log(y[i]))
		}
		res, err := WilcoxonSignedRankedTest(transformX, transformY, alt)
		if err != nil {
			return nil, fmt.Errorf("Error (%v) when calculating the wilcoxon test for input transformX(%v), transformY(%v), alt(%v)", err, transformX, transformY, alt)
		}

		return &BerfWilcoxonSignedRankedTestResult{Estimate: (math.Exp(res.Estimate) - 1) * 100,
			LowerCi: (math.Exp(res.LowerCi) - 1) * 100, UpperCi: (math.Exp(res.UpperCi) - 1) * 100,
			PValue: res.PValue, XMedian: xMedian, YMedian: yMedian}, nil
	}

	res, err := WilcoxonSignedRankedTest(x, y, alt)
	if err != nil {
		return nil, fmt.Errorf("Error (%v) when calculating the wilcoxon test for input x(%v), y(%v), alt(%v)", err, x, y, alt)
	}

	if transform == NormalizeResult {
		return &BerfWilcoxonSignedRankedTestResult{Estimate: res.Estimate / yMedian * 100,
			LowerCi: res.LowerCi / yMedian * 100, UpperCi: res.UpperCi / yMedian * 100,
			PValue: res.PValue, XMedian: xMedian, YMedian: yMedian}, nil
	}

	return &BerfWilcoxonSignedRankedTestResult{Estimate: res.Estimate, LowerCi: res.LowerCi,
		UpperCi: res.UpperCi, PValue: res.PValue, XMedian: xMedian, YMedian: yMedian}, nil
}

// WilcoxonSignedRankedTest conducts WilcoxonSignedRankedTest based on R implementation.
func WilcoxonSignedRankedTest(x, y []float64, alt Hypothesis) (*WilcoxonSignedRankedTestResult, error) {
	if len(x) == 0 {
		return nil, fmt.Errorf("x is missing for the WilcoxonSignedRankedTest")
	}

	if len(y) != 0 && len(x) != len(y) {
		return nil, fmt.Errorf("x and y must have the same length for the WilcoxonSignedRankedTest")
	}

	// Converting the paired data to one sample.
	if len(y) != 0 {
		for i := 0; i < len(x); i++ {
			x[i] = x[i] - y[i]
		}
	}

	zeroes := false
	var xNonZero []float64
	var xAbsNonZero []float64
	for _, ele := range x {
		if ele == 0 {
			zeroes = true
		} else {
			xNonZero = append(xNonZero, ele)
			xAbsNonZero = append(xAbsNonZero, math.Abs(ele))
		}
	}

	xLen := len(xNonZero)
	// exact indicates whether an exact p-value should be computed.
	exact := xLen < 50

	// Find the rank of xAbsNonZero.
	r, hasTies := rank(xAbsNonZero)

	// Get the statistic.
	statistic := getStatistic(xNonZero, r)
	alpha := 1 - confLevel
	var pVal, lowerCi, upperCi, estimate float64

	if exact && !hasTies && !zeroes {
		// Use the exact test for estimation.
		// Calculate pVal
		var err error
		dist, err := newWilcoxonDistribution(xLen)
		if err != nil {
			sklog.Errorf("error (%v) when calculating the wilcoxon distribution for %v", err, xLen)
			return nil, err
		}
		if alt == TwoSided {
			if statistic > float64(xLen*(xLen+1))/4 {
				pVal = dist.pSignRank(statistic-1, false)
			} else {
				pVal = dist.pSignRank(statistic, true)
			}
			pVal = math.Min(2*pVal, 1.0)
		} else if alt == Greater {
			pVal = dist.pSignRank(statistic-1, false)
		} else {
			pVal = dist.pSignRank(statistic, true)
		}

		// Calculate confidence intervals
		diffs := getDiffs(xNonZero)

		qu := 0.0
		achievedAlpha := 0.0
		if alt == TwoSided {
			qu, err = dist.qSignRank(alpha/2, true)
			if err != nil {
				sklog.Errorf("error (%v) when calculating QSignRank with alpha/2 (%v) and xLen (%v)", err, alpha/2, xLen)
				return nil, err
			}
			if qu == 0 {
				qu = 1
			}
			q1 := float64(xLen*(xLen+1))/2 - qu
			achievedAlpha = dist.pSignRank(math.Ceil(q1)-1.0, true)
			achievedAlpha = achievedAlpha * 2
			lowerCi = diffs[int(math.Round(qu))-1]
			upperCi = diffs[int(math.Round(q1))]
		} else if alt == Greater {
			qu, err = dist.qSignRank(alpha, true)
			if err != nil {
				sklog.Errorf("error (%v) when calculating QSignRank with alpha (%v) and xLen (%v)", err, alpha, xLen)
				return nil, err
			}
			if qu == 0 {
				qu = 1
			}
			achievedAlpha = dist.pSignRank(math.Ceil(qu)-1.0, true)
			lowerCi = diffs[int(math.Round(qu))-1]
			upperCi = math.Inf(1)
		} else {
			qu, err = dist.qSignRank(alpha, true)
			if err != nil {
				sklog.Errorf("error (%v) when calculating QSignRank with alpha (%v) and xLen (%v)", err, alpha, xLen)
				return nil, err
			}
			if qu == 0 {
				qu = 1
			}
			q1 := float64(xLen*(xLen+1))/2 - qu
			achievedAlpha = dist.pSignRank(math.Ceil(qu)-1.0, true)
			lowerCi = math.Inf(-1)
			upperCi = diffs[int(math.Round(q1))]
		}

		if achievedAlpha-alpha > alpha/2 {
			sklog.Warning("requested conf.level not achievable")
		}
		estimate = getMedianFromSortedArray(diffs)

		return &WilcoxonSignedRankedTestResult{Estimate: estimate, LowerCi: lowerCi, UpperCi: upperCi, PValue: pVal}, nil
	}
	nTiesSum := getSumOfNTies(r)
	z := statistic - float64(xLen*(xLen+1))/4
	sigma := math.Sqrt(float64(xLen*(xLen+1)*(2*xLen+1))/24 - float64(nTiesSum)/48)
	correction := getCorrection(alt, z)
	correct := true
	z = (z - correction) / sigma
	normalDist := moreStat.StdNormal
	if alt == TwoSided {
		pVal = 2 * math.Min(normalDist.CDF(z), 1-normalDist.CDF(z))
	} else if alt == Greater {
		pVal = 1 - normalDist.CDF(z)
	} else {
		pVal = normalDist.CDF(z)
	}

	// Asymptotic confidence interval for the median in the one-sample case.
	var muMin, muMax, wMuMin, wMuMax, zq float64
	dist := moreStat.StdNormal
	var err error
	if xLen > 0 {
		// These are sample based limits for the median
		muMin = getMin(xNonZero)
		muMax = getMax(xNonZero)
		wMuMin = asymptoticW(xNonZero, muMin, alt, correct)

		if math.IsInf(wMuMin, 0) || math.IsNaN(wMuMin) {
			wMuMax = math.NaN()
		} else {
			wMuMax = asymptoticW(xNonZero, muMax, alt, correct)
		}
	}

	if xLen == 0 || math.IsInf(wMuMax, 0) || math.IsNaN(wMuMax) {
		if alt == Less {
			lowerCi = math.Inf(-1)
			upperCi = math.NaN()
		} else if alt == Greater {
			lowerCi = math.NaN()
			upperCi = math.Inf(1)
		} else {
			lowerCi = math.NaN()
			upperCi = math.NaN()
		}
		if xLen > 0 {
			estimate = (muMin + muMax) / 2.0
		} else {
			estimate = math.NaN()
		}
	} else {
		if alt == TwoSided {
			for true {
				minDiff := wMuMin - dist.InvCDF(1-alpha/2)
				maxDiff := wMuMax - dist.InvCDF(alpha/2)
				if minDiff < 0 || maxDiff > 0 {
					alpha = alpha * 2
				} else {
					break
				}
			}

			if alpha >= 1 || 1-confLevel < alpha*0.75 {
				sklog.Warning("requested confLevel not achievable")
			}

			if alpha < 1 {
				zq = dist.InvCDF(1 - alpha/2)
				lowerCi, err = zeroin(zq, muMin, muMax, 0.0001, xNonZero, alt, correct, asymptoticW)
				if err != nil {
					sklog.Errorf("Error (%v) when using the root finding algorithm to calculate confidence interval", err)
					return nil, err
				}
				zq = dist.InvCDF(alpha / 2)
				upperCi, err = zeroin(zq, muMin, muMax, 0.0001, xNonZero, alt, correct, asymptoticW)
				if err != nil {
					sklog.Errorf("Error (%v) when using the root finding algorithm to calculate confidence interval", err)
					return nil, err
				}
			} else {
				lowerCi = getMedianFromSortedArray(xNonZero)
				upperCi = lowerCi
			}
		} else if alt == Greater {
			for true {
				minDiff := wMuMin - dist.InvCDF(1-alpha/2)
				if minDiff < 0 {
					alpha = alpha * 2
				} else {
					break
				}
			}
			if alpha >= 1 || 1-confLevel < alpha*0.75 {
				sklog.Warning("requested conf.level not achievable")
			}

			if alpha < 1 {
				zq = dist.InvCDF(1 - alpha)
				lowerCi, err = zeroin(zq, muMin, muMax, 0.0001, xNonZero, alt, correct, asymptoticW)
				if err != nil {
					sklog.Errorf("error (%v) when using the root finding algorithm to calculate confidence interval", err)
					return nil, err
				}
			} else {
				lowerCi = getMedianFromSortedArray(xNonZero)
			}
			upperCi = math.Inf(1)
		} else { // alt == Less
			for true {
				maxDiff := wMuMax - dist.InvCDF(alpha/2)
				if maxDiff > 0 {
					alpha = alpha * 2
				} else {
					break
				}
			}
			if alpha >= 1 || 1-confLevel < alpha*0.75 {
				sklog.Warning("requested conf.level not achievable")
			}
			if alpha < 1 {
				zq = dist.InvCDF(alpha)
				upperCi, err = zeroin(zq, muMin, muMax, 0.0001, xNonZero, alt, correct, asymptoticW)
				if err != nil {
					sklog.Errorf("error (%v) when using the root finding algorithm to calculate confidence interval", err)
					return nil, err
				}
			} else {
				upperCi = getMedianFromSortedArray(xNonZero)
			}
			lowerCi = math.Inf(-1)
		}
		// For W(): no continuity correction for estimate
		correct = false
		estimate, err = zeroin(0, muMin, muMax, 0.0001, xNonZero, alt, correct, asymptoticW)
		if err != nil {
			sklog.Errorf("error (%v) when using the root finding algorithm to calculate the point estimate", err)
			return nil, err
		}
	}

	if exact && hasTies {
		sklog.Warning("cannot compute exact p-value and confidence interval with ties")
	}
	if exact && zeroes {
		sklog.Warning("cannot compute exact p-value and confidence interval with zeroes")
	}

	return &WilcoxonSignedRankedTestResult{Estimate: estimate, LowerCi: lowerCi, UpperCi: upperCi, PValue: pVal}, nil
}

// Rank returns the sample ranks of the value in a list.
// It orders the differences from smallest to largest and assigns them their ranks 1,...n (or average rank for ties).
func rank(nums []float64) ([]float64, bool) {
	var valueIndexes []valueIndex
	for index, ele := range nums {
		valueIndexes = append(valueIndexes, valueIndex{value: ele, index: index})
	}
	sort.Slice(valueIndexes, func(i, j int) bool {
		return valueIndexes[i].value < valueIndexes[j].value
	})

	hasTies := false
	var orderedRanks []float64
	for i := 0; i < len(valueIndexes); {
		startRank, ties, v1 := i+1, 0, valueIndexes[i].value
		// Consume samples that tie this sample (including itself).
		for ; i < len(valueIndexes) && valueIndexes[i].value == v1; i++ {
			ties++
		}
		// Assign all tied samples the average rank of the samples.
		rank := float64(i+startRank) / 2

		for j := startRank; j <= i; j++ {
			orderedRanks = append(orderedRanks, rank)
		}
		if ties > 1 {
			hasTies = true
		}
	}

	res := make([]float64, len(valueIndexes))
	for i, ele := range valueIndexes {
		res[ele.index] = orderedRanks[i]
	}

	return res, hasTies
}

// Statistic for the WilcoxonSignedRankedTest is the sum for the ranks whose original values are greater than zeroes.
func getStatistic(x, r []float64) float64 {
	statistics := 0.0
	for i := 0; i < len(x); i++ {
		if x[i] > 0 {
			statistics += r[i]
		}
	}
	return statistics
}

func getDiffs(x []float64) []float64 {
	xLen := len(x)
	diffs := make([][]float64, xLen)
	for i := 0; i < xLen; i++ {
		diffs[i] = make([]float64, xLen)
		for j := 0; j < xLen; j++ {
			diffs[i][j] = x[i] + x[j]
		}
	}

	res := make([]float64, xLen*(xLen+1)/2)
	cnt := 0
	for i := 0; i < xLen; i++ {
		for j := i; j < xLen; j++ {
			res[cnt] += diffs[i][j] / 2.0
			cnt++
		}
	}

	sort.Float64s(res)

	return res
}

func getMedianFromSortedArray(x []float64) float64 {
	len := len(x)
	if len%2 == 1 {
		return x[len/2]
	}
	return (x[len/2] + x[len/2-1]) / 2.0
}

func getSumOfNTies(x []float64) int {
	m := make(map[float64]int)
	for _, val := range x {
		m[val]++
	}

	res := 0
	for _, freq := range m {
		res += freq*freq*freq - freq
	}
	return res
}

func getCorrection(alt Hypothesis, z float64) float64 {
	correction := 0.0
	if alt == TwoSided {
		if z > 0 {
			correction = 0.5
		} else if z < 0 {
			correction = -0.5
		}
	} else if alt == Greater {
		correction = 0.5
	} else {
		correction = -0.5
	}
	return correction
}

func getMin(x []float64) float64 {
	min := math.Inf(1)
	for _, val := range x {
		min = math.Min(min, val)
	}
	return min
}

func getMax(x []float64) float64 {
	max := math.Inf(-1)
	for _, val := range x {
		max = math.Max(max, val)
	}
	return max
}

// asymptoticW is the asymptotic Wilcoxon statistic of x - d.
func asymptoticW(x []float64, d float64, alt Hypothesis, correct bool) float64 {
	var xD []float64
	var xDAbs []float64
	for _, val := range x {
		if val != d {
			xD = append(xD, val-d)
			xDAbs = append(xDAbs, math.Abs(val-d))
		}
	}
	nX := len(xD)
	dR, _ := rank(xDAbs)
	zd := 0.0
	for index, val := range dR {
		if xD[index] > 0 {
			zd += val
		}
	}
	zd = zd - float64(nX*(nX+1))/4
	nTiesSum := getSumOfNTies(dR)
	sigmaCi := math.Sqrt(float64(nX*(nX+1)*(2*nX+1))/24 - float64(nTiesSum)/48)
	if sigmaCi == 0 {
		sklog.Warning("cannot compute confidence interval when all observations are zero or tied")
		return math.NaN()
	}
	correctionCi := 0.0
	if correct {
		correctionCi = getCorrection(alt, zd)
	}
	return (zd - correctionCi) / sigmaCi
}
