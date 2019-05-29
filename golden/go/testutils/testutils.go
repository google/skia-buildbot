package testutils

import (
	"net"
	"sync"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sktest"
	traceservice "go.skia.org/infra/go/trace/service"
	"google.golang.org/grpc"
)

// StartTestTraceDBServer starts up a traceDB server for testing. It stores its
// data at the given path and returns the address at which the server is
// listening as the second return value.
// Upon completion the calling test should call the Stop() function of the
// returned server object.
func StartTraceDBTestServer(t sktest.TestingT, traceDBFileName, shareDBDir string) (*grpc.Server, string) {
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

// These helpers assist in wrapping the Run calls in the wait group
// so we can be sure everything actually runs before the function
// terminates. Ideally, I would have liked to be able to chain multiple
// Run calls to the mock, but testify's mocks only allow one
// Run per Call. We have two helpers then, one if a mock does not already
// have a Run function and the other is for wrapping around a Run function
// that already exists.
// Note: do not call defer wg.Wait() because if any assert fails, it will
// panic, possibly before all the wg.Done() are called, causing a deadlock.
func AsyncHelpers() (*sync.WaitGroup, func(c *mock.Call) *mock.Call, func(f func(args mock.Arguments)) func(mock.Arguments)) {
	wg := sync.WaitGroup{}
	isAsync := func(c *mock.Call) *mock.Call {
		wg.Add(1)
		return c.Run(func(a mock.Arguments) {
			wg.Done()
		})
	}
	asyncWrapper := func(f func(args mock.Arguments)) func(mock.Arguments) {
		wg.Add(1)
		return func(args mock.Arguments) {
			defer wg.Done()
			f(args)
		}
	}
	return &wg, isAsync, asyncWrapper
}
