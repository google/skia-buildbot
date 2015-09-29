package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"go.skia.org/infra/go/util"
)

import (
	assert "github.com/stretchr/testify/require"
)

const (
	TEST_FILE_NAME          = "testingtesting"
	GS_TEST_TIMESTAMP_VALUE = "123"
)

func TestGetCTWorkersProd(t *testing.T) {
	workers := GetCTWorkersProd()
	for i := 0; i < NUM_WORKERS_PROD; i++ {
		assert.Equal(t, fmt.Sprintf(WORKER_NAME_TEMPLATE, i+1), workers[i])
	}
}

func TestTaskFileUtils(t *testing.T) {
	TaskFileDir = os.TempDir()
	taskFilePath := filepath.Join(TaskFileDir, TEST_FILE_NAME)
	defer util.Remove(taskFilePath)

	// Assert that the task file is created.
	if err := CreateTaskFile(TEST_FILE_NAME); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if _, err := os.Stat(taskFilePath); err != nil {
		t.Errorf("Task file %s was not created!", taskFilePath)
	}

	// Assert that DeleteTaskFile deletes the task file.
	DeleteTaskFile(TEST_FILE_NAME)
	if _, err := os.Stat(taskFilePath); err != nil {
		// Expected error
	} else {
		t.Error("Unexpected lack of error")
	}
}

func TestGetMasterLogLink(t *testing.T) {
	expectedLink := fmt.Sprintf("%s/util.test.%s.%s.log.INFO.rmistry-1440425450.02", MASTER_LOGSERVER_LINK, MASTER_NAME, CtUser)
	actualLink := GetMasterLogLink("rmistry-1440425450.02")
	assert.Equal(t, expectedLink, actualLink)
}

func TestCreateTimestampFile(t *testing.T) {
	realDir := filepath.Join(os.TempDir(), "util_test")
	util.Mkdir(realDir, 0755)
	defer util.RemoveAll(realDir)
	timestampFilePath := filepath.Join(realDir, TIMESTAMP_FILE_NAME)
	if err := CreateTimestampFile(realDir); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	// Assert timestamp file exists.
	if _, err := os.Stat(timestampFilePath); err != nil {
		t.Errorf("Timestamp file %s was not created!", timestampFilePath)
	}
	// Assert timestamp file contains int64.
	fileContent, err := ioutil.ReadFile(timestampFilePath)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if _, err := strconv.ParseInt(string(fileContent), 10, 64); err != nil {
		t.Errorf("Unexpected value in %s: %s", timestampFilePath, err)
	}

	// Assert error returned when specified dir does not exist.
	nonexistantDir := filepath.Join(os.TempDir(), "util_test_nonexistant")
	util.RemoveAll(nonexistantDir)
	if err := CreateTimestampFile(nonexistantDir); err != nil {
		// Expected error
	} else {
		t.Error("Unexpected lack of error")
	}
}
