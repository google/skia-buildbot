package main

import (
	"flag"
	"log"
	"time"

	"github.com/davecgh/go-spew/spew"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/filetilestore"
	"go.skia.org/infra/go/grpclog"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	gtypes "go.skia.org/infra/golden/go/types"
	"google.golang.org/grpc"
)

// flags
var (
	address   = flag.String("address", "localhost:9090", "The address of the traceserver gRPC endpoint.")
	tilestore = flag.String("tilestore", "/usr/local/google/home/jcgregorio/projects/tiles/tileStore3/", "The directory of the file tile store.")
)

func diff(tile *tiling.Tile, ts db.DB) error {
	commits := tile.Commits
	startTime := time.Unix(commits[0].CommitTime, 0)
	commitIDs, err := ts.List(startTime, time.Now())
	if err != nil {
		return err
	}

	sklog.Infof("COMMIT ids:\n\n\n %s\n\n\n", spew.Sdump(commitIDs))
	sklog.Infof("LOADING tile")

	traceDBTile, _, err := ts.TileFromCommits(commitIDs)
	if err != nil {
		return err
	}

	minLen := util.MinInt(len(commits), len(traceDBTile.Commits))
	tdbTraces := traceDBTile.Traces

	sklog.Infof("Commits/traces in tilestore:  %d   -   %d", len(commits), len(tile.Traces))
	sklog.Infof("Commits/traces in tracedb  :  %d   -   %d", len(traceDBTile.Commits), len(tdbTraces))

	count := 0
	matchingCount := 0
	for traceID, trace := range tile.Traces {
		_, ok := tdbTraces[traceID]
		if !ok {
			sklog.Fatalf("Trace missing: %s", traceID)
		}

		v1 := trace.(*gtypes.GoldenTrace).Digests[:minLen]
		v2 := tdbTraces[traceID].(*gtypes.GoldenTrace).Digests[:minLen]
		identicalCount := 0
		indices := make([]int, 0, minLen)
		for idx, val := range v1 {
			if val == v2[idx] {
				identicalCount++
			} else {
				indices = append(indices, idx)
			}

		}
		if identicalCount != minLen {
			sklog.Infof("Trace differs by %d / %d / %.2f,  %v", identicalCount, minLen, float64(identicalCount)/float64(minLen), indices)
		} else {
			matchingCount++
		}

		count++
	}
	sklog.Infof("Compared %d traces. Matching: %d", count, matchingCount)

	return nil
}

func main() {
	common.Init()
	grpclog.Init()

	// Load the 0,-1 tile.
	fileTilestore := filetilestore.NewFileTileStore(*tilestore, "gold", time.Hour)
	tile, err := fileTilestore.Get(0, -1)
	if err != nil {
		sklog.Fatalf("Failed to load tile: %s", err)
	}

	// Trim to the last 50 commits.
	begin := 0
	end := tile.LastCommitIndex()
	if end >= 49 {
		begin = end - 49
	}
	sklog.Infof("Loaded Tile")
	tile, err = tile.Trim(begin, end)

	// Set up a connection to the server.
	conn, err := grpc.Dial(*address, grpc.WithInsecure())
	if err != nil {
		sklog.Fatalf("did not connect: %v", err)
	}
	defer util.Close(conn)

	builder := gtypes.GoldenTraceBuilder

	sklog.Infof("START load tracedb.")
	ts, err := db.NewTraceServiceDB(conn, builder)
	if err != nil {
		log.Fatalf("Failed to create db.DB: %s", err)
	}
	sklog.Infof("DONE load tracedb.")
	if err = diff(tile, ts); err != nil {
		sklog.Fatalf("Diff error: %s", err)
	}
}
