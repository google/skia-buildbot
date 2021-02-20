package main

/*
	FileStream example.

	Run like this:

	$ go run ./file_stream.go --logtostderr --project_id=skia-swarming-bots --task_name=filestream_example -o - --local
*/

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/td"
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
	ctx := td.StartRun(projectId, taskId, &taskName, output, local)
	defer td.EndRun(ctx)

	if err := example1(ctx); err != nil {
		td.Fatal(ctx, err)
	}
	if err := example2(ctx); err != nil {
		td.Fatal(ctx, err)
	}
}

func example1(ctx context.Context) (rv error) {
	ctx = td.StartStep(ctx, td.Props("example1"))
	defer td.EndStep(ctx)

	// This script writes logs to a file.
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		return td.FailStep(ctx, fmt.Errorf("Failed to obtain path of current file."))
	}
	script := filepath.Join(filepath.Dir(filename), "write_logs.py")
	fs, err := td.NewFileStream(ctx, "verbose", td.SeverityDebug)
	if err != nil {
		return td.FailStep(ctx, err)
	}
	defer util.Close(fs)
	if _, err := exec.RunCwd(ctx, *workdir, "python", "-u", script, fs.FilePath()); err != nil {
		return td.FailStep(ctx, err)
	}
	return nil
}

func example2(ctx context.Context) (rv error) {
	ctx = td.StartStep(ctx, td.Props("example2"))
	defer td.EndStep(ctx)

	// File streams should also work when the file is copied over.
	tmpFile := filepath.Join(*workdir, "tmpfile")
	if err := util.WithWriteFile(tmpFile, func(w io.Writer) error {
		if _, err := w.Write([]byte("Contents were copied (via os.Rename)")); err != nil {
			return td.FailStep(ctx, err)
		}
		return nil
	}); err != nil {
		return td.FailStep(ctx, err)
	}
	fs, err := td.NewFileStream(ctx, "copied", td.SeverityDebug)
	if err != nil {
		return td.FailStep(ctx, err)
	}
	defer util.Close(fs)
	if _, err := exec.RunCwd(ctx, *workdir, "cp", tmpFile, fs.FilePath()); err != nil {
		return td.FailStep(ctx, err)
	}
	return nil
}
