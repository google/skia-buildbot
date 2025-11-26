package main

import (
	"flag"
	"fmt"
	"os"

	"go.skia.org/infra/perf/go/fuchsia_to_skia_perf/convert"
)

var (
	inputFile = flag.String("input", "", "Path to the input Fuchsia JSON file.")
	outputDir = flag.String("output_dir", "", "Path to the output directory for Skia Perf JSON files.")
	master    = flag.String("master", "", "The master name to use in the output key.")
)

func main() {
	flag.Parse()

	if *inputFile == "" {
		fmt.Fprintln(os.Stderr, "Error: -input flag is required")
		flag.Usage()
		os.Exit(1)
	}

	if *outputDir == "" {
		fmt.Fprintln(os.Stderr, "Error: -output_dir flag is required")
		flag.Usage()
		os.Exit(1)
	}

	if *master == "" {
		fmt.Fprintln(os.Stderr, "Error: -master flag is required")
		flag.Usage()
		os.Exit(1)
	}

	cfg := convert.Config{
		InputFile: *inputFile,
		OutputDir: *outputDir,
		Master:    *master,
	}

	if err := convert.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
