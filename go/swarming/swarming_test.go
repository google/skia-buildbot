package swarming

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	TESTDATA_DIR = "testdata"
	TEST_ISOLATE = "test.isolate"
	TEST_SCRIPT  = "test.py"
)

// TestCreateIsolatedGenJSON verifies that an isolated.gen.json with expected
// values is created from the test isolated files.
func TestCreateIsolatedGenJSON(t *testing.T) {
	unittest.LargeTest(t)
	workDir, err := ioutil.TempDir("", "swarming_work_")
	require.NoError(t, err)
	s, err := NewSwarmingClient(context.Background(), workDir, SWARMING_SERVER, isolate.ISOLATE_SERVER_URL_FAKE, "")
	require.NoError(t, err)
	defer s.Cleanup()

	extraArgs := map[string]string{
		"ARG_1": "arg_1",
		"ARG_2": "arg_2",
	}

	// Pass in a relative path to isolate file. It should return an err.
	genJSON, err := s.CreateIsolatedGenJSON(path.Join(TESTDATA_DIR, TEST_ISOLATE), TESTDATA_DIR, "linux", "testTask1", extraArgs)
	require.Equal(t, "", genJSON)
	require.NotNil(t, err)
	require.Equal(t, "isolate path testdata/test.isolate must be an absolute path", err.Error())

	// Now pass in an absolute path to isolate file. This should succeed.
	absTestDataDir, err := filepath.Abs(TESTDATA_DIR)
	require.NoError(t, err)
	genJSON, err = s.CreateIsolatedGenJSON(path.Join(absTestDataDir, TEST_ISOLATE), TESTDATA_DIR, "linux", "testTask1", extraArgs)
	require.NoError(t, err)
	contents, err := ioutil.ReadFile(genJSON)
	require.NoError(t, err)
	var output struct {
		Version int      `json:"version"`
		Dir     string   `json:"dir"`
		Args    []string `json:"args"`
	}

	err = json.Unmarshal(contents, &output)
	require.NoError(t, err)

	require.Equal(t, 1, output.Version)
	require.Equal(t, TESTDATA_DIR, path.Base(output.Dir))
	expectedArgs := []string{
		"--isolate", path.Join(absTestDataDir, TEST_ISOLATE),
		"--isolated", fmt.Sprintf("%s/testTask1.isolated", s.WorkDir),
		"--config-variable", "OS", "linux",
		"--extra-variable", "ARG_1", "arg_1",
		"--extra-variable", "ARG_2", "arg_2",
	}
	require.Equal(t, expectedArgs, output.Args)
}

// E2E_Success verifies that an islated.gen.json is created, batcharchive works,
// triggering swarming tasks works and collecting swarming tasks works.
func E2E_Success(t *testing.T) {
	// Instantiate the swarming client.
	workDir, err := ioutil.TempDir("", "swarming_work_")
	require.NoError(t, err)
	s, err := NewSwarmingClient(context.Background(), workDir, SWARMING_SERVER, isolate.ISOLATE_SERVER_URL_FAKE, "")
	require.NoError(t, err)
	defer s.Cleanup()

	ctx := context.Background()

	// Create isolated.gen.json files to pass to batcharchive.
	absTestDataDir, err := filepath.Abs(TESTDATA_DIR)
	require.NoError(t, err)
	taskNames := []string{"testTask1", "testTask2"}
	genJSONs := []string{}
	for _, taskName := range taskNames {
		extraArgs := map[string]string{
			"ARG_1": fmt.Sprintf("arg_1_%s", taskName),
			"ARG_2": fmt.Sprintf("arg_2_%s", taskName),
		}
		genJSON, err := s.CreateIsolatedGenJSON(path.Join(absTestDataDir, TEST_ISOLATE), s.WorkDir, "linux", taskName, extraArgs)
		require.NoError(t, err)
		genJSONs = append(genJSONs, genJSON)
	}

	// Batcharchive the task.
	tasksToHashes, err := s.BatchArchiveTargets(ctx, genJSONs, 5*time.Minute)
	require.NoError(t, err)
	require.Equal(t, 2, len(tasksToHashes))
	for _, taskName := range taskNames {
		hash, exists := tasksToHashes[taskName]
		require.True(t, exists)
		require.NotNil(t, hash)
	}

	// Trigger swarming using the isolate hashes.
	dimensions := map[string]string{"pool": "Chrome"}
	tags := map[string]string{"testing": "123"}
	tasks, err := s.TriggerSwarmingTasks(ctx, tasksToHashes, dimensions, tags, map[string]string{}, []string{}, RECOMMENDED_PRIORITY, RECOMMENDED_EXPIRATION, RECOMMENDED_HARD_TIMEOUT, RECOMMENDED_IO_TIMEOUT, false, true, "")
	require.NoError(t, err)

	// Collect both output and file output of all tasks.
	for _, task := range tasks {
		output, outputDir, _, err := task.Collect(ctx, s, true, true)
		require.NoError(t, err)
		output = sanitizeOutput(output)
		require.Equal(t, fmt.Sprintf("arg_1_%s\narg_2_%s\n", task.Title, task.Title), output)
		tagsWithTaskName := map[string]string{"testing": "123", "name": task.Title}
		require.Equal(t, tagsWithTaskName, task.Tags)
		// Verify contents of the outputDir.
		rawFileOutput, err := ioutil.ReadFile(path.Join(outputDir, "output.txt"))
		require.NoError(t, err)
		fileOutput := strings.Replace(string(rawFileOutput), "\r\n", "\n", -1)
		require.Equal(t, "testing\ntesting", fileOutput)
	}
}

