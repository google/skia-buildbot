package main

import (
	"flag"
	"path/filepath"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectID = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskID    = flag.String("task_id", "", "ID of this task.")
	taskName  = flag.String("task_name", "", "Name of the task.")
	workdir   = flag.String("workdir", ".", "Working directory")

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	ctx := td.StartRun(projectID, taskID, taskName, output, local)
	defer td.EndRun(ctx)

	wd, err := os_steps.Abs(ctx, *workdir)
	if err != nil {
		td.Fatal(ctx, err)
	}
	ctx = golang.WithEnv(ctx, wd)
	repoDir := filepath.Join(wd, "buildbot")

	if _, err := exec.RunCwd(ctx, wd, "ls", "protoc"); err != nil {
		// td.Fatal(ctx, err)
	}

	if _, err := exec.RunCwd(ctx, wd, "ls", "bazel"); err != nil {
		// td.Fatal(ctx, err)
	}

	if _, err := exec.RunCwd(ctx, wd, "ls", "bazel/bazel"); err != nil {
		// td.Fatal(ctx, err)
	}

	if _, err := exec.RunCwd(ctx, wd, "ls", "bazel/bazel/bin"); err != nil {
		// td.Fatal(ctx, err)
	}

	if _, err := exec.RunCwd(ctx, wd, "find", "bazel"); err != nil {
		// td.Fatal(ctx, err)
	}

	if _, err := exec.RunCwd(ctx, wd, "ls", repoDir); err != nil {
		// td.Fatal(ctx, err)
	}

	// if _, err := os_steps.Which(ctx, "bazel"); err != nil {
	// 	td.Fatal(ctx, err)
	// }

	// if _, err := exec.RunCwd(ctx, repoDir, "bazel", "build", "--config=remote", "//..."); err != nil {
	// 	td.Fatal(ctx, err)
	// }
}
