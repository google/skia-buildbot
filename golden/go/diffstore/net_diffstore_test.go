package diffstore

import (
	"net"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/mapper/disk_mapper"
	d_utils "go.skia.org/infra/golden/go/diffstore/testutils"
	"google.golang.org/grpc"
)

func TestNetDiffStore(t *testing.T) {
	unittest.LargeTest(t)

	w, cleanup := testutils.TempDir(t)
	defer cleanup()
	baseDir := path.Join(w, d_utils.TEST_DATA_BASE_DIR+"-netdiffstore")
	client, tile := d_utils.GetSetupAndTile(t, baseDir)

	m := disk_mapper.New(&diff.DiffMetrics{})
	memDiffStore, err := NewMemDiffStore(client, baseDir, []string{d_utils.TEST_GCS_BUCKET_NAME}, d_utils.TEST_GCS_IMAGE_DIR, 10, m)
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
	testDiffStore(t, tile, baseDir, netDiffStore, memDiffStore.(*MemDiffStore))
}
