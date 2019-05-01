package diffstore

import (
	"fmt"
	"image"
	"net"
	"net/http/httptest"
	"path"
	"path/filepath"
	"sort"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
	"google.golang.org/grpc"
)

const (
	TEST_N_DIGESTS = 20

	// Prefix for the image url handler.
	IMAGE_URL_PREFIX = "/img/"
)

func TestMemDiffStore(t *testing.T) {
	testutils.LargeTest(t)

	// Get a small tile and get them cached.
	w, cleanup := testutils.TempDir(t)
	defer cleanup()
	baseDir := path.Join(w, TEST_DATA_BASE_DIR+"-diffstore")
	client, tile := getSetupAndTile(t, baseDir)

	mapper := NewGoldDiffStoreMapper(&diff.DiffMetrics{})
	diffStore, err := NewMemDiffStore(client, baseDir, []string{TEST_GCS_BUCKET_NAME}, TEST_GCS_IMAGE_DIR, 10, mapper)
	assert.NoError(t, err)
	memDiffStore := diffStore.(*MemDiffStore)

	testDiffStore(t, tile, baseDir, diffStore, memDiffStore)
}

type DummyDiffStoreMapper struct {
	GoldDiffStoreMapper
}

func (d DummyDiffStoreMapper) DiffFn(leftImg *image.NRGBA, rightImg *image.NRGBA) (interface{}, *image.NRGBA) {
	return 42, nil
}

func TestDiffFn(t *testing.T) {
	testutils.LargeTest(t)

	w, cleanup := testutils.TempDir(t)
	defer cleanup()
	baseDir := path.Join(w, TEST_DATA_BASE_DIR+"-difffn")
	client, _ := getSetupAndTile(t, baseDir)

	// Instantiate a new MemDiffStore with the DummyDiffFn.
	mapper := DummyDiffStoreMapper{GoldDiffStoreMapper: NewGoldDiffStoreMapper(&diff.DiffMetrics{}).(GoldDiffStoreMapper)}
	diffStore, err := NewMemDiffStore(client, baseDir, []string{TEST_GCS_BUCKET_NAME}, TEST_GCS_IMAGE_DIR, 10, mapper)

	assert.NoError(t, err)
	memDiffStore := diffStore.(*MemDiffStore)
	img1 := image.NewNRGBA(image.Rect(1, 2, 3, 4))
	img2 := image.NewNRGBA(image.Rect(9, 8, 7, 6))

	// Check that proper values are returned by the diff function.
	diffMetrics, diffImg := memDiffStore.mapper.DiffFn(img1, img2)
	assert.Equal(t, 42, diffMetrics)
	assert.Nil(t, diffImg)
}

