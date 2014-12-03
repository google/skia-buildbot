package main

// ingest is the command line tool for pulling performance data from Google
// Storage and putting in Tiles. See the code in go/ingester for details on how
// ingestion is done.

import (
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"skia.googlesource.com/buildbot.git/go/auth"
	"skia.googlesource.com/buildbot.git/go/common"
	"skia.googlesource.com/buildbot.git/go/gitinfo"
	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/db"
	"skia.googlesource.com/buildbot.git/perf/go/goldingester"
	"skia.googlesource.com/buildbot.git/perf/go/ingester"
	"skia.googlesource.com/buildbot.git/perf/go/trybot"
)

// flags
var (
	timestampFile  = flag.String("timestamp_file", "/tmp/timestamp.json", "File where timestamp data for ingester runs will be stored.")
	tileDir        = flag.String("tile_dir", "/tmp/tileStore2/", "Path where tiles will be placed.")
	gitRepoDir     = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
	runEvery       = flag.Duration("run_every", 5*time.Minute, "How often the ingester should pull data from Google Storage.")
	runTrybotEvery = flag.Duration("run_trybot_every", 1*time.Minute, "How often the ingester to pull trybot data from Google Storage.")
	run            = flag.String("run", "nano,nano-trybot,golden", "A comma separated list of ingesters to run.")
	graphiteServer = flag.String("graphite_server", "skia-monitoring:2003", "Where is Graphite metrics ingestion server running.")
	doOauth        = flag.Bool("oauth", true, "Run through the OAuth 2.0 flow on startup, otherwise use a GCE service account.")
	oauthCacheFile = flag.String("oauth_cache_file", "/home/perf/google_storage_token.data", "Path to the file where to cache cache the oauth credentials.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
)

// Timestamps is used to read and write the timestamp file, which records the time
// each ingestion last completed successfully.
//
// If an entry doesn't exist it returns BEGINNING_OF_TIME.
//
// Timestamp files look something like:
// {
//      "ingest":1445363563,
//      "trybot":1445363564,
//      "golden":1445363564,
// }
type Timestamps struct {
	Ingester map[string]int64 // Maps ingester name to its timestamp.

	filename string
	mutex    sync.Mutex
}

// NewTimestamp creates a new Timestamps that will read and write to the given
// filename.
func NewTimestamps(filename string) *Timestamps {
	return &Timestamps{
		Ingester: map[string]int64{
			"ingest": config.BEGINNING_OF_TIME.Unix(),
			"trybot": config.BEGINNING_OF_TIME.Unix(),
			"golden": config.BEGINNING_OF_TIME.Unix(),
		},
		filename: filename,
	}
}

// Read the timestamp data from the file.
func (t *Timestamps) Read() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	timestampFile, err := os.Open(t.filename)
	if err != nil {
		glog.Errorf("Error opening timestamp: %s", err)
		return
	}
	defer timestampFile.Close()
	err = json.NewDecoder(timestampFile).Decode(&t.Ingester)
	if err != nil {
		glog.Errorf("Failed to parse file %s: %s", t.filename, err)
	}
}

// Write the timestamp data to the file.
func (t *Timestamps) Write() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	writeTimestampFile, err := os.Create(t.filename)
	if err != nil {
		glog.Errorf("Write Timestamps: Failed to open file %s for writing: %s", t.filename, err)
		return
	}
	defer writeTimestampFile.Close()
	if err := json.NewEncoder(writeTimestampFile).Encode(t.Ingester); err != nil {
		glog.Errorf("Write Timestamps: Failed to encode timestamp file: %s", err)
	}
}

// Process is what each ingestion is wrapped up behind.
//
// A Process is expected to never return, and should be called as a Go routine.
type Process func()

// NewIngestionProcess creates a Process for ingesting data.
func NewIngestionProcess(ts *Timestamps, tsName string, git *gitinfo.GitInfo, tileDir, datasetName string, f ingester.IngestResultsFiles, gsDir string, every time.Duration, metricName string) Process {
	i, err := ingester.NewIngester(git, tileDir, datasetName, f, gsDir, metricName)
	if err != nil {
		glog.Fatalf("Failed to create Ingester: %s", err)
	}

	glog.Infof("Starting %s ingester. Run every %s. Fetch from %s ", tsName, every.String(), gsDir)

	// oneStep is a single round of ingestion.
	oneStep := func() {
		glog.Infof("Running ingester: %s", tsName)
		now := time.Now()
		err := i.Update(true, ts.Ingester[tsName])
		if err != nil {
			glog.Error(err)
		} else {
			ts.Ingester[tsName] = now.Unix()
			ts.Write()
		}
		glog.Infof("Finished running ingester: %s", tsName)
	}

	return func() {
		oneStep()
		for _ = range time.Tick(every) {
			oneStep()
		}
	}
}

func main() {
	common.InitWithMetrics("ingest", *graphiteServer)

	// Initialize the database. We might not need the oauth dialog if it fails.
	db.Init(db.ProdDatabaseConfig(*local))

	var client *http.Client
	var err error
	if *doOauth {
		config := auth.DefaultOAuthConfig(*oauthCacheFile)
		client, err = auth.RunFlow(config)
		if err != nil {
			glog.Fatalf("Failed to auth: %s", err)
		}
	} else {
		client = nil
		// Add back service account access here when it's fixed.
	}

	ingester.Init(client)
	ts := NewTimestamps(*timestampFile)
	ts.Read()
	glog.Infof("Timestamps: %#v\n", ts.Ingester)

	git, err := gitinfo.NewGitInfo(*gitRepoDir, true, false)
	if err != nil {
		glog.Fatal("Failed loading Git info: %s\n", err)
	}

	// ingesters is a list of all the types of ingestion we can do.
	ingesters := map[string]Process{
		"nano":        NewIngestionProcess(ts, "ingest", git, *tileDir, config.DATASET_NANO, ingester.NanoBenchIngestion, "nano-json-v1", *runEvery, "nano-ingest"),
		"nano-trybot": NewIngestionProcess(ts, "trybot", git, *tileDir, config.DATASET_NANO, trybot.TrybotIngestion, "trybot/nano-json-v1", *runTrybotEvery, "nano-trybot"),
		"golden":      NewIngestionProcess(ts, "golden", git, *tileDir, config.DATASET_GOLDEN, goldingester.GoldenIngester, "dm-json-v1", *runEvery, "golden-ingest"),
	}

	for _, name := range strings.Split(*run, ",") {
		glog.Infof("Process name: %s", name)
		if process, ok := ingesters[name]; ok {
			go process()
		} else {
			glog.Fatalf("Not a valid ingester name: %s", name)
		}
	}

	select {}
}
