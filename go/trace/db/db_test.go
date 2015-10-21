package db

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/trace/service"
	"go.skia.org/infra/go/util"
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

func TestAdd(t *testing.T) {
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

	// Set up a connection to the server.
	conn, err := grpc.Dial(port, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("did not connect: %v", err)
	}
	defer util.Close(conn)
	ts, err := NewTraceServiceDB(conn, types.PerfTraceBuilder)
	if err != nil {
		t.Fatalf("Failed to create tracedb.DB: %s", err)
	}
	defer util.Close(ts)

	now := time.Now()

	commitIDs := []*CommitID{
		&CommitID{
			Timestamp: now,
			ID:        "abc123",
			Source:    "master",
		},
		&CommitID{
			Timestamp: now.Add(time.Minute),
			ID:        "xyz789",
			Source:    "master",
		},
	}

	entries := map[string]*Entry{
		"key:8888:android": &Entry{
			Params: map[string]string{
				"config":   "8888",
				"platform": "android",
				"type":     "skp",
			},
			Value: types.BytesFromFloat64(0.01),
		},
		"key:gpu:win8": &Entry{
			Params: map[string]string{
				"config":   "gpu",
				"platform": "win8",
				"type":     "skp",
			},
			Value: types.BytesFromFloat64(1.234),
		},
	}

	err = ts.Add(commitIDs[0], entries)

	assert.NoError(t, err)
	tile, err := ts.TileFromCommits(commitIDs)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(tile.Traces))
	assert.Equal(t, 2, len(tile.Commits))

	tr := tile.Traces["key:8888:android"].(*types.PerfTrace)
	assert.Equal(t, 0.01, tr.Values[0])
	assert.True(t, tr.IsMissing(1))
	assert.Equal(t, "8888", tr.Params()["config"])

	tr = tile.Traces["key:gpu:win8"].(*types.PerfTrace)
	assert.Equal(t, 1.234, tr.Values[0])
	assert.True(t, tr.IsMissing(1))

	assert.Equal(t, "abc123", tile.Commits[0].Hash)
	assert.Equal(t, "xyz789", tile.Commits[1].Hash)

	foundCommits, err := ts.List(now, now.Add(time.Hour))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(foundCommits))
}