func TestNetDiffStore(t *testing.T) {
	testutils.LargeTest(t)

	w, cleanup := testutils.TempDir(t)
	defer cleanup()
	baseDir := path.Join(w, TEST_DATA_BASE_DIR+"-netdiffstore")
	client, tile := getSetupAndTile(t, baseDir)

	mapper := NewGoldDiffStoreMapper(&diff.DiffMetrics{})
	memDiffStore, err := NewMemDiffStore(client, baseDir, []string{TEST_GCS_BUCKET_NAME}, TEST_GCS_IMAGE_DIR, 10, mapper)
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

func testDiffStore(t *testing.T, tile *tiling.Tile, baseDir string, diffStore diff.DiffStore, memDiffStore *MemDiffStore) {
	// Pick the test with highest number of digests.
	byName := map[string]util.StringSet{}
	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		name := gTrace.Keys[types.PRIMARY_KEY_FIELD]
		if _, ok := byName[name]; !ok {
			byName[name] = util.StringSet{}
		}
		byName[name].AddLists(gTrace.Digests)
	}
	testDigests := make([][]string, 0, len(byName))
	for _, digests := range byName {
		delete(digests, types.MISSING_DIGEST)
		testDigests = append(testDigests, digests.Keys())
	}
	sort.Sort(digestsSlice(testDigests))

	// Warm the digests and make sure they are in the cache.
	digests := testDigests[0][:TEST_N_DIGESTS]
	diffStore.WarmDigests(diff.PRIORITY_NOW, digests, true)

	for _, d := range digests {
		assert.True(t, memDiffStore.imgLoader.IsOnDisk(d), fmt.Sprintf("Could not find '%s'", d))
	}

	// Warm the diffs and make sure they are in the cache.
	diffStore.WarmDiffs(diff.PRIORITY_NOW, digests, digests)
	memDiffStore.sync()

	diffIDs := make([]string, 0, len(digests)*len(digests))
	for _, d1 := range digests {
		for _, d2 := range digests {
			if d1 != d2 {
				id := memDiffStore.mapper.DiffID(d1, d2)
				diffIDs = append(diffIDs, id)
				assert.True(t, memDiffStore.diffMetricsCache.Contains(id))
			}
		}
	}

	// Get the results and make sure they are correct.
	foundDiffs := make(map[string]map[string]interface{}, len(digests))
	ti := timer.New("Get warmed diffs.")
	for _, oneDigest := range digests {
		found, err := diffStore.Get(diff.PRIORITY_NOW, oneDigest, digests)
		assert.NoError(t, err)
		foundDiffs[oneDigest] = found

		// Load the diff from disk and compare.
		for twoDigest, dr := range found {
			id := memDiffStore.mapper.DiffID(oneDigest, twoDigest)
			loadedDr, err := memDiffStore.metricsStore.loadDiffMetrics(id)
			assert.NoError(t, err)
			assert.Equal(t, dr, loadedDr, "Comparing: %s", id)
		}
	}
	ti.Stop()
	testDiffs(t, baseDir, memDiffStore, digests, digests, foundDiffs)

	// Get the results directly and make sure they are correct.
	digests = testDigests[1][:TEST_N_DIGESTS]
	ti = timer.New("Get cold diffs")
	foundDiffs = make(map[string]map[string]interface{}, len(digests))
	for _, oneDigest := range digests {
		found, err := diffStore.Get(diff.PRIORITY_NOW, oneDigest, digests)
		assert.NoError(t, err)
		foundDiffs[oneDigest] = found
	}
	ti.Stop()
	testDiffs(t, baseDir, memDiffStore, digests, digests, foundDiffs)

	// Diff against an arbitrary GCS location.
	gcsImgID := GCSPathToImageID(TEST_GCS_SECONDARY_BUCKET, TEST_PATH_IMG_1)
	foundDiffs = map[string]map[string]interface{}{}
	for _, oneDigest := range digests {
		found, err := diffStore.Get(diff.PRIORITY_NOW, oneDigest, []string{gcsImgID})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(found))
		foundDiffs[oneDigest] = found
	}
	testDiffs(t, baseDir, memDiffStore, digests, []string{gcsImgID}, foundDiffs)
}

func testDiffs(t *testing.T, baseDir string, diffStore *MemDiffStore, leftDigests, rightDigests []string, result map[string]map[string]interface{}) {
	diffStore.sync()
	for _, left := range leftDigests {
		for _, right := range rightDigests {
			if left != right {
				_, ok := result[left][right]
				assert.True(t, ok, fmt.Sprintf("left: %s, right:%s", left, right))
				diffPath := diffStore.mapper.DiffPath(left, right)
				assert.True(t, fileutil.FileExists(filepath.Join(diffStore.localDiffDir, diffPath)), fmt.Sprintf("Could not find %s", diffPath))
			}
		}
	}
}

type DummyDiffMetrics struct {
	NumDiffPixels     int
	PercentDiffPixels float32
}

type digestsSlice [][]string

func (d digestsSlice) Len() int           { return len(d) }
func (d digestsSlice) Less(i, j int) bool { return len(d[i]) > len(d[j]) }
func (d digestsSlice) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
