package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	inputFile  = flag.String("input", "", "Path to the input Fuchsia JSON file.")
	outputFile = flag.String("output", "", "Path to the output Skia Perf JSON file.")
)

func main() {
	flag.Parse()

	if *inputFile == "" {
		fmt.Fprintln(os.Stderr, "Error: -input flag is required")
		flag.Usage()
		os.Exit(1)
	}

	if *outputFile == "" {
		fmt.Fprintln(os.Stderr, "Error: -output flag is required")
		flag.Usage()
		os.Exit(1)
	}

	cfg := Config{
		InputFile:  *inputFile,
		OutputFile: *outputFile,
	}

	if err := Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
