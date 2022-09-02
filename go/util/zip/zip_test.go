package zip

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func createFile(dir, prefix, content string, t *testing.T) string {
	f, err := ioutil.TempFile(dir, prefix)
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

func assertFileExists(dir, path, content string, t *testing.T) {
	c, err := ioutil.ReadFile(filepath.Join(dir, filepath.Base(path)))
	require.NoError(t, err)
	require.Equal(t, content, string(c))
}

func TestZipE2E(t *testing.T) {

	// Create a directory in temp.
	targetDir, err := ioutil.TempDir("", "zip_test")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, targetDir)

	// Populate the target dir.
	// Create two files in target dir.
	f1 := createFile(targetDir, "temp1", "testing1", t)
	f2 := createFile(targetDir, "temp2", "testing2", t)
	// Create subdir.
	subDir, err := ioutil.TempDir(targetDir, "zip_test")
	require.NoError(t, err)
	// Create one file in subdir.
	f3 := createFile(subDir, "temp3", "testing3", t)
	// Create an empty subdir.
	emptySubDir, err := ioutil.TempDir(targetDir, "zip_test")
	require.NoError(t, err)

	// Zip the directory.
	outputDir, err := ioutil.TempDir("", "zip_location")
	defer testutils.RemoveAll(t, outputDir)
	zipPath := filepath.Join(outputDir, "test.zip")
	err = Directory(zipPath, targetDir)
	require.NoError(t, err)
	// Assert that zip was created
	_, err = os.Stat(zipPath)
	require.NoError(t, err)

	// Test UnZipping.
	err = UnZip(outputDir, zipPath)
	require.NoError(t, err)
	// Assert that the 3 zipped files are in the right locations.
	assertFileExists(outputDir, f1, "testing1", t)
	assertFileExists(outputDir, f2, "testing2", t)
	assertFileExists(filepath.Join(outputDir, filepath.Base(subDir)), f3, "testing3", t)
	// Assert that the empty subdir was created.
	_, err = os.Stat(filepath.Join(outputDir, filepath.Base(emptySubDir)))
	require.NoError(t, err)
}
