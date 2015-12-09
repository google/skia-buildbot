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
	"go.skia.org/infra/go/grpclog"
	"go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
	ptypes "go.skia.org/infra/perf/go/types"
	"google.golang.org/grpc"
)

// flags
var (
	address    = flag.String("address", "localhost:9090", "The address of the traceserver gRPC endpoint.")
	cpuprofile = flag.String("cpuprofile", "", "Write cpu profile to file.")
	dataset    = flag.String("dataset", "gold", "The name of the dataset in the file tile store.")
	gold       = flag.Bool("gold", true, "The type of data in the tile, true for gold, false for perf.")
)

func _main(ts db.DB) {
	week := time.Hour * 24 * 7
	commits, err := ts.List(time.Now().Add(-week), time.Now())
	if err != nil {
		glog.Errorf("Failed to load commits: %s", err)
		return
	}
	if len(commits) > 50 {
		commits = commits[:50]
	}

	begin := time.Now()
	_, err = ts.TileFromCommits(commits)
	if err != nil {
		glog.Errorf("Failed to load Tile: %s", err)
		return
	}
	glog.Infof("Time to load tile: %v", time.Now().Sub(begin))
	// Now load a second time.
	begin = time.Now()
	_, err = ts.TileFromCommits(commits)
	if err != nil {
		glog.Errorf("Failed to load Tile: %s", err)
		return
	}
	glog.Infof("Time to load tile the second time: %v", time.Now().Sub(begin))
}

func main() {
	common.Init()
	grpclog.Init()

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
		_main(ts)
	} else {
		_main(ts)
	}
}