// E2E_OnFailure verifies that an islated.gen.json is created, batcharchive
// works, triggering swarming tasks works and collecting swarming tasks with one
// failure works.
func E2E_OneFailure(t *testing.T) {
	// Instantiate the swarming client.
	workDir, err := ioutil.TempDir("", "swarming_work_")
	require.NoError(t, err)
	ctx := context.Background()
	s, err := NewSwarmingClient(ctx, workDir, SWARMING_SERVER, isolate.ISOLATE_SERVER_URL_FAKE, "")
	require.NoError(t, err)
	defer s.Cleanup()

	// Create isolated.gen.json files to pass to batcharchive.
	absTestDataDir, err := filepath.Abs(TESTDATA_DIR)
	require.NoError(t, err)
	taskNames := []string{"testTask1", "testTask2"}
	genJSONs := []string{}
	for _, taskName := range taskNames {
		extraArgs := map[string]string{
			"ARG_1": fmt.Sprintf("arg_1_%s", taskName),
			"ARG_2": fmt.Sprintf("arg_2_%s", taskName),
		}
		// Add an empty 2nd argument for testTask1 to cause a failure.
		if taskName == "testTask1" {
			extraArgs["ARG_2"] = ""
		}
		genJSON, err := s.CreateIsolatedGenJSON(path.Join(absTestDataDir, TEST_ISOLATE), s.WorkDir, "linux", taskName, extraArgs)
		require.NoError(t, err)
		genJSONs = append(genJSONs, genJSON)
	}

	// Batcharchive the task.
	tasksToHashes, err := s.BatchArchiveTargets(ctx, genJSONs, 5*time.Minute)
	require.NoError(t, err)
	require.Equal(t, 2, len(tasksToHashes))
	for _, taskName := range taskNames {
		hash, exists := tasksToHashes[taskName]
		require.True(t, exists)
		require.NotNil(t, hash)
	}

	// Trigger swarming using the isolate hashes.
	dimensions := map[string]string{"pool": "Chrome"}
	tags := map[string]string{"testing": "123"}
	tasks, err := s.TriggerSwarmingTasks(ctx, tasksToHashes, dimensions, tags, map[string]string{}, []string{}, RECOMMENDED_PRIORITY, RECOMMENDED_EXPIRATION, RECOMMENDED_HARD_TIMEOUT, RECOMMENDED_IO_TIMEOUT, false, false, "")
	require.NoError(t, err)

	// Collect testTask1. It should have failed.
	output1, outputDir1, _, err1 := tasks[0].Collect(ctx, s, true, true)
	require.Equal(t, tags, tasks[0].Tags)
	output1 = sanitizeOutput(output1)
	require.Equal(t, "", output1)
	require.Equal(t, "", outputDir1)
	require.NotNil(t, err1)
	require.True(t, strings.HasPrefix(err1.Error(), "Swarming trigger for testTask1 failed with: Command exited with exit status 1: "))

	// Collect testTask2. It should have succeeded.
	output2, outputDir2, _, err2 := tasks[1].Collect(ctx, s, true, true)
	require.NoError(t, err2)
	require.Equal(t, tags, tasks[1].Tags)
	output2 = sanitizeOutput(output2)
	require.Equal(t, fmt.Sprintf("arg_1_%s\narg_2_%s\n", tasks[1].Title, tasks[1].Title), output2)
	// Verify contents of the outputDir.
	rawFileOutput, err := ioutil.ReadFile(path.Join(outputDir2, "output.txt"))
	require.NoError(t, err)
	fileOutput := strings.Replace(string(rawFileOutput), "\r\n", "\n", -1)
	require.Equal(t, "testing\ntesting", fileOutput)
}

// sanitizeOutput makes the task output consistent. Sometimes the outputs comes
// back with "\r\n" and sometimes with "\n". This function makes it always be "\n".
func sanitizeOutput(output string) string {
	return strings.Replace(output, "\r\n", "\n", -1)
}
