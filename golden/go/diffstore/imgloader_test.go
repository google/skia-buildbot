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
	"go.skia.org/infra/go/gcs"
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

	// GCS bucket names.
	BucketFoo = "foo"
	BucketBar = "bar"

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

func TestImageLoaderGet_assertDigestsAreCorrect(t *testing.T) {
	assert.Equal(t, skTextToDigest(t, SkTextImage1), Digest1)
	assert.Equal(t, skTextToDigest(t, SkTextImage2), Digest2)
}

type Mocks struct {
	FooBucketClient *test_gcsclient.MockGCSClient
	BarBucketClient *test_gcsclient.MockGCSClient
}

func setUp(t *testing.T) (*ImageLoader, *Mocks, func()) {
	mocks := &Mocks{}

	// Create temporary directories.
	tmpDir, cleanup := testutils.TempDir(t)
	imgDir := filepath.Join(tmpDir, "images")
	assert.Nil(t, os.Mkdir(imgDir, 0777))

	// Build bucket name to GCSClient map.
	mocks.FooBucketClient = test_gcsclient.NewMockClient()
	mocks.BarBucketClient = test_gcsclient.NewMockClient()
	gsBucketClients := map[string]gcs.GCSClient{
		BucketFoo: mocks.FooBucketClient,
		BucketBar: mocks.BarBucketClient,
	}

	// Compute an arbitrary cache size.
	imgCacheCount, _ := getCacheCounts(10)

	// Create the ImageLoader instance.
	imageLoader, err := NewImgLoader(gsBucketClients, tmpDir, imgDir, GsImageBaseDir, imgCacheCount, &disk_mapper.DiskMapper{})
	assert.NoError(t, err)

	return imageLoader, mocks, cleanup
}

func TestImageLoaderGet_singleDigest_foundInFirstBucket(t *testing.T) {
	unittest.SmallTest(t)

	imageLoader, mocks, tearDown := setUp(t)
	defer tearDown()

	// Digest1 is found in bucket Foo.
	mocks.FooBucketClient.On(
		"GetFileObjectAttrs",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath).Return(
		&storage.ObjectAttrs{MD5: digestToMD5Bytes(t, Digest1)}, nil)

	// Digest1 is read from bucket Foo.
	mocks.FooBucketClient.On(
		"FileReader",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath).Return(
		ioutil.NopCloser(skTextToPng(t, SkTextImage1)), nil)

	images, _, _ := imageLoader.Get(1, types.DigestSlice{Digest1})

	assertImageEqual(t, images[0], skTextToImage(t, SkTextImage1))
}

func TestImageLoaderGet_singleDigest_foundInSecondBucket(t *testing.T) {
	unittest.SmallTest(t)

	imageLoader, mocks, tearDown := setUp(t)
	defer tearDown()

	// Digest1 is not found in bucket Foo.
	mocks.FooBucketClient.On(
		"GetFileObjectAttrs",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath).Return(&storage.ObjectAttrs{}, errors.New("not found"))

	// Digest1 is found in bucket Bar.
	mocks.BarBucketClient.On(
		"GetFileObjectAttrs",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath).Return(
		&storage.ObjectAttrs{MD5: digestToMD5Bytes(t, Digest1)}, nil)

	// Digest1 is read from bucket Bar.
	mocks.BarBucketClient.On(
		"FileReader",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath).Return(
		ioutil.NopCloser(skTextToPng(t, SkTextImage1)), nil)

	images, _, _ := imageLoader.Get(1, types.DigestSlice{Digest1})

	assertImageEqual(t, images[0], skTextToImage(t, SkTextImage1))
}

func TestImageLoaderGet_multipleDigests_foundInMultipleBuckets(t *testing.T) {
	unittest.SmallTest(t)

	imageLoader, mocks, tearDown := setUp(t)
	defer tearDown()

	// Digest1 is found in bucket Foo.
	mocks.FooBucketClient.On(
		"GetFileObjectAttrs",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath).Return(
		&storage.ObjectAttrs{MD5: digestToMD5Bytes(t, Digest1)}, nil)

	// Digest1 is read from bucket Foo.
	mocks.FooBucketClient.On(
		"FileReader",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath).Return(
		ioutil.NopCloser(skTextToPng(t, SkTextImage1)), nil)

	// Digest2 is not found in bucket Foo.
	mocks.FooBucketClient.On(
		"GetFileObjectAttrs",
		mock.AnythingOfType("*context.emptyCtx"),
		Image2GsPath).Return(&storage.ObjectAttrs{}, errors.New("not found"))

	// Digest2 is found in bucket Bar.
	mocks.BarBucketClient.On(
		"GetFileObjectAttrs",
		mock.AnythingOfType("*context.emptyCtx"),
		Image2GsPath).Return(
		&storage.ObjectAttrs{MD5: digestToMD5Bytes(t, Digest2)}, nil)

	// Digest2 is read from bucket Bar.
	mocks.BarBucketClient.On(
		"FileReader",
		mock.AnythingOfType("*context.emptyCtx"),
		Image2GsPath).Return(
		ioutil.NopCloser(skTextToPng(t, SkTextImage2)), nil)

	images, _, _ := imageLoader.Get(1, types.DigestSlice{Digest1, Digest2})

	assertImageEqual(t, images[0], skTextToImage(t, SkTextImage1))
	assertImageEqual(t, images[1], skTextToImage(t, SkTextImage2))
}

func assertImageEqual(t *testing.T, got *image.NRGBA, want *image.NRGBA) {
	assert.Equal(t, len(got.Pix), len(want.Pix))
	assert.Equal(t, got.Stride, want.Stride)
	for i := 0; i < len(got.Pix); i++ {
		assert.Equal(t, got.Pix[i], want.Pix[i])
	}
}

func TestImageLoaderGet_singleDigest_notFoundInAnyBuckets(t *testing.T) {
	unittest.SmallTest(t)

	imageLoader, mocks, tearDown := setUp(t)
	defer tearDown()

	// Digest1 is not found in bucket Foo.
	mocks.FooBucketClient.On(
		"GetFileObjectAttrs",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath).Return(&storage.ObjectAttrs{}, errors.New("not found"))

	// Digest1 is not found in bucket Bar.
	mocks.BarBucketClient.On(
		"GetFileObjectAttrs",
		mock.AnythingOfType("*context.emptyCtx"),
		Image1GsPath).Return(&storage.ObjectAttrs{}, errors.New("not found"))

	_, _, err := imageLoader.Get(1, types.DigestSlice{Digest1})
	assert.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "Failed finding image"))
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
