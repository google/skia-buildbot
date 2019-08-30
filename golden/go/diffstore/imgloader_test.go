package diffstore

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"io/ioutil"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diffstore/common"
	"go.skia.org/infra/golden/go/diffstore/mapper/disk_mapper"
	"go.skia.org/infra/golden/go/image/text"
	"go.skia.org/infra/golden/go/types"
)

const (
	gsImageBaseDir = "dm-images-v1"

	// These digests match the images below.
	digest1 = types.Digest("bde6b72edc996515916348e8f4dd406d")
	digest2 = types.Digest("96f28080f8cebfdb463bb00724aba779")

	image1GsPath = gsImageBaseDir + "/" + string(digest1) + ".png"
	image2GsPath = gsImageBaseDir + "/" + string(digest2) + ".png"

	skTextImage1 = `! SKTEXTSIMPLE
	1 5
	0x00000000
	0x01000000
	0x00010000
	0x00000100
	0x00000001`

	skTextImage2 = `! SKTEXTSIMPLE
	1 5
	0x01000000
	0x02000000
	0x00020000
	0x00000200
	0x00000002`
)

// These images (of type *image.NRGBA) are created from the SKTEXTSIMPLE images defined above, and
// are assumed to be used in a read-only manner throughout the tests.
var image1 = skTextToImage(skTextImage1)
var image2 = skTextToImage(skTextImage2)

// Convenience AnythingOfTypeArgument to keep mock.On(...) method calls short.
var anyCtx = mock.AnythingOfType("*context.emptyCtx")

func TestImageLoaderExpectedDigestsAreCorrect(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, imageToDigest(image1), digest1)
	assert.Equal(t, imageToDigest(image2), digest2)
}

// Sets up the mock GCSClient and temp folder for images, and returns the test ImageLoader instance.
func setUp(t *testing.T) (*ImageLoader, *test_gcsclient.MockGCSClient, func()) {
	// Create temporary directory.
	tmpDir, cleanup := testutils.TempDir(t)

	// Build mock GCSClient.
	mockBucketClient := test_gcsclient.NewMockClient()

	// Only used for logging errors, which only some tests produce.
	mockBucketClient.On("Bucket").Return("test-bucket").Maybe()

	// Compute an arbitrary cache size.
	imgCacheCount, _ := getCacheCounts(10)

	// Create the ImageLoader instance.
	imageLoader, err := NewImgLoader(mockBucketClient, tmpDir, gsImageBaseDir, imgCacheCount, &disk_mapper.DiskMapper{})
	assert.NoError(t, err)

	return imageLoader, mockBucketClient, cleanup
}

func TestImageLoaderGetSingleDigestFoundInBucket(t *testing.T) {
	unittest.SmallTest(t)
	imageLoader, mockClient, tearDown := setUp(t)
	defer tearDown()
	defer mockClient.AssertExpectations(t)

	// digest1 is present in the GCS bucket.
	oa := &storage.ObjectAttrs{MD5: digestToMD5Bytes(digest1)}
	mockClient.On("GetFileObjectAttrs", anyCtx, image1GsPath).Return(oa, nil)

	// digest1 is read.
	reader := ioutil.NopCloser(imageToPng(image1))
	mockClient.On("FileReader", anyCtx, image1GsPath).Return(reader, nil)

	// Get image.
	images, err := imageLoader.Get(1, types.DigestSlice{digest1})

	// Assert that the correct image was returned.
	assert.NoError(t, err)
	assert.Len(t, images, 1)
	assert.Equal(t, images[0], image1)
}

func TestImageLoaderGetSingleDigestNotFound(t *testing.T) {
	unittest.SmallTest(t)
	imageLoader, mockClient, tearDown := setUp(t)
	defer tearDown()
	defer mockClient.AssertExpectations(t)

	// digest1 is NOT present in the GCS bucket.
	var oa *storage.ObjectAttrs = nil
	mockClient.On("GetFileObjectAttrs", anyCtx, image1GsPath).Return(oa, errors.New("not found"))

	// Get images.
	_, err := imageLoader.Get(1, types.DigestSlice{digest1})

	// Assert that retrieval failed.
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unable to retrieve attributes")
}

