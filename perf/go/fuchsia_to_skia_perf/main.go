package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/perf/go/fuchsia_to_skia_perf/convert"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

var (
	inputFile = flag.String("input", "", "Path to the input Fuchsia JSON file.")
	outputDir = flag.String("output_dir", "", "Path to the output directory for Skia Perf JSON files.")
	master    = flag.String("master", "", "The master name to use in the output key.")
	gcsBucket = flag.String("gcs_bucket", "", "Optional. GCS bucket to upload results to.")
	date      = flag.String("date", "", "Optional. Date in YYYY-MM-DD format to use for the GCS path prefix (e.g., ingest/YYYY/MM/DD).")
)

func main() {
	flag.Parse()

	if *inputFile == "" {
		fmt.Fprintln(os.Stderr, "Error: -input flag is required")
		flag.Usage()
		os.Exit(1)
	}

	if *outputDir == "" && *gcsBucket == "" {
		fmt.Fprintln(os.Stderr, "Error: At least one of -output_dir or -gcs_bucket must be provided")
		flag.Usage()
		os.Exit(1)
	}

	if *master == "" {
		fmt.Fprintln(os.Stderr, "Error: -master flag is required")
		flag.Usage()
		os.Exit(1)
	}

	if *date != "" {
		_, err := time.Parse("2006-01-02", *date)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid date format for --date: %v. Please use YYYY-MM-DD.\n", err)
			os.Exit(1)
		}
	}

	cfg := convert.Config{
		InputFile: *inputFile,
		OutputDir: *outputDir,
		Master:    *master,
		GCSBucket: *gcsBucket,
		Date:      *date,
	}

	if *gcsBucket != "" {
		ctx := context.Background()
		ts, err := google.DefaultTokenSource(ctx, storage.ScopeReadWrite)
		if err != nil {
			fmt.Printf("Error: Failed to get TokenSource for GCS: %s\n", err)
			os.Exit(1)
		}
		storageClient, err := storage.NewClient(ctx, option.WithTokenSource(ts))
		if err != nil {
			fmt.Printf("Error: Failed to authenticate to cloud storage: %s\n", err)
			os.Exit(1)
		}
		cfg.GCSClient = storageClient
		fmt.Printf("GCS Upload enabled to bucket %s\n", *gcsBucket)
	}

	if err := convert.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
