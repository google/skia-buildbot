package main

// skia_ingestion is the server process that runs an arbitary number of
// ingesters and stores them in traceDB backends.

import (
	"flag"
	"net/http"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	_ "go.skia.org/infra/perf/go/perfingestion"
	storage "google.golang.org/api/storage/v1"
)

// Command line flags.
var (
	doOauth          = flag.Bool("oauth", true, "Run through the OAuth 2.0 flow on startup, otherwise use a GCE service account.")
	graphiteServer   = flag.String("graphite_server", "skia-monitoring:2003", "Where is Graphite metrics ingestion server running.")
	local            = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	oauthCacheFile   = flag.String("oauth_cache_file", "/home/perf/google_storage_token.data", "Path to the file where to cache cache the oauth credentials.")
	configFilename   = flag.String("config_filename", "default.toml", "Configuration file in TOML format.")
	clientSecretFile = flag.String("client_secrets", "client_secret.json", "The file name for the client_secret.json file.")
)

func main() {
	defer common.LogPanic()
	common.InitWithMetrics("skia-ingestion", graphiteServer)

	// Initialize oauth client and start the ingesters.
	var client *http.Client = nil
	if *doOauth || *local {
		var err error
		client, err = auth.NewClientWithTransport(*local, *oauthCacheFile, *clientSecretFile, nil, storage.CloudPlatformScope)
		if err != nil {
			glog.Fatalf("Failed to auth: %s", err)
		}
	}

	// Start the ingesters.
	config, err := sharedconfig.ConfigFromTomlFile(*configFilename)
	if err != nil {
		glog.Fatalf("Unable to read config file %s. Got error: %s", *configFilename, err)
	}

	ingesters, err := ingestion.IngestersFromConfig(config, client)
	if err != nil {
		glog.Fatalf("Unable to instantiate ingesters: %s", err)
	}
	for _, oneIngester := range ingesters {
		oneIngester.Start()
	}

	// Run the ingesters forever.
	select {}
}
