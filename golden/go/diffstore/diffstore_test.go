package diffstore

import (
	"fmt"
	"net"
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

const TEST_N_DIGESTS = 20

func TestMemDiffStore(t *testing.T) {
	testutils.LargeTest(t)
	testutils.SkipIfShort(t)

	// Get a small tile and get them cached.
	baseDir := TEST_DATA_BASE_DIR + "-diffstore"
	client, tile := getSetupAndTile(t, baseDir)
	defer testutils.RemoveAll(t, baseDir)

	diffStore, err := NewMemDiffStore(client, baseDir, []string{TEST_GCS_BUCKET_NAME}, TEST_GCS_IMAGE_DIR, 10)
	assert.NoError(t, err)
	memDiffStore := diffStore.(*MemDiffStore)

	testDiffStore(t, tile, baseDir, diffStore, memDiffStore)
}

func TestNetDiffStore(t *testing.T) {
	testutils.LargeTest(t)
	testutils.SkipIfShort(t)

	baseDir := TEST_DATA_BASE_DIR + "-netdiffstore"
	client, tile := getSetupAndTile(t, baseDir)
	defer testutils.RemoveAll(t, baseDir)

	memDiffStore, err := NewMemDiffStore(client, baseDir, []string{TEST_GCS_BUCKET_NAME}, TEST_GCS_IMAGE_DIR, 10)
	assert.NoError(t, err)

	// Start the server that wraps around the MemDiffStore.
	serverImpl := NewDiffServiceServer(memDiffStore)
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

	netDiffStore, err := NewNetDiffStore(conn)
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
				id := combineDigests(d1, d2)
				diffIDs = append(diffIDs, id)
				assert.True(t, memDiffStore.diffMetricsCache.Contains(id))
			}
		}
	}

	// Get the results and make sure they are correct.
	foundDiffs := make(map[string]map[string]*diff.DiffMetrics, len(digests))
	ti := timer.New("Get warmed diffs.")
	for _, oneDigest := range digests {
		found, err := diffStore.Get(diff.PRIORITY_NOW, oneDigest, digests)
		assert.NoError(t, err)
		foundDiffs[oneDigest] = found

		// Load the diff from disk and compare.
		for twoDigest, dr := range found {
			id := combineDigests(oneDigest, twoDigest)
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
	foundDiffs = make(map[string]map[string]*diff.DiffMetrics, len(digests))
	for _, oneDigest := range digests {
		found, err := diffStore.Get(diff.PRIORITY_NOW, oneDigest, digests)
		assert.NoError(t, err)
		foundDiffs[oneDigest] = found
	}
	ti.Stop()
	memDiffStore.sync()
	testDiffs(t, baseDir, memDiffStore, digests, digests, foundDiffs)
}

func testDiffs(t *testing.T, baseDir string, diffStore *MemDiffStore, leftDigests, rightDigests []string, result map[string]map[string]*diff.DiffMetrics) {
	for _, left := range leftDigests {
		for _, right := range rightDigests {
			if left != right {
				l, r := left, right
				if r > l {
					l, r = r, l
				}
				_, ok := result[l][r]
				assert.True(t, ok, fmt.Sprintf("left: %s, right:%s", left, right))
				diffPath := fileutil.TwoLevelRadixPath(diffStore.localDiffDir, getDiffImgFileName(left, right))
				assert.True(t, fileutil.FileExists(diffPath), fmt.Sprintf("Could not find %s", diffPath))
			}
		}
	}
}

// func (d *MemDiffStore) ServeImageHandler(w http.ResponseWriter, r *http.Request) {
// func (d *MemDiffStore) ServeDiffImageHandler(w http.ResponseWriter, r *http.Request) {
// func (d *MemDiffStore) UnavailableDigests() map[string]bool {
// func (d *MemDiffStore) PurgeDigests(digests []string, purgeGS bool) error {
type digestsSlice [][]string

func (d digestsSlice) Len() int           { return len(d) }
func (d digestsSlice) Less(i, j int) bool { return len(d[i]) > len(d[j]) }
func (d digestsSlice) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
