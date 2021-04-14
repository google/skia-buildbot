package main

import (
	"context"
	"flag"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/emulators"
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
	bzl, bzlCleanup, err := bazel.New(ctx, gitDir.Dir(), *local, *rbeKey)
	if err != nil {
		td.Fatal(ctx, err)
	}
	defer bzlCleanup()

	// Print out the Bazel version for debugging purposes.
	if _, err := bzl.Do(ctx, "version"); err != nil {
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
	bazelCommand := []string{"//gold-client/cmd/goldctl", "--"}
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
		"--key", "source_type:infra",
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
				"--add-test-optional-key", "build_system:bazel",
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

func testOnRBE(ctx context.Context, bzl *bazel.Bazel) error {
	// Run all tests in the repository. The tryjob will fail upon any failing tests.
	if _, err := bzl.DoOnRBE(ctx, "test", "//...", "--test_output=errors"); err != nil {
		return err
	}

	// Upload to Gold all screenshots produced by Puppeteer tests in the previous step.
	return uploadPuppeteerScreenshotsToGold(ctx, bzl)
}

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
	}

	// Start the emulators. When running this task driver locally (e.g. with --local), this will kill
	// any existing emulator instances prior to launching all emulators.
	if err := emulators.StartAllEmulators(); err != nil {
		return err
	}
	defer func() {
		if err := emulators.StopAllEmulators(); err != nil {
			rvErr = err
		}
	}()
	time.Sleep(5 * time.Second) // Give emulators time to boot.

	// Set *_EMULATOR_HOST environment variables.
	emulatorHostEnvVars := []string{}
	for _, emulator := range emulators.AllEmulators {
		// We need to set the *_EMULATOR_HOST variable for the current emulator before we can retrieve
		// its value via emulators.GetEmulatorHostEnvVar().
		if err := emulators.SetEmulatorHostEnvVar(emulator); err != nil {
			return err
		}
		name := emulators.GetEmulatorHostEnvVarName(emulator)
		value := emulators.GetEmulatorHostEnvVar(emulator)
		emulatorHostEnvVars = append(emulatorHostEnvVars, fmt.Sprintf("%s=%s", name, value))
	}
	ctx = td.WithEnv(ctx, emulatorHostEnvVars)

	// Run all tests in the repository. The tryjob will fail upon any failing tests.
	_, err := bzl.DoOnRBE(ctx, "test", "//...", "--test_output=errors")
	return err
}
