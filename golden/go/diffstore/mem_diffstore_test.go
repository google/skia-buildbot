package diffstore

import (
	"context"
	"fmt"
	"path"
	"sort"
	"testing"

	"cloud.google.com/go/storage"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/common"
	"go.skia.org/infra/golden/go/diffstore/mapper/disk_mapper"
	d_utils "go.skia.org/infra/golden/go/diffstore/testutils"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/types"
	"google.golang.org/api/option"
)

const (
	TEST_N_DIGESTS = 20

	// Prefix for the image url handler.
	IMAGE_URL_PREFIX = "/img/"
)

func TestMemDiffStore(t *testing.T) {
	unittest.LargeTest(t)

	// Get a small tile and get them cached.
	w, cleanup := testutils.TempDir(t)
	defer cleanup()
	baseDir := path.Join(w, d_utils.TEST_DATA_BASE_DIR+"-diffstore")
	client, tile := d_utils.GetSetupAndTile(t, baseDir)
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	assert.NoError(t, err)
	gcsClient := gcsclient.New(storageClient, d_utils.TEST_GCS_BUCKET_NAME)

	m := disk_mapper.New(&diff.DiffMetrics{})
	// MemDiffStore is built with a nil FailureStore as it is not used by this test.
	diffStore, err := NewMemDiffStore(gcsClient, baseDir, d_utils.TEST_GCS_IMAGE_DIR, 10, m, nil)
	assert.NoError(t, err)
	memDiffStore := diffStore.(*MemDiffStore)

	testDiffStore(t, tile, baseDir, diffStore, memDiffStore)
}

func testDiffStore(t *testing.T, tile *tiling.Tile, baseDir string, diffStore diff.DiffStore, memDiffStore *MemDiffStore) {
	// Pick the test with highest number of digests.
	byName := map[types.TestName]types.DigestSet{}
	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		name := gTrace.TestName()
		if _, ok := byName[name]; !ok {
			byName[name] = types.DigestSet{}
		}
		byName[name].AddLists(gTrace.Digests)
	}
	testDigests := make(digestSliceSlice, 0, len(byName))
	for _, digests := range byName {
		delete(digests, types.MISSING_DIGEST)
		testDigests = append(testDigests, digests.Keys())
	}
	sort.Sort(digestSliceSlice(testDigests))

	// Warm the digests and make sure they are in the cache.
	digests := testDigests[0][:TEST_N_DIGESTS]
	diffStore.WarmDigests(diff.PRIORITY_NOW, digests, true)

	for _, d := range digests {
		assert.True(t, memDiffStore.imgLoader.Contains(d), fmt.Sprintf("Could not find '%s'", d))
	}

	// Warm the diffs and make sure they are in the cache.
	diffStore.WarmDiffs(diff.PRIORITY_NOW, digests, digests)
	memDiffStore.sync()

	// TODO(kjlubick): assert something with this diffIDs slice?
	diffIDs := make([]string, 0, len(digests)*len(digests))
	for _, d1 := range digests {
		for _, d2 := range digests {
			if d1 != d2 {
				id := common.DiffID(d1, d2)
				diffIDs = append(diffIDs, id)
				assert.True(t, memDiffStore.diffMetricsCache.Contains(id))
			}
		}
	}

	// Get the results and make sure they are correct.
	foundDiffs := make(map[types.Digest]map[types.Digest]interface{}, len(digests))
	ti := timer.New("Get warmed diffs.")
	for _, oneDigest := range digests {
		found, err := diffStore.Get(diff.PRIORITY_NOW, oneDigest, digests)
		assert.NoError(t, err)
		foundDiffs[oneDigest] = found

		// Load the diff from disk and compare.
		for twoDigest, dr := range found {
			id := common.DiffID(oneDigest, twoDigest)
			loadedDr, err := memDiffStore.metricsStore.LoadDiffMetrics(id)
			assert.NoError(t, err)
			assert.Equal(t, dr, loadedDr, "Comparing: %s", id)
		}
	}
	ti.Stop()
	testDiffs(t, memDiffStore, digests, digests, foundDiffs)

	// Get the results directly and make sure they are correct.
	digests = testDigests[1][:TEST_N_DIGESTS]
	ti = timer.New("Get cold diffs")
	foundDiffs = make(map[types.Digest]map[types.Digest]interface{}, len(digests))
	for _, oneDigest := range digests {
		found, err := diffStore.Get(diff.PRIORITY_NOW, oneDigest, digests)
		assert.NoError(t, err)
		foundDiffs[oneDigest] = found
	}
	ti.Stop()
	testDiffs(t, memDiffStore, digests, digests, foundDiffs)
}

