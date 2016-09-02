package diffstore

import (
	"fmt"
	"sort"
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

// func (d *MemDiffStore) WarmDigests(priority int64, digests []string) {
// func (d *MemDiffStore) Warm(priority int64, leftDigests []string, rightDigests []string) {
// func (d *MemDiffStore) KeepDigests(Digests []string) {}
// func (d *MemDiffStore) Get(priority int64, leftDigests, rightDigests []string) (map[string]map[string]*DiffRecord, error) {
// func (d *MemDiffStore) diffMetricsWorker(priority int64, id string) (interface{}, error) {
// func (d *MemDiffStore) saveDiffInfoAsync(diffID, leftDigest, rightDigest string, dr *DiffRecord, imgBytes []byte) {
// func (d *MemDiffStore) loadDiffMetric(id string) (*DiffRecord, error) {
// func (d *MemDiffStore) saveDiffMetric(id string, dr *DiffRecord) error {

const TEST_N_DIGESTS = 20

func TestDiffStore(t *testing.T) {
	// Get a small tile and get them cached.
	baseDir := TEST_DATA_BASE_DIR + "-diffstore"
	client, tile := getSetupAndTile(t, baseDir)
	defer testutils.RemoveAll(t, baseDir)

	diffStore, err := New(client, baseDir, TEST_GS_BUCKET_NAME, TEST_GS_IMAGE_DIR)
	assert.NoError(t, err)

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
	diffStore.WarmDigests(PRIORITY_NOW, digests)
	diffStore.imgLoader.sync()
	for _, d := range digests {
		assert.True(t, diffStore.imgLoader.imageCache.Contains(d), fmt.Sprintf("Coult nof find '%s'", d))
	}

	// Warm the diffs and make sure they are in the cache.
	diffStore.Warm(PRIORITY_NOW, digests, digests)
	diffStore.sync()

	diffIDs := make([]string, 0, len(digests)*len(digests))
	for _, d1 := range digests {
		for _, d2 := range digests {
			if d1 != d2 {
				id := combineDigests(d1, d2)
				diffIDs = append(diffIDs, id)
				assert.True(t, diffStore.diffMetricsCache.Contains(id))
			}
		}
	}

	// Get the results and make sure they are correct.
	foundDiffs := make(map[string]map[string]*DiffRecord, len(digests))
	ti := timer.New("Get warmed diffs.")
	for _, oneDigest := range digests {
		found, err := diffStore.Get(PRIORITY_NOW, oneDigest, digests)
		assert.NoError(t, err)
		foundDiffs[oneDigest] = found

		// Load the diff from disk and compare.
		for twoDigest, dr := range found {
			id := combineDigests(oneDigest, twoDigest)
			loadedDr, err := diffStore.loadDiffMetric(id)
			assert.NoError(t, err)
			assert.Equal(t, dr, loadedDr, "Comparing: %s", id)
		}
	}
	ti.Stop()
	testDiffs(t, baseDir, diffStore, digests, digests, foundDiffs)

	// Get the results directly and make sure they are correct.
	digests = testDigests[1][:TEST_N_DIGESTS]
	ti = timer.New("Get cold diffs")
	foundDiffs = make(map[string]map[string]*DiffRecord, len(digests))
	for _, oneDigest := range digests {
		found, err := diffStore.Get(PRIORITY_NOW, oneDigest, digests)
		assert.NoError(t, err)
		foundDiffs[oneDigest] = found
	}
	ti.Stop()
	diffStore.sync()
	testDiffs(t, baseDir, diffStore, digests, digests, foundDiffs)
}

func testDiffs(t *testing.T, baseDir string, diffStore *MemDiffStore, leftDigests, rightDigests []string, result map[string]map[string]*DiffRecord) {
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
