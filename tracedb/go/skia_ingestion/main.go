package main

// skia_ingestion is the server process that runs an arbitrary number of
// ingesters and stores them in traceDB backends.

import (
	"context"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"google.golang.org/api/option"
	storage "google.golang.org/api/storage/v1"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gevent"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	_ "go.skia.org/infra/golden/go/goldingestion"
	_ "go.skia.org/infra/golden/go/pdfingestion"
)

// Command line flags.
var (
	configFilename     = flag.String("config_filename", "default.json5", "Configuration file in JSON5 format.")
	dsNamespace        = flag.String("ds_namespace", "", "Cloud datastore namespace to be used by this instance.")
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	memProfile         = flag.Duration("memprofile", 0, "Duration for which to profile memory. After this duration the program writes the memory profile and exits.")
	noCloudLog         = flag.Bool("no_cloud_log", false, "Disables cloud logging. Primarily for running locally.")
	projectID          = flag.String("project_id", common.PROJECT_ID, "GCP project ID.")
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	serviceAccountFile = flag.String("service_account_file", "", "Credentials file for service account.")
)

func main() {
	// Parse the options. So we can configure logging.
	flag.Parse()

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

	ctx := context.Background()

	// Initialize oauth client and start the ingesters.
	client, err := auth.NewJWTServiceAccountClient("", *serviceAccountFile, nil, storage.CloudPlatformScope)
	if err != nil {
		sklog.Fatalf("Failed to auth: %s", err)
	}

	// Initialize the datastore client.
	tokenSrc, err := auth.NewJWTServiceAccountTokenSource("", *serviceAccountFile, storage.CloudPlatformScope)
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account to get token source: %s", err)
	}

	if err := ds.InitWithOpt(*projectID, *dsNamespace, option.WithTokenSource(tokenSrc)); err != nil {
		sklog.Fatalf("Unable to configure cloud datastore: %s", err)
	}

	// Start the ingesters.
	config, err := sharedconfig.ConfigFromJson5File(*configFilename)
	if err != nil {
		sklog.Fatalf("Unable to read config file %s. Got error: %s", *configFilename, err)
	}

	// Set up the eventbus.
	var eventBus eventbus.EventBus
	if config.EventTopic != "" {
		nodeName, err := gevent.GetNodeName(appName, *local)
		if err != nil {
			sklog.Fatalf("Error getting node name: %s", err)
		}
		eventBus, err = gevent.New(*projectID, config.EventTopic, nodeName, option.WithTokenSource(tokenSrc))
		if err != nil {
			sklog.Fatalf("Error creating global eventbus: %s", err)
		}
		sklog.Infof("Global eventbus for topic '%s' and subscriber '%s' created. %v", config.EventTopic, nodeName, eventBus == nil)
	} else {
		eventBus = eventbus.New()
	}

	ingesters, err := ingestion.IngestersFromConfig(ctx, config, client, eventBus)
	if err != nil {
		sklog.Fatalf("Unable to instantiate ingesters: %s", err)
	}
	for _, oneIngester := range ingesters {
		oneIngester.Start(ctx)
	}

	// Enable the memory profiler if memProfile was set.
	if *memProfile > 0 {
		writeProfileFn := func() {
			sklog.Infof("\nWriting Memory Profile")
			f, err := ioutil.TempFile("./", "memory-profile")
			if err != nil {
				sklog.Fatalf("Unable to create memory profile file: %s", err)
			}
			if err := pprof.WriteHeapProfile(f); err != nil {
				sklog.Fatalf("Unable to write memory profile file: %v", err)
			}
			util.Close(f)
			sklog.Infof("Memory profile written to %s", f.Name())

			os.Exit(0)
		}

		// Write the profile after the given time or whenever we get a SIGINT signal.
		time.AfterFunc(*memProfile, writeProfileFn)
		cleanup.AtExit(writeProfileFn)
	}

	// Run the ingesters forever.
	select {}
}
