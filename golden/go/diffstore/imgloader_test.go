package diffstore

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"image"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diffstore/common"
	"go.skia.org/infra/golden/go/diffstore/mapper/disk_mapper"
	"go.skia.org/infra/golden/go/image/text"
	"go.skia.org/infra/golden/go/types"
)

const (
	GsImageBaseDir = "dm-images-v1"

	Image1GsPath = GsImageBaseDir + "/" + string(Digest1) + ".png"
	Image2GsPath = GsImageBaseDir + "/" + string(Digest2) + ".png"

	Digest1 = types.Digest("bde6b72edc996515916348e8f4dd406d")
	Digest2 = types.Digest("96f28080f8cebfdb463bb00724aba779")

	SkTextImage1 = `! SKTEXTSIMPLE
	1 5
	0x00000000
	0x01000000
	0x00010000
	0x00000100
	0x00000001`

	SkTextImage2 = `! SKTEXTSIMPLE
	1 5
	0x01000000
	0x02000000
	0x00020000
	0x00000200
	0x00000002`
)

func TestImageLoader_testData_assertDigestsAreCorrect(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, skTextToDigest(t, SkTextImage1), Digest1)
	assert.Equal(t, skTextToDigest(t, SkTextImage2), Digest2)
}

func setUp(t *testing.T) (*ImageLoader, *test_gcsclient.MockGCSClient, func()) {
	// Create temporary directories.
	tmpDir, cleanup := testutils.TempDir(t)
	imgDir := filepath.Join(tmpDir, "images")
	assert.Nil(t, os.Mkdir(imgDir, 0777))

	// Build mock GCSClient.
	mockBucketClient := test_gcsclient.NewMockClient()
	mockBucketClient.On("Bucket").Return("FooBucket")

	// Compute an arbitrary cache size.
	imgCacheCount, _ := getCacheCounts(10)

	// Create the ImageLoader instance.
	imageLoader, err := NewImgLoader(mockBucketClient, tmpDir, imgDir, GsImageBaseDir, imgCacheCount, &disk_mapper.DiskMapper{})
	assert.NoError(t, err)

	return imageLoader, mockBucketClient, cleanup
}

func TestImageLoader_Get_singleDigest_foundInBucket(t *testing.T) {
	unittest.SmallTest(t)

	imageLoader, mockClient, tearDown := setUp(t)
	defer tearDown()

	assertImagesDirExistsAndIsEmpty(t, imageLoader)

	// Digest1 is present in the GCS bucket.
	mockClient.On(
		"GetFileObjectAttrs",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath,
	).Return(&storage.ObjectAttrs{MD5: digestToMD5Bytes(t, Digest1)}, nil)

	// Digest1 is read.
	mockClient.On(
		"FileReader",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath,
	).Return(ioutil.NopCloser(skTextToPng(t, SkTextImage1)), nil)

	// Get image and persist it to disk.
	images, pendingWritesWG, _ := imageLoader.Get(1, types.DigestSlice{Digest1})
	pendingWritesWG.Wait()

	// Assert that the correct image was returned and persisted to disk.
	assert.Equal(t, len(images), 1)
	assertImageEqual(t, images[0], skTextToImage(t, SkTextImage1))
	assertImageCorrectlyPersistedToDisk(t, imageLoader, Digest1, skTextToImage(t, SkTextImage1))
}

func TestImageLoader_Get_singleDigest_notFound(t *testing.T) {
	unittest.SmallTest(t)

	imageLoader, mockClient, tearDown := setUp(t)
	defer tearDown()

	assertImagesDirExistsAndIsEmpty(t, imageLoader)

	// Digest1 is NOT present in the GCS bucket.
	mockClient.On(
		"GetFileObjectAttrs",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath,
	).Return(&storage.ObjectAttrs{}, errors.New("not found"))

	// Get images.
	_, _, err := imageLoader.Get(1, types.DigestSlice{Digest1})

	// Assert that retrieval failed and no images were written to disk.
	assert.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "Unable to retrieve attributes"))
	assertImagesDirExistsAndIsEmpty(t, imageLoader)
}

