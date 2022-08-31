package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"

	sk_exec "go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/lib/bazel"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectID     = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskID        = flag.String("task_id", "", "ID of this task.")
	taskName      = flag.String("task_name", "", "Name of the task.")
	workDirFlag   = flag.String("workdir", ".", "Working directory.")
	rbe           = flag.Bool("rbe", false, "Whether to run Bazel on RBE or locally.")
	rbeKey        = flag.String("rbe_key", "", "Path to the service account key to use for RBE.")
	ramdiskSizeGb = flag.Int("ramdisk_gb", 40, "Size of ramdisk to use, in GB.")

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
	if _, err = checkout.EnsureGitCheckout(ctx, repoPath, repoState); err != nil {
		td.Fatal(ctx, err)
	}

	// Set up Bazel.
	opts := bazel.BazelOptions{
		// We want the cache to be on a bigger disk than default. The root disk, where the home
		// directory (and default Bazel cache) lives, is only 15 GB on our GCE VMs.
		CachePath: "/mnt/pd0/bazel_cache",
	}
	if err := bazel.EnsureBazelRCFile(ctx, opts); err != nil {
		td.Fatal(ctx, err)
	}

	if err := bazelCmd(ctx, repoPath, "run", "//cmd/presubmit", "--", "--repo_dir="+repoPath); err != nil {
		td.Fatal(ctx, err)
	}
}

// bazelCmd invokes the given Bazel command in the provided directory.
func bazelCmd(ctx context.Context, checkoutDir string, args ...string) error {
	step := fmt.Sprintf("bazel %s", args)
	return td.Do(ctx, td.Props(step), func(ctx context.Context) error {
		runCmd := &sk_exec.Command{
			Name:       "bazelisk",
			Args:       args,
			InheritEnv: true, // Makes sure bazelisk is on PATH
			Dir:        checkoutDir,
			LogStdout:  true,
			LogStderr:  true,
		}
		_, err := sk_exec.RunCommand(ctx, runCmd)
		if err != nil {
			return err
		}
		return nil
	})
}
