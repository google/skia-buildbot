package main

import (
	"flag"
	"path/filepath"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/task_driver/go/lib/bazel"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/git_steps"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectID         = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskID            = flag.String("task_id", "", "ID of this task.")
	taskName          = flag.String("task_name", "", "Name of the task.")
	workDirFlag       = flag.String("workdir", ".", "Working directory.")
	rbeKey            = flag.String("rbe_key", "", "Path to the service account key to use for RBE.")
	bazelCacheDir     = flag.String("bazel_cache_dir", "", "Path to the Bazel cache directory.")
	bazelRepoCacheDir = flag.String("bazel_repo_cache_dir", "", "Path to the Bazel repository cache directory.")

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
	ts, err := git_steps.Init(ctx, *local)
	if err != nil {
		td.Fatal(ctx, err)
	}
	if !*local {
		client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
		g, err := gerrit.NewGerrit("https://skia-review.googlesource.com", client)
		if err != nil {
			td.Fatal(ctx, err)
		}
		email, err := g.GetUserEmail(ctx)
		if err != nil {
			td.Fatal(ctx, err)
		}
		if _, err := gitauth.New(ts, "/tmp/.gitcookies", true, email); err != nil {
			td.Fatal(ctx, err)
		}
	}
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
	opts := bazel.BazelOptions{
		CachePath:           *bazelCacheDir,
		RepositoryCachePath: *bazelRepoCacheDir,
	}
	bzl, err := bazel.New(ctx, gitDir.Dir(), *rbeKey, opts)
	if err != nil {
		td.Fatal(ctx, err)
	}

	if _, err = bzl.DoOnRBE(ctx, "run", "//cmd/presubmit", "--", "--commit"); err != nil {
		td.Fatal(ctx, err)
	}
}
