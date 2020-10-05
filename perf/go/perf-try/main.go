package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/types"
)

type sample struct {
	traceID string
	params  paramtools.Params
	values  []float32
}

type total struct {
	keyValue string
	total    float32
}

type totalSlice []total

func (p totalSlice) Len() int           { return len(p) }
func (p totalSlice) Less(i, j int) bool { return p[i].total < p[j].total }
func (p totalSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func main() {
	// Open and read TraceSet
	var ts types.TraceSet

	err := util.WithReadFile(os.Args[1], func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&ts)
	})
	if err != nil {
		log.Fatal(err)
	}

	ps := paramtools.ParamSet{}
	samples := make([]sample, 0, len(ts))
	for traceID, values := range ts {
		params, err := query.ParseKey(traceID)
		if err != nil {
			log.Fatal(err)
		}
		ps.AddParams(params)
		samples = append(samples, sample{
			traceID: traceID,
			params:  params,
			values:  values,
		})
	}

	// Do analysis

	totals := map[string]float32{}
	for _, s := range samples {
		x := s.values[0]
		y := s.values[1]
		if x == vec32.MissingDataSentinel || y == vec32.MissingDataSentinel {
			continue
		}
		percentChange := (y - x) / x
		for key, value := range s.params {
			keyValue := key + "=" + value
			totals[keyValue] += percentChange
		}
	}

	sortableTotals := []total{}

	for keyValue, value := range totals {
		sortableTotals = append(sortableTotals, total{
			keyValue: keyValue,
			total:    value,
		})
	}

	sort.Sort(totalSlice(sortableTotals))

	for i, top := range sortableTotals {
		fmt.Printf("%d %60s %g\n", i+1, top.keyValue, top.total)
	}
}
