package npy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sbinet/npyio"
	"github.com/stretchr/testify/assert"
)

func TestNpyReader_ReadFloat32(t *testing.T) {
	// Create a temporary directory.
	tempDir, err := os.MkdirTemp("", "npy-test")
	assert.NoError(t, err)
	defer func() {
		err := os.RemoveAll(tempDir)
		assert.NoError(t, err)
	}()

	// Create a temporary .npy file.
	filePath := filepath.Join(tempDir, "test.npy")
	f, err := os.Create(filePath)
	assert.NoError(t, err)

	// Write some data to the file.
	expectedData := [2][3]float32{
		{1.0, 2.0, 3.0},
		{4.0, 5.0, 6.0},
	}
	expectedDataToCompare := [][]float32{}
	for i := 0; i < len(expectedData); i++ {
		expectedDataToCompare = append(expectedDataToCompare, expectedData[i][:])
	}
	err = npyio.Write(f, expectedData)
	assert.NoError(t, err)
	err = f.Close()
	assert.NoError(t, err)

	// Create a new NpyReader.
	reader := NewNpyReader(filePath)

	// Read the data.
	actualData, err := reader.ReadFloat32()
	assert.NoError(t, err)

	// Convert the response to a fixed size array.

	// Check the data.
	assert.Equal(t, expectedDataToCompare, actualData)
}

func TestNpyReader_ReadFloat32_FileDoesNotExist(t *testing.T) {
	// Create a new NpyReader with a non-existent file.
	reader := NewNpyReader("non-existent-file.npy")

	// Read the data.
	_, err := reader.ReadFloat32()
	assert.Error(t, err)
}

func TestNpyReader_ReadFloat32_DifferentDtype(t *testing.T) {
	// Create a temporary directory.
	tempDir, err := os.MkdirTemp("", "npy-test")
	assert.NoError(t, err)
	defer func() {
		err := os.RemoveAll(tempDir)
		assert.NoError(t, err)
	}()

	// Create a temporary .npy file with int32 data.
	filePath := filepath.Join(tempDir, "test_int32.npy")
	f, err := os.Create(filePath)
	assert.NoError(t, err)

	intData := [2][3]int32{
		{10, 20, 30},
		{40, 50, 60},
	}
	err = npyio.Write(f, intData)
	assert.NoError(t, err)
	err = f.Close()
	assert.NoError(t, err)

	// Create a new NpyReader.
	reader := NewNpyReader(filePath)

	// Attempt to read the int32 data as float32. This should result in an error.
	_, err = reader.ReadFloat32()
	assert.Error(t, err)
}
