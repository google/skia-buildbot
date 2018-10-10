package main

/*
	FileStream example.

	Run like this:

	$ go run ./file_stream.go --logtostderr --project_id=skia-swarming-bots --task_name=filestream_example -o - --local
*/

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/task_driver"
	"go.skia.org/infra/go/util"
)

var (
	// Required flags for all TaskDrivers.
	projectId = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId    = flag.String("task_id", "", "ID of this task.")
	taskName  = flag.String("task_name", "", "Name of the task.")
	output    = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
	local     = flag.Bool("local", false, "True if running locally (as opposed to in production)")

	// Flags for this TaskDriver.
	workdir = flag.String("workdir", os.TempDir(), "Working directory.")
)

func main() {
	// Setup.
	taskName := "FileStream Example"
	s := task_driver.MustInit(projectId, taskId, &taskName, output, local)
	defer s.Done(nil)

	if err := example1(s); err != nil {
		sklog.Fatal(err)
	}
	if err := example2(s); err != nil {
		sklog.Fatal(err)
	}
}

func example1(s *task_driver.Step) (rv error) {
	s = s.Step().Name("example1").Start()
	defer s.Done(&rv)

	// This script writes logs to a file.
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		return fmt.Errorf("Failed to obtain path of current file.")
	}
	script := filepath.Join(filepath.Dir(filename), "write_logs.py")
	fs := s.NewFileStream("verbose", sklog.DEBUG)
	defer util.Close(fs)
	_, err := exec.RunCwd(s.Ctx(), *workdir, "python", "-u", script, fs.FilePath())
	return err
}

func example2(s *task_driver.Step) (rv error) {
	s = s.Step().Name("example2").Start()
	defer s.Done(&rv)

	// File streams should also work when the file is copied over.
	tmpFile := filepath.Join(*workdir, "tmpfile")
	if err := util.WithWriteFile(tmpFile, func(w io.Writer) error {
		_, err := w.Write([]byte("Contents were copied (via os.Rename)"))
		return err
	}); err != nil {
		return err
	}
	fs := s.NewFileStream("copied", sklog.DEBUG)
	defer util.Close(fs)
	_, err := exec.RunCwd(s.Ctx(), *workdir, "cp", tmpFile, fs.FilePath())
	return err
}
