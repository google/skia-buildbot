package nanostat

import (
	"math"
	"sort"

	"github.com/aclements/go-moremath/stats"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/perf/go/ingest/parser"
)

// An Order defines a sort order for a slice of Rows.
type Order func(rows []Row, i, j int) bool

// ByName sorts tables by the trace id name column
func ByName(rows []Row, i, j int) bool {
	return rows[i].Name < rows[j].Name
}

// ByDelta sorts tables by the Delta column.
func ByDelta(rows []Row, i, j int) bool {
	// Always sort the NaN results (insignificant changes) to the top.
	if math.IsNaN(rows[i].Delta) {
		return true
	}
	return rows[i].Delta < rows[j].Delta
}

// Reverse returns the reverse of the given order.
func Reverse(order Order) Order {
	return func(rows []Row, i, j int) bool { return order(rows, j, i) }
}

// Sort sorts a Table t (in place) by the given order.
func Sort(rows []Row, order Order) {
	sort.SliceStable(rows, func(i, j int) bool { return order(rows, i, j) })
}

// Metrics are calculated for each test.
type Metrics struct {
	Mean    float64
	StdDev  float64
	Values  []float64 // May have outliers removed.
	Percent float64
}

// CalculateMetrics returns Metrics fo rthe given samples.
func CalculateMetrics(config Config, samples parser.Samples) Metrics {
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

// Row is a single row in the results.
type Row struct {
	Name    string
	Samples [2]Metrics
	Params  paramtools.Params
	Delta   float64
	P       float64
	Note    string
}

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

	// Order specifies the row display order for this table.
	Order Order

	// IQRR, if true, causes outliers to be removed via the Interquartile Rule.
	IQRR bool

	// All, if true, returns all rows, even if no significant change was seen.
	All bool

	// Test is the kind of statistical test to do.
	Test Test
}

// Analyze returns an analysis of the samples as a slice of Row.
func Analyze(config Config, before, after map[string]parser.Samples) []Row {
	ret := []Row{}

	for traceID, beforeSamples := range before {
		afterSamples, ok := after[traceID]
		if !ok {
			continue
		}
		beforeMetrics := CalculateMetrics(config, beforeSamples)
		afterMetrics := CalculateMetrics(config, afterSamples)

		p := 0.0
		note := ""

		if config.Test == UTest {
			mwResults, err := stats.MannWhitneyUTest(beforeMetrics.Values, afterMetrics.Values, stats.LocationDiffers)
			if err != nil {
				note = err.Error()
			}
			p = mwResults.P
		} else {
			wtResult, err := stats.TwoSampleWelchTTest(stats.Sample{Xs: beforeMetrics.Values}, stats.Sample{Xs: afterMetrics.Values}, stats.LocationDiffers)
			if err != nil {
				note = err.Error()
			}
			p = wtResult.P
		}

		delta := math.NaN()
		if p < config.Alpha {
			if afterMetrics.Mean == beforeMetrics.Mean {
				delta = 0
			} else {
				delta = ((afterMetrics.Mean / beforeMetrics.Mean) - 1) * 100
			}
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

	return ret
}