func TestImageLoaderGetMultipleDigestsAllFoundInBucket(t *testing.T) {
	unittest.SmallTest(t)
	imageLoader, mockClient, tearDown := setUp(t)
	defer tearDown()
	defer mockClient.AssertExpectations(t)

	// digest1 is present in the GCS bucket.
	oa1 := &storage.ObjectAttrs{MD5: digestToMD5Bytes(digest1)}
	mockClient.On("GetFileObjectAttrs", anyCtx, image1GsPath).Return(oa1, nil)

	// digest1 is read.
	reader1 := ioutil.NopCloser(imageToPng(image1))
	mockClient.On("FileReader", anyCtx, image1GsPath).Return(reader1, nil)

	// digest2 is present in the GCS bucket.
	oa2 := &storage.ObjectAttrs{MD5: digestToMD5Bytes(digest2)}
	mockClient.On("GetFileObjectAttrs", anyCtx, image2GsPath).Return(oa2, nil)

	// digest2 is read.
	reader2 := ioutil.NopCloser(imageToPng(image2))
	mockClient.On("FileReader", anyCtx, image2GsPath).Return(reader2, nil)

	// Get images.
	images, err := imageLoader.Get(1, types.DigestSlice{digest1, digest2})

	// Assert that the correct images were returned.
	assert.NoError(t, err)
	assert.Len(t, images, 2)
	assert.Equal(t, images[0], image1)
	assert.Equal(t, images[1], image2)
}

func TestImageLoaderGetMultipleDigestsDigest1FoundInBucketDigest2NotFound(t *testing.T) {
	unittest.SmallTest(t)
	imageLoader, mockClient, tearDown := setUp(t)
	defer tearDown()
	defer mockClient.AssertExpectations(t)

	// digest1 is present in the GCS bucket.
	oa1 := &storage.ObjectAttrs{MD5: digestToMD5Bytes(digest1)}
	mockClient.On("GetFileObjectAttrs", anyCtx, image1GsPath).Return(oa1, nil)

	// digest1 is read.
	reader := ioutil.NopCloser(imageToPng(image1))
	mockClient.On("FileReader", anyCtx, image1GsPath).Return(reader, nil)

	// digest2 is NOT present in the GCS bucket.
	var oa2 *storage.ObjectAttrs = nil
	mockClient.On("GetFileObjectAttrs", anyCtx, image2GsPath).Return(oa2, errors.New("not found"))

	// Get images.
	_, err := imageLoader.Get(1, types.DigestSlice{digest1, digest2})

	// Assert that retrieval failed.
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unable to retrieve attributes")
}

func TestImageLoaderWarm(t *testing.T) {
	unittest.SmallTest(t)
	imageLoader, mockClient, tearDown := setUp(t)
	defer tearDown()

	// digest1 is present in the GCS bucket.
	oa1 := &storage.ObjectAttrs{MD5: digestToMD5Bytes(digest1)}
	mockClient.On("GetFileObjectAttrs", anyCtx, image1GsPath).Return(oa1, nil).
		Once() // This ensures that Get doesn't hit GCS after a call to Warm for the same digest.

	// digest1 is read.
	reader1 := ioutil.NopCloser(imageToPng(image1))
	mockClient.On("FileReader", anyCtx, image1GsPath).Return(reader1, nil).
		Once() // This ensures that Get doesn't hit GCS after a call to Warm for the same digest.

	// digest2 is present in the GCS bucket.
	oa2 := &storage.ObjectAttrs{MD5: digestToMD5Bytes(digest2)}
	mockClient.On("GetFileObjectAttrs", anyCtx, image2GsPath).Return(oa2, nil).
		Once() // This ensures that Get doesn't hit GCS after a call to Warm for the same digest.

	// digest2 is read.
	reader2 := ioutil.NopCloser(imageToPng(image2))
	mockClient.On("FileReader", anyCtx, image2GsPath).Return(reader2, nil).
		Once() // This ensures that Get doesn't hit GCS after a call to Warm for the same digest.

	// Fetch both images from GCS and cache them in memory.
	imageLoader.Warm(1, types.DigestSlice{digest1, digest2}, true)

	// Assert that the mocked methods were called as expected.
	mockClient.AssertExpectations(t)

	// Assert that the images are in the cache.
	assert.True(t, imageLoader.Contains(digest1))
	assert.True(t, imageLoader.Contains(digest2))

	// Get cached images from memory. This shouldn't hit GCS. If it does, the mockClient will panic
	// as per the Once() calls.
	images, err := imageLoader.Get(1, types.DigestSlice{digest1, digest2})

	// Assert that the correct images were returned.
	assert.NoError(t, err)
	assert.Len(t, images, 2)
	assert.Equal(t, images[0], image1)
	assert.Equal(t, images[1], image2)
}

