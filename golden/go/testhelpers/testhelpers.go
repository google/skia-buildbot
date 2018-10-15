package testhelpers

import (
	"net"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/trace/service"
	"google.golang.org/grpc"
)

// StartTestTraceDBServer starts up a traceDB server for testing. It stores its
// data at the given path and returns the address at which the server is
// listening as the second return value.
// Upon completion the calling test should call the Stop() function of the
// returned server object.
func StartTraceDBTestServer(t assert.TestingT, traceDBFileName, shareDBDir string) (*grpc.Server, string) {
	traceDBServer, err := traceservice.NewTraceServiceServer(traceDBFileName)
	assert.NoError(t, err)

	lis, err := net.Listen("tcp", "localhost:0")
	assert.NoError(t, err)

	server := grpc.NewServer()
	traceservice.RegisterTraceServiceServer(server, traceDBServer)

	go func() {
		// We ignore the error, because calling the Stop() function always causes
		// an error and we are primarily interested in using this to test other code.
		_ = server.Serve(lis)
	}()

	return server, lis.Addr().String()
}
