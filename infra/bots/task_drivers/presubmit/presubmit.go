package main

import (
	"flag"
	"path/filepath"

	"go.skia.org/infra/task_driver/go/lib/bazel"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectID   = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskID      = flag.String("task_id", "", "ID of this task.")
	taskName    = flag.String("task_name", "", "Name of the task.")
	workDirFlag = flag.String("workdir", ".", "Working directory.")
	rbeKey      = flag.String("rbe_key", "", "Path to the service account key to use for RBE.")

	checkoutFlags = checkout.SetupFlags(nil)

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	flag.Parse()

	// Setup.
	ctx := td.StartRun(projectID, taskID, taskName, output, local)
	defer td.EndRun(ctx)

	// Compute work dir path.
	workDir, err := os_steps.Abs(ctx, *workDirFlag)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Check out the code.
	repoState, err := checkout.GetRepoState(checkoutFlags)
	if err != nil {
		td.Fatal(ctx, err)
	}
	repoPath := filepath.Join(workDir, "repo")
	gitDir, err := checkout.EnsureGitCheckout(ctx, repoPath, repoState)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Set up Bazel.
	bzl, err := bazel.New(ctx, gitDir.Dir(), *local, *rbeKey)
	if err != nil {
		td.Fatal(ctx, err)
	}

	if _, err = bzl.DoOnRBE(ctx, "run", "//cmd/presubmit", "--", "--commit"); err != nil {
		td.Fatal(ctx, err)
	}
}
