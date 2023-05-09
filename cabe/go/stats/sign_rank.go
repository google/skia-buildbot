package stats

import (
	"fmt"
	"math"
)

// dist represents the probability distribution of the sign rank test.
type dist struct {
	n   int
	pmf []float64
}

// newWilcoxonDistribution is based on scipy: https://github.com/scipy/scipy/blob/main/scipy/stats/_hypotests.py#L600
func newWilcoxonDistribution(n int) (*dist, error) {
	if n <= 0 {
		return nil, fmt.Errorf("input n (%v) should be a non zero integer", n)
	}

	size := n*(n+1)/2 + 1
	c := make([]float64, size)
	c[0] = 1.0
	cSize := 1
	for k := 1; k <= n; k++ {
		prevC := c
		c = make([]float64, size)
		m := cSize
		curSize := k*(k+1)/2 + 1
		for i := 0; i < m; i++ {
			c[i] = prevC[i] * 0.5
		}
		j := 0
		for i := curSize - m; i < curSize; i++ {
			c[i] += prevC[j] * 0.5
			j++
		}
		cSize = curSize
	}
	return &dist{
		n:   n,
		pmf: c,
	}, nil
}

// pSignRank is the distribution function for the Wilcoxon Signed Rank statistic obtained from a sample with size n.
func (dist dist) pSignRank(x float64, lowerTail bool) float64 {
	x = math.Round(x + 1e-7)
	if x < 0 {
		if lowerTail {
			return 0
		}
		return 1
	}

	if x >= float64(len(dist.pmf)-1) {
		if lowerTail {
			return 1
		}
		return 0
	}

	p := 0.0
	for i := 0; i <= int(x); i++ {
		p += dist.pmf[i]
	}

	if lowerTail {
		return p
	}
	return 1 - p
}

// qSignRank is the quantile function for the Wilcoxon Signed Rank statistic obtained from a sample with size n.
func (dist dist) qSignRank(x float64, lowerTail bool) (float64, error) {
	if x < 0 || x > 1 {
		return 0, fmt.Errorf("input probability x (%v) should between 0 and 1", x)
	}
	if lowerTail {
		if x == 0 {
			return 0, nil
		}
		if x == 1 {
			return float64(len(dist.pmf) - 1), nil
		}
	} else {
		if x == 1 {
			return 0, nil
		}
		if x == 0 {
			return float64(len(dist.pmf) - 1), nil
		}
	}

	if !lowerTail {
		x = 1 - x
	}

	p := 0.0
	q := 0
	if x <= 0.5 {
		for true {
			if q >= len(dist.pmf) {
				return 0, fmt.Errorf("q (%v) should be less than length of pmf (%v)", q, len(dist.pmf))
			}
			p += dist.pmf[q]
			if p >= x {
				break
			}
			q++
		}
	} else {
		x = 1 - x
		for true {
			p += dist.pmf[q]
			if p > x {
				q = len(dist.pmf) - 1 - q
				break
			}
			q++
		}
	}

	return float64(q), nil
}
