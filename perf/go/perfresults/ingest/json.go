package ingest

import (
	"math"

	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/perfresults"
)

// toMeasurement converts Histogram into format.SingleMeasurement
//
// Each trace will have only value at a point, we aggregate the samples in several ways and
// generate several traces per each aggregation.
func toMeasurement(h perfresults.Histogram) []format.SingleMeasurement {
	if len(h.SampleValues) == 0 {
		return nil
	}
	ms := make([]format.SingleMeasurement, 0, len(perfresults.AggregationMapping))
	for a, f := range perfresults.AggregationMapping {
		m := f(h)
		if math.IsInf(m, 0) || math.IsNaN(m) {
			continue
		}
		ms = append(ms, format.SingleMeasurement{
			Value:       a,
			Measurement: float32(m),
		})
	}
	return ms
}

// ConvertPerfResultsFormat converts format.Format.
//
// format.Format can be used later to be loaded by the ingestor for charting.
func ConvertPerfResultsFormat(pr *perfresults.PerfResults, hash string, headers map[string]string, links map[string]string) format.Format {
	ct := 0
	results := make([]format.Result, len(pr.Histograms))
	for key, hist := range pr.Histograms {
		results[ct] = format.Result{
			Key: map[string]string{
				"chart": key.ChartName,
				"unit":  key.Unit,
				"story": key.Story,
				"arch":  key.Architecture,
				"os":    key.OSName,
			},
		}
		if stat := toMeasurement(hist); stat != nil {
			results[ct].Measurements = map[string][]format.SingleMeasurement{
				"stat": stat,
			}
		}
		ct++
	}
	return format.Format{
		Version: format.FileFormatVersion,
		GitHash: hash,
		Key:     headers,
		Links:   links,
		Results: results,
	}
}
