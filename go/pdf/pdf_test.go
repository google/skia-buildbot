package pdf

import (
	"bytes"
	"crypto/md5"
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/skia-dev/glog"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
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
		glog.Infof("%v\n", err)
		return false
	}
	checksum2, err := md5OfFile(path2)
	if err != nil {
		glog.Infof("%v\n", err)
		return false
	}
	return 0 == bytes.Compare(checksum1, checksum2)
}

func testRasterizer(t *testing.T, rasterizer Rasterizer, expectation string) {
	assert.True(t, rasterizer.Enabled(), "%s.Enabled() failed.", rasterizer.String())

	testDataDir, err := testutils.TestDataDir()
	assert.Nil(t, err, "TestDataDir missing: %v", err)

	tempDir, err := ioutil.TempDir("", "pdf_test_")
	assert.Nil(t, err, "ioutil.TempDir failed")
	defer util.RemoveAll(tempDir)

	pdfSrcPath := path.Join(testDataDir, "minimal.pdf")
	assert.True(t, fileutil.FileExists(pdfSrcPath), "Path '%s' does not exist", pdfSrcPath)
	pdfInputPath := path.Join(tempDir, "minimal.pdf")

	err = os.Symlink(pdfSrcPath, pdfInputPath)
	assert.Nil(t, err, "Symlink failed")
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
	assert.True(t, filesEqual(outputFileName, expectedOutput), "png output not correct")
}

func TestRasterizePdfium(t *testing.T) {
	testRasterizer(t, Pdfium{}, "minimalPdfium.png")
}

func TestRasterizePoppler(t *testing.T) {
	testRasterizer(t, Poppler{}, "minimalPoppler.png")
}
