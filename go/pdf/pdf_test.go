package pdf

import (
	"bytes"
	"crypto/md5"
	"image"
	"image/draw"
	_ "image/png"
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/image/text"
)

func md5OfFile(path string) (sum []byte, err error) {
	md5 := md5.New()
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer util.Close(f)
	if _, err = io.Copy(md5, f); err != nil {
		return
	}
	sum = md5.Sum(nil)
	return
}

func filesEqual(path1, path2 string) bool {
	checksum1, err := md5OfFile(path1)
	if err != nil {
		sklog.Infof("%v\n", err)
		return false
	}
	checksum2, err := md5OfFile(path2)
	if err != nil {
		sklog.Infof("%v\n", err)
		return false
	}
	return 0 == bytes.Compare(checksum1, checksum2)
}

func readImg(t testutils.TestingT, path string) *image.NRGBA {
	infile, err := os.Open(path)
	assert.Nil(t, err)
	defer testutils.AssertCloses(t, infile)

	img, _, err := image.Decode(infile)
	assert.Nil(t, err)

	// This accounts for the case when the image encoder (i.e. pnmtopng) writes a paletted image.
	ret := image.NewNRGBA(img.Bounds())
	draw.Draw(ret, img.Bounds(), img, image.Pt(0, 0), draw.Src)
	return ret
}

func imagesEqual(t testutils.TestingT, path1, path2 string) {
	img_1 := readImg(t, path1)
	img_2 := readImg(t, path2)

	var buf_1 bytes.Buffer
	assert.Nil(t, text.Encode(&buf_1, img_1))

	var buf_2 bytes.Buffer
	assert.Nil(t, text.Encode(&buf_2, img_2))
	assert.Equal(t, string(buf_1.Bytes()), string(buf_2.Bytes()))
}

func testRasterizer(t *testing.T, rasterizer Rasterizer, expectation string) {
	assert.True(t, rasterizer.Enabled(), "%s.Enabled() failed.", rasterizer.String())

	testDataDir, err := testutils.TestDataDir()
	assert.NoError(t, err, "TestDataDir missing: %v", err)

	tempDir, err := ioutil.TempDir("", "pdf_test_")
	assert.NoError(t, err, "ioutil.TempDir failed")
	defer util.RemoveAll(tempDir)

	pdfSrcPath := path.Join(testDataDir, "minimal.pdf")
	assert.True(t, fileutil.FileExists(pdfSrcPath), "Path '%s' does not exist", pdfSrcPath)
	pdfInputPath := path.Join(tempDir, "minimal.pdf")

	err = os.Symlink(pdfSrcPath, pdfInputPath)
	assert.NoError(t, err, "Symlink failed")
	assert.True(t, fileutil.FileExists(pdfInputPath), "Path '%s' does not exist", pdfInputPath)

	outputFileName := path.Join(tempDir, "test.png")

	badPath := path.Join(tempDir, "this_file_should_really_not_exist.pdf")

	if err := rasterizer.Rasterize(badPath, outputFileName); err == nil {
		t.Errorf(": Got '%v' Want '%v'", err, nil)
	}

	if err := rasterizer.Rasterize(pdfInputPath, outputFileName); err != nil {
		t.Errorf(": Got '%v' Want '!nil'", err)
	}

	expectedOutput := path.Join(testDataDir, expectation)
	imagesEqual(t, outputFileName, expectedOutput)
	// 	assert.True(t, filesEqual(outputFileName, expectedOutput), "png output not correct")
}

func TestRasterizePdfium(t *testing.T) {
	testutils.MediumTest(t)
	testRasterizer(t, Pdfium{}, "minimalPdfium.png")
}

func TestRasterizePoppler(t *testing.T) {
	testutils.MediumTest(t)
	testRasterizer(t, Poppler{}, "minimalPoppler.png")
}
