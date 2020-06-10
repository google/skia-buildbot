package diffstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/common"
	"go.skia.org/infra/golden/go/diffstore/metricsstore/fs_metricsstore"
	diffstore_mocks "go.skia.org/infra/golden/go/diffstore/mocks"
	"go.skia.org/infra/golden/go/image/text"
	one_by_five "go.skia.org/infra/golden/go/testutils/data_one_by_five"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Missing digest (valid, but arbitrary).
	missingDigest       = types.Digest("ffffffffffffffffffffffffffffffff")
	missingDigestGsPath = gcsImageBaseDir + "/" + string(missingDigest) + ".png"

	// Digest for the 16-bit image stored in the testdata directory. This is the same image used to
	// test preservation of color space information in skbug.com/9483.
	digest16BitImage       = types.Digest("8a90a2f1245dc87d96bfb74bdc4ab97e")
	digest16BitImageGsPath = gcsImageBaseDir + "/" + string(digest16BitImage) + ".png"

	// MD5 hash of the 16-bit PNG image above. Needed for the storage.ObjectAttrs instance returned by
	// the mock GCS client.
	md5Hash16BitImage = "22ea2cb4e3eabd2bb3faba7a07e18b7a" // = md5sum(8a90a2f1245dc87d96bfb74bdc4ab97e.png)

	invalidDigest1 = types.Digest("invaliddigest1")
	invalidDigest2 = types.Digest("invaliddigest2")

	gcsTestBucket = "skia-infra-testdata"
)

func TestMD5Hash16BitImageIsCorrect(t *testing.T) {
	unittest.SmallTest(t)
	b, err := testutils.ReadFileBytes(fmt.Sprintf("%s.png", digest16BitImage))
	require.NoError(t, err)
	require.Equal(t, md5Hash16BitImage, bytesToMD5HashString(b))
}

// TestMemDiffStoreGetSunnyDay tests the case where we are getting metrics for two digests
// that are both cache misses.
func TestMemDiffStoreGetSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mms := &diffstore_mocks.MetricsStore{}
	mgc := test_gcsclient.NewMockClient()
	defer mms.AssertExpectations(t)
	defer mgc.AssertExpectations(t)

	// These diffs are the actual diffs between the respective 2 images.
	// These values were computed by using the default algorithm and manual inspection.
	dm1_2 := &diff.DiffMetrics{
		NumDiffPixels:    5,
		PixelDiffPercent: 100,
		MaxRGBADiffs:     [4]int{1, 1, 1, 1},
		Diffs: map[string]float32{
			"combined": 0.6262243,
			"percent":  100,
			"pixel":    5,
		},
	}
	dm1_3 := &diff.DiffMetrics{
		NumDiffPixels:    4,
		PixelDiffPercent: 80,
		MaxRGBADiffs:     [4]int{2, 0, 1, 2},
		Diffs: map[string]float32{
			"combined": 0.6859943,
			"percent":  80,
			"pixel":    4,
		},
	}

	// Assume everything is a cache miss
	expectedDiffIDs := []string{common.DiffID(digest1, digest2), common.DiffID(digest1, digest3)}
	mms.On("LoadDiffMetrics", testutils.AnyContext, expectedDiffIDs).Return([]*diff.DiffMetrics{nil, nil}, nil)

	expectImageWillBeRead(mgc, image1GCSPath, image1MD5Hash, image1)
	expectImageWillBeRead(mgc, image2GCSPath, image2MD5Hash, image2)
	expectImageWillBeRead(mgc, image3GCSPath, image3MD5Hash, image3)

	mms.On("SaveDiffMetrics", testutils.AnyContext, common.DiffID(digest1, digest2), dm1_2).Return(nil)
	mms.On("SaveDiffMetrics", testutils.AnyContext, common.DiffID(digest1, digest3), dm1_3).Return(nil)

	diffStore, err := NewMemDiffStore(mgc, gcsImageBaseDir, 1, mms)
	require.NoError(t, err)

	diffDigests := []types.Digest{digest2, digest3}

	diffs, err := diffStore.Get(context.Background(), digest1, diffDigests)
	require.NoError(t, err)
	assert.Len(t, diffs, 2)
	assert.Equal(t, dm1_2, diffs[digest2])
	assert.Equal(t, dm1_3, diffs[digest3])
}

