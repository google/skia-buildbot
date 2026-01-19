package main

import (
	"context"
	"flag"
	"path"
	"path/filepath"
	"strings"

	"fmt"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/task_driver/go/lib/bazel"
	"go.skia.org/infra/task_driver/go/lib/checkout"
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

	gitDir git.Checkout
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
	opts := bazel.BazelOptions{
		CachePath:           *bazelCacheDir,
		RepositoryCachePath: *bazelRepoCacheDir,
	}
	bzl, err := bazel.New(ctx, gitDir.Dir(), *rbeKey, opts)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Print out the Bazel version for debugging purposes.
	if _, err := bzl.Do(ctx, "version"); err != nil {
		td.Fatal(ctx, err)
	}

	// Run all tests in the repository. The tryjob will fail upon any failing tests.
	//
	// We invoke Bazel with --remote_download_minimal to avoid "no space left on device errors". See
	// https://bazel.build/reference/command-line-reference#flag--remote_download_minimal.
	//
	// We use --remote_download_regex to explicitly download the outputs.zip files, which contain
	// the Puppeteer screenshots required by the uploadPuppeteerScreenshotsToGold step.
	// We also match the puppeteer-test-screenshots directory in case the outputs are not zipped (Bazel 8+).
	// See https://bazel.build/reference/command-line-reference#flag--remote_download_regex.
	if _, err := bzl.DoOnRBE(ctx, "test", "//...", "--remote_download_minimal", "--remote_download_regex=.*(outputs.zip|puppeteer-test-screenshots/.*)", "--test_output=errors"); err != nil {
		td.Fatal(ctx, err)
	}

	// Upload to Gold all screenshots produced by Puppeteer tests in the previous step.
	if err := uploadPuppeteerScreenshotsToGold(ctx, bzl); err != nil {
		td.Fatal(ctx, err)
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
		"--passfail", // Enable blocking on mismatch.
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
	err = td.Do(ctx, td.Props(fmt.Sprintf("Add %d images to goldctl", len(fileInfos))), func(ctx context.Context) error {
		var anyErr error
		for _, fileInfo := range fileInfos {
			testName := strings.TrimSuffix(filepath.Base(fileInfo.Name()), filepath.Ext(fileInfo.Name()))
			args := []string{
				"imgtest", "add",
				"--work-dir", goldctlWorkDir,
				"--png-file", filepath.Join(puppeteerScreenshotsDir, fileInfo.Name()),
				"--test-name", testName,
			}
			if err := goldctl(ctx, bzl, args...); err != nil {
				anyErr = err
			}
		}
		return anyErr
	})
	if err != nil {
		return err
	}

	// Finalize and upload screenshots to Gold.
	return goldctl(ctx, bzl, "imgtest", "finalize", "--work-dir", goldctlWorkDir)
}
