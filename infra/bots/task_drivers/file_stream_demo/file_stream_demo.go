package main

import (
	"flag"
	"io"
	"path/filepath"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/test_automation"
	"go.skia.org/infra/go/util"
)

const (
	TASK_NAME = "FileStream Demo"
)

var (
	projectId = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId    = flag.String("task_id", "", "ID of this task.")
	workdir   = flag.String("workdir", ".", "Working directory")
	local     = flag.Bool("local", false, "True if running locally (as opposed to in production)")
	output    = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	common.Init()
	s, err := test_automation.Init(*projectId, *taskId, TASK_NAME, *output, *local)
	if err != nil {
		sklog.Fatal(err)
	}
	defer s.Done(nil)

	if err := example1(s); err != nil {
		sklog.Fatal(err)
	}
	if err := example2(s); err != nil {
		sklog.Fatal(err)
	}
}

func example1(s *test_automation.Step) (rv error) {
	s = s.Step().Name("example1").Start()
	defer s.Done(&rv)

	// This script writes logs to a file.
	script := "/usr/local/google/home/borenet/go/src/go.skia.org/infra/write_logs.py"
	fs := s.NewFileStream("verbose")
	defer util.Close(fs)
	_, err := exec.RunCwd(s.Ctx(), *workdir, "python", "-u", script, fs.FilePath())
	return err
}

func example2(s *test_automation.Step) (rv error) {
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
	fs := s.NewFileStream("copied")
	defer util.Close(fs)
	_, err := exec.RunCwd(s.Ctx(), *workdir, "cp", tmpFile, fs.FilePath())
	return err
}
