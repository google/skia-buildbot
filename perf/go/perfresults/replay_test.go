package perfresults

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"flag"
	"net/http"
	"os"
	"path"
	"strings"
	"testing"

	"cloud.google.com/go/httpreplay"
	"cloud.google.com/go/rpcreplay"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2/google"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
)

var recordPath = flag.String("record_path", "", "The path the replayer writes real backend responses.")

func setupReplay(t *testing.T, replayName string) *http.Client {
	// if the recordPath is not given, then we will replay from the testdata;
	// otherwise we will record the traffic and save to the given path;
	if *recordPath == "" {
		replayFile := path.Join("testdata", replayName)
		hr, err := httpreplay.NewReplayer(replayFile)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		c, err := hr.Client(ctx)
		require.NoError(t, err)
		t.Cleanup(func() {
			hr.Close()
			cancel()
		})
		return c
	}

	replayFile := path.Join(*recordPath, replayName)
	hr, err := httpreplay.NewRecorder(replayFile, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	c, err := hr.Client(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		hr.Close()
		cancel()
	})
	return c
}

func newRBEReplay(t *testing.T, ctx context.Context, casInstance string, replayName string) *RBEPerfLoader {
	if !strings.HasPrefix(casInstance, "projects/") {
		casInstance = "projects/" + casInstance + "/instances/default_instance"
	}

	ts, err := google.DefaultTokenSource(ctx)
	require.NoError(t, err)

	opts := []grpc.DialOption{
		grpc.WithPerRPCCredentials(oauth.TokenSource{TokenSource: ts}),
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
	}

	if *recordPath == "" {
		replayFile := path.Join("testdata", replayName)
		f, err := os.Open(replayFile + ".rpc")
		require.NoError(t, err)
		gr, err := gzip.NewReader(f)
		require.NoError(t, err)

		rr, err := rpcreplay.NewReplayerReader(gr)
		require.NoError(t, err)
		opts = append(opts, rr.DialOptions()...)
		t.Cleanup(func() {
			rr.Close()
			gr.Close()
			f.Close()
		})
	} else {
		replayFile := path.Join(*recordPath, replayName)
		f, err := os.Create(replayFile + ".rpc")
		require.NoError(t, err)
		gw := gzip.NewWriter(f)

		rr, err := rpcreplay.NewRecorderWriter(gw, nil)
		require.NoError(t, err)
		opts = append(opts, rr.DialOptions()...)

		t.Cleanup(func() {
			rr.Close()
			gw.Close()
			f.Close()
		})
	}

	conn, err := grpc.Dial(rbeServiceAddress, opts...)
	require.NoError(t, err)
	c, err := client.NewClientFromConnection(ctx, casInstance, conn, conn)
	require.NoError(t, err)
	return &RBEPerfLoader{Client: c}
}
