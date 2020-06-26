// gold_ingestion is the server process that runs an arbitrary number of
// ingesters and stores them in traceDB backends.

package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	// See https://golang.org/pkg/net/http/pprof/
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/vcsinfo/bt_vcs"
	"go.skia.org/infra/golden/go/eventbus"
	"go.skia.org/infra/golden/go/gevent"
	"go.skia.org/infra/golden/go/ingestion"
	"go.skia.org/infra/golden/go/ingestion/fs_ingestionstore"
	"google.golang.org/api/option"

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
		btInstanceID    = flag.String("bt_instance", "", "ID of the BigTable instance that contains Git metadata")
		btProjectID     = flag.String("bt_project_id", common.PROJECT_ID, "GCP project ID that houses the BigTable Instance")
		configFilename  = flag.String("config_filename", "default.json5", "Configuration file in JSON5 format.")
		fsNamespace     = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'flutter', 'skia', etc")
		fsProjectID     = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
		gitBTTableID    = flag.String("git_bt_table", "", "ID of the BigTable table that contains Git metadata")
		hang            = flag.Bool("hang", false, "If true, just hang and do nothing.")
		httpPort        = flag.String("http_port", ":9091", "The http port where ready-ness endpoints are served.")
		local           = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
		promPort        = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
		pubsubProjectID = flag.String("pubsub_project_id", "", "Project ID that houses the pubsub topics (e.g. for ingestion).")
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

	common.InitWithMust(appName, logOpts...)

	ctx := context.Background()

	// Initialize oauth client and start the ingesters.
	tokenSrc, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_USERINFO_EMAIL, storage.ScopeFullControl, pubsub.ScopePubSub, pubsub.ScopeCloudPlatform, swarming.AUTH_SCOPE, auth.SCOPE_GERRIT)
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
	config, err := ingestion.ConfigFromJson5File(*configFilename)
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
		eventBus, err = gevent.New(*pubsubProjectID, config.EventTopic, sID, option.WithTokenSource(tokenSrc))
		if err != nil {
			sklog.Fatalf("Error creating global eventbus: %s", err)
		}
		sklog.Infof("Global eventbus for topic '%s' and subscriber '%s' created. %v", config.EventTopic, sID, eventBus == nil)
	} else {
		eventBus = eventbus.New()
	}

	// Set up the gitstore if we have the necessary bigtable configuration.
	if *btInstanceID == "" || *gitBTTableID == "" {
		sklog.Fatalf("Missing BigTable configuration")
	}

	btConf := &bt_gitstore.BTConfig{
		ProjectID:  *btProjectID,
		InstanceID: *btInstanceID,
		TableID:    *gitBTTableID,
		AppProfile: appName,
	}

	gitStore, err := bt_gitstore.New(ctx, btConf, config.GitRepoURL)
	if err != nil {
		sklog.Fatalf("could not instantiate gitstore for %s", config.GitRepoURL)
	}

	// Set up VCS instance to track master.
	gitilesRepo := gitiles.NewRepo(config.GitRepoURL, client)
	vcs, err := bt_vcs.New(ctx, gitStore, "master", gitilesRepo)
	if err != nil {
		sklog.Fatalf("could not instantiate BT VCS for %s", config.GitRepoURL)
	}

	sklog.Infof("Created vcs client based on BigTable.")

	// Instantiate the secondary repo if one was specified.
	// TODO(kjlubick): make this support bigtable git also.
	if config.SecondaryRepoURL != "" {
		// TODO(kjlubick) Check up tracestore_impl's isOnMaster to make sure it
		// works with what is put here.
		sklog.Fatalf("Not yet implemented to have a secondary repo url")
	}

	// Set up the ingesters in the background.
	var ingesters []*ingestion.Ingester
	go func() {
		var err error
		ingesters, err = ingestion.IngestersFromConfig(ctx, config.Ingesters, client, eventBus, ingestionStore, vcs)
		if err != nil {
			sklog.Fatalf("Unable to instantiate ingesters: %s", err)
		}
		for _, oneIngester := range ingesters {
			if err := oneIngester.Start(ctx); err != nil {
				sklog.Fatalf("Unable to start ingester: %s", err)
			}
		}
	}()

	// Set up the http handler to indicate readiness and start serving.
	http.HandleFunc("/healthz", httputils.ReadyHandleFunc)

	log.Fatal(http.ListenAndServe(*httpPort, nil))
}
