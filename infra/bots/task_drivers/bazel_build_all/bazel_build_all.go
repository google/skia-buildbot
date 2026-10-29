package main

import (
	"flag"
	"path"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/lib/bazel"
	"go.skia.org/infra/task_driver/go/lib/checkout"
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
	ramdiskSizeGb     = flag.Int("ramdisk_gb", 40, "Size of ramdisk to use, in GB.")
	bazelCacheDir     = flag.String("bazel_cache_dir", "", "Path to the Bazel cache directory.")
	bazelRepoCacheDir = flag.String("bazel_repo_cache_dir", "", "Path to the Bazel repository cache directory.")

	checkoutFlags = checkout.SetupFlags(nil)

	// Optional flags.
	clean  = flag.Bool("clean", false, "If true, use a temporary directory for the Bazel cache and clean it up after building.")
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
	gitDir, err := checkout.EnsureGitCheckout(ctx, path.Join(workDir, "repo"), repoState)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Use a temporary cache directory if --clean is specified. This prevents us
	// from clobbering the global cache for other tasks.
	cacheDir := *bazelCacheDir
	repoCacheDir := *bazelRepoCacheDir
	if *clean {
		cacheDir, err = os_steps.TempDir(ctx, "", "bazel_cache_*")
		if err != nil {
			td.Fatal(ctx, err)
		}
		repoCacheDir = ""
		defer func() {
			if err := os_steps.RemoveAll(ctx, cacheDir); err != nil {
				sklog.Errorf("Failed to remove temporary Bazel cache directory: %s", err)
			}
		}()
	}

	opts := bazel.BazelOptions{
		CachePath:           cacheDir,
		RepositoryCachePath: repoCacheDir,
	}
	bzl, err := bazel.New(ctx, gitDir.Dir(), *rbeKey, opts)
	if err != nil {
		td.Fatal(ctx, err)
	}
	defer bzl.Cleanup()

	// The Bazel server writes files into the cache directory which we may not
	// have permission to remove. Have Bazel itself clean the cache before we
	// attempt to remove it.
	if *clean {
		defer func() {
			if _, err := bzl.Do(ctx, "clean", "--expunge"); err != nil {
				sklog.Errorf("Failed to clean Bazel cache: %s", err)
			}
		}()
	}

	// Print out the Bazel version for debugging purposes.
	if _, err := bzl.Do(ctx, "version"); err != nil {
		td.Fatal(ctx, err)
	}

	// Build all code in the repository. The tryjob will fail upon any build errors.
	//
	// We invoke Bazel with --remote_download_minimal to avoid "no space left on device errors". See
	// https://bazel.build/reference/command-line-reference#flag--remote_download_minimal.
	if _, err := bzl.DoOnRBE(ctx, "build", "//...", "--remote_download_minimal"); err != nil {
		td.Fatal(ctx, err)
	}
}
