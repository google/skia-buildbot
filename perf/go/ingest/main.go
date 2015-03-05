package main

// ingest is the command line tool for pulling performance data from Google
// Storage and putting in Tiles. See the code in go/ingester for details on how
// ingestion is done.

import (
	"flag"
	"net/http"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	cconfig "go.skia.org/infra/go/config"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/perf/go/db"
	_ "go.skia.org/infra/perf/go/goldingester"
	"go.skia.org/infra/perf/go/ingester"
	_ "go.skia.org/infra/perf/go/trybot"
)

var (
	configFilename = flag.String("config_filename", "default.toml", "Config filename.")
)

type IngesterConfig struct {
	RunEvery        cconfig.TomlDuration // How often the ingester should pull data from Google Storage.
	RunTrybotEvery  cconfig.TomlDuration // How often the ingester to pull trybot data from Google Storage.
	NCommits        int                  // Minimum number of commits that should be ingested.
	MinDays         int                  // Minimum number of days that should be covered by the ingested commits.
	StatusDir       string               // Path where the ingest process keeps its status between restarts.
	MetricName      string               // What to call this ingester's data when imported to Graphite
	ExtraParams     map[string]string    // Any additional needed parameters (ingester specific)
	ConstructorName string               // Named constructor for this ingester; must have been registered.
	//    If not provided, ConstructorName will default to the dataset name
}

type IngestConfig struct {
	Common    cconfig.Common
	Ingesters map[string]IngesterConfig
}

var config IngestConfig

// ProcessStarter wraps a function to start an ingester.
//
// A Process will return immediately and start the necessary goroutines.
type ProcessStarter func()

// NewIngestionProcess creates a Process for ingesting data.
func NewIngestionProcess(git *gitinfo.GitInfo, tileDir, datasetName string, ri ingester.ResultIngester, gsBucket, gsDir string, every time.Duration, nCommits int, minDuration time.Duration, statusDir, metricName string) ProcessStarter {
	return func() {
		i, err := ingester.NewIngester(git, tileDir, datasetName, ri, nCommits, minDuration, gsBucket, gsDir, statusDir, metricName)
		if err != nil {
			glog.Fatalf("Failed to create Ingester: %s", err)
		}

		glog.Infof("Starting %s ingester. Run every %s. Fetch from %s ", datasetName, every.String(), gsDir)

		// oneStep is a single round of ingestion.
		oneStep := func() {
			glog.Infof("Running ingester: %s", datasetName)
			err := i.Update()
			if err != nil {
				glog.Error(err)
			}
			glog.Infof("Finished running ingester: %s", datasetName)
		}

		// Start the ingester.
		go func() {
			oneStep()
			for _ = range time.Tick(every) {
				oneStep()
			}
		}()
	}
}

func main() {
	// Setup DB flags.
	database.SetupFlags(db.PROD_DB_HOST, db.PROD_DB_PORT, database.USER_RW, db.PROD_DB_NAME)

	common.InitWithMetricsCB("ingest", func() string {
		// Read toml config file.
		if _, err := toml.DecodeFile(*configFilename, &config); err != nil {
			glog.Fatalf("Failed to decode config file: %s", err)
		}

		return config.Common.GraphiteServer
	})

	// Initialize the database. We might not need the oauth dialog if it fails.
	conf, err := database.ConfigFromFlagsAndMetadata(config.Common.Local, db.MigrationSteps())
	if err != nil {
		glog.Fatal(err)
	}
	db.Init(conf)

	var client *http.Client
	if config.Common.DoOAuth {
		oauthConfig := auth.DefaultOAuthConfig(config.Common.OAuthCacheFile)
		client, err = auth.RunFlow(oauthConfig)
		if err != nil {
			glog.Fatalf("Failed to auth: %s", err)
		}
	} else {
		client = nil
		// Add back service account access here when it's fixed.
	}

	ingester.Init(client)

	git, err := gitinfo.NewGitInfo(config.Common.GitRepoDir, true, false)
	if err != nil {
		glog.Fatalf("Failed loading Git info: %s\n", err)
	}

	for dataset, ingesterConfig := range config.Ingesters {
		// Get duration equivalent to the number of days.
		minDuration := 24 * time.Hour * time.Duration(ingesterConfig.MinDays)

		constructorName := ingesterConfig.ConstructorName
		if constructorName == "" {
			constructorName = dataset
		}

		constructor := ingester.Constructor(constructorName)
		resultIngester := constructor()

		glog.Infof("Process name: %s", dataset)
		startProcess := NewIngestionProcess(git, config.Common.TileDir, dataset, resultIngester, ingesterConfig.ExtraParams["GSBucket"], ingesterConfig.ExtraParams["GSDir"], ingesterConfig.RunEvery.Duration, ingesterConfig.NCommits, minDuration, ingesterConfig.StatusDir, ingesterConfig.MetricName)
		startProcess()
	}

	select {}
}
