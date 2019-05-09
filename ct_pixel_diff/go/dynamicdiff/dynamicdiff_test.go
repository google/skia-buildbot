package dynamicdiff

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/ct_pixel_diff/go/common"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/diffstore"
	"go.skia.org/infra/golden/go/mocks"
)

const (
	// Test image IDs for PixelDiffIDPathMapper.
	TEST_PIXEL_DIFF_LEFT  = common.ImageID("lchoi-20170714123456/nopatch/1/http___www_google_com")
	TEST_PIXEL_DIFF_RIGHT = common.ImageID("lchoi-20170714123456/withpatch/1/http___www_google_com")

	// PNG extension.
	DOT_EXT = ".png"

	// Directory with testdata.
	TEST_DATA_BASE_DIR = "./testdata"

	// Bucket to test loading images through a GS path.
	TEST_GS_BUCKET = "cluster-telemetry"

	// Directory to test loading images through a GS path.
	TEST_GS_BASE_DIR = "tasks/pixel_diff_runs"

	// Image to test loading images through a GS path.
	TEST_IMG_PATH = common.ImageID("lchoi-20170804012953/nopatch/1/http___www_google_com")
)

func TestIsDynamicContentPixel(t *testing.T) {
	testutils.SmallTest(t)
	assert.True(t, isDynamicContentPixel(0, 255, 255))
	assert.False(t, isDynamicContentPixel(128, 128, 128))
}

func TestDeltaOffset(t *testing.T) {
	testutils.SmallTest(t)
	assert.Equal(t, 6, deltaOffset(765))
	assert.Equal(t, 2, deltaOffset(256))
	assert.Equal(t, 0, deltaOffset(1))
}

func TestDynamicContentDiff(t *testing.T) {
	testutils.SmallTest(t)

	left := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	left.SetNRGBA(0, 0, color.NRGBA{0, 255, 255, 255})
	left.SetNRGBA(0, 1, color.NRGBA{7, 7, 7, 255})

	right := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	right.SetNRGBA(0, 1, color.NRGBA{7, 7, 7, 255})
	right.SetNRGBA(1, 0, color.NRGBA{128, 128, 128, 255})
	right.SetNRGBA(1, 1, color.NRGBA{0, 255, 255, 255})

	// Calculate the diff. Only two pixels are not cyan and out of those, only one
	// is different.
	diffMetrics, diffImg := DynamicContentDiff(left, right)

	// Verify the diff image is correct.
	expectedImg := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	expectedImg.SetNRGBA(0, 0, color.NRGBA{0, 255, 255, 255})
	expectedImg.SetNRGBA(1, 0, color.NRGBA{241, 105, 19, 255})
	expectedImg.SetNRGBA(1, 1, color.NRGBA{0, 255, 255, 255})
	assert.Equal(t, expectedImg, diffImg)

	// Verify the diff metrics are correct.
	expectedDiffMetrics := &DynamicDiffMetrics{
		NumDiffPixels:    1,
		PixelDiffPercent: 50,
		MaxRGBDiffs:      []int{128, 128, 128},
		NumStaticPixels:  2,
		NumDynamicPixels: 2,
	}
	assert.Equal(t, expectedDiffMetrics, diffMetrics)
}

func TestPixelDiffStoreMapper(t *testing.T) {
	testutils.SmallTest(t)

	mapper := PixelDiffStoreMapper{}
	dirs := strings.Split(string(TEST_PIXEL_DIFF_LEFT), "/")

	// Test DiffID and SplitDiffID
	expectedDiffID := strings.Join([]string{dirs[0], dirs[2], dirs[3]}, ":")
	actualDiffID := mapper.DiffID(TEST_PIXEL_DIFF_LEFT, TEST_PIXEL_DIFF_RIGHT)
	actualLeft, actualRight := mapper.SplitDiffID(expectedDiffID)
	assert.Equal(t, expectedDiffID, actualDiffID)
	assert.Equal(t, TEST_PIXEL_DIFF_LEFT, actualLeft)
	assert.Equal(t, TEST_PIXEL_DIFF_RIGHT, actualRight)

	// Test DiffPath
	expectedDiffPath := dirs[0] + "/" + dirs[3] + DOT_EXT
	actualDiffPath := mapper.DiffPath(TEST_PIXEL_DIFF_LEFT, TEST_PIXEL_DIFF_RIGHT)
	assert.Equal(t, expectedDiffPath, actualDiffPath)

	// Test ImagePaths
	expectedLocalPath := string(TEST_PIXEL_DIFF_LEFT + DOT_EXT)
	runID := strings.Split(dirs[0], "-")
	timeStamp := runID[1]
	// YYYY/MM/DD/HH directories
	datePath := filepath.Join(timeStamp[0:4], timeStamp[4:6], timeStamp[6:8], timeStamp[8:10])
	expectedGSPath := filepath.Join(datePath, expectedLocalPath)
	localPath, gsBucket, gsPath := mapper.ImagePaths(TEST_PIXEL_DIFF_LEFT)
	assert.Equal(t, expectedLocalPath, localPath)
	assert.Equal(t, expectedGSPath, gsPath)
	assert.Equal(t, "", gsBucket)

	// Test IsValidDiffImgID
	// Trim the image extension first
	expectedDiffImgID := expectedDiffPath[:len(expectedDiffPath)-len(DOT_EXT)]
	assert.True(t, mapper.IsValidDiffImgID(expectedDiffImgID))

	// Test IsValidImgID
	assert.True(t, mapper.IsValidImgID(string(TEST_PIXEL_DIFF_LEFT)))
	assert.True(t, mapper.IsValidImgID(string(TEST_PIXEL_DIFF_RIGHT)))
}

// Tests loading GS images that are specified through a path.
func TestImageLoaderGetGSPath(t *testing.T) {
	testutils.MediumTest(t)

	baseDir := TEST_DATA_BASE_DIR + "-imgloader"
	defer testutils.RemoveAll(t, baseDir)

	assert.NoError(t, os.MkdirAll(baseDir, 0777))
	client := mocks.GetHTTPClient(t)

	workingDir := filepath.Join(baseDir, "images")
	assert.Nil(t, os.Mkdir(workingDir, 0777))

	gsBuckets := []string{TEST_GS_BUCKET}

	mapper := PixelDiffStoreMapper{}
	imgLoader, err := diffstore.NewImgLoader(client, baseDir, workingDir, gsBuckets, TEST_GS_BASE_DIR, 0, mapper)
	assert.NoError(t, err)

	// Get the images and wait until they are written to disk
	_, pendingWrites, err := imgLoader.Get(1, []common.ImageID{TEST_IMG_PATH})
	assert.NoError(t, err)
	pendingWrites.Wait()

	assert.NoError(t, err)
	assert.True(t, fileutil.FileExists(filepath.Join(workingDir, string(TEST_IMG_PATH)+DOT_EXT)))
}
