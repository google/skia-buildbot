package diffstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path"
	"sort"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/common"
	"go.skia.org/infra/golden/go/diffstore/mapper/disk_mapper"
	"go.skia.org/infra/golden/go/diffstore/metricsstore/bolt_metricsstore"
	diffstore_mocks "go.skia.org/infra/golden/go/diffstore/mocks"
	d_utils "go.skia.org/infra/golden/go/diffstore/testutils"
	"go.skia.org/infra/golden/go/types"
	"google.golang.org/api/option"
)

const (
	TEST_N_DIGESTS = 20

	// Prefix for the image url handler.
	IMAGE_URL_PREFIX = "/img/"

	// Missing digest (valid, but arbitrary).
	missingDigest       = types.Digest("ffffffffffffffffffffffffffffffff")
	missingDigestGsPath = gsImageBaseDir + "/" + string(missingDigest) + ".png"

	// Diff between skTextImage1 and skTextImage2.
	skTextDiffImages1And2 = `! SKTEXTSIMPLE
	1 5
	0xfdd0a2ff
	0xfdd0a2ff
	0xfdd0a2ff
	0xfdd0a2ff
	0xc6dbefff`

	// Digest for the 16-bit image stored in the testdata directory. This is the same image used to
	// test preservation of color space information in skbug.com/9483.
	digest16BitImage       = types.Digest("8a90a2f1245dc87d96bfb74bdc4ab97e")
	digest16BitImageGsPath = gsImageBaseDir + "/" + string(digest16BitImage) + ".png"

	// MD5 hash of the 16-bit PNG image above. Needed for the storage.ObjectAttrs instance returned by
	// the mock GCS client.
	md5Hash16BitImage = "22ea2cb4e3eabd2bb3faba7a07e18b7a" // = md5sum(8a90a2f1245dc87d96bfb74bdc4ab97e.png)
)

func TestMd5Hash16BitImageIsCorrect(t *testing.T) {
	unittest.SmallTest(t)
	bytes, err := testutils.ReadFileBytes(fmt.Sprintf("%s.png", digest16BitImage))
	require.NoError(t, err)
	require.Equal(t, md5Hash16BitImage, bytesToMd5HashString(bytes))
}

func TestMemDiffStore(t *testing.T) {
	unittest.LargeTest(t)

	// Get a small tile and get them cached.
	w, cleanup := testutils.TempDir(t)
	defer cleanup()
	baseDir := path.Join(w, d_utils.TEST_DATA_BASE_DIR+"-diffstore")
	client, tile := d_utils.GetSetupAndTile(t, baseDir)
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	require.NoError(t, err)
	gcsClient := gcsclient.New(storageClient, d_utils.TEST_GCS_BUCKET_NAME)

	m := disk_mapper.New(&diff.DiffMetrics{})
	mStore, err := bolt_metricsstore.New(baseDir)
	require.NoError(t, err)
	// MemDiffStore is built with a nil FailureStore as it is not used by this test.
	diffStore, err := NewMemDiffStore(gcsClient, d_utils.TEST_GCS_IMAGE_DIR, 10, m, mStore, nil)
	require.NoError(t, err)
	memDiffStore := diffStore.(*MemDiffStore)

	testDiffStore(t, tile, diffStore, memDiffStore)
}

func testDiffStore(t *testing.T, tile *tiling.Tile, diffStore diff.DiffStore, memDiffStore *MemDiffStore) {
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
		require.True(t, memDiffStore.imgLoader.Contains(d), fmt.Sprintf("Could not find '%s'", d))
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
				require.True(t, memDiffStore.diffMetricsCache.Contains(id))
			}
		}
	}

	// Get the results and make sure they are correct.
	foundDiffs := make(map[types.Digest]map[types.Digest]*diff.DiffMetrics, len(digests))
	ti := timer.New("Get warmed diffs.")
	for _, oneDigest := range digests {
		found, err := diffStore.Get(diff.PRIORITY_NOW, oneDigest, digests)
		require.NoError(t, err)
		foundDiffs[oneDigest] = found

		// Load the diff from disk and compare.
		for twoDigest, dr := range found {
			id := common.DiffID(oneDigest, twoDigest)
			loadedDr, err := memDiffStore.metricsStore.LoadDiffMetrics(id)
			require.NoError(t, err)
			require.Equal(t, dr, loadedDr, "Comparing: %s", id)
		}
	}
	ti.Stop()
	testDiffs(t, memDiffStore, digests, digests, foundDiffs)

	// Get the results directly and make sure they are correct.
	digests = testDigests[1][:TEST_N_DIGESTS]
	ti = timer.New("Get cold diffs")
	foundDiffs = make(map[types.Digest]map[types.Digest]*diff.DiffMetrics, len(digests))
	for _, oneDigest := range digests {
		found, err := diffStore.Get(diff.PRIORITY_NOW, oneDigest, digests)
		require.NoError(t, err)
		foundDiffs[oneDigest] = found
	}
	ti.Stop()
	testDiffs(t, memDiffStore, digests, digests, foundDiffs)
}

