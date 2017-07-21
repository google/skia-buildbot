package diffstore

import (
	"fmt"
	"image"
	"net"
	"path/filepath"
	"sort"
	"strings"
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

	// Test image IDs for PixelDiffIDPathMapper.
	TEST_PIXEL_DIFF_LEFT  = "lchoi-20170714123456/nopatch/1/http___www_google_com"
	TEST_PIXEL_DIFF_RIGHT = "lchoi-20170714123456/withpatch/1/http___www_google_com"

	// PNG extension.
	DOT_EXT = ".png"
)

func TestMemDiffStore(t *testing.T) {
	testutils.LargeTest(t)
	testutils.SkipIfShort(t)

	// Get a small tile and get them cached.
	baseDir := TEST_DATA_BASE_DIR + "-diffstore"
	client, tile := getSetupAndTile(t, baseDir)
	defer testutils.RemoveAll(t, baseDir)

	diffStore, err := NewMemDiffStore(client, nil, baseDir, []string{TEST_GCS_BUCKET_NAME}, TEST_GCS_IMAGE_DIR, 10, nil)
	assert.NoError(t, err)
	memDiffStore := diffStore.(*MemDiffStore)

	testDiffStore(t, tile, baseDir, diffStore, memDiffStore)
}

// Dummy Diff Function used to test MemDiffStore.DiffFn.
func DummyDiffFn(leftImg *image.NRGBA, rightImg *image.NRGBA) (interface{}, *image.NRGBA) {
	return 42, nil
}

func TestDiffFn(t *testing.T) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)

	baseDir := TEST_DATA_BASE_DIR + "-difffn"
	client, _ := getSetupAndTile(t, baseDir)
	defer testutils.RemoveAll(t, baseDir)

	// Instantiate a new MemDiffStore with the DummyDiffFn.
	diffStore, err := NewMemDiffStore(client, DummyDiffFn, baseDir, []string{TEST_GCS_BUCKET_NAME}, TEST_GCS_IMAGE_DIR, 10, nil)
	assert.NoError(t, err)
	memDiffStore := diffStore.(*MemDiffStore)
	img1 := image.NewNRGBA(image.Rect(1, 2, 3, 4))
	img2 := image.NewNRGBA(image.Rect(9, 8, 7, 6))

	// Check that proper values are returned by the diff function.
	diffMetrics, diffImg := memDiffStore.diffFn(img1, img2)
	assert.Equal(t, diffMetrics, 42)
	assert.Nil(t, diffImg)
}

func TestNetDiffStore(t *testing.T) {
	testutils.LargeTest(t)
	testutils.SkipIfShort(t)

	baseDir := TEST_DATA_BASE_DIR + "-netdiffstore"
	client, tile := getSetupAndTile(t, baseDir)
	defer testutils.RemoveAll(t, baseDir)

	memDiffStore, err := NewMemDiffStore(client, nil, baseDir, []string{TEST_GCS_BUCKET_NAME}, TEST_GCS_IMAGE_DIR, 10, nil)
	assert.NoError(t, err)

	// Start the server that wraps around the MemDiffStore.
	codec := MetricMapCodec{}
	serverImpl := NewDiffServiceServer(memDiffStore, codec)
	lis, err := net.Listen("tcp", "localhost:0")
	assert.NoError(t, err)

	// Start the server.
	server := grpc.NewServer()
	RegisterDiffServiceServer(server, serverImpl)
	go func() {
		_ = server.Serve(lis)
	}()
	defer server.Stop()

	// Create the NetDiffStore.
	addr := lis.Addr().String()
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, conn.Close())
	}()

	netDiffStore, err := NewNetDiffStore(conn, "", codec)
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
	memDiffStore.imgLoader.sync()
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
			loadedDr, err := memDiffStore.metricsStore.loadDiffMetric(id)
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

func TestGoldIDPathMapper(t *testing.T) {
	testutils.SmallTest(t)

	mapper := GoldIDPathMapper{}

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

func TestPixelDiffIDPathMapper(t *testing.T) {
	testutils.SmallTest(t)

	mapper := PixelDiffIDPathMapper{}
	dirs := strings.Split(TEST_PIXEL_DIFF_LEFT, "/")

	// Test DiffID and SplitDiffID
	expectedDiffID := strings.Join([]string{dirs[0], dirs[2], dirs[3]}, ":")
	actualDiffID := mapper.DiffID(TEST_PIXEL_DIFF_LEFT, TEST_PIXEL_DIFF_RIGHT)
	actualLeft, actualRight := mapper.SplitDiffID(expectedDiffID)
	assert.Equal(t, expectedDiffID, actualDiffID)
	assert.Equal(t, TEST_PIXEL_DIFF_LEFT, actualLeft)
	assert.Equal(t, TEST_PIXEL_DIFF_RIGHT, actualRight)

	// Test DiffPath
	expectedDiffPath := dirs[0] + "/" + dirs[3] + DOT_EXT
	actualDiffPath := mapper.DiffPath(TEST_PIXEL_DIFF_LEFT, TEST_PIXEL_DIFF_RIGHT)
	assert.Equal(t, expectedDiffPath, actualDiffPath)

	// Test ImagePaths
	expectedLocalPath := TEST_PIXEL_DIFF_LEFT + DOT_EXT
	runID := strings.Split(dirs[0], "-")
	timeStamp := runID[1]
	yearMonthDay := filepath.Join(timeStamp[0:4], timeStamp[4:6], timeStamp[6:8])
	expectedGSPath := filepath.Join(yearMonthDay, expectedLocalPath)
	localPath, gsPath := mapper.ImagePaths(TEST_PIXEL_DIFF_LEFT)
	assert.Equal(t, expectedLocalPath, localPath)
	assert.Equal(t, expectedGSPath, gsPath)

	// Test IsValidDiffImgID
	// Trim the image extension first
	expectedDiffImgID := expectedDiffPath[:len(expectedDiffPath)-len(DOT_EXT)]
	assert.True(t, mapper.IsValidDiffImgID(expectedDiffImgID))

	// Test IsValidImgID
	assert.True(t, mapper.IsValidImgID(TEST_PIXEL_DIFF_LEFT))
	assert.True(t, mapper.IsValidImgID(TEST_PIXEL_DIFF_RIGHT))
}

// func (d *MemDiffStore) ServeImageHandler(w http.ResponseWriter, r *http.Request) {
// func (d *MemDiffStore) ServeDiffImageHandler(w http.ResponseWriter, r *http.Request) {
// func (d *MemDiffStore) UnavailableDigests() map[string]bool {
// func (d *MemDiffStore) PurgeDigests(digests []string, purgeGS bool) error {
type digestsSlice [][]string

func (d digestsSlice) Len() int           { return len(d) }
func (d digestsSlice) Less(i, j int) bool { return len(d[i]) > len(d[j]) }
func (d digestsSlice) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
