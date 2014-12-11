// Utility that contains methods for both CT master and worker scripts.
package util

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
)

// GetCTWorkers returns an array of all CT workers.
func GetCTWorkers() []string {
	workers := make([]string, NUM_WORKERS)
	for i := 0; i < NUM_WORKERS; i++ {
		workers[i] = fmt.Sprintf(WORKER_NAME_TEMPLATE, i)
	}
	return workers
}

// CreateTimestampFile creates a TIMESTAMP file in the specified dir. The dir must
// exist else an error is returned.
func CreateTimestampFile(dir string) error {
	// Create the task file in TaskFileDir.
	timestampFilePath := filepath.Join(dir, TIMESTAMP_FILE_NAME)
	out, err := os.Create(timestampFilePath)
	if err != nil {
		return fmt.Errorf("Could not create %s: %s", timestampFilePath, err)
	}
	defer out.Close()
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	w := bufio.NewWriter(out)
	if _, err := w.WriteString(strconv.FormatInt(timestamp, 10)); err != nil {
		return fmt.Errorf("Could not write to %s: %s", timestampFilePath, err)
	}
	w.Flush()
	return nil
}

// CreateTaskFile creates a taskName file in the TaskFileDir dir. It signifies
// that the worker is currently busy doing a particular task.
func CreateTaskFile(taskName string) error {
	// Create TaskFileDir if it does not exist.
	if _, err := os.Stat(TaskFileDir); err != nil {
		if os.IsNotExist(err) {
			// Dir does not exist create it.
			if err := os.MkdirAll(TaskFileDir, 0700); err != nil {
				return fmt.Errorf("Could not create %s: %s", TaskFileDir, err)
			}
		} else {
			// There was some other error.
			return err
		}
	}
	// Create the task file in TaskFileDir.
	taskFilePath := filepath.Join(TaskFileDir, taskName)
	if _, err := os.Create(taskFilePath); err != nil {
		return fmt.Errorf("Could not create %s: %s", taskFilePath, err)
	}
	return nil
}

// DeleteTaskFile deletes a taskName file in the TaskFileDir dir. It should be
// called when the worker is done executing a particular task.
func DeleteTaskFile(taskName string) error {
	taskFilePath := filepath.Join(TaskFileDir, taskName)
	if err := os.Remove(taskFilePath); err != nil {
		return fmt.Errorf("Could not delete %s: %s", taskFilePath, err)
	}
	return nil
}

func TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	glog.Infof("===== %s took %s =====", name, elapsed)
}

// WriteLog implements the io.Writer interface and writes to glog and an output
// file (if specified).
type WriteLog struct {
	LogFunc    func(format string, args ...interface{})
	OutputFile *os.File
}

func (wl WriteLog) Write(p []byte) (n int, err error) {
	wl.LogFunc("%s", string(p))
	// Write to file if specified.
	if wl.OutputFile != nil {
		if n, err := wl.OutputFile.WriteString(string(p)); err != nil {
			glog.Errorf("Could not write to %s: %s", wl.OutputFile.Name(), err)
			return n, err
		}
	}
	return len(p), nil
}

// ExecuteCmd executes the specified binary with the specified args and env.
// Stdout and Stderr are written to stdoutFile and stderrFile respectively if
// specified. If not specified then stdout and stderr will be outputted only to
// glog. Note: It is the responsibility of the caller to close stdoutFile and
// stderrFile.
func ExecuteCmd(binary string, args, env []string, timeout time.Duration, stdoutFile, stderrFile *os.File) error {
	// Add the current PATH to the env.
	env = append(env, "PATH="+os.Getenv("PATH"))

	// Create the cmd obj.
	cmd := exec.Command(binary, args...)
	cmd.Env = env

	// Attach WriteLog to command.
	cmd.Stdout = WriteLog{LogFunc: glog.Infof, OutputFile: stdoutFile}
	cmd.Stderr = WriteLog{LogFunc: glog.Errorf, OutputFile: stderrFile}

	// Execute cmd.
	glog.Infof("Executing %s %s", strings.Join(cmd.Env, " "), strings.Join(cmd.Args, " "))
	cmd.Start()
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-time.After(timeout):
		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("Failed to kill timed out process: %s", err)
		}
		<-done // allow goroutine to exit
		glog.Errorf("Command killed since it took longer than %f secs", timeout.Seconds())
		return fmt.Errorf("Command killed since it took longer than %f secs", timeout.Seconds())
	case err := <-done:
		if err != nil {
			return fmt.Errorf("Process done with error: %s", err)
		}
	}
	return nil
}

// SyncDir runs "gclient sync" on the specified directory.
func SyncDir(dir string) error {
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("Could not chdir to %s: %s", dir, err)
	}
	args := []string{"sync"}
	return ExecuteCmd(BINARY_GCLIENT, args, []string{}, 5*time.Minute, nil, nil)
}

func BuildSkiaTools() error {
	if err := os.Chdir(SkiaTreeDir); err != nil {
		return fmt.Errorf("Could not chdir to %s: %s", SkiaTreeDir, err)
	}
	// Run "make clean".
	ExecuteCmd(BINARY_MAKE, []string{"clean"}, []string{}, 5*time.Minute, nil, nil)
	// Build tools.
	return ExecuteCmd(BINARY_MAKE, []string{"tools", "BUILDTYPE=Release"}, []string{"GYP_DEFINES=\"skia_warnings_as_errors=0\""}, 5*time.Minute, nil, nil)
}
