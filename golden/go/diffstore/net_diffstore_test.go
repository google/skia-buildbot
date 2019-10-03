package diffstore

import (
	"context"
	"net"
	"net/http/httptest"
	"path"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/mapper/disk_mapper"
	"go.skia.org/infra/golden/go/diffstore/metricsstore/bolt_metricsstore"
	d_utils "go.skia.org/infra/golden/go/diffstore/testutils"
	"go.skia.org/infra/golden/go/mocks"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

func TestNetDiffStore(t *testing.T) {
	unittest.LargeTest(t)

	w, cleanup := testutils.TempDir(t)
	defer cleanup()
	baseDir := path.Join(w, d_utils.TEST_DATA_BASE_DIR+"-netdiffstore")
	client, tile := d_utils.GetSetupAndTile(t, baseDir)
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	assert.NoError(t, err)
	gcsClient := gcsclient.New(storageClient, d_utils.TEST_GCS_BUCKET_NAME)

	m := disk_mapper.New(&diff.DiffMetrics{})
	mfs := &mocks.FailureStore{}
	mStore, err := bolt_metricsstore.New(baseDir, m)
	assert.NoError(t, err)
	memDiffStore, err := NewMemDiffStore(gcsClient, d_utils.TEST_GCS_IMAGE_DIR, 10, m, mStore, mfs)
	assert.NoError(t, err)

	// Start the server that wraps around the MemDiffStore.
	codec := MetricMapCodec{}
	serverImpl := NewDiffServiceServer(memDiffStore, codec)
	lis, err := net.Listen("tcp", "localhost:0")
	assert.NoError(t, err)

	// Start the grpc server.
	server := grpc.NewServer()
	RegisterDiffServiceServer(server, serverImpl)
	go func() {
		_ = server.Serve(lis)
	}()
	defer server.Stop()

	// Start the http server.
	imgHandler, err := memDiffStore.ImageHandler(IMAGE_URL_PREFIX)
	assert.NoError(t, err)

	httpServer := httptest.NewServer(imgHandler)
	defer func() { httpServer.Close() }()

	// Create the NetDiffStore.
	addr := lis.Addr().String()
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, conn.Close())
	}()

	netDiffStore, err := NewNetDiffStore(conn, httpServer.Listener.Addr().String(), codec)
	assert.NoError(t, err)

	// run tests against it.
	testDiffStore(t, tile, netDiffStore, memDiffStore.(*MemDiffStore))
}
