package main

// skia_ingestion is the server process that runs an arbitary number of
// ingesters and stores them in traceDB backends.

import (
	"context"
	"crypto/md5"
	"flag"
	"io"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/config"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	_ "go.skia.org/infra/golden/go/goldingestion"
)

// Command line flags.
var (
	inputDir = flag.String("input_dir", "", "Input directory to poll for intestion")
	nDays    = flag.Int("n_days", 10, "Duration to ingest.")
)

func main() {
	common.Init()

	inputSource, err := ingestion.NewFileSystemSource("local-dir", *inputDir)
	if err != nil {
		sklog.Fatalf("Failed to open input dir %s. Got error: %s", *inputDir, err)
	}
	ingesterConf := sharedconfig.IngesterConfig{
		RunEvery: config.Duration{
			Duration: 5 * time.Minute,
		},
		MinDays: *nDays,
	}

	sources := []ingestion.Source{inputSource}
	processor := &BenchProcessor{}

	ingester, err := ingestion.NewIngester("bench-ingester", &ingesterConf, nil, sources, processor, nil, nil)
	if err != nil {
		sklog.Fatalf("Unable to create ingester: %s", err)
	}

	if err := ingester.Start(context.TODO()); err != nil {
		sklog.Fatalf("Error starting ingester: %s", err)
	}

	// Run the ingester forever.
	select {}
}

type BenchProcessor struct{}

func (b BenchProcessor) Process(ctx context.Context, resultsFile ingestion.ResultFileLocation) error {
	r, err := resultsFile.Open()
	if err != nil {
		return err
	}
	defer util.Close(r)

	md5Hash := md5.New()
	bytesWritten, err := io.Copy(md5Hash, r)
	if err != nil {
		return err
	}

	sklog.Infof("Processed %s (%x, %d)", resultsFile.Name(), md5Hash.Sum(nil), bytesWritten)
	return nil
}
