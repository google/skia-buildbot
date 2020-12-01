package samplestats

import (
	"math"

	"github.com/aclements/go-moremath/stats"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/ingest/parser"
)

const defaultAlpha = 0.05

// Test is the kind of statistical test we are doing.
type Test string

// UTest is the Mann-Whitney U test.
const UTest Test = "utest"

// TTest is the Two Sample Welch test.
const TTest Test = "ttest"

// Config controls the analysis done on the samples.
type Config struct {
	// Alpha is the p-value cutoff to report a change as significant.
	Alpha float64

	// Order is used to sort the results.
	Order Order

	// IQRR, if true, causes outliers to be removed via the Interquartile Rule.
	IQRR bool

	// All, if true, returns all rows, even if no significant change was seen
	// for a now. If false then only return rows with significant changes.
	All bool

	// Test is the kind of statistical test to do.
	Test Test
}

// Result from Analyze.
type Result struct {
	// Rows, with one Row per result.
	Rows []Row

	// Skipped is the number of results we skipped, because either we couldn't
	// calculate the statistics, or there wasn't data in both 'before' and
	// 'after'.
	Skipped int
}

// Row is a single row in the results.
type Row struct {
	// Name of sample, i.e. its trace name.
	Name string

	// Samples are the metrics for both samples, the first is 'before', the
	// second is 'after'. See Analyze().
	Samples [2]Metrics

	// The full set of Params for the trace.
	Params paramtools.Params

	// The change in mean between before and after samples, as a percent. I.e.
	// from 0 to 100.
	Delta float64

	// P is p-value for the specified test for the null hypothesis that the
	// samples are from the same population.
	P float64

	// Note of any issues that arose during calculations.
	Note string
}

// Analyze returns an analysis of the samples as a slice of Row.
func Analyze(config Config, before, after map[string]parser.Samples) Result {
	ret := []Row{}
	skipped := 0

	allTraceIDs := util.NewStringSet()
	for traceID := range before {
		allTraceIDs[traceID] = true
	}
	for traceID := range after {
		allTraceIDs[traceID] = true
	}

	for _, traceID := range allTraceIDs.Keys() {
		beforeSamples, ok := before[traceID]
		if !ok {
			skipped += 1
			continue
		}
		afterSamples, ok := after[traceID]
		if !ok {
			skipped += 1
			continue
		}
		beforeMetrics := calculateMetrics(config, beforeSamples)
		afterMetrics := calculateMetrics(config, afterSamples)

		p := 1.0
		note := ""

		if config.Test == UTest || config.Test == "" {
			mwResults, err := stats.MannWhitneyUTest(beforeMetrics.Values, afterMetrics.Values, stats.LocationDiffers)
			if err != nil {
				note = err.Error()
			} else {
				p = mwResults.P
			}
		} else {
			wtResult, err := stats.TwoSampleWelchTTest(stats.Sample{Xs: beforeMetrics.Values}, stats.Sample{Xs: afterMetrics.Values}, stats.LocationDiffers)
			if err != nil {
				note = err.Error()
			} else {
				p = wtResult.P
			}
		}

		delta := math.NaN()
		alpha := config.Alpha
		if alpha == 0 {
			alpha = defaultAlpha
		}

		if p < alpha {
			if afterMetrics.Mean == beforeMetrics.Mean {
				delta = 0
			} else {
				delta = ((afterMetrics.Mean / beforeMetrics.Mean) - 1) * 100
			}
		} else if !config.All {
			continue
		}
		ret = append(ret, Row{
			Name:    traceID,
			Delta:   delta,
			P:       p,
			Samples: [2]Metrics{beforeMetrics, afterMetrics},
			Note:    note,
			Params:  beforeSamples.Params,
		})
	}

	// Sort the rows.
	if len(ret) > 0 {
		if config.Order != nil {
			Sort(ret, config.Order)
		}
	}

	return Result{
		Rows:    ret,
		Skipped: skipped,
	}
}