// TestMemDiffStoreGetIntegration performs the Get operation backed by real GCS and a firestore
// emulator.
func TestMemDiffStoreGetIntegration(t *testing.T) {
	unittest.LargeTest(t)

	// The test bucket is a public bucket, so we don't need to worry about authentication.
	unauthedClient := httputils.DefaultClientConfig().Client()

	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(unauthedClient))
	require.NoError(t, err)
	gcsClient := gcsclient.New(storageClient, gcsTestBucket)

	// create a client against the firestore emulator.
	c, cleanup := firestore.NewClientForTesting(context.Background(), t)
	defer cleanup()
	fsMetrics := fs_metricsstore.New(c)

	// These are two nearly identical images in the skia-infra-testdata bucket.
	// The names are arbitrary (they don't actually correspond with the hash of the pixels).
	original := types.Digest("000da2ce46164b5027ee964b8c040335")
	cross := types.Digest("cccd4f34d847bd8a540c7c9cf1602107")

	// There are 5 pixels in the cross image that are black instead of white.
	// These values were computed by using the default algorithm and manual inspection.
	dm := &diff.DiffMetrics{
		NumDiffPixels:    5,
		PixelDiffPercent: 0.0010146104,
		MaxRGBADiffs:     [4]int{255, 255, 255, 0},
		Diffs: map[string]float32{
			"combined": 0.02964251,
			"percent":  0.0010146104,
			"pixel":    5,
		},
	}

	diffStore, err := NewMemDiffStore(gcsClient, gcsImageBaseDir, 1, fsMetrics)
	require.NoError(t, err)

	diffDigests := []types.Digest{cross}

	diffs, err := diffStore.Get(context.Background(), original, diffDigests)
	require.NoError(t, err)
	assert.Len(t, diffs, 1)
	assert.Equal(t, dm, diffs[cross])

	// make sure they are actually stored

	actual, err := fsMetrics.LoadDiffMetrics(context.Background(), []string{common.DiffID(original, cross)})
	require.NoError(t, err)
	assert.Equal(t, dm, actual[0])
}

// TestMemDiffStoreGetPartialCacheMatch tests the case where we are getting metrics for two digests
// and one is a cache hit and the other is a cache miss.
func TestMemDiffStoreGetPartialCacheMatch(t *testing.T) {
	unittest.SmallTest(t)

	mms := &diffstore_mocks.MetricsStore{}
	mgc := test_gcsclient.NewMockClient()
	defer mms.AssertExpectations(t)
	defer mgc.AssertExpectations(t)

	// These diffs are the actual diffs between the respective 2 images.
	// These values were computed by using the default algorithm and manual inspection.
	dm1_2 := &diff.DiffMetrics{
		NumDiffPixels:    5,
		PixelDiffPercent: 100,
		MaxRGBADiffs:     [4]int{1, 1, 1, 1},
		Diffs: map[string]float32{
			"combined": 0.6262243,
			"percent":  100,
			"pixel":    5,
		},
	}
	dm1_3 := &diff.DiffMetrics{
		NumDiffPixels:    4,
		PixelDiffPercent: 80,
		MaxRGBADiffs:     [4]int{2, 0, 1, 2},
		Diffs: map[string]float32{
			"combined": 0.6859943,
			"percent":  80,
			"pixel":    4,
		},
	}

	// One cache hit, one cache miss.
	expectedDiffIDs := []string{common.DiffID(digest1, digest2), common.DiffID(digest1, digest3)}
	mms.On("LoadDiffMetrics", testutils.AnyContext, expectedDiffIDs).Return([]*diff.DiffMetrics{dm1_2, nil}, nil)

	expectImageWillBeRead(mgc, image1GCSPath, image1MD5Hash, image1)
	expectImageWillBeRead(mgc, image3GCSPath, image3MD5Hash, image3)

	mms.On("SaveDiffMetrics", testutils.AnyContext, common.DiffID(digest1, digest3), dm1_3).Return(nil)

	diffStore, err := NewMemDiffStore(mgc, gcsImageBaseDir, 1, mms)
	require.NoError(t, err)

	diffDigests := []types.Digest{digest2, digest3}

	diffs, err := diffStore.Get(context.Background(), digest1, diffDigests)
	require.NoError(t, err)
	assert.Len(t, diffs, 2)
	assert.Equal(t, dm1_2, diffs[digest2])
	assert.Equal(t, dm1_3, diffs[digest3])
}

