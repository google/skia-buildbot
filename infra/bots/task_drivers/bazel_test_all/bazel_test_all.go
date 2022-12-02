package main

import (
	"context"
	"flag"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/emulators/cockroachdb_instance"
	"go.skia.org/infra/go/emulators/gcp_emulator"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/task_driver/go/lib/bazel"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectID          = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskID             = flag.String("task_id", "", "ID of this task.")
	taskName           = flag.String("task_name", "", "Name of the task.")
	workDirFlag        = flag.String("workdir", ".", "Working directory.")
	buildbucketBuildID = flag.String("buildbucket_build_id", "", "ID of the Buildbucket build.")
	rbe                = flag.Bool("rbe", false, "Whether to run Bazel on RBE or locally.")
	rbeKey             = flag.String("rbe_key", "", "Path to the service account key to use for RBE.")
	ramdiskSizeGb      = flag.Int("ramdisk_gb", 40, "Size of ramdisk to use, in GB.")
	bazelCacheDir      = flag.String("bazel_cache_dir", "", "Path to the Bazel cache directory.")
	bazelRepoCacheDir  = flag.String("bazel_repo_cache_dir", "", "Path to the Bazel repository cache directory.")

	checkoutFlags = checkout.SetupFlags(nil)

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")

	// Various paths.
	workDir string

	gitDir *git.Checkout
)

func main() {
	// Setup.
	ctx := td.StartRun(projectID, taskID, taskName, output, local)
	defer td.EndRun(ctx)

	// Compute various directory paths.
	var err error
	workDir, err = os_steps.Abs(ctx, *workDirFlag)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Check out the code.
	repoState, err := checkout.GetRepoState(checkoutFlags)
	if err != nil {
		td.Fatal(ctx, err)
	}
	gitDir, err = checkout.EnsureGitCheckout(ctx, path.Join(workDir, "repo"), repoState)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Set up Bazel.
	var (
		bzl        *bazel.Bazel
		bzlCleanup func()
	)
	if !*rbe && !*local {
		// Infra-PerCommit-Test-Bazel-Local uses a ramdisk as the Bazel cache in order to prevent
		// CockroachDB "disk stall detected" errors on GCE VMs due to slow I/O.
		bzl, bzlCleanup, err = bazel.NewWithRamdisk(ctx, gitDir.Dir(), *rbeKey, *ramdiskSizeGb)
	} else {
		opts := bazel.BazelOptions{
			CachePath:           *bazelCacheDir,
			RepositoryCachePath: *bazelRepoCacheDir,
		}
		bzl, err = bazel.New(ctx, gitDir.Dir(), *rbeKey, opts)
		bzlCleanup = func() {}
	}
	if err != nil {
		td.Fatal(ctx, err)
	}
	defer bzlCleanup()

	// Print out the Bazel version for debugging purposes.
	if _, err := bzl.Do(ctx, "version"); err != nil {
		td.Fatal(ctx, err)
	}

	// Run "npm cache clean -f".
	if _, err := exec.RunCwd(ctx, path.Join(workDir, "repo"), "npm", "cache", "clean", "-f"); err != nil {
		td.Fatal(ctx, err)
	}

	// Run the tests.
	if *rbe {
		if err := testOnRBE(ctx, bzl); err != nil {
			td.Fatal(ctx, err)
		}
	} else {
		if err := testLocally(ctx, bzl); err != nil {
			td.Fatal(ctx, err)
		}
	}
}

// goldctl invokes goldctl with the given arguments.
func goldctl(ctx context.Context, bzl *bazel.Bazel, args ...string) error {
	bazelCommand := []string{
		// Unset this flag, which is set in //.bazelrc to point to //bazel/get_workspace_status.sh.
		// This script invokes "git fetch", which can be slow, and we'll be invoking goldctl via
		// Bazel hundreds of times (once per screenshot).
		"--workspace_status_command=",
		"//gold-client/cmd/goldctl",
		"--",
	}
	bazelCommand = append(bazelCommand, args...)
	_, err := bzl.Do(ctx, "run", bazelCommand...)
	return err
}

