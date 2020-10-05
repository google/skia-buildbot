package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"sort"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/types"
)

type sample struct {
	traceID     string
	params      paramtools.Params
	values      []float32
	percentDiff float32
}

type sampleSlice []sample

func (p sampleSlice) Len() int           { return len(p) }
func (p sampleSlice) Less(i, j int) bool { return p[i].percentDiff > p[j].percentDiff }
func (p sampleSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type total struct {
	keyValue string
	total    float32
}

type totalSlice []total

func (p totalSlice) Len() int           { return len(p) }
func (p totalSlice) Less(i, j int) bool { return p[i].total > p[j].total }
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

	// Convert TraceSet into []sample.
	for traceID, values := range ts {
		params, err := query.ParseKey(traceID)
		if err != nil {
			log.Fatal(err)
		}
		ps.AddParams(params)

		tryValue := values[0]
		currentValue := values[1]
		if tryValue == vec32.MissingDataSentinel || currentValue == vec32.MissingDataSentinel {
			continue
		}
		percentDiff := (tryValue - currentValue) / currentValue
		if math.IsNaN(float64(percentDiff)) {
			continue
		}

		samples = append(samples, sample{
			traceID:     traceID,
			params:      params,
			values:      values,
			percentDiff: percentDiff,
		})
	}

	// Sort
	sort.Sort(sampleSlice(samples))

	// Do analysis by params key=value.
	totals := map[string]float32{}
	for _, s := range samples {
		for key, value := range s.params {
			keyValue := key + "=" + value
			totals[keyValue] += s.percentDiff
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

	const n = 10

	fmt.Println("By Params")
	for i, top := range sortableTotals[:n] {
		fmt.Printf("%4d %40s %g\n", i+1, top.keyValue, top.total)
	}
	fmt.Println(" ...")
	for i, top := range sortableTotals[len(sortableTotals)-n:] {
		fmt.Printf("%4d %40s %g\n", len(sortableTotals)-n+i+1, top.keyValue, top.total)
	}

	lastParams := paramtools.Params{}
	fmt.Println("\nIndividual")
	for i, top := range samples[:n] {
		fmt.Printf("%4d %g\n", i+1, top.percentDiff)
		for key, value := range top.params {
			if value != lastParams[key] {
				fmt.Printf("\t%s=%s\n", key, value)
			}
		}
		fmt.Println()
		lastParams = top.params
	}
	fmt.Println(" ...")
	lastParams = paramtools.Params{}
	for i, top := range samples[len(samples)-n:] {
		fmt.Printf("%4d %g\n", len(samples)-n+i+1, top.percentDiff)
		for key, value := range top.params {
			if value != lastParams[key] {
				fmt.Printf("\t%s=%s\n", key, value)
			}
		}
		fmt.Println()
		lastParams = top.params
	}

}
