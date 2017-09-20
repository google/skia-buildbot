package util

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
)

func createFile(dir, prefix, content string, t *testing.T) *os.File {
	f, err := ioutil.TempFile(dir, prefix)
	assert.Nil(t, err)
	f.WriteString(content)
	return f
}

func assertFileExists(dir, path, content string, t *testing.T) {
	c, err := ioutil.ReadFile(filepath.Join(dir, filepath.Base(path)))
	assert.Nil(t, err)
	assert.Equal(t, content, string(c))
}

func TestZipE2E(t *testing.T) {
	testutils.MediumTest(t)

	// Create a directory in temp.
	targetDir, err := ioutil.TempDir("", "zip_test")
	assert.Nil(t, err)
	defer RemoveAll(targetDir)

	// Populate the target dir.
	// Create two files in target dir.
	f1 := createFile(targetDir, "temp1", "testing1", t)
	f2 := createFile(targetDir, "temp2", "testing2", t)
	// Create subdir.
	subDir, err := ioutil.TempDir(targetDir, "zip_test")
	assert.Nil(t, err)
	// Create one file in subdir.
	f3 := createFile(subDir, "temp3", "testing3", t)

	// Zip the directory.
	outputDir, err := ioutil.TempDir("", "zip_location")
	defer RemoveAll(outputDir)
	zipPath := filepath.Join(outputDir, "test.zip")
	err = ZipIt(targetDir, zipPath)
	assert.Nil(t, err)
	// Assert that zip was created
	_, err = os.Stat(zipPath)
	assert.Nil(t, err)
	assert.False(t, os.IsNotExist(err))

	// Test UnZipping.
	err = UnZip(zipPath, outputDir)
	assert.Nil(t, err)
	// Assert that the 3 zipped files are in the right locations.
	assertFileExists(outputDir, f1.Name(), "testing1", t)
	assertFileExists(outputDir, f2.Name(), "testing2", t)
	assertFileExists(filepath.Join(outputDir, filepath.Base(subDir)), f3.Name(), "testing3", t)
}