func testDiffs(t *testing.T, diffStore *MemDiffStore, leftDigests, rightDigests types.DigestSlice, result map[types.Digest]map[types.Digest]interface{}) {
	diffStore.sync()
	for _, left := range leftDigests {
		for _, right := range rightDigests {
			if left != right {
				_, ok := result[left][right]
				assert.True(t, ok, fmt.Sprintf("left: %s, right:%s", left, right))
			}
		}
	}
}

func TestFailureHandling(t *testing.T) {
	unittest.MediumTest(t)

	// Get a small tile and get them cached.
	w, cleanup := testutils.TempDir(t)
	defer cleanup()
	baseDir := path.Join(w, d_utils.TEST_DATA_BASE_DIR+"-diffstore-failure")
	client, tile := d_utils.GetSetupAndTile(t, baseDir)
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	assert.NoError(t, err)
	gcsClient := gcsclient.New(storageClient, d_utils.TEST_GCS_BUCKET_NAME)

	invalidDigest_1 := types.Digest("invaliddigest1")
	invalidDigest_2 := types.Digest("invaliddigest2")

	// Set up mock FailureStore.
	mfs := &mocks.FailureStore{}
	defer mfs.AssertExpectations(t)

	// FailureStore calls for invalid digest #1.
	mfs.On("AddDigestFailure", diffFailureMatcher(invalidDigest_1, "http_error")).Return(nil)
	mfs.On("AddDigestFailureIfNew", diffFailureMatcher(invalidDigest_1, "other")).Return(nil)

	// FailureStore calls for invalid digest #2.
	mfs.On("AddDigestFailure", diffFailureMatcher(invalidDigest_2, "http_error")).Return(nil)
	mfs.On("AddDigestFailureIfNew", diffFailureMatcher(invalidDigest_2, "other")).Return(nil)

	// FailureStore.UnavailableDigests() call after trying to retrieve invalid digests.
	mfs.On("UnavailableDigests").Return(map[types.Digest]*diff.DigestFailure{
		invalidDigest_1: {Digest: invalidDigest_1, Reason: "http_error"},
		invalidDigest_2: {Digest: invalidDigest_2, Reason: "http_error"},
	}).Once()

	mfs.On("PurgeDigestFailures", types.DigestSlice{invalidDigest_1, invalidDigest_2}).Return(nil)

	// FailureStore.UnavailableDigests() call after purging the above digest failures.
	mfs.On("UnavailableDigests").Return(map[types.Digest]*diff.DigestFailure{})

	m := disk_mapper.New(&diff.DiffMetrics{})
	diffStore, err := NewMemDiffStore(gcsClient, baseDir, d_utils.TEST_GCS_IMAGE_DIR, 10, m, mfs)
	assert.NoError(t, err)

	validDigestSet := types.DigestSet{}
	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		validDigestSet.AddLists(gTrace.Digests)
	}
	delete(validDigestSet, types.MISSING_DIGEST)

	validDigests := validDigestSet.Keys()
	mainDigest := validDigests[0]
	diffDigests := append(validDigests[1:6], invalidDigest_1, invalidDigest_2)

	diffs, err := diffStore.Get(diff.PRIORITY_NOW, mainDigest, diffDigests)
	assert.NoError(t, err)
	assert.Equal(t, len(diffDigests)-2, len(diffs))

	unavailableDigests := diffStore.UnavailableDigests()
	assert.Equal(t, 2, len(unavailableDigests))
	assert.NotNil(t, unavailableDigests[invalidDigest_1])
	assert.NotNil(t, unavailableDigests[invalidDigest_2])

	assert.NoError(t, diffStore.PurgeDigests(types.DigestSlice{invalidDigest_1, invalidDigest_2}, true))
	unavailableDigests = diffStore.UnavailableDigests()
	assert.Equal(t, 0, len(unavailableDigests))
}