func testDiffs(t *testing.T, diffStore *MemDiffStore, leftDigests, rightDigests types.DigestSlice, result map[types.Digest]map[types.Digest]*diff.DiffMetrics) {
	diffStore.sync()
	for _, left := range leftDigests {
		for _, right := range rightDigests {
			if left != right {
				_, ok := result[left][right]
				require.True(t, ok, fmt.Sprintf("left: %s, right:%s", left, right))
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
	require.NoError(t, err)
	gcsClient := gcsclient.New(storageClient, d_utils.TEST_GCS_BUCKET_NAME)

	invalidDigest_1 := types.Digest("invaliddigest1")
	invalidDigest_2 := types.Digest("invaliddigest2")

	// Set up mock FailureStore.
	mfs := &diffstore_mocks.FailureStore{}
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
	mStore, err := bolt_metricsstore.New(baseDir)
	require.NoError(t, err)
	diffStore, err := NewMemDiffStore(gcsClient, d_utils.TEST_GCS_IMAGE_DIR, 10, m, mStore, mfs)
	require.NoError(t, err)

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
	require.NoError(t, err)
	require.Equal(t, len(diffDigests)-2, len(diffs))

	unavailableDigests := diffStore.UnavailableDigests()
	require.Equal(t, 2, len(unavailableDigests))
	require.NotNil(t, unavailableDigests[invalidDigest_1])
	require.NotNil(t, unavailableDigests[invalidDigest_2])

	require.NoError(t, diffStore.PurgeDigests(types.DigestSlice{invalidDigest_1, invalidDigest_2}, true))
	unavailableDigests = diffStore.UnavailableDigests()
	require.Equal(t, 0, len(unavailableDigests))
}

func TestCodec(t *testing.T) {
	unittest.MediumTest(t)

	w, cleanup := testutils.TempDir(t)
	defer cleanup()
	baseDir := path.Join(w, d_utils.TEST_DATA_BASE_DIR+"-codec")
	client, _ := d_utils.GetSetupAndTile(t, baseDir)
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	require.NoError(t, err)
	gcsClient := gcsclient.New(storageClient, d_utils.TEST_GCS_BUCKET_NAME)

	// Instantiate a new MemDiffStore with a codec for the test struct defined above.
	// MemDiffStore is built with a nil FailureStore as it is not used by this test.
	m := disk_mapper.New(&diff.DiffMetrics{})
	mStore, err := bolt_metricsstore.New(baseDir)
	require.NoError(t, err)
	diffStore, err := NewMemDiffStore(gcsClient, d_utils.TEST_GCS_IMAGE_DIR, 10, m, mStore, nil)
	require.NoError(t, err)
	memDiffStore := diffStore.(*MemDiffStore)

	diffID := common.DiffID(types.Digest("abc"), types.Digest("def"))
	diffMetrics := &diff.DiffMetrics{
		NumDiffPixels:    100,
		PixelDiffPercent: 0.5,
	}

	err = memDiffStore.metricsStore.SaveDiffMetrics(diffID, diffMetrics)
	require.NoError(t, err)

	// Verify the returned diff metrics object has the same type and same contents
	// as the object that was saved to the metricsStore.
	metrics, err := memDiffStore.metricsStore.LoadDiffMetrics(diffID)
	require.NoError(t, err)
	require.Equal(t, diffMetrics, metrics)
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

	require.NoError(t, err)
	require.Equal(t, expectedLeftImage, actualLeftImage)
	require.Equal(t, expectedRightImage, actualRightImage)
}

func TestDecodeImagesErrorDecodingLeftImage(t *testing.T) {
	unittest.SmallTest(t)

	// Inputs.
	leftBytes := []byte("I'm not a PNG image")
	rightBytes := imageToPng(image1).Bytes()

	// Call code under test.
	_, _, err := decodeImages(leftBytes, rightBytes)

	require.Error(t, err)
}

func TestDecodeImagesErrorDecodingRightImage(t *testing.T) {
	unittest.SmallTest(t)

	// Inputs.
	leftBytes := imageToPng(image1).Bytes()
	rightBytes := []byte("I'm not a PNG image")

	// Call code under test.
	_, _, err := decodeImages(leftBytes, rightBytes)

	require.Error(t, err)
}

func TestDecodeImagesErrorDecodingLeftAndRightImages(t *testing.T) {
	unittest.SmallTest(t)

	// Inputs.
	leftBytes := []byte("I'm not a PNG image")
	rightBytes := []byte("I'm not a PNG image")

	// Call code under test.
	_, _, err := decodeImages(leftBytes, rightBytes)

	require.Error(t, err)
}

func TestMemDiffStoreImageHandler(t *testing.T) {
	unittest.SmallTest(t)

	// This is a white-box test. The mock failure store and GCS client below return only what is
	// needed for this test to pass, and nothing more (e.g. field "MD5" in *storage.ObjectAttrs).

	// Build mock FailureStore.
	mockFailureStore := &diffstore_mocks.FailureStore{}
	defer mockFailureStore.AssertExpectations(t)

	// Failure is stored.
	mockFailureStore.On("AddDigestFailure", diffFailureMatcher(missingDigest, "http_error")).Return(nil)
	mockFailureStore.On("AddDigestFailureIfNew", diffFailureMatcher(missingDigest, "other")).Return(nil)

	// Build mock GCSClient.
	mockBucketClient := test_gcsclient.NewMockClient()
	defer mockBucketClient.AssertExpectations(t)

	// Only used for logging errors, which only some tests produce.
	mockBucketClient.On("Bucket").Return("test-bucket")

	// missingDigest is not present in the GCS bucket.
	var oa *storage.ObjectAttrs
	mockBucketClient.On("GetFileObjectAttrs", testutils.AnyContext, missingDigestGsPath).Return(oa, errors.New("not found"))

	// digest1 is present in the GCS bucket.
	oa1 := &storage.ObjectAttrs{MD5: md5HashToBytes(image1Md5Hash)}
	mockBucketClient.On("GetFileObjectAttrs", testutils.AnyContext, image1GsPath).Return(oa1, nil)

	// digest1 is read.
	pngImage1Bytes := imageToPng(image1).Bytes()
	reader1 := ioutil.NopCloser(bytes.NewBuffer(pngImage1Bytes))
	mockBucketClient.On("FileReader", testutils.AnyContext, image1GsPath).Return(reader1, nil)

	// digest2 is present in the GCS bucket.
	oa2 := &storage.ObjectAttrs{MD5: md5HashToBytes(image2Md5Hash)}
	mockBucketClient.On("GetFileObjectAttrs", testutils.AnyContext, image2GsPath).Return(oa2, nil)

	// digest2 is read.
	reader2 := ioutil.NopCloser(imageToPng(image2))
	mockBucketClient.On("FileReader", testutils.AnyContext, image2GsPath).Return(reader2, nil)

	// digest16BitImage is present in the GCS bucket.
	oa3 := &storage.ObjectAttrs{MD5: md5HashToBytes(md5Hash16BitImage)}
	mockBucketClient.On("GetFileObjectAttrs", testutils.AnyContext, digest16BitImageGsPath).Return(oa3, nil)

	// digest16BitImage is read.
	bytes16BitImage, err := testutils.ReadFileBytes(fmt.Sprintf("%s.png", digest16BitImage))
	require.NoError(t, err)
	reader3 := ioutil.NopCloser(bytes.NewReader(bytes16BitImage))
	mockBucketClient.On("FileReader", testutils.AnyContext, digest16BitImageGsPath).Return(reader3, nil)

	// Dummy mapper.
	m := disk_mapper.New(&diff.DiffMetrics{})

	// Metrics store.
	mStore := &diffstore_mocks.MetricsStore{}

	// Build MemDiffStore instance under test.
	diffStore, err := NewMemDiffStore(mockBucketClient, gsImageBaseDir, 10, m, mStore, mockFailureStore)
	require.NoError(t, err)

	// Get the HTTP handler function under test.
	handlerFn, err := diffStore.ImageHandler("/img/")
	require.NoError(t, err)

	// Executes GET requests.
	get := func(urlFmt string, elem ...interface{}) *httptest.ResponseRecorder {
		url := fmt.Sprintf(urlFmt, elem...)

		// Create request.
		req, err := http.NewRequest("GET", url, nil)
		require.NoError(t, err)

		// Create the ResponseRecorder that will be returned after the request is served.
		rr := httptest.NewRecorder()

		// Serve request.
		handlerFn.ServeHTTP(rr, req)

		return rr
	}

	// Invalid digest.
	rr := get("/img/images/foo.png")
	require.Equal(t, http.StatusNotFound, rr.Code)
	require.Equal(t, "no-cache, no-store, must-revalidate", rr.Header().Get("Cache-Control"))

	// Missing digest.
	rr = get("/img/images/%s.png", missingDigest)
	require.Equal(t, http.StatusNotFound, rr.Code)
	require.Equal(t, "no-cache, no-store, must-revalidate", rr.Header().Get("Cache-Control"))

	// Image 1.
	rr = get("/img/images/%s.png", digest1)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, pngImage1Bytes, rr.Body.Bytes())
	require.Equal(t, "public, max-age=43200", rr.Header().Get("Cache-Control"))

	// Diff between images 1 and 2.
	rr = get("/img/diffs/%s-%s.png", digest1, digest2)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, imageToPng(skTextToImage(skTextDiffImages1And2)).Bytes(), rr.Body.Bytes())
	require.Equal(t, "public, max-age=43200", rr.Header().Get("Cache-Control"))

	// 16-bit image is returned verbatim as found in GCS. See skbug.com/9483 for more context.
	rr = get("/img/images/%s.png", digest16BitImage)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, bytes16BitImage, rr.Body.Bytes())
	require.Equal(t, "public, max-age=43200", rr.Header().Get("Cache-Control"))
}

// Allows for sorting slices of digests by the length (longer slices first)
type digestSliceSlice []types.DigestSlice

func (d digestSliceSlice) Len() int           { return len(d) }
func (d digestSliceSlice) Less(i, j int) bool { return len(d[i]) > len(d[j]) }
func (d digestSliceSlice) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
