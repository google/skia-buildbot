package analysisserver

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	cpb "go.skia.org/infra/cabe/go/proto"
)

func startTestServer(t *testing.T) (cpb.AnalysisClient, func()) {
	serverListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := grpc.NewServer()
	cpb.RegisterAnalysisServer(server, New(nil))

	go func() {
		require.NoError(t, server.Serve(serverListener))
	}()

	clientConn, err := grpc.Dial(
		serverListener.Addr().String(),
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithTimeout(2*time.Second))

	require.NoError(t, err)

	closer := func() {
		// server.Stop will close the listener for us:
		// https://pkg.go.dev/google.golang.org/grpc#Server.Stop
		// Explicitly closing the listener causes the server.Serve
		// call to return an error, which causes this test to fail
		// even when the code under test behaves as expected.
		server.Stop()
	}

	client := cpb.NewAnalysisClient(clientConn)
	return client, closer
}

func TestAnalysisServiceServer_GetAnalysis(t *testing.T) {
	test := func(name string, request *cpb.GetAnalysisRequest, wantResponse *cpb.GetAnalysisResponse, wantError bool) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			client, closer := startTestServer(t)
			defer closer()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			r, err := client.GetAnalysis(ctx, request)

			if wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			diff := cmp.Diff(wantResponse, r, cmpopts.EquateEmpty(), cmpopts.IgnoreUnexported(cpb.GetAnalysisResponse{}))

			assert.Equal(t, diff, "", "diff should be empty")
		})
	}

	test("empty request", &cpb.GetAnalysisRequest{}, &cpb.GetAnalysisResponse{}, false)
}
