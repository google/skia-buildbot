package goldpushk

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestFileCopierCopy(t *testing.T) {
	unittest.SmallTest(t)

	fcp := FileCopierImpl{}

	// Create test file.
	tempDir, cleanup := testutils.TempDir(t)
	defer cleanup()
	src := filepath.Join(tempDir, "foo")
	dst := filepath.Join(tempDir, "bar")

	// Try to copy source file, which does not yet exist.
	err := fcp.Copy(src, dst)
	assert.Error(t, err)

	// Create source file.
	srcFileContents := "Hello, world!"
	testutils.WriteFile(t, src, srcFileContents)

	// Assert that the source file was created.
	contents, err := readFile(src)
	assert.NoError(t, err)
	assert.Equal(t, srcFileContents, contents)

	// Assert that the destination file does not exist.
	_, err = readFile(dst)
	assert.Error(t, err)

	// Copy file.
	err = fcp.Copy(src, dst)
	assert.NoError(t, err)

	// Assert that destination file exists and has the right content.
	dstFileContents, err := readFile(dst)
	assert.NoError(t, err)
	assert.Equal(t, srcFileContents, dstFileContents)
}

// readFile returns the contents of the file at src.
func readFile(src string) (string, error) {
	f, err := os.Open(src)
	if err != nil {
		return "", err
	}
	contents, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(contents), nil
}
