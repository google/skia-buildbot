package diffstore

import (
	"context"
	"net"
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/common"
	"go.skia.org/infra/golden/go/diffstore/metricsstore/fs_metricsstore"
	"go.skia.org/infra/golden/go/types"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

// TestNetDiffStoreIntegration sets up a memDiffStore backed by real GCS and the firestore emulator.
// Then, it connects to that memDiffStore over the network and retrieves diffs.
func TestNetDiffStoreIntegration(t *testing.T) {
	unittest.LargeTest(t)

	// The test bucket is a public bucket, so we don't need to worry about authentication.
	unauthedClient := httputils.DefaultClientConfig().Client()

	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(unauthedClient))
	require.NoError(t, err)
	gcsClient := gcsclient.New(storageClient, gcsTestBucket)

	// create a client against the firestore emulator.
	c, cleanup := firestore.NewClientForTesting(context.Background(), t)
	defer cleanup()
	fsMetrics := fs_metricsstore.New(c)

	memDiffStore, err := NewMemDiffStore(gcsClient, gcsImageBaseDir, 1, fsMetrics)
	require.NoError(t, err)

	// These are two nearly identical images in the skia-infra-testdata bucket.
	// The names are arbitrary (they don't actually correspond with the hash of the pixels).
	original := types.Digest("000da2ce46164b5027ee964b8c040335")
	cross := types.Digest("cccd4f34d847bd8a540c7c9cf1602107")

	// There are 5 pixels in the cross image that are black instead of white.
	// These values were computed by using the default algorithm and manual inspection.
	dm := &diff.DiffMetrics{
		NumDiffPixels:    5,
		PixelDiffPercent: 0.0010146104,
		MaxRGBADiffs:     [4]int{255, 255, 255, 0},
		CombinedMetric:   0.02964251,
	}

	// Start the server that wraps around the MemDiffStore.
	serverImpl := NewDiffServiceServer(memDiffStore)
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	// Start the grpc server.
	server := grpc.NewServer()
	RegisterDiffServiceServer(server, serverImpl)
	go func() {
		_ = server.Serve(lis)
	}()
	defer server.Stop()

	// Start the http server.
	imgHandler, err := memDiffStore.ImageHandler(imgWebPath)
	require.NoError(t, err)

	httpServer := httptest.NewServer(imgHandler)
	defer func() { httpServer.Close() }()

	// Create the NetDiffStore.
	addr := lis.Addr().String()
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, conn.Close())
	}()

	netDiffStore, err := NewNetDiffStore(context.Background(), conn, httpServer.Listener.Addr().String())
	require.NoError(t, err)

	diffDigests := []types.Digest{cross}

	diffs, err := netDiffStore.Get(context.Background(), original, diffDigests)
	require.NoError(t, err)
	assert.Len(t, diffs, 1)
	assert.Equal(t, dm, diffs[cross])

	// make sure they are actually stored in the backend
	actual, err := fsMetrics.LoadDiffMetrics(context.Background(), []string{common.DiffID(original, cross)})
	require.NoError(t, err)
	assert.Equal(t, dm, actual[0])
}
