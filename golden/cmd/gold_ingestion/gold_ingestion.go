// gold_ingestion is the server process that runs an arbitrary number of
// ingesters and stores them to the appropriate backends.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/vcsinfo/bt_vcs"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/eventbus"
	"go.skia.org/infra/golden/go/gevent"
	"go.skia.org/infra/golden/go/ingestion"
	"go.skia.org/infra/golden/go/ingestion/fs_ingestionstore"
	"go.skia.org/infra/golden/go/ingestion_processors"
)

const (
	// This subscription ID doesn't have to be unique instance by instance
	// because the unique topic id it is listening to will suffice.
	// By setting the subscriber ID to be the same on all instances of the ingester,
	// only one of the ingesters will get each event (usually).
	subscriptionID = "gold-ingestion"
)

type ingestionServerConfig struct {
	config.Common

	// Configuration for one or more ingester (e.g. one for master branch and one for tryjobs).
	Ingesters map[string]ingestion.Config `json:"ingestion_configs"`

	// HTTP service address (e.g., ':9000')
	Port string `json:"port"`

	// Metrics service address (e.g., ':10110')
	PromPort string `json:"prom_port"`

	// PubsubEventTopic the event topic used for ingestion
	PubsubEventTopic string `json:"pubsub_event_topic" optional:"true"`

	// Project ID that houses the pubsub topics (e.g. for ingestion).
	PubsubProjectID string `json:"pubsub_project_id"`

	// TODO(kjlubick) Restore this functionality. Without it, we cannot ingest from internal jobs.
	// URL of the secondary repo that has GitRepoURL as a dependency.
	SecondaryRepoURL string `json:"secondary_repo_url" optional:"true"`
	// Regular expression to extract the commit hash from the DEPS file.
	SecondaryRepoRegEx string `json:"secondary_repo_regex" optional:"true"`
}

func main() {
	// Command line flags.
	var (
		commonInstanceConfig = flag.String("common_instance_config", "", "Path to the json5 file containing the configuration that needs to be the same across all services for a given instance.")
		thisConfig           = flag.String("config", "", "Path to the json5 file containing the configuration specific to baseline server.")
		hang                 = flag.Bool("hang", false, "Stop and do nothing after reading the flags. Good for debugging containers.")
	)

	// Parse the options. So we can configure logging.
	flag.Parse()

	if *hang {
		sklog.Info("Hanging")
		select {}
	}

	var isc ingestionServerConfig
	if err := config.LoadFromJSON5(&isc, commonInstanceConfig, thisConfig); err != nil {
		sklog.Fatalf("Reading config: %s", err)
	}
	sklog.Infof("Loaded config %#v", isc)

	_, appName := filepath.Split(os.Args[0])

	// Set up the logging options.
	logOpts := []common.Opt{
		common.PrometheusOpt(&isc.PromPort),
	}

	common.InitWithMust(appName, logOpts...)

	ingestion.Register(ingestion_processors.PrimaryBranchBigTable())
	ingestion.Register(ingestion_processors.ChangelistFirestore())

	ctx := context.Background()

	// Initialize oauth client and start the ingesters.
	tokenSrc, err := auth.NewDefaultTokenSource(isc.Local, auth.SCOPE_USERINFO_EMAIL, storage.ScopeFullControl, pubsub.ScopePubSub, pubsub.ScopeCloudPlatform, swarming.AUTH_SCOPE, auth.SCOPE_GERRIT)
	if err != nil {
		sklog.Fatalf("Failed to auth: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(tokenSrc).With2xxOnly().WithDialTimeout(time.Second * 10).Client()

	// Auth note: the underlying firestore.NewClient looks at the GOOGLE_APPLICATION_CREDENTIALS
	// env variable, so we don't need to supply a token source.
	fsClient, err := firestore.NewClient(context.Background(), isc.FirestoreProjectID, "gold", isc.FirestoreNamespace, nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}
	ingestionStore := fs_ingestionstore.New(fsClient)

	// Set up the eventbus.
	var eventBus eventbus.EventBus
	if isc.PubsubEventTopic != "" {
		sID := subscriptionID
		if isc.Local {
			// This allows us to have an independent ingester when running locally.
			sID += "-local"
		}
		eventBus, err = gevent.New(isc.PubsubProjectID, isc.PubsubEventTopic, sID, option.WithTokenSource(tokenSrc))
		if err != nil {
			sklog.Fatalf("Error creating global eventbus: %s", err)
		}
		sklog.Infof("Global eventbus for topic '%s' and subscriber '%s' created.", isc.PubsubEventTopic, sID)
	} else {
		eventBus = eventbus.New()
	}

	// Set up the gitstore
	btConf := &bt_gitstore.BTConfig{
		InstanceID: isc.BTInstance,
		ProjectID:  isc.BTProjectID,
		TableID:    isc.GitBTTable,
		AppProfile: appName,
	}

	gitStore, err := bt_gitstore.New(ctx, btConf, isc.GitRepoURL)
	if err != nil {
		sklog.Fatalf("could not instantiate gitstore for %s: %s", isc.GitRepoURL, err)
	}

	// Set up VCS instance to track master.
	gitilesRepo := gitiles.NewRepo(isc.GitRepoURL, client)
	vcs, err := bt_vcs.New(ctx, gitStore, isc.GitRepoBranch, gitilesRepo)
	if err != nil {
		sklog.Fatalf("could not instantiate BT VCS for %s", isc.GitRepoURL)
	}

	sklog.Infof("Created vcs client based on BigTable.")

	// Instantiate the secondary repo if one was specified.
	// TODO(kjlubick): make this support bigtable git also. skbug.com/9553
	if isc.SecondaryRepoURL != "" {
		// TODO(kjlubick) Check up tracestore_impl's isOnMaster to make sure it works with what is
		//  put here.
		sklog.Fatalf("Not yet implemented to have a secondary repo url")
	}
	// Set up the ingesters in the background.
	var ingesters []*ingestion.Ingester
	go func() {
		var err error
		ingesters, err = ingestion.IngestersFromConfig(ctx, isc.Ingesters, client, eventBus, ingestionStore, vcs)
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

	log.Fatal(http.ListenAndServe(isc.Port, nil))
}