func TestImageLoader_Get_multipleDigests_allFoundInBucket(t *testing.T) {
	unittest.SmallTest(t)

	imageLoader, mockClient, tearDown := setUp(t)
	defer tearDown()

	assertImagesDirExistsAndIsEmpty(t, imageLoader)

	// Digest1 is present in the GCS bucket.
	mockClient.On(
		"GetFileObjectAttrs",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath,
	).Return(&storage.ObjectAttrs{MD5: digestToMD5Bytes(t, Digest1)}, nil)

	// Digest1 is read.
	mockClient.On(
		"FileReader",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath,
	).Return(ioutil.NopCloser(skTextToPng(t, SkTextImage1)), nil)

	// Digest2 is present in the GCS bucket.
	mockClient.On(
		"GetFileObjectAttrs",
		mock.AnythingOfType("*context.emptyCtx"),
		Image2GsPath,
	).Return(&storage.ObjectAttrs{MD5: digestToMD5Bytes(t, Digest2)}, nil)

	// Digest2 is read.
	mockClient.On(
		"FileReader",
		mock.AnythingOfType("*context.emptyCtx"),
		Image2GsPath,
	).Return(ioutil.NopCloser(skTextToPng(t, SkTextImage2)), nil)

	// Get images and persist them to disk.
	images, pendingWritesWG, _ := imageLoader.Get(1, types.DigestSlice{Digest1, Digest2})
	pendingWritesWG.Wait()

	// Assert that the correct images were returned and persisted to disk.
	assert.Equal(t, len(images), 2)
	assertImageEqual(t, images[0], skTextToImage(t, SkTextImage1))
	assertImageEqual(t, images[1], skTextToImage(t, SkTextImage2))
	assertImageCorrectlyPersistedToDisk(t, imageLoader, Digest1, skTextToImage(t, SkTextImage1))
	assertImageCorrectlyPersistedToDisk(t, imageLoader, Digest2, skTextToImage(t, SkTextImage2))
}

func TestImageLoader_Get_multipleDigests_digest1FoundInBucket_digest2NotFound(t *testing.T) {
	unittest.SmallTest(t)

	imageLoader, mockClient, tearDown := setUp(t)
	defer tearDown()

	assertImagesDirExistsAndIsEmpty(t, imageLoader)

	// Digest1 is present in the GCS bucket.
	mockClient.On(
		"GetFileObjectAttrs",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath,
	).Return(&storage.ObjectAttrs{MD5: digestToMD5Bytes(t, Digest1)}, nil)

	// Digest1 is read.
	mockClient.On(
		"FileReader",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath,
	).Return(ioutil.NopCloser(skTextToPng(t, SkTextImage1)), nil)

	// Digest2 is NOT present in the GCS bucket.
	mockClient.On(
		"GetFileObjectAttrs",
		mock.AnythingOfType("*context.emptyCtx"),
		Image2GsPath,
	).Return(&storage.ObjectAttrs{}, errors.New("not found"))

	// Get images and persist them to disk.
	_, _, err := imageLoader.Get(1, types.DigestSlice{Digest1, Digest2})

	// Assert that retrieval failed and no images were written to disk.
	assert.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "Unable to retrieve attributes"))
}