// TestMemDiffStoreGetIdentity tests the case where the mainDigest is in the list of things
// to diff against.
func TestMemDiffStoreGetIdentity(t *testing.T) {
	unittest.SmallTest(t)

	mms := &diffstore_mocks.MetricsStore{}
	mgc := test_gcsclient.NewMockClient()
	defer mms.AssertExpectations(t)
	defer mgc.AssertExpectations(t)

	// These diffs are the actual diffs between the respective 2 images.
	// These values were computed by using the default algorithm and manual inspection.
	dm1_2 := &diff.DiffMetrics{
		NumDiffPixels:    5,
		PixelDiffPercent: 100,
		MaxRGBADiffs:     [4]int{1, 1, 1, 1},
		Diffs: map[string]float32{
			"combined": 0.6262243,
			"percent":  100,
			"pixel":    5,
		},
	}
	dm1_3 := &diff.DiffMetrics{
		NumDiffPixels:    4,
		PixelDiffPercent: 80,
		MaxRGBADiffs:     [4]int{2, 0, 1, 2},
		Diffs: map[string]float32{
			"combined": 0.6859943,
			"percent":  80,
			"pixel":    4,
		},
	}

	// Assume everything is a cache hit
	expectedDiffIDs := []string{common.DiffID(digest1, digest2), common.DiffID(digest1, digest3)}
	mms.On("LoadDiffMetrics", testutils.AnyContext, expectedDiffIDs).Return([]*diff.DiffMetrics{dm1_2, dm1_3}, nil)

	diffStore, err := NewMemDiffStore(mgc, gcsImageBaseDir, 1, mms)
	require.NoError(t, err)

	diffDigests := []types.Digest{digest2, digest1, digest3}

	diffs, err := diffStore.Get(context.Background(), digest1, diffDigests)
	require.NoError(t, err)
	assert.Len(t, diffs, 2)
	assert.Equal(t, dm1_2, diffs[digest2])
	assert.Equal(t, dm1_3, diffs[digest3])
}

// TestFailureHandlingGet tests a case where two digests are not found in the GCS bucket.
func TestFailureHandlingGet(t *testing.T) {
	unittest.SmallTest(t)

	mms := &diffstore_mocks.MetricsStore{}
	mgc := test_gcsclient.NewMockClient()
	defer mms.AssertExpectations(t)
	defer mgc.AssertExpectations(t)

	dm := &diff.DiffMetrics{
		// This data is arbitrary - just to make sure we get the right object
		MaxRGBADiffs: [4]int{1, 2, 3, 4},
	}

	expectedDiffIDs := []string{common.DiffID(digest1, digest2), common.DiffID(digest1, invalidDigest1), common.DiffID(digest1, invalidDigest2)}
	mms.On("LoadDiffMetrics", testutils.AnyContext, expectedDiffIDs).Return([]*diff.DiffMetrics{dm, nil, nil}, nil)

	// mgc succeeds for digest1 (which is loaded anyway in an attempt to compare against the two
	// invalid digests).
	expectImageWillBeRead(mgc, image1GCSPath, image1MD5Hash, image1)

	// mgc should fail to return the invalid digest (the first of which should cause the rest to fail.
	img := fmt.Sprintf("%s/%s.png", gcsImageBaseDir, invalidDigest1)
	mgc.On("GetFileObjectAttrs", testutils.AnyContext, img).Return(nil, errors.New("not found"))
	mgc.On("Bucket").Return("whatever")

	diffStore, err := NewMemDiffStore(mgc, gcsImageBaseDir, 1, mms)
	require.NoError(t, err)

	diffDigests := []types.Digest{digest2, invalidDigest1, invalidDigest2}

	_, err = diffStore.Get(context.Background(), digest1, diffDigests)
	require.Error(t, err)
}

// TestMetricsStoreFlakiness tests that we still can compute diffs if the metricStore is down.
func TestMetricsStoreFlakiness(t *testing.T) {
	unittest.SmallTest(t)

	mms := &diffstore_mocks.MetricsStore{}
	mgc := test_gcsclient.NewMockClient()
	defer mms.AssertExpectations(t)
	defer mgc.AssertExpectations(t)

	// These diffs are the actual diffs between the respective 2 images.
	// These values were computed by using the default algorithm and manual inspection.
	dm1_2 := &diff.DiffMetrics{
		NumDiffPixels:    5,
		PixelDiffPercent: 100,
		MaxRGBADiffs:     [4]int{1, 1, 1, 1},
		Diffs: map[string]float32{
			"combined": 0.6262243,
			"percent":  100,
			"pixel":    5,
		},
	}

	// Assume we can't read or write to metricsstore
	mms.On("LoadDiffMetrics", testutils.AnyContext, mock.Anything).Return(nil, errors.New("out of quota"))
	mms.On("SaveDiffMetrics", testutils.AnyContext, mock.Anything, mock.Anything).Return(errors.New("out of quota"))

	expectImageWillBeRead(mgc, image1GCSPath, image1MD5Hash, image1)
	expectImageWillBeRead(mgc, image2GCSPath, image2MD5Hash, image2)

	diffStore, err := NewMemDiffStore(mgc, gcsImageBaseDir, 1, mms)
	require.NoError(t, err)

	diffDigests := []types.Digest{digest2}

	diffs, err := diffStore.Get(context.Background(), digest1, diffDigests)
	require.NoError(t, err)
	assert.Len(t, diffs, 1)
	assert.Equal(t, dm1_2, diffs[digest2])
}

