// importtile allows importing a .gob based Tile into tracedb.
//
// It also has hooks for profiling.
package main

import (
	"flag"
	"log"
	"os"
	"runtime/pprof"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/filetilestore"
	"go.skia.org/infra/go/grpclog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
	ptypes "go.skia.org/infra/perf/go/types"
	"google.golang.org/grpc"
)

// flags
var (
	skip       = flag.Bool("skip", false, "Skip the Tile import.")
	address    = flag.String("address", "localhost:9090", "The address of the traceserver gRPC endpoint.")
	cpuprofile = flag.String("cpuprofile", "", "Write cpu profile to file.")
	tilestore  = flag.String("tilestore", "/usr/local/google/home/jcgregorio/projects/tiles/tileStore3/", "The directory of the file tile store.")
	dataset    = flag.String("dataset", "gold", "The name of the dataset in the file tile store.")
	gold       = flag.Bool("gold", true, "The type of data in the tile, true for gold, false for perf.")
)

func _main(tile *tiling.Tile, ts db.DB) {
	commits := []*db.CommitID{}
	for i, commit := range tile.Commits {
		cid := &db.CommitID{
			Timestamp: time.Unix(commit.CommitTime, 0),
			ID:        commit.Hash,
			Source:    "master",
		}
		commits = append(commits, cid)
		values := map[string]*db.Entry{}
		if !*skip {
			for traceid, tr := range tile.Traces {
				if !tr.IsMissing(i) {
					values[traceid] = &db.Entry{
						Value:  []byte(tr.(*types.GoldenTrace).Values[i]),
						Params: tr.Params(),
					}
				}
			}
			if err := ts.Add(cid, values); err != nil {
				glog.Errorf("Failed to add data: %s", err)
			}
		}
	}
	begin := time.Now()
	if len(commits) > 30 {
		commits = commits[:30]
	}
	_, err := ts.TileFromCommits(commits)
	if err != nil {
		glog.Fatalf("Failed to scan Tile: %s", err)
	}
	glog.Infof("Time to load tile: %v", time.Now().Sub(begin))
}

func main() {
	common.Init()
	grpclog.Init()

	// Load the 0,-1 tile.
	tilestore := filetilestore.NewFileTileStore(*tilestore, *dataset, time.Hour)
	tile, err := tilestore.Get(0, -1)
	if err != nil {
		glog.Fatalf("Failed to load tile: %s", err)
	}
	glog.Infof("Loaded Tile")

	// Set up a connection to the server.
	conn, err := grpc.Dial(*address, grpc.WithInsecure())
	if err != nil {
		glog.Fatalf("did not connect: %v", err)
	}
	defer util.Close(conn)

	// Build a TraceService client.
	builder := ptypes.PerfTraceBuilder
	if *gold {
		builder = types.GoldenTraceBuilder
	}
	ts, err := db.NewTraceServiceDB(conn, builder)
	if err != nil {
		log.Fatalf("Failed to create db.DB: %s", err)
	}
	glog.Infof("Opened tracedb")
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			glog.Fatalf("Failed to open profiling file: %s", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			glog.Fatalf("Failed to start profiling: %s", err)
		}
		defer pprof.StopCPUProfile()
		_main(tile, ts)
	} else {
		_main(tile, ts)
	}
}
