package util

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/util"
)

// Will need a local valid google_storage_token.data file with read write access
// to run the below test.
func Auth_TestDownloadSwarmingArtifacts(t *testing.T) {
	testPagesetsDirName := filepath.Join("unit-tests", "util", "page_sets")

	gs, err := NewGcsUtil(nil)
	assert.NoError(t, err)

	localDir, err := ioutil.TempDir("", "util_test_")
	assert.NoError(t, err)
	defer util.RemoveAll(localDir)
	pageSetToIndex, err := gs.DownloadSwarmingArtifacts(localDir, testPagesetsDirName, "10k", 1, 2)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	// Examine contents of returned dictionary.
	assert.Equal(t, 2, len(pageSetToIndex))
	assert.Equal(t, 1, pageSetToIndex[filepath.Join(localDir, "1.py")])
	assert.Equal(t, 2, pageSetToIndex[filepath.Join(localDir, "2.py")])
	// Examine contents of the local directory.
	files, err := ioutil.ReadDir(localDir)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(files))
	assert.Equal(t, "1.py", files[0].Name())
	assert.Equal(t, "2.py", files[1].Name())
}