// TODO(lovisolo): Add test cases for multiple digests, and decide what to do about purgeGCS=false.
func TestImageLoaderPurgeImages(t *testing.T) {
	unittest.SmallTest(t)
	imageLoader, mockClient, tearDown := setUp(t)
	defer tearDown()

	// digest1 is present in the GCS bucket.
	oa := &storage.ObjectAttrs{MD5: digestToMD5Bytes(digest1)}
	mockClient.On("GetFileObjectAttrs", anyCtx, image1GsPath).Return(oa, nil)

	// digest1 is read.
	reader1 := ioutil.NopCloser(imageToPng(image1))
	mockClient.On("FileReader", anyCtx, image1GsPath).Return(reader1, nil)

	// Fetch digest from GCS and and cache it in memory.
	imageLoader.Warm(1, types.DigestSlice{digest1}, true)

	// Assert that the image is in the cache.
	assert.True(t, imageLoader.Contains(digest1))

	// digest1 is deleted.
	mockClient.On("DeleteFile", anyCtx, image1GsPath).Return(nil)

	// Purge image.
	err := imageLoader.PurgeImages(types.DigestSlice{digest1}, true)
	assert.NoError(t, err)

	// Assert that the image was removed from the cache.
	assert.False(t, imageLoader.Contains(digest1))
}

// Decodes an SKTEXTSIMPLE image.
func skTextToImage(s string) *image.NRGBA {
	buf := bytes.NewBufferString(s)
	img, err := text.Decode(buf)
	if err != nil {
		// This indicates an error with the static test data which is initialized before executing the
		// tests, thus we panic instead of asserting the absence of errors with assert.NoError.
		panic(fmt.Sprintf("Failed to decode a valid image: %s", err))
	}
	return img.(*image.NRGBA)
}

// Takes an image and returns a PNG-encoded bytes.Buffer.
func imageToPng(image *image.NRGBA) *bytes.Buffer {
	buf := new(bytes.Buffer)
	err := common.EncodeImg(buf, image)
	if err != nil {
		// This indicates an error with the static test data which is initialized before executing the
		// tests, thus we panic instead of asserting the absence of errors with assert.NoError.
		panic(fmt.Sprintf("Failed to encode image as PNG: %s", err))
	}
	return buf
}

// Takes an image and returns a string with its MD5 hash.
func imageToDigest(image *image.NRGBA) types.Digest {
	md5 := md5.New()
	md5.Write(imageToPng(image).Bytes())
	return types.Digest(hex.EncodeToString(md5.Sum(nil)))
}

// Takes a string with an MD5 hash and encodes it as a byte array.
func digestToMD5Bytes(digest types.Digest) []byte {
	bytes, err := hex.DecodeString(string(digest))
	if err != nil {
		// This indicates an error with the static test data which is initialized before executing the
		// tests, thus we panic instead of asserting the absence of errors with assert.NoError.
		panic(fmt.Sprintf("Failed to encode digest as MD5 bytes: %s", err))
	}
	return bytes
}

func TestGetGSRelPath(t *testing.T) {
	unittest.SmallTest(t)

	digest := types.Digest("098f6bcd4621d373cade4e832627b4f6")
	expectedGSPath := string(digest + ".png")
	gsPath := getGSRelPath(digest)
	assert.Equal(t, expectedGSPath, gsPath)
}
