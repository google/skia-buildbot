package main

import (
	"errors"
	"flag"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectId = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId    = flag.String("task_id", "", "ID of this task.")
	taskName  = flag.String("task_name", "", "Name of the task.")
	workdir   = flag.String("workdir", ".", "Working directory")

	// DM
	dm = flag.String("dm", "", "Path to the dm executable.")

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	// Run DM.
	if *dm == "" {
		td.Fatal(ctx, errors.New("--dm is required."))
	}
	cmd := append([]string{*dm}, flag.Args()...)
	_, err := exec.RunCwd(ctx, ".", cmd...)
	if err != nil {
		td.Fatal(ctx, err)
	}
}
