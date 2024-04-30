package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/perfresults"
	"go.skia.org/infra/perf/go/perfresults/ingest"
)

var (
	buildID   = flag.Int64("build_id", 0, "The Buildbucket ID where it contains the benchmark runs.")
	outputDir = flag.String("output", "", "The output folder where all JSON files will be saved to.")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Simple CLI to fetch perf results from a buildbucket and output their JSON files.\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	ctx := context.Background()
	info, results, err := perfresults.NewLoader().LoadPerfResults(ctx, *buildID)
	if err != nil {
		sklog.Errorf("Failed to fetch perf results from (%v), %v", *buildID, err)
		return
	}

	links := map[string]string{
		"buildbucket": fmt.Sprintf("https://cr-buildbucket.appspot.com/build/%v", *buildID),
	}
	for benchmark := range results {
		header := map[string]string{
			"benchmark":    benchmark,
			"git_revision": info.Revision,
		}
		f := ingest.ConvertPerfResultsFormat(results[benchmark], info.GetPosition(), header, links)

		fn := path.Join(*outputDir, fmt.Sprintf("%v_%v.json", benchmark, *buildID))
		out, err := os.Create(fn)
		if err != nil {
			sklog.Errorf("Failed to create the file for output (%v) %v.", fn, err)
			continue
		}

		b, err := json.Marshal(f)
		if err != nil {
			sklog.Errorf("Failed to marshal benchmark (%v) %v", benchmark, err)
			continue
		}

		_, err = out.Write(b)
		if err != nil {
			sklog.Errorf("Failed to write to the file (%v) %v.", fn, err)
			continue
		}

		// Print out the output files, so the downstream can read from it.
		fmt.Printf("%s\n", fn)
		sklog.Errorf("Finished writing %v.", fn)
	}
}