func TestCodec(t *testing.T) {
	unittest.MediumTest(t)

	w, cleanup := testutils.TempDir(t)
	defer cleanup()
	baseDir := path.Join(w, d_utils.TEST_DATA_BASE_DIR+"-codec")
	client, _ := d_utils.GetSetupAndTile(t, baseDir)
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	assert.NoError(t, err)
	gcsClient := gcsclient.New(storageClient, d_utils.TEST_GCS_BUCKET_NAME)

	// Instantiate a new MemDiffStore with a codec for the test struct defined above.
	// MemDiffStore is built with a nil FailureStore as it is not used by this test.
	m := disk_mapper.New(&d_utils.DummyDiffMetrics{})
	diffStore, err := NewMemDiffStore(gcsClient, baseDir, d_utils.TEST_GCS_IMAGE_DIR, 10, m, nil)
	assert.NoError(t, err)
	memDiffStore := diffStore.(*MemDiffStore)

	diffID := common.DiffID(types.Digest("abc"), types.Digest("def"))
	diffMetrics := &d_utils.DummyDiffMetrics{
		NumDiffPixels:     100,
		PercentDiffPixels: 0.5,
	}
	err = memDiffStore.metricsStore.SaveDiffMetrics(diffID, diffMetrics)
	assert.NoError(t, err)

	// Verify the returned diff metrics object has the same type and same contents
	// as the object that was saved to the metricsStore.
	metrics, err := memDiffStore.metricsStore.LoadDiffMetrics(diffID)
	assert.NoError(t, err)
	assert.Equal(t, diffMetrics, metrics)
}

func TestDecodeImagesSuccess(t *testing.T) {
	unittest.SmallTest(t)

	// Expected output.
	expectedLeftImage := image1
	expectedRightImage := image2

	// Inputs.
	leftBytes := imageToPng(expectedLeftImage).Bytes()
	rightBytes := imageToPng(expectedRightImage).Bytes()

	// Call code under test.
	actualLeftImage, actualRightImage, err := decodeImages(leftBytes, rightBytes)

	assert.NoError(t, err)
	assert.Equal(t, expectedLeftImage, actualLeftImage)
	assert.Equal(t, expectedRightImage, actualRightImage)
}

func TestDecodeImagesErrorDecodingLeftImage(t *testing.T) {
	unittest.SmallTest(t)

	// Inputs.
	leftBytes := []byte("I'm not a PNG image")
	rightBytes := imageToPng(image1).Bytes()

	// Call code under test.
	_, _, err := decodeImages(leftBytes, rightBytes)

	assert.Error(t, err)
}

func TestDecodeImagesErrorDecodingRightImage(t *testing.T) {
	unittest.SmallTest(t)

	// Inputs.
	leftBytes := imageToPng(image1).Bytes()
	rightBytes := []byte("I'm not a PNG image")

	// Call code under test.
	_, _, err := decodeImages(leftBytes, rightBytes)

	assert.Error(t, err)
}

func TestDecodeImagesErrorDecodingLeftAndRightImages(t *testing.T) {
	unittest.SmallTest(t)

	// Inputs.
	leftBytes := []byte("I'm not a PNG image")
	rightBytes := []byte("I'm not a PNG image")

	// Call code under test.
	_, _, err := decodeImages(leftBytes, rightBytes)

	assert.Error(t, err)
}

// Allows for sorting slices of digests by the length (longer slices first)
type digestSliceSlice []types.DigestSlice

func (d digestSliceSlice) Len() int           { return len(d) }
func (d digestSliceSlice) Less(i, j int) bool { return len(d[i]) > len(d[j]) }
func (d digestSliceSlice) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
