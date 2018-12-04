package db

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"go.skia.org/infra/go/trace/db/perftypes"
	traceservice "go.skia.org/infra/go/trace/service"
	"go.skia.org/infra/go/util"
	"google.golang.org/grpc"
)

const (
	TMP_PREFIX = "tracedb_test"
)

type fatalf func(format string, args ...interface{})
type cleanup func()

// setupClientServerForTesting is a utility func used in tests.
//
// It starts a server running in a Go routine and then connects
// a client to that server, which is returned as DB. In addition it
// returns a func that should be called to clean up both the client
// and the server after they are done being used.
func setupClientServerForTesting(f fatalf) (DB, cleanup) {
	file, err := ioutil.TempFile("", TMP_PREFIX)
	if err != nil {
		f("Could not create temporary file: %s", err)
	}
	// First spin up a traceservice server that we will talk to.
	server, err := traceservice.NewTraceServiceServer(file.Name())
	if err != nil {
		f("Failed to initialize the tracestore server: %s", err)
	}

	// Start the server on an open port.
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		f("failed to listen: %v", err)
	}
	port := lis.Addr().String()
	s := grpc.NewServer()
	traceservice.RegisterTraceServiceServer(s, server)
	go func() {
		f("Failed while serving: %s", s.Serve(lis))
	}()

	// Set up a connection to the server.
	conn, err := grpc.Dial(port, grpc.WithInsecure(), grpc.WithMaxMsgSize(MAX_MESSAGE_SIZE))
	if err != nil {
		f("did not connect: %v", err)
	}
	ts, err := NewTraceServiceDB(conn, perftypes.PerfTraceBuilder)
	if err != nil {
		f("Failed to create tracedb.DB: %s", err)
	}
	cl := func() {
		util.Close(conn)
		util.Close(ts)
		if err := os.Remove(file.Name()); err != nil {
			fmt.Printf("Failed to clean up %s: %s", file.Name(), err)
		}
	}
	return ts, cl
}
