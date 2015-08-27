package main

// ingest is the command line tool for pulling performance data from Google
// Storage and putting in Tiles. See the code in go/ingester for details on how
// ingestion is done.

import (
	"flag"
	"net/http"
	"path/filepath"
	"time"

	storage "code.google.com/p/google-api-go-client/storage/v1"
	"github.com/skia-dev/glog"
	androidbuildinternal "go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	cconfig "go.skia.org/infra/go/config"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/ingester"
	"go.skia.org/infra/go/util"
	gconfig "go.skia.org/infra/golden/go/config"
	golddb "go.skia.org/infra/golden/go/db"
	"go.skia.org/infra/golden/go/goldingester"
	goldtrybot "go.skia.org/infra/golden/go/trybot"
	"go.skia.org/infra/perf/go/db"
	_ "go.skia.org/infra/perf/go/perfingester"
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
	ConstructorName string               // Named constructor for this ingester (defaults to the dataset name).
	DBHost          string               // Hostname of the MySQL database server used by this ingester.
	DBName          string               // Name of the MySQL database used by this ingester.
	DBPort          int                  // Port number of the MySQL database used by this ingester.
}

type IngestConfig struct {
	Common    cconfig.Common
	Ingesters map[string]*IngesterConfig
}

var config IngestConfig

// ProcessStarter wraps a function to start an ingester.
//
// A Process will return immediately and start the necessary goroutines.
type ProcessStarter func()

// NewIngestionProcess creates a Process for ingesting data.
func NewIngestionProcess(git *gitinfo.GitInfo, tileDir, datasetName string, ri ingester.ResultIngester, config map[string]string, every time.Duration, nCommits int, minDuration time.Duration, statusDir, metricName string) ProcessStarter {
	return func() {
		i, err := ingester.NewIngester(git, tileDir, datasetName, ri, nCommits, minDuration, config, statusDir, metricName)
		if err != nil {
			glog.Fatalf("Failed to create Ingester: %s", err)
		}

		glog.Infof("Starting %s ingester. Run every %s.", datasetName, every.String())

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
	defer common.LogPanic()
	// Setup DB flags.
	dbConf := db.DBConfigFromFlags()

	common.InitWithMetricsCB("ingest", func() string {
		common.DecodeTomlFile(*configFilename, &config)
		return config.Common.GraphiteServer
	})

	// Initialize the database. We might not need the oauth dialog if it fails.
	if !config.Common.Local {
		if err := dbConf.GetPasswordFromMetadata(); err != nil {
			glog.Fatal(err)
		}
	}
	if err := dbConf.InitDB(); err != nil {
		glog.Fatal(err)
	}

	// Get a backoff transport.
	transport := util.NewBackOffTransport()

	// Determine the oauth scopes we are going to use. Storage is used by all
	// ingesters.
	scopes := []string{storage.CloudPlatformScope}
	for _, ingesterConfig := range config.Ingesters {
		if ingesterConfig.ConstructorName == gconfig.CONSTRUCTOR_ANDROID_GOLD {
			scopes = append(scopes, androidbuildinternal.AndroidbuildInternalScope)
		}
	}

	// Initialize the oauth client that is used to access all scopes.
	var client *http.Client
	var err error
	if config.Common.Local {
		if config.Common.DoOAuth {
			client, err = auth.InstalledAppClient(config.Common.OAuthCacheFile,
				config.Common.OAuthClientSecretFile,
				transport, scopes...)
			if err != nil {
				glog.Fatalf("Failed to auth: %s", err)
			}
		} else {
			client = nil
			// Add back service account access here when it's fixed.
		}
	} else {
		// Assume we are on a GCE instance.
		client = auth.GCEServiceAccountClient(transport)
	}

	// Initialize the ingester and gold ingester.
	ingester.Init(client)
	if _, ok := config.Ingesters[gconfig.CONSTRUCTOR_GOLD]; ok {
		if err := goldingester.Init(client, filepath.Join(config.Ingesters["gold"].StatusDir, "android-build-info")); err != nil {
			glog.Fatalf("Unable to initialize GoldIngester: %s", err)
		}
	}

	// TODO(stephana): Cleanup the way trybots are instantiated so that each trybot has it's own
	// database connection and they are all instantiated the same way instead of the
	// one-off approaches.
	if ingesterConf, ok := config.Ingesters[gconfig.CONSTRUCTOR_GOLD_TRYBOT]; ok {
		dbConf := &database.DatabaseConfig{
			Host:           ingesterConf.DBHost,
			Port:           ingesterConf.DBPort,
			User:           database.USER_RW,
			Name:           ingesterConf.DBName,
			MigrationSteps: golddb.MigrationSteps(),
		}

		if !config.Common.Local {
			if err := dbConf.GetPasswordFromMetadata(); err != nil {
				glog.Fatal(err)
			}
		}
		vdb, err := dbConf.NewVersionedDB()
		if err != nil {
			glog.Fatalf("Unable to open db connection: %s", err)
		}
		goldtrybot.Init(vdb)
	}

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
		startProcess := NewIngestionProcess(git,
			config.Common.TileDir,
			dataset,
			resultIngester,
			ingesterConfig.ExtraParams,
			ingesterConfig.RunEvery.Duration,
			ingesterConfig.NCommits,
			minDuration,
			ingesterConfig.StatusDir,
			ingesterConfig.MetricName)
		startProcess()
	}

	select {}
}
