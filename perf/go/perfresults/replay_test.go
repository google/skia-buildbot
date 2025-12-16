package perfresults

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"flag"
	"net/http"
	"os"
	"strings"
	"testing"

	"cloud.google.com/go/httpreplay"
	"cloud.google.com/go/rpcreplay"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"golang.org/x/oauth2/google"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
)

var record = flag.Bool("record", false, "If set, record HTTP requests and responses to testdata.")

func setupReplay(t *testing.T, replayName string) *http.Client {
	ignoreHeaders := []string{
		"User-Agent",
		"X-Goog-User-Project",
		"X-Prpc-Max-Response-Size",
	}
	// if the record is not set, then we will replay from the testdata;
	// otherwise we will record the traffic and save to the testdata.
	testDataFile := testutils.TestDataFilename(t, replayName)
	if !*record {
		httpreplay.DebugHeaders()
		hr, err := httpreplay.NewReplayer(testDataFile)
		require.NoError(t, err)
		for _, header := range ignoreHeaders {
			hr.IgnoreHeader(header)
		}

		ctx, cancel := context.WithCancel(context.Background())
		c, err := hr.Client(ctx)
		require.NoError(t, err)
		t.Cleanup(func() {
			hr.Close()
			cancel()
		})
		return c
	}

	hr, err := httpreplay.NewRecorder(testDataFile, nil)
	require.NoError(t, err)
	hr.RemoveRequestHeaders(ignoreHeaders...)

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

	testDataBaseName := replayName + ".rpc"
	if !*record {
		f := testutils.GetReader(t, testDataBaseName)
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
		replayFile := testutils.TestDataFilename(t, testDataBaseName)
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
