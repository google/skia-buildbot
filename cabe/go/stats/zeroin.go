package stats

import (
	"errors"
	"fmt"
	"math"
)

// zeroin: obtain a function zero with the given range.
// Implementation based on zeroin.c in R 4.1.3 (https://cran.r-project.org/bin/windows/base/old/4.1.3/)
// Algorithm
//
//		G.Forsythe, M.Malcolm, C.Moler, Computer methods for mathematical computations.
//	  M., Mir, 1980, p.180 of the Russian edition.
//
// The function makes use of the bisection procedure combined with the linear or quadric
// inverse interpolation. At every step program operates on three abscissae - a, b, and c.
//
//	b - the last and the best approximation to the root
//	a - the last but one approximation
//	c - the last but one or even earlier approximation than a that
//	    1) |f(b)| <= |f(c)|
//	    2) f(b) and f(c) have opposite signs, i.e. b and c confine the root
//
// At every step zeroin selects one of the two new approximations, the former being obtained by
// the bisection procedure and the latter resulting in the interpolation (if a,b, and c are
// all different the quadric interpolation is utilized, otherwise the linear one). If the latter
// (i.e. obtained by the interpolation) point is reasonable (i.e. lies within the current
// interval [b,c] not being too close to the boundaries) it is accepted. The bisection result
// is used in the other case.
func zeroin(zq, a, b, tol float64, nums []float64, alt Hypothesis, correct bool, f func([]float64, float64, Hypothesis, bool) float64) (float64, error) {
	// Calculate f(a).
	fa := f(nums, a, alt, correct)
	fa = fa - zq

	if fa == 0 {
		return a, nil
	}

	// Calculate f(b).
	fb := f(nums, b, alt, correct)
	fb = fb - zq

	if fb == 0 {
		return b, nil
	}

	// f(a) and f(b) should have opposite signs
	if fa*fb >= 0 {
		return math.NaN(), errors.New("f() values at end points should be of opposite sign")
	}

	var prevStep, newStep, tolAct, p, q, cb, t1, t2, c, fc, epsilon float64
	c, fc = a, fa
	epsilon = math.Nextafter(1.0, 2.0) - 1.0
	it := 0
	// Repeat until f(b) == 0 or |b-a| is small enough (convergence).
	for fb != 0 {
		it++
		if it > 1000 {
			return math.NaN(), fmt.Errorf("iteration(%d) exceeds the maximum iteration", it)
		}

		// prevStep is the step at previous iteration.
		prevStep = b - a

		if math.Abs(fc) < math.Abs(fb) {
			a = b
			b = c
			c = a
			fa = fb
			fb = fc
			fc = fa
		}

		// tolAct is the actual tolerance
		tolAct = 2*epsilon*math.Abs(b) + tol/2
		// newStep is the step at this iteration.
		newStep = (c - b) / 2

		if math.Abs(newStep) <= tolAct || fb == 0 {
			return b, nil
		}

		if math.Abs(prevStep) >= tolAct && math.Abs(fa) > math.Abs(fb) {
			cb = c - b
			if a == c {
				// Linear interpolation.
				t1 = fb / fa
				p = cb * t1
				q = 1.0 - t1
			} else {
				// Quadratic inverse interpolation.
				q = fa / fc
				t1 = fb / fc
				t2 = fb / fa
				p = t2 * (cb*q*(q-t1) - (b-a)*(t1-1.0))
				q = (q - 1.0) * (t1 - 1.0) * (t2 - 1.0)
			}
			if p > 0 {
				q = -q
			} else {
				p = -p
			}
			if p < (0.75*cb*q-math.Abs(tolAct*q)/2) && p < math.Abs(prevStep*q/2) {
				newStep = p / q
			}
		}
		if math.Abs(newStep) < tolAct {
			if newStep > 0 {
				newStep = tolAct
			} else {
				newStep = -tolAct
			}
		}

		a, fa = b, fb
		b += newStep
		fb = f(nums, b, alt, correct)
		fb = fb - zq
		if (fb > 0 && fc > 0) || (fb < 0 && fc < 0) {
			c, fc = a, fa
		}
	}
	return b, nil
}