// uploadPuppeteerScreenshotsToGold gathers all screenshots produced by Puppeteer tests and uploads
// them to Gold.
func uploadPuppeteerScreenshotsToGold(ctx context.Context, bzl *bazel.Bazel) error {
	// Extract screenshots.
	puppeteerScreenshotsDir, err := os_steps.TempDir(ctx, "", "puppeteer-screenshots-*")
	if err != nil {
		return err
	}
	if _, err := bzl.Do(ctx, "run", "//:extract_puppeteer_screenshots", "--", "--output_dir", puppeteerScreenshotsDir); err != nil {
		return err
	}

	// Create working directory for goldctl.
	goldctlWorkDir, err := os_steps.TempDir(ctx, "", "goldctl-workdir-*")
	if err != nil {
		return err
	}

	// Authorize goldctl.
	if *local {
		if err := goldctl(ctx, bzl, "auth", "--work-dir", goldctlWorkDir); err != nil {
			return err
		}
	} else {
		if err := goldctl(ctx, bzl, "auth", "--work-dir", goldctlWorkDir, "--luci"); err != nil {
			return err
		}
	}

	// Initialize goldctl.
	args := []string{
		"imgtest", "init",
		"--work-dir", goldctlWorkDir,
		"--instance", "skia-infra",
		"--git_hash", *checkoutFlags.Revision,
		"--corpus", "infra",
		"--key", "build_system:bazel",
	}
	if *checkoutFlags.PatchIssue != "" && *checkoutFlags.PatchSet != "" {
		extraArgs := []string{
			"--crs", "gerrit",
			"--cis", "buildbucket",
			"--changelist", *checkoutFlags.PatchIssue,
			"--patchset", *checkoutFlags.PatchSet, // Note that this is the patchset "order", i.e. a positive integer.
			"--jobid", *buildbucketBuildID,
		}
		args = append(args, extraArgs...)
	}
	if err := goldctl(ctx, bzl, args...); err != nil {
		return err
	}

	// Add screenshots.
	fileInfos, err := os_steps.ReadDir(ctx, puppeteerScreenshotsDir)
	if err != nil {
		return err
	}
	err = td.Do(ctx, td.Props("Add images to goldctl"), func(ctx context.Context) error {
		for _, fileInfo := range fileInfos {
			testName := strings.TrimSuffix(filepath.Base(fileInfo.Name()), filepath.Ext(fileInfo.Name()))
			args := []string{
				"imgtest", "add",
				"--work-dir", goldctlWorkDir,
				"--png-file", filepath.Join(puppeteerScreenshotsDir, fileInfo.Name()),
				"--test-name", testName,
			}
			if err := goldctl(ctx, bzl, args...); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Finalize and upload screenshots to Gold.
	return goldctl(ctx, bzl, "imgtest", "finalize", "--work-dir", goldctlWorkDir)
}

// testOnRBE is only called for the Infra-PerCommit-Test-Bazel-RBE task.
func testOnRBE(ctx context.Context, bzl *bazel.Bazel) error {
	// Run all tests in the repository. The tryjob will fail upon any failing tests.
	if _, err := bzl.DoOnRBE(ctx, "test", "//...", "--test_output=errors"); err != nil {
		return err
	}

	// Upload to Gold all screenshots produced by Puppeteer tests in the previous step.
	return uploadPuppeteerScreenshotsToGold(ctx, bzl)
}

// testLocally is only called for the Infra-PerCommit-Test-Bazel-Local task.
func testLocally(ctx context.Context, bzl *bazel.Bazel) (rvErr error) {
	// We skip the following steps when running on a developer's workstation because we assume that
	// the environment already has everything we need to run this task driver (the repository checkout
	// has a .git directory, the Go environment variables are properly set, etc.).
	if !*local {
		// Set up go.
		ctx = golang.WithEnv(ctx, workDir)

		// Check out depot_tools at the exact revision expected by tests (defined in recipes.cfg), and
		// make it available to tests by by adding it to the PATH.
		var depotToolsDir string
		err := td.Do(ctx, td.Props("Check out depot_tools"), func(ctx context.Context) error {
			var err error
			depotToolsDir, err = depot_tools.Sync(ctx, workDir, filepath.Join(gitDir.Dir(), recipe_cfg.RECIPE_CFG_PATH))
			return err
		})
		if err != nil {
			return err
		}
		ctx = td.WithEnv(ctx, []string{"PATH=%(PATH)s:" + depotToolsDir})

		// If the emulators are already running for any reason, kill them first. This prevents "Address
		// already in use" errors on GCE bots.
		if err = emulators.ForceStopAllEmulators(); err != nil {
			return err
		}
	}

	// Start the emulators.
	if _, err := cockroachdb_instance.StartCockroachDBIfNotRunning(); err != nil {
		return err
	}
	if err := gcp_emulator.StartAllIfNotRunning(); err != nil {
		return err
	}
	defer func() {
		if err := emulators.StopAllEmulators(); err != nil {
			rvErr = err
		}
	}()

	// Set *_EMULATOR_HOST environment variables.
	var emulatorHostEnvVars []string
	for _, emulator := range emulators.AllEmulators {
		name := emulators.GetEmulatorHostEnvVarName(emulator)
		value := emulators.GetEmulatorHostEnvVar(emulator)
		if value == "" {
			return fmt.Errorf("ENV VAR %s is empty", name)
		}
		emulatorHostEnvVars = append(emulatorHostEnvVars, fmt.Sprintf("%s=%s", name, value))
	}

	ctx = td.WithEnv(ctx, emulatorHostEnvVars)

	// Run all tests in the repository. The tryjob will fail upon any failing tests.
	//
	// We specify an explicit location for the vpython VirtualEnv root directory by piping through
	// the VPYTHON_VIRTUALENV_ROOT environment variable, which points to the cache/vpython Swarming
	// cache. The rationale is that some of our Go tests perform steps such as the following:
	//
	//   1. Create a temporary directory.
	//   2. Invoke a Python script, with $HOME pointing to said temporary directory.
	//   3. Delete the temporary directory before exiting.
	//
	// vpython creates its VirtualEnv root at the path specified by the VPYTHON_VIRTUALENV_ROOT
	// environment variable, defaulting to $HOME/.vpython-root if unset, and populates this
	// directory with read-only files. If we leave VPYTHON_VIRTUALENV_ROOT unset, step 3 above
	// will try to delete said read-only files and fail with "permission denied".
	//
	// Note that this isn't necessary in Infra-PerCommit-Test-Bazel-RBE because the "python" binary
	// is provided by the RBE toolchain container image, and not by the vpython CIPD package, as is
	// the case with Infra-PerCommit-Test-Bazel-Local.
	_, err := bzl.Do(ctx, "test", "//...", "--test_output=errors", "--test_env=VPYTHON_VIRTUALENV_ROOT")
	return err
}
