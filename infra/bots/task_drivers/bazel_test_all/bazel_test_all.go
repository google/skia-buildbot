package main

import (
	"context"
	"flag"
	"fmt"
	"path"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectID   = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskID      = flag.String("task_id", "", "ID of this task.")
	taskName    = flag.String("task_name", "", "Name of the task.")
	workDirFlag = flag.String("workdir", ".", "Working directory.")
	rbe         = flag.Bool("rbe", false, "Whether to run Bazel on RBE or locally.")

	checkoutFlags = checkout.SetupFlags(nil)

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")

	// Various paths.
	workDir             string
	skiaInfraRbeKeyFile string
	bazelCacheDir       string

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
	skiaInfraRbeKeyFile = filepath.Join(workDir, "skia_infra_rbe_key", "rbe-ci.json")

	// Temporary directory for the Bazel cache.
	//
	// We cannot use the default Bazel cache location ($HOME/.cache/bazel) because:
	//
	//  - The cache can be large (>10G).
	//  - Swarming bots have limited storage space on the root partition (15G).
	//  - Because the above, the Bazel build fails with a "no space left on device" error.
	//  - The Bazel cache under $HOME/.cache/bazel lingers after the tryjob completes, causing the
	//    Swarming bot to be quarantined due to low disk space.
	//  - Generally, it's considered poor hygiene to leave a bot in a different state.
	//
	// The temporary directory created by the below function call lives under /mnt/pd0, which has
	// significantly more storage space, and will be wiped after the tryjob completes.
	//
	// Reference: https://docs.bazel.build/versions/master/output_directories.html#current-layout.
	bazelCacheDir, err = os_steps.TempDir(ctx, "", "bazel-user-cache-*")
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

	// Print out the Bazel version for debugging purposes.
	bazel(ctx, "version")

	// Run the tests.
	if *rbe {
		testOnRBE(ctx)
	} else {
		testLocally(ctx)
	}

	// Clean up the temporary Bazel cache directory when running locally, because during development,
	// we do not want to leave behind a ~10GB Bazel cache directory under /tmp after each run.
	//
	// This is not necessary under Swarming because the temporary directory will be cleaned up
	// automatically.
	if *local {
		if err := os_steps.RemoveAll(ctx, bazelCacheDir); err != nil {
			td.Fatal(ctx, err)
		}
	}
}

// By invoking Bazel via this function, we ensure that we will always use the temporary cache.
func bazel(ctx context.Context, args ...string) {
	command := []string{"bazel", "--output_user_root=" + bazelCacheDir}
	command = append(command, args...)
	if _, err := exec.RunCwd(ctx, gitDir.Dir(), command...); err != nil {
		td.Fatal(ctx, err)
	}
}

func testOnRBE(ctx context.Context) {
	// Run all tests in the repository. The tryjob will fail upon any failing tests.
	bazel(ctx, "test", "//...", "--test_output=errors", "--config=remote", "--google_credentials="+skiaInfraRbeKeyFile)

	// TODO(lovisolo): Upload Puppeteer test screenshots to Gold.
}

func testLocally(ctx context.Context) {
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
			if err != nil {
				td.Fatal(ctx, err)
			}
			return nil
		})
		if err != nil {
			td.Fatal(ctx, err)
		}
		ctx = td.WithEnv(ctx, []string{"PATH=%(PATH)s:" + depotToolsDir})
	}

	// Start the emulators. When running this task driver locally (e.g. with --local), this will kill
	// any existing emulator instances prior to launching all emulators.
	if err := emulators.StartAllEmulators(); err != nil {
		td.Fatal(ctx, err)
	}
	defer func() {
		if err := emulators.StopAllEmulators(); err != nil {
			td.Fatal(ctx, err)
		}
	}()
	time.Sleep(5 * time.Second) // Give emulators time to boot.

	// Set *_EMULATOR_HOST environment variables.
	emulatorHostEnvVars := []string{}
	for _, emulator := range emulators.AllEmulators {
		// We need to set the *_EMULATOR_HOST variable for the current emulator before we can retrieve
		// its value via emulators.GetEmulatorHostEnvVar().
		if err := emulators.SetEmulatorHostEnvVar(emulator); err != nil {
			td.Fatal(ctx, err)
		}
		name := emulators.GetEmulatorHostEnvVarName(emulator)
		value := emulators.GetEmulatorHostEnvVar(emulator)
		emulatorHostEnvVars = append(emulatorHostEnvVars, fmt.Sprintf("%s=%s", name, value))
	}
	ctx = td.WithEnv(ctx, emulatorHostEnvVars)

	// Run all tests in the repository. The tryjob will fail upon any failing tests.
	bazel(ctx, "test", "//...", "--test_output=errors")
}
