package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/util"
	storage "google.golang.org/api/storage/v1"
)

// Will need a local valid google_storage_token.data file with read write access
// to run the below test.
func Auth_TestDownloadWorkerArtifacts(t *testing.T) {
	testPagesetsDirName := filepath.Join("unit-tests", "util", "page_sets")

	client, err := auth.NewDefaultClient(true, auth.SCOPE_FULL_CONTROL)
	if err != nil {
		t.Errorf("Failed to authenticate: %s", err)
	}

	gs, err := NewGsUtil(client)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	tmpDir := filepath.Join(os.TempDir(), "util_test")
	StorageDir = tmpDir
	defer util.RemoveAll(tmpDir)
	if err := gs.DownloadWorkerArtifacts(testPagesetsDirName, "10k", 1); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	// Examine contents of the local directory.
	localDir := filepath.Join(tmpDir, testPagesetsDirName, "10k")
	files, err := ioutil.ReadDir(localDir)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	assert.Equal(t, 3, len(files))
	assert.Equal(t, "TIMESTAMP", files[0].Name())
	assert.Equal(t, "alexa1-1.py", files[1].Name())
	assert.Equal(t, "alexa2-2.py", files[2].Name())
}

// Will need a local valid google_storage_token.data file with read write access
// to run the below test.
func Auth_TestUploadWorkerArtifacts(t *testing.T) {
	client, err := auth.NewDefaultClient(true, auth.SCOPE_FULL_CONTROL)
	if err != nil {
		t.Errorf("Failed to authenticate: %s", err)
	}

	gs, err := NewGsUtil(client)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	testDir := "testupload"
	testPagesetType := "10ktest"
	StorageDir = "testdata"
	if err := gs.UploadWorkerArtifacts(testDir, testPagesetType, 1); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	// Examine contents of the remote directory and then clean it up.
	service, err := storage.New(gs.client)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	gsDir := filepath.Join(testDir, testPagesetType, "slave1")
	resp, err := service.Objects.List(GS_BUCKET_NAME).Prefix(gsDir + "/").Do()
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	assert.Equal(t, 3, len(resp.Items))
	for index, fileName := range []string{"TIMESTAMP", "alexa1-1.py", "alexa2-2.py"} {
		filePath := fmt.Sprintf("%s/%s", gsDir, fileName)
		defer util.LogErr(service.Objects.Delete(GS_BUCKET_NAME, filePath).Do())
		assert.Equal(t, filePath, resp.Items[index].Name)
	}
}

func TestAreTimestampsEqual(t *testing.T) {
	gs, err := NewGsUtil(util.NewTimeoutClient())
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}

	tmpDir := filepath.Join(os.TempDir(), "util_test")
	util.Mkdir(tmpDir, 0777)
	defer util.RemoveAll(tmpDir)

	f, err := os.Create(filepath.Join(tmpDir, TIMESTAMP_FILE_NAME))
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	defer util.Close(f)

	// Test with matching timestamps.
	if _, err := f.WriteString(GS_TEST_TIMESTAMP_VALUE); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	result1, err := gs.AreTimeStampsEqual(tmpDir, "unit-tests/util/")
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	assert.True(t, result1)

	// Test with differing timestamps.
	if _, err := f.WriteString(GS_TEST_TIMESTAMP_VALUE); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	result2, err := gs.AreTimeStampsEqual(tmpDir, "unit-tests/util/")
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	assert.False(t, result2)

	// Test with Google Storage timestamp missing.
	result3, err := gs.AreTimeStampsEqual(tmpDir, "unit-tests/util/dummy_name/")
	if err == nil {
		t.Error("Expected an error")
	}
	assert.False(t, result3)

	// Test with local timestamp missing.
	result4, err := gs.AreTimeStampsEqual(tmpDir+"dummy_name", "unit-tests/util/")
	if err == nil {
		t.Error("Expected an error")
	}
	assert.False(t, result4)
}
