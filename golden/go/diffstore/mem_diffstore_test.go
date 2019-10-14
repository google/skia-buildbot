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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/common"
	"go.skia.org/infra/golden/go/diffstore/metricsstore/bolt_metricsstore"
	diffstore_mocks "go.skia.org/infra/golden/go/diffstore/mocks"
	d_utils "go.skia.org/infra/golden/go/diffstore/testutils"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
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

	mStore, err := bolt_metricsstore.New(baseDir)
	require.NoError(t, err)
	// MemDiffStore is built with a nil FailureStore as it is not used by this test.
	diffStore, err := NewMemDiffStore(gcsClient, d_utils.TEST_GCS_IMAGE_DIR, 10, mStore, nil)
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
	memDiffStore.warmDigests(context.Background(), digests, true)

	for _, d := range digests {
		require.True(t, memDiffStore.imgLoader.Contains(d), fmt.Sprintf("Could not find '%s'", d))
	}

	// Warm the diffs and make sure they are in the cache.
	memDiffStore.warmDiffs(diff.PRIORITY_NOW, digests, digests)
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
		found, err := diffStore.Get(context.Background(), oneDigest, digests)
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
		found, err := diffStore.Get(context.Background(), oneDigest, digests)
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

const (
	invalidDigest1 = types.Digest("invaliddigest1")
	invalidDigest2 = types.Digest("invaliddigest2")
)

// TestFailureHandlingGet tests a case where two digests are not found in the GCS bucket.
func TestFailureHandlingGet(t *testing.T) {
	unittest.SmallTest(t)

	mms := &diffstore_mocks.MetricsStore{}
	mgc := test_gcsclient.NewMockClient()
	mfs := &diffstore_mocks.FailureStore{}
	defer mms.AssertExpectations(t)
	defer mgc.AssertExpectations(t)
	defer mfs.AssertExpectations(t)

	dm := &diff.DiffMetrics{
		// This data is arbitrary - just to make sure we get the right object
		MaxRGBADiffs: []int{1, 2, 3, 4},
	}

	mms.On("LoadDiffMetrics", common.DiffID(data.AlphaGood1Digest, data.BetaUntriaged1Digest)).Return(dm, nil)
	// Assume everything else is a cache miss
	mms.On("LoadDiffMetrics", mock.Anything).Return(nil, nil)

	// mgc succeeds for AlphaGood1Digest
	oa := &storage.ObjectAttrs{MD5: md5HashToBytes(image1Md5Hash)}
	img := fmt.Sprintf("%s/%s.png", d_utils.TEST_GCS_IMAGE_DIR, data.AlphaGood1Digest)
	mgc.On("GetFileObjectAttrs", testutils.AnyContext, img).Return(oa, nil)
	reader := ioutil.NopCloser(imageToPng(image1))
	mgc.On("FileReader", testutils.AnyContext, img).Return(reader, nil)

	// mgc should fail to return the invalid digests
	img = fmt.Sprintf("%s/%s.png", d_utils.TEST_GCS_IMAGE_DIR, invalidDigest1)
	mgc.On("GetFileObjectAttrs", testutils.AnyContext, img).Return(nil, errors.New("not found"))
	img = fmt.Sprintf("%s/%s.png", d_utils.TEST_GCS_IMAGE_DIR, invalidDigest2)
	mgc.On("GetFileObjectAttrs", testutils.AnyContext, img).Return(nil, errors.New("not found"))
	mgc.On("Bucket").Return("whatever")

	// FailureStore calls for invalid digest #1.
	mfs.On("AddDigestFailure", diffFailureMatcher(invalidDigest1, "http_error")).Return(nil)
	mfs.On("AddDigestFailureIfNew", diffFailureMatcher(invalidDigest1, "other")).Return(nil)

	// FailureStore calls for invalid digest #2.
	mfs.On("AddDigestFailure", diffFailureMatcher(invalidDigest2, "http_error")).Return(nil)
	mfs.On("AddDigestFailureIfNew", diffFailureMatcher(invalidDigest2, "other")).Return(nil)

	diffStore, err := NewMemDiffStore(mgc, d_utils.TEST_GCS_IMAGE_DIR, 1, mms, mfs)
	require.NoError(t, err)

	diffDigests := []types.Digest{data.BetaUntriaged1Digest, invalidDigest1, invalidDigest2}

	diffs, err := diffStore.Get(context.Background(), data.AlphaGood1Digest, diffDigests)
	require.NoError(t, err)
	assert.Len(t, diffs, 1)
	assert.Equal(t, dm, diffs[data.BetaUntriaged1Digest])
}

// TestGetUnavailable makes sure that memdiffstore shells out to the underlying failurestore
// for its Unavailable() call.
func TestGetUnavailable(t *testing.T) {
	unittest.SmallTest(t)

	mfs := &diffstore_mocks.FailureStore{}
	defer mfs.AssertExpectations(t)

	df := map[types.Digest]*diff.DigestFailure{
		invalidDigest1: {Digest: invalidDigest1, Reason: "http_error"},
		invalidDigest2: {Digest: invalidDigest2, Reason: "http_error"},
	}
	mfs.On("UnavailableDigests").Return(df).Once()

	// Everything but mfs is ignored for this test
	diffStore, err := NewMemDiffStore(nil, d_utils.TEST_GCS_IMAGE_DIR, 1, nil, mfs)
	require.NoError(t, err)

	unavailableDigests, err := diffStore.UnavailableDigests(context.Background())
	require.NoError(t, err)
	assert.Len(t, unavailableDigests, 2)
	assert.Equal(t, df, unavailableDigests)
}

// TestPurgeDigests makes we correctly purge digests from both metrics and GCS
func TestPurgeDigests(t *testing.T) {
	unittest.SmallTest(t)

	mms := &diffstore_mocks.MetricsStore{}
	mgc := test_gcsclient.NewMockClient()
	mfs := &diffstore_mocks.FailureStore{}
	defer mms.AssertExpectations(t)
	defer mgc.AssertExpectations(t)
	defer mfs.AssertExpectations(t)

	oa := &storage.ObjectAttrs{MD5: md5HashToBytes(image1Md5Hash)}
	img := fmt.Sprintf("%s/%s.png", d_utils.TEST_GCS_IMAGE_DIR, invalidDigest2)
	mgc.On("GetFileObjectAttrs", testutils.AnyContext, img).Return(oa, nil)
	mgc.On("DeleteFile", testutils.AnyContext, img).Return(nil)

	mms.On("PurgeDiffMetrics", types.DigestSlice{invalidDigest1}).Return(nil)
	mms.On("PurgeDiffMetrics", types.DigestSlice{invalidDigest2}).Return(nil)

	mfs.On("PurgeDigestFailures", types.DigestSlice{invalidDigest1}).Return(nil)
	mfs.On("PurgeDigestFailures", types.DigestSlice{invalidDigest2}).Return(nil)

	diffStore, err := NewMemDiffStore(mgc, d_utils.TEST_GCS_IMAGE_DIR, 1, mms, mfs)
	require.NoError(t, err)

	require.NoError(t, diffStore.PurgeDigests(context.Background(), types.DigestSlice{invalidDigest1}, false))
	require.NoError(t, diffStore.PurgeDigests(context.Background(), types.DigestSlice{invalidDigest2}, true))
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

	// Metrics store.
	mStore := &diffstore_mocks.MetricsStore{}

	// Build MemDiffStore instance under test.
	diffStore, err := NewMemDiffStore(mockBucketClient, gsImageBaseDir, 10, mStore, mockFailureStore)
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
