// gold_ingestion is the server process that runs an arbitrary number of
// ingesters and stores them in traceDB backends.

package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gevent"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/ingestion/fs_ingestionstore"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"

	// See https://golang.org/pkg/net/http/pprof/
	net_pprof "net/http/pprof"

	// The init() of this package register several ingestion.Processors to
	// handle the files we locate in GCS (e.g. master branch, tryjobs, etc).
	_ "go.skia.org/infra/golden/go/ingestion_processors"
)

const (
	// This subscription ID doesn't have to be unique instance by instance
	// because the unique topic id it is listening to will suffice.
	// By setting the subscriber ID to be the same on all instances of the ingester,
	// only one of the ingesters will get each event (usually).
	subscriptionID = "gold-ingestion"
)

func main() {
	// Command line flags.
	var (
		configFilename  = flag.String("config_filename", "default.json5", "Configuration file in JSON5 format.")
		fsNamespace     = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'flutter', 'skia', etc")
		fsProjectID     = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
		gitBTInstanceID = flag.String("git_bt_instance", "", "ID of the BigTable instance that contains Git metadata")
		gitBTTableID    = flag.String("git_bt_table", "", "ID of the BigTable table that contains Git metadata")
		hang            = flag.Bool("hang", false, "If true, just hang and do nothing.")
		httpPort        = flag.String("http_port", ":9091", "The http port where ready-ness endpoints are served.")
		local           = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
		memProfile      = flag.Duration("memprofile", 0, "Duration for which to profile memory. After this duration the program writes the memory profile and exits.")
		noCloudLog      = flag.Bool("no_cloud_log", false, "Disables cloud logging. Primarily for running locally.")
		projectID       = flag.String("project_id", common.PROJECT_ID, "GCP project ID.")
		promPort        = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
		debugPProf      = flag.Bool("debug_pprof", false, "turn pprof things on http_port if true")
	)

	// Parse the options. So we can configure logging.
	flag.Parse()

	if *hang {
		sklog.Infof("--hang provided; doing nothing.")
		httputils.RunHealthCheckServer(*httpPort)
	}

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
	tokenSrc, err := auth.NewDefaultTokenSource(*local, storage.ScopeFullControl, pubsub.ScopePubSub, pubsub.ScopeCloudPlatform)
	if err != nil {
		sklog.Fatalf("Failed to auth: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(tokenSrc).With2xxOnly().WithDialTimeout(time.Second * 10).Client()

	if *fsNamespace == "" || *fsProjectID == "" {
		sklog.Fatalf("You must specify --fs_namespace and --fs_project_id")
	}
	// Auth note: the underlying firestore.NewClient looks at the
	// GOOGLE_APPLICATION_CREDENTIALS env variable, so we don't need to supply
	// a token source.
	fsClient, err := firestore.NewClient(context.Background(), *fsProjectID, "gold", *fsNamespace, nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}
	ingestionStore := fs_ingestionstore.New(fsClient)

	// Start the ingesters.
	config, err := sharedconfig.ConfigFromJson5File(*configFilename)
	if err != nil {
		sklog.Fatalf("Unable to read config file %s. Got error: %s", *configFilename, err)
	}

	// Set up the eventbus.
	var eventBus eventbus.EventBus
	if config.EventTopic != "" {
		sID := subscriptionID
		if *local {
			// This allows us to have an independent ingester
			// when running locally.
			sID += "-local"
		}
		eventBus, err = gevent.New(*projectID, config.EventTopic, sID, option.WithTokenSource(tokenSrc))
		if err != nil {
			sklog.Fatalf("Error creating global eventbus: %s", err)
		}
		sklog.Infof("Global eventbus for topic '%s' and subscriber '%s' created. %v", config.EventTopic, sID, eventBus == nil)
	} else {
		eventBus = eventbus.New()
	}

	// Set up the gitstore if we have the necessary bigtable configuration.
	var btConf *bt_gitstore.BTConfig = nil
	if *gitBTInstanceID != "" && *gitBTTableID != "" {
		btConf = &bt_gitstore.BTConfig{
			ProjectID:  *projectID,
			InstanceID: *gitBTInstanceID,
			TableID:    *gitBTTableID,
		}
	}

	// Set up the ingesters in the background.
	var ingesters []*ingestion.Ingester
	go func() {
		var err error
		ingesters, err = ingestion.IngestersFromConfig(ctx, config, client, eventBus, ingestionStore, btConf)
		if err != nil {
			sklog.Fatalf("Unable to instantiate ingesters: %s", err)
		}
		for _, oneIngester := range ingesters {
			if err := oneIngester.Start(ctx); err != nil {
				sklog.Fatalf("Unable to start ingester: %s", err)
			}
		}
	}()

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

	// Set up the http handler to indicate ready-ness and start serving.
	http.HandleFunc("/healthz", httputils.ReadyHandleFunc)

	if *debugPProf {
		http.HandleFunc("/debug/pprof/", net_pprof.Index)
	}

	log.Fatal(http.ListenAndServe(*httpPort, nil))
}
