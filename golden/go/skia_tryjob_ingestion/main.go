package main

// skia_ingestion is the server process that runs an arbitary number of
// ingesters and stores them in traceDB backends.

import (
	"context"
	"flag"
	"os"
	"path/filepath"

	"go.skia.org/infra/golden/go/goldingestion"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/config"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/sklog"
	_ "go.skia.org/infra/golden/go/goldingestion"
	_ "go.skia.org/infra/golden/go/pdfingestion"
	storage "google.golang.org/api/storage/v1"
)

var (
	// General command line flags.
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	noCloudLog         = flag.Bool("no_cloud_log", false, "Disables cloud logging. Primarily for running locally.")
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	serviceAccountFile = flag.String("service_account_file", "", "Credentials file for service account.")

	// Tryjob ingester related flags.
	ingesterID     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	pollInterval   = flag.Duration("interval", 0, "Duration for which to profile memory. After this duration the program writes the memory profile and exits.")
	timeWindowDays = flag.Int("time_window_days", 0, "Duration for which to profile memory. After this duration the program writes the memory profile and exits.")
	statusDir      = flag.String("status_dir", "", "Metrics service address (e.g., ':10110')")
	metricName     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	gsSource       = flag.String("gs_source_path", "", "Google storage source path.")
)

func main() {
	// Parse the options. So we can configure logging.
	flag.Parse()

	defer common.LogPanic()
	_, appName := filepath.Split(os.Args[0])

	// Set up the logging options.
	logOpts := []common.Opt{
		common.PrometheusOpt(promPort),
	}

	// Should we disable cloud logging.
	if !*noCloudLog {
		logOpts = append(logOpts, common.CloudLoggingOpt())
	}
	common.InitWithMust(appName, logOpts...)

	// Initialize oauth client and start the ingesters.
	client, err := auth.NewJWTServiceAccountClient("", *serviceAccountFile, nil, storage.CloudPlatformScope)
	if err != nil {
		sklog.Fatalf("Failed to auth: %s", err)
	}

	// Set up the ingesters.
	tjpConfig := &goldingestion.TryjobProcessorConfig{
		GerritURL          string
		CloudProjectID     string
		CDSNamespace       string
		ServiceAccountFile string
		BuildBucketURL     string
		BucketName         string

	}
	tryjobProcessor, err := goldingestion.NewGoldTryjobProcessor(tjpConfig)
	if err != nil {
		sklog.Fatalf("Unable to instantiate tryjob processor: %s", err)
	}

	// Get the bucket and path from the cloud storage path.
	srcBucket, srcDir := gcs.SplitGSPath(*gsSource)

	// Set up the ingesters.
	ctx := context.Background()
	ingesterConfig := &sharedconfig.IngesterConfig{
		RunEvery:   config.Duration{Duration: *pollInterval},
		MinDays:    *timeWindowDays,
		StatusDir:  *statusDir,
		MetricName: *metricName,
		Sources: []*sharedconfig.DataSource{
			&sharedconfig.DataSource{Bucket: srcBucket, Dir: srcDir},
		},
	}

	gsSrc, err := ingestion.NewGoogleStorageSource(*ingesterID, srcBucket, srcDir, client)
	if err != nil {
		sklog.Fatalf("Unable to instantiate GCS sourcd for '%s/%s': %s", srcBucket, srcDir, err)
	}

	ingester, err := ingestion.NewIngester(*ingesterID, ingesterConfig, nil, []ingestion.Source{gsSrc}, tryjobProcessor)
	if err != nil {
		sklog.Fatalf("Unable to instantiate ingester: %s", err)
	}

	// Start the ingester and run forever.
	ingester.Start(ctx)
	select {}
}
