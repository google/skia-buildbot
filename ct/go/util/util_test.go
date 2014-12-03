package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

import (
	assert "github.com/stretchr/testify/require"
)

const (
	TEST_FILE_NAME          = "testingtesting"
	GS_TEST_TIMESTAMP_VALUE = "123"
)

func TestGetCTWorkers(t *testing.T) {
	workers := GetCTWorkers()
	for i := 0; i < NUM_WORKERS; i++ {
		assert.Equal(t, fmt.Sprintf(WORKER_NAME_TEMPLATE, i), workers[i])
	}
}

func TestTaskFileUtils(t *testing.T) {
	TaskFileDir = os.TempDir()
	taskFilePath := filepath.Join(TaskFileDir, TEST_FILE_NAME)
	defer os.Remove(taskFilePath)

	// Assert that the task file is created.
	if err := CreateTaskFile(TEST_FILE_NAME); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if _, err := os.Stat(taskFilePath); err != nil {
		t.Errorf("Task file %s was not created!", taskFilePath)
	}

	// Assert that DeleteTaskFile deletes the task file.
	if err := DeleteTaskFile(TEST_FILE_NAME); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if _, err := os.Stat(taskFilePath); err != nil {
		// Expected error
	} else {
		t.Error("Unexpected lack of error")
	}
}

func TestCreateTimestampFile(t *testing.T) {
	realDir := filepath.Join(os.TempDir(), "util_test")
	os.Mkdir(realDir, 0755)
	defer os.RemoveAll(realDir)
	timestampFilePath := filepath.Join(realDir, TIMESTAMP_FILE_NAME)
	if err := CreateTimestampFile(realDir); err != nil {
		t.Error("Unexpected error: %s", err)
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
		t.Error("Unexpected value in %s: %s", timestampFilePath, err)
	}

	// Assert error returned when specified dir does not exist.
	nonexistantDir := filepath.Join(os.TempDir(), "util_test_nonexistant")
	os.RemoveAll(nonexistantDir)
	if err := CreateTimestampFile(nonexistantDir); err != nil {
		// Expected error
	} else {
		t.Error("Unexpected lack of error")
	}
}
