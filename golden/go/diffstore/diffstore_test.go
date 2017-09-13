package diffstore

import (
	"fmt"
	"image"
	"net"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"testing"

	"google.golang.org/grpc"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
)

const (
	TEST_N_DIGESTS = 20

	// Test digests for GoldIDPathMapper.
	TEST_GOLD_LEFT  = "098f6bcd4621d373cade4e832627b4f6"
	TEST_GOLD_RIGHT = "1660f0783f4076284bc18c5f4bdc9608"

	// PNG extension.
	DOT_EXT = ".png"

	// Prefix for the image url handler.
	IMAGE_URL_PREFIX = "/img/"
)

func TestMemDiffStore(t *testing.T) {
	testutils.LargeTest(t)
	testutils.SkipIfShort(t)

	// Get a small tile and get them cached.
	baseDir := TEST_DATA_BASE_DIR + "-diffstore"
	client, tile := getSetupAndTile(t, baseDir)
	defer testutils.RemoveAll(t, baseDir)

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
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)

	baseDir := TEST_DATA_BASE_DIR + "-difffn"
	client, _ := getSetupAndTile(t, baseDir)
	defer testutils.RemoveAll(t, baseDir)

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
	testutils.SkipIfShort(t)

	baseDir := TEST_DATA_BASE_DIR + "-netdiffstore"
	client, tile := getSetupAndTile(t, baseDir)
	defer testutils.RemoveAll(t, baseDir)

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
		name := gTrace.Params_[types.PRIMARY_KEY_FIELD]
		if _, ok := byName[name]; !ok {
			byName[name] = util.StringSet{}
		}
		byName[name].AddLists(gTrace.Values)
	}
	testDigests := make([][]string, 0, len(byName))
	for _, digests := range byName {
		delete(digests, types.MISSING_DIGEST)
		testDigests = append(testDigests, digests.Keys())
	}
	sort.Sort(digestsSlice(testDigests))

	// Warm the digests and make sure they are in the cache.
	digests := testDigests[0][:TEST_N_DIGESTS]
	diffStore.WarmDigests(diff.PRIORITY_NOW, digests, false)
	memDiffStore.imgLoader.Sync()
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
	memDiffStore.sync()
	testDiffs(t, baseDir, memDiffStore, digests, digests, foundDiffs)
}

func testDiffs(t *testing.T, baseDir string, diffStore *MemDiffStore, leftDigests, rightDigests []string, result map[string]map[string]interface{}) {
	for _, left := range leftDigests {
		for _, right := range rightDigests {
			if left != right {
				l, r := left, right
				if r > l {
					l, r = r, l
				}
				_, ok := result[l][r]
				assert.True(t, ok, fmt.Sprintf("left: %s, right:%s", left, right))
				diffPath := diffStore.mapper.DiffPath(left, right)
				assert.True(t, fileutil.FileExists(filepath.Join(diffStore.localDiffDir, diffPath)), fmt.Sprintf("Could not find %s", diffPath))
			}
		}
	}
}

func TestGoldDiffStoreMapper(t *testing.T) {
	testutils.SmallTest(t)

	mapper := GoldDiffStoreMapper{}

	// Test DiffID and SplitDiffID
	expectedDiffID := TEST_GOLD_LEFT + ":" + TEST_GOLD_RIGHT
	actualDiffID := mapper.DiffID(TEST_GOLD_LEFT, TEST_GOLD_RIGHT)
	actualLeft, actualRight := mapper.SplitDiffID(expectedDiffID)
	assert.Equal(t, expectedDiffID, actualDiffID)
	assert.Equal(t, TEST_GOLD_LEFT, actualLeft)
	assert.Equal(t, TEST_GOLD_RIGHT, actualRight)

	// Test DiffPath
	twoLevelRadix := TEST_GOLD_LEFT[0:2] + "/" + TEST_GOLD_LEFT[2:4] + "/"
	expectedDiffPath := twoLevelRadix + TEST_GOLD_LEFT + "-" +
		TEST_GOLD_RIGHT + DOT_EXT
	actualDiffPath := mapper.DiffPath(TEST_GOLD_LEFT, TEST_GOLD_RIGHT)
	assert.Equal(t, expectedDiffPath, actualDiffPath)

	// Test ImagePaths
	expectedLocalPath := twoLevelRadix + TEST_GOLD_LEFT + DOT_EXT
	expectedGSPath := TEST_GOLD_LEFT + DOT_EXT
	localPath, gsPath := mapper.ImagePaths(TEST_GOLD_LEFT)
	assert.Equal(t, expectedLocalPath, localPath)
	assert.Equal(t, expectedGSPath, gsPath)

	// Test IsValidDiffImgID
	// Trim the two level radix path and image extension first
	expectedDiffImgID := expectedDiffPath[len(twoLevelRadix) : len(expectedDiffPath)-len(DOT_EXT)]
	assert.True(t, mapper.IsValidDiffImgID(expectedDiffImgID))

	// Test IsValidImgID
	assert.True(t, mapper.IsValidImgID(TEST_GOLD_LEFT))
	assert.True(t, mapper.IsValidImgID(TEST_GOLD_RIGHT))
}

type DummyDiffMetrics struct {
	NumDiffPixels     int
	PercentDiffPixels float32
}

func TestCodec(t *testing.T) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)

	baseDir := TEST_DATA_BASE_DIR + "-codec"
	client, _ := getSetupAndTile(t, baseDir)
	defer testutils.RemoveAll(t, baseDir)

	// Instantiate a new MemDiffStore with a codec for the test struct defined above.
	mapper := NewGoldDiffStoreMapper(&DummyDiffMetrics{})
	diffStore, err := NewMemDiffStore(client, baseDir, []string{TEST_GCS_BUCKET_NAME}, TEST_GCS_IMAGE_DIR, 10, mapper)
	assert.NoError(t, err)
	memDiffStore := diffStore.(*MemDiffStore)

	diffID := mapper.DiffID(TEST_GOLD_LEFT, TEST_GOLD_RIGHT)
	diffMetrics := &DummyDiffMetrics{
		NumDiffPixels:     100,
		PercentDiffPixels: 0.5,
	}
	err = memDiffStore.metricsStore.saveDiffMetrics(diffID, diffMetrics)
	assert.NoError(t, err)

	// Verify the returned diff metrics object has the same type and same contents
	// as the object that was saved to the metricsStore.
	metrics, err := memDiffStore.metricsStore.loadDiffMetrics(diffID)
	assert.NoError(t, err)
	assert.Equal(t, diffMetrics, metrics)
}

type digestsSlice [][]string

func (d digestsSlice) Len() int           { return len(d) }
func (d digestsSlice) Less(i, j int) bool { return len(d[i]) > len(d[j]) }
func (d digestsSlice) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
