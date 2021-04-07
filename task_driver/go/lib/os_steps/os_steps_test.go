package os_steps

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_driver/go/td"
)

func TestOsSteps(t *testing.T) {
	unittest.MediumTest(t)
	tr := td.StartTestRun(t)
	defer tr.Cleanup()

	// Root-level step.
	s := tr.Root()

	// We're basically just verifying that the os utils work.
	expect := []td.StepResult{}

	// Stat the nonexistent dir.
	expect = append(expect, td.StepResultException)
	dir1 := filepath.Join(tr.Dir(), "test_dir")
	fi, err := Stat(s, dir1)
	require.True(t, os.IsNotExist(err))

	// Try to remove the dir.
	expect = append(expect, td.StepResultSuccess)
	err = RemoveAll(s, dir1)
	require.NoError(t, err) // os.RemoveAll doesn't return error if the dir doesn't exist.

	// Create the dir.
	expect = append(expect, td.StepResultSuccess)
	err = MkdirAll(s, dir1)
	require.NoError(t, err)

	// Stat the dir.
	expect = append(expect, td.StepResultSuccess)
	fi, err = Stat(s, dir1)
	require.NoError(t, err)
	require.True(t, fi.IsDir())

	// Try to create the dir again.
	expect = append(expect, td.StepResultSuccess)
	err = MkdirAll(s, dir1)
	require.NoError(t, err) // os.MkdirAll doesn't return error if the dir already exists.

	// Create a tempDir inside the dir.
	expect = append(expect, td.StepResultSuccess)
	tempDir, err := TempDir(s, dir1, "test_prefix_")
	require.NoError(t, err)

	// Verify the tempDir exists.
	expect = append(expect, td.StepResultSuccess)
	fi, err = Stat(s, tempDir)
	require.NoError(t, err)
	require.True(t, fi.IsDir())

	// Rename the tempDir.
	expect = append(expect, td.StepResultSuccess)
	newTempDir := filepath.Join(dir1, "newtmpdir")
	err = Rename(s, tempDir, newTempDir)
	require.NoError(t, err)

	// Stat the renamed tempDir.
	expect = append(expect, td.StepResultSuccess)
	fi, err = Stat(s, newTempDir)
	require.NoError(t, err)
	require.True(t, fi.IsDir())

	// Remove the dir.
	expect = append(expect, td.StepResultSuccess)
	err = RemoveAll(s, dir1)
	require.NoError(t, err)

	// Stat the dir.
	expect = append(expect, td.StepResultException)
	fi, err = Stat(s, dir1)
	require.True(t, os.IsNotExist(err))

	// Ensure that we got the expected step results.
	results := tr.EndRun(false, nil)
	require.Equal(t, len(results.Steps), len(expect))
	for idx, stepResult := range results.Steps {
		require.Equal(t, stepResult.Result, expect[idx])
	}
}

func TestCopy(t *testing.T) {
	unittest.MediumTest(t)

	tr := td.StartTestRun(t)
	defer tr.Cleanup()

	// Root-level step.
	s := tr.Root()

	// Helper for writing a file, copying it, and checking the result.
	expect := []td.StepResult{}
	fileNum := 0
	testCopy := func(mode os.FileMode, contents []byte) {
		// Write the source file.
		src := filepath.Join(tr.Dir(), fmt.Sprintf("src-%d", fileNum))
		dst := filepath.Join(tr.Dir(), fmt.Sprintf("dst-%d", fileNum))
		fileNum++
		require.NoError(t, ioutil.WriteFile(src, contents, mode))
		// Set the mode again to work around umask.
		require.NoError(t, os.Chmod(src, mode))
		srcStat, err := os.Stat(src)
		require.NoError(t, err)
		// Self-check; ensure that we actually got the mode we wanted for the
		// source file.
		require.Equal(t, mode, srcStat.Mode())

		// Copy the file.
		expect = append(expect, td.StepResultSuccess)
		require.NoError(t, Copy(s, src, dst))

		// Check the mode and contents of the resulting file.
		dstStat, err := os.Stat(dst)
		require.NoError(t, err)
		require.Equal(t, srcStat.Mode(), dstStat.Mode())
		resultContents, err := ioutil.ReadFile(dst)
		require.NoError(t, err)
		require.Equal(t, contents, resultContents)
	}

	testCopy(0644, []byte("hello world"))
	testCopy(0755, []byte("run this"))
	testCopy(0600, []byte("private stuff here"))
	testCopy(0777, []byte("this is for everyone!"))
}
