package zip

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sklog"
)

// createTestZip creates a zip file in memory with specified files and directories.
func createTestZip(t *testing.T) []byte {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Add a directory
	_, err := zipWriter.Create("testdir/")
	require.NoError(t, err)

	// Add a file in the root
	file1, err := zipWriter.Create("file1.txt")
	require.NoError(t, err)
	_, err = file1.Write([]byte("content of file 1"))
	require.NoError(t, err)

	// Add a file inside the directory
	file2, err := zipWriter.Create("testdir/file2.txt")
	require.NoError(t, err)
	_, err = file2.Write([]byte("content of file 2"))
	require.NoError(t, err)

	require.NoError(t, zipWriter.Close())
	return buf.Bytes()
}

func TestExtractZipData(t *testing.T) {
	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "zip_test")
	require.NoError(t, err)
	defer func() {
		err := os.RemoveAll(tempDir) // Clean up after the test
		if err != nil {
			sklog.Errorf("Error deleting temporary directory")
		}
	}()
	// Create a dummy zip file content
	zipContent := createTestZip(t)

	// Test 1: Successful extraction
	t.Run("successful extraction", func(t *testing.T) {
		err := ExtractZipData(zipContent, tempDir)
		assert.NoError(t, err)

		// Verify extracted files and directories
		assert.DirExists(t, filepath.Join(tempDir, "testdir"))
		assert.FileExists(t, filepath.Join(tempDir, "file1.txt"))
		assert.FileExists(t, filepath.Join(tempDir, "testdir", "file2.txt"))

		content1, err := os.ReadFile(filepath.Join(tempDir, "file1.txt"))
		require.NoError(t, err)
		assert.Equal(t, "content of file 1", string(content1))

		content2, err := os.ReadFile(filepath.Join(tempDir, "testdir", "file2.txt"))
		require.NoError(t, err)
		assert.Equal(t, "content of file 2", string(content2))
	})

	// Test 2: Invalid zip content
	t.Run("invalid zip content", func(t *testing.T) {
		invalidZipContent := []byte("this is not a zip file")
		err := ExtractZipData(invalidZipContent, tempDir)
		assert.Error(t, err)
	})

	// Test 3: Extraction to a non-existent destination directory (should create it)
	t.Run("non-existent destination", func(t *testing.T) {
		nonExistentDir := filepath.Join(tempDir, "new_dest")
		err := ExtractZipData(zipContent, nonExistentDir)
		assert.NoError(t, err)
		assert.DirExists(t, nonExistentDir)

		assert.FileExists(t, filepath.Join(nonExistentDir, "file1.txt"))
		err = os.RemoveAll(nonExistentDir) // Clean up the created directory
		assert.NoError(t, err)
	})
}
