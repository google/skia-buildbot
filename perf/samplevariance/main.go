// Take as input a single Perf JSON file and emits a single CSV line with the trace name, the mean, the min, the max, and all the samples.
package main

import (
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
	for traceid, samples := range parser.GetSamplesFromLegacyFormat(benchData) {
		sort.Float64s(samples.Values)
		mean := stats.Mean(samples.Values)
		fmt.Println(traceid, mean, samples.Values[0], samples.Values[len(samples.Values)-1])
	}
}
