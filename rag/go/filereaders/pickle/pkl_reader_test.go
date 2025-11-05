package pickle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPickleReader_Read_FileDoesNotExist(t *testing.T) {
	// Create a new PickleReader with a non-existent file.
	reader := NewPickleReader("non-existent-file.pkl")

	// Read the data.
	_, err := reader.Read()
	assert.Error(t, err)
}

func TestPickleReader_Read_MalformedFile(t *testing.T) {
	// Create a temporary directory.
	tempDir, err := os.MkdirTemp("", "pickle-test")
	assert.NoError(t, err)
	defer func() {
		err := os.RemoveAll(tempDir)
		assert.NoError(t, err)
	}()

	// Create a temporary file with malformed data.
	filePath := filepath.Join(tempDir, "malformed.pkl")
	err = os.WriteFile(filePath, []byte("this is not a pickle file"), 0644)
	assert.NoError(t, err)

	// Create a new PickleReader.
	reader := NewPickleReader(filePath)

	// Read the data.
	_, err = reader.Read()
	assert.Error(t, err)
}
