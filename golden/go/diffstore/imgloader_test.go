package diffstore

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"io"
	"io/ioutil"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/common"
	"go.skia.org/infra/golden/go/image/text"
	one_by_five "go.skia.org/infra/golden/go/testutils/data_one_by_five"
	"go.skia.org/infra/golden/go/types"
)

const (
	gcsImageBaseDir = "dm-images-v1"

	// These digests correspond to the images below and are arbitrarily chosen.
	digest1 = types.Digest("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	digest2 = types.Digest("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	digest3 = types.Digest("cccccccccccccccccccccccccccccccc")

	// Fake GCS paths to the test images.
	image1GCSPath = gcsImageBaseDir + "/" + string(digest1) + ".png"
	image2GCSPath = gcsImageBaseDir + "/" + string(digest2) + ".png"
	image3GCSPath = gcsImageBaseDir + "/" + string(digest3) + ".png"

	// MD5 hashes of the PNG files (not to be confused with the Digest of these images, which is
	// the MD5 hash of the pixel values).
	image1MD5Hash = "bde6b72edc996515916348e8f4dd406d" // = md5sum(aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png)
	image2MD5Hash = "96f28080f8cebfdb463bb00724aba779" // = md5sum(bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.png)
	image3MD5Hash = "fb463a46f01baa8ef0b9d1d87ba6e421" // = md5sum(cccccccccccccccccccccccccccccccc.png)
)

// These images (of type *image.NRGBA) are assumed to be used in a read-only manner throughout
// the tests.
var image1 = text.MustToNRGBA(one_by_five.ImageOne)
var image2 = text.MustToNRGBA(one_by_five.ImageTwo)
var image3 = text.MustToNRGBA(one_by_five.ImageSix)

func TestImageLoaderExpectedMd5HashesAreCorrect(t *testing.T) {
	unittest.SmallTest(t)
	require.Equal(t, bytesToMD5HashString(imageToPng(image1).Bytes()), image1MD5Hash)
	require.Equal(t, bytesToMD5HashString(imageToPng(image2).Bytes()), image2MD5Hash)
	require.Equal(t, bytesToMD5HashString(imageToPng(image3).Bytes()), image3MD5Hash)
}

// Sets up the mock GCSClient and temp folder for images, and returns the test ImageLoader instance.
func setUp(t *testing.T) (*ImageLoader, *test_gcsclient.GCSClient) {
	// Build mock GCSClient.
	mockBucketClient := test_gcsclient.NewMockClient()

	// Only used for logging errors, which only some tests produce.
	mockBucketClient.On("Bucket").Return("test-bucket").Maybe()

	// Compute an arbitrary cache size.
	imgCacheCount, _ := getCacheCounts(10)

	// Create the ImageLoader instance.
	imageLoader, err := NewImgLoader(mockBucketClient, gcsImageBaseDir, imgCacheCount)
	require.NoError(t, err)

	return imageLoader, mockBucketClient
}

func TestImageLoaderGetSingleDigestFoundInBucket(t *testing.T) {
	unittest.SmallTest(t)
	imageLoader, mockClient := setUp(t)

	// digest1 is present in the GCS bucket.
	expectImageWillBeRead(mockClient, image1GCSPath, image1MD5Hash, image1)

	// Get image.
	images, err := imageLoader.Get(context.Background(), types.DigestSlice{digest1})

	// Assert that the correct image was returned.
	require.NoError(t, err)
	require.Len(t, images, 1)
	require.Equal(t, images[0], imageToPng(image1).Bytes())
}

func TestImageLoaderGetSingleDigestNotFound(t *testing.T) {
	unittest.SmallTest(t)
	imageLoader, mockClient := setUp(t)

	// digest1 is NOT present in the GCS bucket.
	var oa *storage.ObjectAttrs
	mockClient.On("GetFileObjectAttrs", testutils.AnyContext, image1GCSPath).Return(oa, errors.New("not found"))

	// Get images.
	_, err := imageLoader.Get(context.Background(), types.DigestSlice{digest1})

	// Assert that retrieval failed.
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unable to retrieve attributes")
}

func TestImageLoaderGetMultipleDigestsAllFoundInBucket(t *testing.T) {
	unittest.SmallTest(t)
	imageLoader, mockClient := setUp(t)

	// digest1 and digest2 are present in the GCS bucket.
	expectImageWillBeRead(mockClient, image1GCSPath, image1MD5Hash, image1)
	expectImageWillBeRead(mockClient, image2GCSPath, image2MD5Hash, image2)

	// Get images.
	images, err := imageLoader.Get(context.Background(), types.DigestSlice{digest1, digest2})

	// Assert that the correct images were returned.
	require.NoError(t, err)
	require.Len(t, images, 2)
	require.Equal(t, images[0], imageToPng(image1).Bytes())
	require.Equal(t, images[1], imageToPng(image2).Bytes())
}

func TestImageLoaderGetMultipleDigestsDigest1FoundInBucketDigest2NotFound(t *testing.T) {
	unittest.SmallTest(t)
	imageLoader, mockClient := setUp(t)

	// digest1 is present in the GCS bucket.
	expectImageWillBeRead(mockClient, image1GCSPath, image1MD5Hash, image1)

	// digest2 is NOT present in the GCS bucket.
	var oa2 *storage.ObjectAttrs = nil
	mockClient.On("GetFileObjectAttrs", testutils.AnyContext, image2GCSPath).Return(oa2, errors.New("not found"))

	// Get images.
	_, err := imageLoader.Get(context.Background(), types.DigestSlice{digest1, digest2})

	// Assert that retrieval failed.
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unable to retrieve attributes")
}

// TODO(lovisolo): Add test cases for multiple digests, and decide what to do about purgeGCS=false.
func TestImageLoaderPurgeImages(t *testing.T) {
	unittest.SmallTest(t)
	imageLoader, mockClient := setUp(t)

	defer mockClient.AssertExpectations(t)

	// digest1 is present in the GCS bucket.
	expectImageWillBeRead(mockClient, image1GCSPath, image1MD5Hash, image1)

	// Fetch digest from GCS and and cache it in memory.
	_, err := imageLoader.Get(context.Background(), types.DigestSlice{digest1})
	require.NoError(t, err)

	// Assert that the image is in the cache.
	require.True(t, imageLoader.Contains(digest1))

	// digest1 is deleted.
	mockClient.On("DeleteFile", testutils.AnyContext, image1GCSPath).Return(nil)

	// Purge image.
	err = imageLoader.purgeImages(context.Background(), types.DigestSlice{digest1}, true)
	require.NoError(t, err)

	// Assert that the image was removed from the cache.
	require.False(t, imageLoader.Contains(digest1))
}

// Takes an image and returns a PNG-encoded bytes.Buffer.
func imageToPng(image *image.NRGBA) *bytes.Buffer {
	buf := &bytes.Buffer{}
	err := common.EncodeImg(buf, image)
	if err != nil {
		// This indicates an error with the static test data which is initialized before executing the
		// tests, thus we panic instead of asserting the absence of errors with require.NoError.
		panic(fmt.Sprintf("Failed to encode image as PNG: %s", err))
	}
	return buf
}

// Takes a byte slice and returns its MD5 hash as a human-readable string.
func bytesToMD5HashString(bytes []byte) string {
	m := md5.New()
	m.Write(bytes)
	return hex.EncodeToString(m.Sum(nil))
}

// Takes a string with an MD5 hash and encodes it as a byte array.
func md5HashToBytes(md5Hash string) []byte {
	b, err := hex.DecodeString(md5Hash)
	if err != nil {
		// This indicates an error with the static test data which is initialized before executing the
		// tests, thus we panic instead of asserting the absence of errors with require.NoError.
		panic(fmt.Sprintf("Failed to encode digest as MD5 bytes: %s", err))
	}
	return b
}

// diffFailureMatcher is necessary due to the timestamp stored in field DigestFailure.TS.
func diffFailureMatcher(digest types.Digest, reason diff.DiffErr) interface{} {
	return mock.MatchedBy(func(failure *diff.DigestFailure) bool {
		return failure.Digest == digest && failure.Reason == reason
	})
}

func TestGetGSRelPath(t *testing.T) {
	unittest.SmallTest(t)

	digest := types.Digest("098f6bcd4621d373cade4e832627b4f6")
	expectedGSPath := string(digest + ".png")
	gsPath := getGCSRelPath(digest)
	require.Equal(t, expectedGSPath, gsPath)
}

// expectImageWillBeRead adds the mocked expectations for reading the given image with the
// given MD5 hash and will provide the given image data.
func expectImageWillBeRead(mgc *test_gcsclient.GCSClient, gcsPath, hash string, img *image.NRGBA) {
	oa := &storage.ObjectAttrs{MD5: md5HashToBytes(hash)}
	mgc.On("GetFileObjectAttrs", testutils.AnyContext, gcsPath).Return(oa, nil)
	// By making this a function, FileReader can safely be called more than once.
	newReader := func(context.Context, string) io.ReadCloser {
		return ioutil.NopCloser(imageToPng(img))
	}
	mgc.On("FileReader", testutils.AnyContext, gcsPath).Return(newReader, nil)
}
