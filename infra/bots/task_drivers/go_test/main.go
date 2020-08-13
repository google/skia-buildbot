package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/lib/dirs"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectId = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId    = flag.String("task_id", "", "ID of this task.")
	taskName  = flag.String("task_name", "", "Name of the task.")
	workdir   = flag.String("workdir", ".", "Working directory")

	// Optional flags.
	local        = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output       = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
	runEmulators = flag.Bool("run_emulators", false, "If true, run emulators for various services.")
)

func main() {
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	// Setup.
	var wd string
	var infraDir string
	var env []string
	runEmulatorsPath := filepath.Join(infraDir, "scripts", "run_emulators", "run_emulators")
	if err := td.Do(ctx, td.Props("Setup").Infra(), func(ctx context.Context) error {
		var err error
		wd, err = os_steps.Abs(ctx, *workdir)
		if err != nil {
			return err
		}
		infraDir = filepath.Join(wd, "buildbot")

		// We get depot_tools via isolate. It's required for some tests.
		env = append(env, fmt.Sprintf("SKIABOT_TEST_DEPOT_TOOLS=%s", dirs.DepotTools(*workdir)))

		// Initialize the Git repo. We receive the code via Isolate, but it
		// doesn't include the .git dir.
		gd := git.GitDir(infraDir)
		if gitVer, err := gd.Git(ctx, "version"); err != nil {
			return err
		} else {
			sklog.Infof("Git version %s", gitVer)
		}
		if _, err := gd.Git(ctx, "init"); err != nil {
			return err
		}
		if _, err := gd.Git(ctx, "config", "--local", "user.name", "Skia bots"); err != nil {
			return err
		}
		if _, err := gd.Git(ctx, "config", "--local", "user.email", "fake@skia.bots"); err != nil {
			return err
		}
		if _, err := gd.Git(ctx, "add", "."); err != nil {
			return err
		}
		if _, err := gd.Git(ctx, "commit", "--no-verify", "-m", "Fake commit to satisfy recipe tests"); err != nil {
			return err
		}

		// Print Go info.
		ctx = golang.WithEnv(ctx, wd)
		goExc, goVer, err := golang.Info(ctx)
		if err != nil {
			return err
		}
		sklog.Infof("Using Go from %s", goExc)
		sklog.Infof("Go version %s", goVer)

		// Sync dependencies.
		if err := golang.ModDownload(ctx, infraDir); err != nil {
			return err
		}
		if err := golang.InstallCommonDeps(ctx, infraDir); err != nil {
			return err
		}

		// Start the emulators, if necessary.
		if *runEmulators {
			if _, err := exec.RunCwd(ctx, infraDir, runEmulatorsPath, "start"); err != nil {
				return err
			}
			env = append(env,
				"DATASTORE_EMULATOR_HOST=localhost:8891",
				"BIGTABLE_EMULATOR_HOST=localhost:8892",
				"PUBSUB_EMULATOR_HOST=localhost:8893",
				"FIRESTORE_EMULATOR_HOST=localhost:8894",
			)
		}
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}

	// Defer the teardown steps.
	defer func() {
		if err := td.Do(ctx, td.Props("Teardown").Infra(), func(ctx context.Context) error {
			// Sanity check; none of the above should have modified the go.mod file.
			if _, err := git.GitDir(infraDir).Git(ctx, "diff", "--no-ext-diff", "--exit-code", "go.mod"); err != nil {
				return err
			}
			// Stop the emulators, if necessary.
			if *runEmulators {
				if _, err := exec.RunCwd(ctx, infraDir, runEmulatorsPath, "stop"); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			td.Fatal(ctx, err)
		}
	}()

	// Run the tests.
	ctx = golang.WithEnv(ctx, wd)
	cmd := append([]string{"./..."}, flag.Args()...)
	if err := golang.Test(td.WithEnv(ctx, env), infraDir, cmd...); err != nil {
		td.Fatal(ctx, err)
	}
}
