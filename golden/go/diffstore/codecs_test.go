package diffstore

import (
	"image"
	_ "image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skia-dev/glog"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/testutils"
)

const (
	TEST_IMG_STORAGE_BUCKET = "skia-infra-testdata"
	TEST_IMG_STORAGE_PATH   = "gold-testdata/diffstore_test_images.tar.gz"
	IMG_FILES_DIR           = "./test_images"
	N_TEST_IMAGES           = 15
)

func TestNRGBACodec(t *testing.T) {
	imgs, names := loadImages(t, N_TEST_IMAGES)
	assert.Equal(t, N_TEST_IMAGES, len(imgs))
	testImageCodec(t, imgs, names)
}

func loadImages(t assert.TestingT, nImages int) ([]*image.NRGBA, []string) {
	// testutils.RemoveAll(t, IMG_FILES_DIR)
	assert.NoError(t, gs.DownloadTestDataArchive(t, TEST_IMG_STORAGE_BUCKET, TEST_IMG_STORAGE_PATH, IMG_FILES_DIR))
	defer testutils.RemoveAll(t, IMG_FILES_DIR)

	// Read all images into the directory.
	fileInfos, err := ioutil.ReadDir(IMG_FILES_DIR)
	assert.Nil(t, err)

	imgs := make([]*image.NRGBA, 0, len(fileInfos))
	names := make([]string, 0, len(fileInfos))
	for _, fi := range fileInfos {
		fName := filepath.Join(IMG_FILES_DIR, fi.Name())

		if strings.HasSuffix(fName, ".png") {
			glog.Infof("Opening %s\n", fName)
			var img image.Image
			func() {
				imgFile, err := os.Open(fName)
				assert.Nil(t, err)
				defer imgFile.Close()

				img, _, err = image.Decode(imgFile)
				assert.Nil(t, err)
			}()

			if nrgbaImg, ok := img.(*image.NRGBA); ok {
				imgs = append(imgs, nrgbaImg)
				names = append(names, fName)

				if nImages > 0 && len(imgs) >= nImages {
					break
				}
			}
		}
	}
	return imgs, names
}

func testImageCodec(t assert.TestingT, imgs []*image.NRGBA, names []string) {
	var codec NRGBACodec
	for idx, img := range imgs {
		glog.Infof("Image: %s", names[idx])
		imgBytes, err := codec.Encode(img)
		assert.Nil(t, err)

		glog.Infof("Image: %d", len(img.Pix))
		glog.Infof("Ecoded %d bytes", len(imgBytes))
		assert.Equal(t, len(img.Pix)+24, len(imgBytes))

		newImgRet, err := codec.Decode(imgBytes)
		assert.Nil(t, err)
		newImg := newImgRet.(*image.NRGBA)

		assert.Equal(t, img.Bounds().Dx(), newImg.Bounds().Dx())
		assert.Equal(t, img.Bounds().Dy(), newImg.Bounds().Dy())
		assert.Equal(t, img.Stride, newImg.Stride)
		assert.Equal(t, len(img.Pix), len(newImg.Pix))
		assert.Equal(t, img, newImg)
	}
}
