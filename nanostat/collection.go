package main

import (
	"fmt"
	"math"
	"sort"

	"github.com/aclements/go-moremath/stats"
	"go.skia.org/infra/perf/go/ingest/parser"
)

// An Order defines a sort order for a slice of Rows.
type Order func(rows []Row, i, j int) bool

// ByName sorts tables by the trace id name column
func ByName(rows []Row, i, j int) bool {
	return rows[i].Name < rows[j].Name
}

// ByDelta sorts tables by the Delta column,
// reversing the order when larger is better (for "speed" results).
func ByDelta(rows []Row, i, j int) bool {
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

type Metrics struct {
	Mean    float64
	StdDev  float64
	Values  []float64 // May have outliers removed.
	Percent float64
}

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

type Row struct {
	Name    string
	Samples [2]Metrics
	Delta   float64
	P       float64
	Note    string
}

type Config struct {
	// Alpha is the p-value cutoff to report a change as significant.
	// If zero, it defaults to 0.05.
	Alpha float64

	// Order specifies the row display order for this table.
	Order Order

	IQRR bool
}

func Analyze(config Config, before, after map[string]parser.Samples) []Row {
	ret := []Row{}

	for traceID, beforeSamples := range before {
		afterSamples, ok := after[traceID]
		if !ok {
			continue
		}
		beforeMetrics := CalculateMetrics(config, beforeSamples)
		afterMetrics := CalculateMetrics(config, afterSamples)
		mwResults, err := stats.MannWhitneyUTest(beforeMetrics.Values, afterMetrics.Values, stats.LocationDiffers)
		if err != nil {
			continue
		}
		note := ""
		if err == stats.ErrZeroVariance {
			note = "(zero variance)"
		} else if err == stats.ErrSampleSize {
			note = "(too few samples)"
		} else if err == stats.ErrSamplesEqual {
			note = "(all equal)"
		} else if err != nil {
			note = fmt.Sprintf("(%s)", err)
		}

		delta := math.NaN()
		if mwResults.P < config.Alpha {
			if afterMetrics.Mean == beforeMetrics.Mean {
				delta = 0
			} else {
				delta = ((afterMetrics.Mean / beforeMetrics.Mean) - 1) * 100
			}
		}
		ret = append(ret, Row{
			Name:    traceID,
			Delta:   delta,
			P:       mwResults.P,
			Samples: [2]Metrics{beforeMetrics, afterMetrics},
			Note:    note,
		})
	}

	// Do sorting here.
	if len(ret) > 0 {
		if config.Order != nil {
			Sort(ret, config.Order)
		}
	}

	return ret
}
