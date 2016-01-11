// tracedbbuilder_test has unit tests for go/trace/db/builder.go that are Perf specific.
package tracedbbuilder

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/trace/service"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/types"
	"google.golang.org/grpc"
)

const (
	FILENAME = "/tmp/tracedb_test.db"
)

func cleanup() {
	if err := os.Remove(FILENAME); err != nil {
		fmt.Printf("Failed to clean up %s: %s", FILENAME, err)
	}
}

func TestNewTraceDBBuilder(t *testing.T) {
	defer cleanup()

	// First spin up a traceservice server that we wil talk to.
	server, err := traceservice.NewTraceServiceServer(FILENAME)
	if err != nil {
		t.Fatalf("Failed to initialize the tracestore server: %s", err)
	}

	// Start the server on an open port.
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	port := lis.Addr().String()
	s := grpc.NewServer()
	traceservice.RegisterTraceServiceServer(s, server)
	go func() {
		t.Fatalf("Failed while serving: %s", s.Serve(lis))
	}()

	// Get a Git repo.
	tr := util.NewTempRepo()
	defer tr.Cleanup()

	// Set up a connection to the server.
	conn, err := grpc.Dial(port, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("did not connect: %v", err)
	}
	ts, err := db.NewTraceServiceDB(conn, types.PerfTraceBuilder)
	if err != nil {
		t.Fatalf("Failed to create tracedb.DB: %s", err)
	}
	defer util.Close(ts)

	git, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "testrepo"), false, false)
	if err != nil {
		t.Fatal(err)
	}
	hashes := git.LastN(50)
	assert.Equal(t, 50, len(hashes))

	// Populate the tracedb with some data.
	commitID := &db.CommitID{
		Timestamp: git.Timestamp(hashes[1]).Unix(),
		ID:        hashes[1],
		Source:    "master",
	}

	entries := map[string]*db.Entry{
		"key:8888:android": &db.Entry{
			Params: map[string]string{
				"config":   "8888",
				"platform": "android",
				"type":     "skp",
			},
			Value: types.BytesFromFloat64(0.01),
		},
		"key:gpu:win8": &db.Entry{
			Params: map[string]string{
				"config":   "gpu",
				"platform": "win8",
				"type":     "skp",
			},
			Value: types.BytesFromFloat64(1.234),
		},
	}

	err = ts.Add(commitID, entries)
	if err != nil {
		t.Fatalf("Failed to add data to traceservice: %s", err)
	}

	evt := eventbus.New(nil)
	traceDB, err := db.NewTraceServiceDBFromAddress(port, types.PerfTraceBuilder)
	assert.Nil(t, err)

	builder, err := db.NewMasterTileBuilder(traceDB, git, 50, evt)
	if err != nil {
		t.Fatalf("Failed to construct TraceStore: %s", err)
	}
	tile := builder.GetTile()
	assert.Equal(t, 50, len(tile.Commits))
	assert.Equal(t, 2, len(tile.Traces))
	assert.Equal(t, commitID.ID, tile.Commits[1].Hash)
	assert.Equal(t, "Joe Gregorio (jcgregorio@google.com)", tile.Commits[1].Author)

	ptrace := tile.Traces["key:8888:android"].(*types.PerfTrace)
	assert.Equal(t, 0.01, ptrace.Values[1])
	assert.Equal(t, config.MISSING_DATA_SENTINEL, ptrace.Values[0])

	ptrace = tile.Traces["key:gpu:win8"].(*types.PerfTrace)
	assert.Equal(t, 1.234, ptrace.Values[1])
	assert.Equal(t, config.MISSING_DATA_SENTINEL, ptrace.Values[0])

	// Now add more data to the tracestore, trigger LoadTile() and check the results.
	commitID = &db.CommitID{
		Timestamp: git.Timestamp(hashes[2]).Unix(),
		ID:        hashes[2],
		Source:    "master",
	}

	entries = map[string]*db.Entry{
		"key:8888:android": &db.Entry{
			Params: map[string]string{
				"config":   "8888",
				"platform": "android",
				"type":     "skp",
			},
			Value: types.BytesFromFloat64(0.02),
		},
		"key:565:ubuntu": &db.Entry{
			Params: map[string]string{
				"config":   "565",
				"platform": "ubuntu",
				"type":     "skp",
			},
			Value: types.BytesFromFloat64(2.345),
		},
	}

	err = ts.Add(commitID, entries)
	if err != nil {
		t.Fatalf("Failed to add data to traceservice: %s", err)
	}

	// Access the LoadTile method without exposing it in the MasterBuilder interface.
	type hasLoadTile interface {
		LoadTile() error
	}
	masterBuilder := builder.(hasLoadTile)
	if err := masterBuilder.LoadTile(); err != nil {
		t.Fatalf("Failed to force load Tile: %s", err)
	}

	tile = builder.GetTile()
	assert.Equal(t, 50, len(tile.Commits))
	assert.Equal(t, 3, len(tile.Traces))
	assert.Equal(t, commitID.ID, tile.Commits[2].Hash)

	ptrace = tile.Traces["key:8888:android"].(*types.PerfTrace)
	assert.Equal(t, 0.01, ptrace.Values[1])
	assert.Equal(t, 0.02, ptrace.Values[2])
	assert.Equal(t, config.MISSING_DATA_SENTINEL, ptrace.Values[0])
	assert.Equal(t, config.MISSING_DATA_SENTINEL, ptrace.Values[3])

	ptrace = tile.Traces["key:gpu:win8"].(*types.PerfTrace)
	assert.Equal(t, 1.234, ptrace.Values[1])
	assert.Equal(t, config.MISSING_DATA_SENTINEL, ptrace.Values[0])

	ptrace = tile.Traces["key:565:ubuntu"].(*types.PerfTrace)
	assert.Equal(t, 2.345, ptrace.Values[2])
	assert.Equal(t, config.MISSING_DATA_SENTINEL, ptrace.Values[3])
}
