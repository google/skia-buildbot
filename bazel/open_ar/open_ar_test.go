package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestExtract_MultipleFilesInArchive(t *testing.T) {
	unittest.MediumTest(t)
	outputDir := t.TempDir()

	// We expect to find 3 files in the output directory, called test1, test2, and test3
	// They were filled with 1024 random bytes at creation and have distinct sha256 hashes.
	require.NoError(t, extract("testdata/ar_archive.ar", outputDir))

	verifyFile := func(name, sha256Hash string) {
		b, err := os.ReadFile(filepath.Join(outputDir, name))
		require.NoError(t, err)
		h := sha256.Sum256(b)
		actualHash := hex.EncodeToString(h[:])
		assert.Equal(t, sha256Hash, actualHash, "Wrong hash for file %s", name)
	}
	// These hashes were produced manually by extracting the archive and using sha256sum
	verifyFile("test1", "75672462a61dbcf8df0f6e722f715a6e674e9d24faebf683953c0b4fe0c89a8c")
	verifyFile("test2", "e4d0969bc21f51ebb2328d2354c62ff3b17eb46c37b039a7f967fd03fccbe6c2")
	verifyFile("test3", "3afd01133a953573fce7b189c99bdaa10739bee5b2efd9c34baa4ed31cb6fb72")
}
