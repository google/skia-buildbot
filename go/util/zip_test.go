package util

import (
	"testing"

	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
)

func TestZip(t *testing.T) {
	testutils.MediumTest(t)

	// Create a directory in temp.
	// Create 2 files in that directory.
	// Zip them.
	err := ZipIt("/tmp/ziptest", "/tmp/test.zip")
	assert.Nil(t, err)
	// assert the destination file exists.
	// delete the temp directory.

	// Unzip
	// assert the tmp directory exists
	// assert that the 2 source files exist.
}
