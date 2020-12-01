package samplestats

import (
	"github.com/aclements/go-moremath/stats"
	"go.skia.org/infra/perf/go/ingest/parser"
)

// Metrics are calculated for each test.
type Metrics struct {
	Mean    float64
	StdDev  float64
	Values  []float64 // May have outliers removed.
	Percent float64
}

// calculateMetrics returns Metrics for the given samples.
func calculateMetrics(config Config, samples parser.Samples) Metrics {
	retValues := []float64{}

	if config.IQRR {
		// Discard outliers.
		values := stats.Sample{Xs: samples.Values}
		q1 := values.Quantile(0.25)
		q3 := values.Quantile(0.75)
		lo := q1 - 1.5*(q3-q1)
		hi := q3 + 1.5*(q3-q1)
		for _, value := range samples.Values {
			if lo <= value && value <= hi {
				retValues = append(retValues, value)
			}
		}
	} else {
		retValues = samples.Values
	}
	// Compute statistics of remaining data.
	mean := stats.Mean(retValues)
	stddev := stats.StdDev(retValues)
	percent := stddev / mean * 100

	return Metrics{
		Mean:    mean,
		StdDev:  stddev,
		Percent: percent,
		Values:  retValues,
	}
}
