package main

// ingest is the command line tool for pulling performance data from Google
// Storage and putting in Tiles. See the code in go/ingester for details on how
// ingestion is done.

import (
	"encoding/json"
	"flag"
	"net"
	"os"
	"time"

	"sync"

	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/ingester"
	"skia.googlesource.com/buildbot.git/perf/go/trybot"

	"github.com/golang/glog"
	"github.com/rcrowley/go-metrics"
)

// flags
var (
	timestampFile  = flag.String("timestamp_file", "/tmp/timestamp.json", "File where timestamp data for ingester runs will be stored.")
	tileDir        = flag.String("tile_dir", "/tmp/tileStore2/", "Path where tiles will be placed.")
	gitRepoDir     = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
	runEvery       = flag.Duration("run_every", 5*time.Minute, "How often the ingester should pull data from Google Storage.")
	runTrybotEvery = flag.Duration("run_trybot_every", 1*time.Minute, "How often the ingester to pull trybot data from Google Storage.")
	isSingleShot   = flag.Bool("single_shot", false, "Run the ingester only once.")
	runIngest      = flag.Bool("run_ingest", true, "Run the ingester for buildbot data.")
	runTrybot      = flag.Bool("run_trybot", true, "Run the ingester for trybot data.")
	graphiteServer = flag.String("graphite_server", "skia-monitoring-b:2003", "Where is Graphite metrics ingestion server running.")
)

func Init() {
	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, 1*time.Minute)
	addr, _ := net.ResolveTCPAddr("tcp", *graphiteServer)
	go metrics.Graphite(metrics.DefaultRegistry, 1*time.Minute, "ingest", addr)
}

// Timestamps is used to read and write the timestamp file, which records the time
// each ingestion last completed successfully.
//
// If an entry doesn't exist it returns BEGINNING_OF_TIME.
//
// Timestamp files look something like:
// {
//      "ingest":1445363563,
//      "trybot":1445363564,
// }
type Timestamps struct {
	Ingest int64 `json:"ingest"`
	Trybot int64 `json:"trybot"`

	filename string
	mutex    sync.Mutex
}

// NewTimestamp creates a new Timestamps that will read and write to the given
// filename.
func NewTimestamps(filename string) *Timestamps {
	return &Timestamps{
		Ingest:   config.BEGINNING_OF_TIME.Unix(),
		Trybot:   config.BEGINNING_OF_TIME.Unix(),
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
	err = json.NewDecoder(timestampFile).Decode(t)
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
	if err := json.NewEncoder(writeTimestampFile).Encode(t); err != nil {
		glog.Errorf("Write Timestamps: Failed to encode timestamp file: %s", err)
	}
}

func main() {
	flag.Parse()
	Init()
	ingester.Init()
	trybot.Init()
	ts := NewTimestamps(*timestampFile)
	ts.Read()

	ingestNanoBench, err := ingester.NewIngester(*gitRepoDir, *tileDir, true, ingester.NanoBenchIngestion, "nano-json-v1")
	if err != nil {
		glog.Fatalf("Failed to create Ingester: %s", err)
	}

	ingestTrybot, err := ingester.NewIngester(*gitRepoDir, *tileDir, true, trybot.TrybotIngestion, "trybot/nano-json-v1")
	if err != nil {
		glog.Fatalf("Failed to create Trybot Ingester: %s", err)
	}

	// TODO(jcgregorio) Add ingester.Register("name", fn, "directory_name") then make this a slice of Ingesters.

	oneNanoBench := func() {
		now := time.Now()
		err := ingestNanoBench.Update(true, ts.Ingest)
		if err != nil {
			glog.Error(err)
		} else {
			ts.Ingest = now.Unix()
			ts.Write()
		}
	}

	oneTrybot := func() {
		now := time.Now()
		err := ingestTrybot.Update(true, ts.Trybot)
		if err != nil {
			glog.Error(err)
		} else {
			ts.Trybot = now.Unix()
			ts.Write()
		}
	}

	if *runIngest {
		oneNanoBench()
		if !*isSingleShot {
			go func() {
				for _ = range time.Tick(*runEvery) {
					oneNanoBench()
				}
			}()
		}
	}

	if *runTrybot {
		oneTrybot()
		if !*isSingleShot {
			go func() {
				for _ = range time.Tick(*runTrybotEvery) {
					oneTrybot()
				}
			}()
		}
	}

	if !*isSingleShot {
		select {}
	}
}