// TestPurgeDigests makes we correctly purge digests from both metrics and GCS
func TestPurgeDigests(t *testing.T) {
	unittest.SmallTest(t)

	mms := &diffstore_mocks.MetricsStore{}
	mgc := test_gcsclient.NewMockClient()
	defer mms.AssertExpectations(t)
	defer mgc.AssertExpectations(t)

	oa := &storage.ObjectAttrs{}
	img := fmt.Sprintf("%s/%s.png", gcsImageBaseDir, invalidDigest2)
	mgc.On("GetFileObjectAttrs", testutils.AnyContext, img).Return(oa, nil)
	mgc.On("DeleteFile", testutils.AnyContext, img).Return(nil)

	mms.On("PurgeDiffMetrics", testutils.AnyContext, types.DigestSlice{invalidDigest1}).Return(nil)
	mms.On("PurgeDiffMetrics", testutils.AnyContext, types.DigestSlice{invalidDigest2}).Return(nil)

	diffStore, err := NewMemDiffStore(mgc, gcsImageBaseDir, 1, mms)
	require.NoError(t, err)

	require.NoError(t, diffStore.PurgeDigests(context.Background(), types.DigestSlice{invalidDigest1}, false))
	require.NoError(t, diffStore.PurgeDigests(context.Background(), types.DigestSlice{invalidDigest2}, true))
}

func TestMemDiffStoreImageHandler(t *testing.T) {
	unittest.SmallTest(t)

	// This is a white-box test. The mock GCS client below return only what is
	// needed for this test to pass, and nothing more (e.g. field "MD5" in *storage.ObjectAttrs).

	// Build mock GCSClient.
	mockBucketClient := test_gcsclient.NewMockClient()
	defer mockBucketClient.AssertExpectations(t)

	// Only used for logging errors, which only some tests produce.
	mockBucketClient.On("Bucket").Return("test-bucket")

	// missingDigest is not present in the GCS bucket.
	var oa *storage.ObjectAttrs
	mockBucketClient.On("GetFileObjectAttrs", testutils.AnyContext, missingDigestGsPath).Return(oa, errors.New("not found"))

	// digest1 is present in the GCS bucket.
	oa1 := &storage.ObjectAttrs{MD5: md5HashToBytes(image1MD5Hash)}
	mockBucketClient.On("GetFileObjectAttrs", testutils.AnyContext, image1GCSPath).Return(oa1, nil)

	// digest1 is read.
	pngImage1Bytes := imageToPng(image1).Bytes()
	reader1 := ioutil.NopCloser(bytes.NewBuffer(pngImage1Bytes))
	mockBucketClient.On("FileReader", testutils.AnyContext, image1GCSPath).Return(reader1, nil)

	// digest2 is present in the GCS bucket.
	oa2 := &storage.ObjectAttrs{MD5: md5HashToBytes(image2MD5Hash)}
	mockBucketClient.On("GetFileObjectAttrs", testutils.AnyContext, image2GCSPath).Return(oa2, nil)

	// digest2 is read.
	reader2 := ioutil.NopCloser(imageToPng(image2))
	mockBucketClient.On("FileReader", testutils.AnyContext, image2GCSPath).Return(reader2, nil)

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
	diffStore, err := NewMemDiffStore(mockBucketClient, gcsImageBaseDir, 10, mStore)
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
	d := text.MustToNRGBA(one_by_five.DiffImageOneAndTwo)
	require.Equal(t, imageToPng(d).Bytes(), rr.Body.Bytes())
	require.Equal(t, "public, max-age=43200", rr.Header().Get("Cache-Control"))

	// 16-bit image is returned verbatim as found in GCS. See skbug.com/9483 for more context.
	rr = get("/img/images/%s.png", digest16BitImage)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, bytes16BitImage, rr.Body.Bytes())
	require.Equal(t, "public, max-age=43200", rr.Header().Get("Cache-Control"))
}

func TestDecodeImageSuccess(t *testing.T) {
	unittest.SmallTest(t)

	// Inputs.
	b := imageToPng(image1).Bytes()

	actual, err := common.DecodeImg(bytes.NewReader(b))
	require.NoError(t, err)
	require.Equal(t, image1, actual)
}

func TestDecodeImagesInvalid(t *testing.T) {
	unittest.SmallTest(t)

	b := []byte("I'm not a PNG image")

	_, err := common.DecodeImg(bytes.NewReader(b))
	require.Error(t, err)
}
