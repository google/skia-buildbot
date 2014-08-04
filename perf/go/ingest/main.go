package main

// ingest is the command line tool for pulling performance data from Google
// Storage and putting in Tiles. See the code in go/ingester for details on how
// ingestion is done.

import (
	"flag"
	"net"
	"time"

	"skia.googlesource.com/buildbot.git/perf/go/ingester"

	"github.com/golang/glog"
	"github.com/rcrowley/go-metrics"
)

// flags
var (
	timestampFile = flag.String("timestamp_file", "/tmp/timestamp.json", "File where timestamp data for ingester runs will be stored.")
	tileDir       = flag.String("tile_dir", "/tmp/tileStore2/", "Path where tiles will be placed.")
	gitRepoDir    = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
	runEvery      = flag.Duration("run_every", 15*time.Minute, "How often the ingester to pull data from Google Storage.")
	isSingleShot  = flag.Bool("single_shot", false, "Run the ingester only once.")
)

func Init() {
	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, 1*time.Minute)
	addr, _ := net.ResolveTCPAddr("tcp", "skia-monitoring-b:2003")
	go metrics.Graphite(metrics.DefaultRegistry, 1*time.Minute, "ingest", addr)
}

func main() {
	flag.Parse()
	Init()
	ingester.Init()

	i, err := ingester.NewIngester(*gitRepoDir, *tileDir, true, *timestampFile)
	if err != nil {
		glog.Fatalf("Failed to create Ingester: %s", err)
	}

	i.Update(true)
	if *isSingleShot {
		return
	}

	for _ = range time.Tick(*runEvery) {
		i.Update(true)
	}
}
