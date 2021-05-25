// Take as input a single Perf JSON file and emits a single CSV line with the trace name, the mean, the min, the max, and all the samples.
package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/aclements/go-moremath/stats"
	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/ingest/parser"
)

func main() {
	benchData, err := format.ParseLegacyFormat(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	w := csv.NewWriter(os.Stdout)
	for traceid, samples := range parser.GetSamplesFromLegacyFormat(benchData) {
		if samples.Params["source_type"] != "skp" || samples.Params["sub_result"] != "min_ms" {
			continue
		}
		sort.Float64s(samples.Values)
		values := stats.Sample{Xs: samples.Values}
		medianFloat := values.Quantile(0.5)
		median := fmt.Sprintf("%f", medianFloat)
		mean := fmt.Sprintf("%f", stats.Mean(samples.Values))
		min := fmt.Sprintf("%f", samples.Values[0])
		max := fmt.Sprintf("%f", samples.Values[len(samples.Values)-1])
		ratio := fmt.Sprintf("%f", medianFloat/samples.Values[0])
		w.Write([]string{
			traceid, mean, median, min, max, ratio,
		})
	}
	w.Flush()
}