func TestImageLoader_Warm(t *testing.T) {
	unittest.SmallTest(t)

	imageLoader, mockClient, tearDown := setUp(t)
	defer tearDown()

	assertImagesDirExistsAndIsEmpty(t, imageLoader)

	// Digest1 is present in the GCS bucket.
	mockClient.On(
		"GetFileObjectAttrs",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath,
	).Return(
		&storage.ObjectAttrs{MD5: digestToMD5Bytes(t, Digest1)},
		nil,
	).Once() // This ensures that Get doesn't hit GCS after a call to Warm for the same digest.

	// Digest1 is read.
	mockClient.On(
		"FileReader",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath,
	).Return(
		ioutil.NopCloser(skTextToPng(t, SkTextImage1)),
		nil,
	).Once() // This ensures that Get doesn't hit GCS after a call to Warm for the same digest.

	// Digest2 is present in the GCS bucket.
	mockClient.On(
		"GetFileObjectAttrs",
		mock.AnythingOfType("*context.emptyCtx"),
		Image2GsPath,
	).Return(
		&storage.ObjectAttrs{MD5: digestToMD5Bytes(t, Digest2)},
		nil,
	).Once() // This ensures that Get doesn't hit GCS after a call to Warm for the same digest.

	// Digest2 is read.
	mockClient.On(
		"FileReader",
		mock.AnythingOfType("*context.emptyCtx"),
		Image2GsPath,
	).Return(
		ioutil.NopCloser(skTextToPng(t, SkTextImage2)),
		nil,
	).Once() // This ensures that Get doesn't hit GCS after a call to Warm for the same digest.

	// Download both images to disk and cache them in memory.
	imageLoader.Warm(1, types.DigestSlice{Digest1, Digest2}, true)

	// Assert that the images were persisted to disk.
	assert.True(t, imageLoader.IsOnDisk(Digest1))
	assert.True(t, imageLoader.IsOnDisk(Digest2))
	assertImageCorrectlyPersistedToDisk(t, imageLoader, Digest1, skTextToImage(t, SkTextImage1))
	assertImageCorrectlyPersistedToDisk(t, imageLoader, Digest2, skTextToImage(t, SkTextImage2))

	// Get cached images from memory. This shouldn't hit GCS. If it does, the mockClient above should
	// produce an error as per the Once() calls.
	images, _, _ := imageLoader.Get(1, types.DigestSlice{Digest1, Digest2})

	// Assert that the correct images were returned.
	assert.Equal(t, len(images), 2)
	assertImageEqual(t, images[0], skTextToImage(t, SkTextImage1))
	assertImageEqual(t, images[1], skTextToImage(t, SkTextImage2))
}

func assertImagesDirExistsAndIsEmpty(t *testing.T, imageLoader *ImageLoader) {
	assert.DirExists(t, imageLoader.localImgDir)
	image1LocalPath, _ := ImagePaths(Digest1)
	image2LocalPath, _ := ImagePaths(Digest2)
	assert.False(t, fileutil.FileExists(filepath.Join(imageLoader.localImgDir, image1LocalPath)))
	assert.False(t, fileutil.FileExists(filepath.Join(imageLoader.localImgDir, image2LocalPath)))
}

func assertImageCorrectlyPersistedToDisk(t *testing.T, imageLoader *ImageLoader, digest types.Digest, want *image.NRGBA) {
	localRelPath, _ := ImagePaths(digest)
	localAbsolutePath := filepath.Join(imageLoader.localImgDir, localRelPath)
	got, err := common.LoadImg(localAbsolutePath)
	assert.NoError(t, err)
	assertImageEqual(t, got, want)
}

func assertImageEqual(t *testing.T, got *image.NRGBA, want *image.NRGBA) {
	assert.Equal(t, len(got.Pix), len(want.Pix))
	assert.Equal(t, got.Stride, want.Stride)
	for i := 0; i < len(got.Pix); i++ {
		assert.Equal(t, got.Pix[i], want.Pix[i])
	}
}

func skTextToImage(t *testing.T, s string) *image.NRGBA {
	buf := bytes.NewBufferString(s)
	img, err := text.Decode(buf)
	if err != nil {
		t.Fatalf("Failed to decode a valid image: %s", err)
	}
	return img.(*image.NRGBA)
}

func skTextToPng(t *testing.T, skTextImage string) *bytes.Buffer {
	buf := new(bytes.Buffer)
	err := common.EncodeImg(buf, skTextToImage(t, skTextImage))
	assert.NoError(t, err)
	return buf
}

func skTextToDigest(t *testing.T, skTextImage string) types.Digest {
	md5 := md5.New()
	md5.Write(skTextToPng(t, skTextImage).Bytes())
	return types.Digest(hex.EncodeToString(md5.Sum(nil)))
}

func digestToMD5Bytes(t *testing.T, digest types.Digest) []byte {
	bytes, err := hex.DecodeString(string(digest))
	assert.NoError(t, err)
	return bytes
}

func TestImagePaths(t *testing.T) {
	unittest.SmallTest(t)

	digest := types.Digest("098f6bcd4621d373cade4e832627b4f6")
	expectedLocalPath := filepath.Join("09", "8f", string(digest)+".png")
	expectedGSPath := string(digest + ".png")
	localPath, gsPath := ImagePaths(digest)
	assert.Equal(t, expectedLocalPath, localPath)
	assert.Equal(t, expectedGSPath, gsPath)
}
