package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"

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
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	// Setup Go.
	wd, err := os_steps.Abs(ctx, *workdir)
	if err != nil {
		td.Fatal(ctx, err)
	}
	ctx = golang.WithEnv(ctx, wd)
	infraDir := filepath.Join(wd, "buildbot")

	// We get depot_tools via isolate. It's required for some tests.
	ctx = td.WithEnv(ctx, []string{fmt.Sprintf("SKIABOT_TEST_DEPOT_TOOLS=%s", dirs.DepotTools(*workdir))})

	// Initialize the Git repo. We receive the code via Isolate, but it
	// doesn't include the .git dir.
	gd := git.GitDir(infraDir)
	if gitVer, err := gd.Git(ctx, "version"); err != nil {
		td.Fatal(ctx, err)
	} else {
		sklog.Infof("Git version %s", gitVer)
	}
	if _, err := gd.Git(ctx, "init"); err != nil {
		td.Fatal(ctx, err)
	}
	if _, err := gd.Git(ctx, "config", "--local", "user.name", "Skia bots"); err != nil {
		td.Fatal(ctx, err)
	}
	if _, err := gd.Git(ctx, "config", "--local", "user.email", "fake@skia.bots"); err != nil {
		td.Fatal(ctx, err)
	}
	if _, err := gd.Git(ctx, "add", "."); err != nil {
		td.Fatal(ctx, err)
	}
	if _, err := gd.Git(ctx, "commit", "-m", "Fake commit to satisfy recipe tests"); err != nil {
		td.Fatal(ctx, err)
	}

	// For Large/Race, start the Cloud Datastore emulator.
	if strings.Contains(*taskName, "Large") || strings.Contains(*taskName, "Race") {
		runEmulators := filepath.Join(infraDir, "scripts", "run_emulators", "run_emulators")
		if _, err := exec.RunCwd(ctx, infraDir, runEmulators, "start"); err != nil {
			td.Fatal(ctx, err)
		}
		ctx = td.WithEnv(ctx, []string{
			"DATASTORE_EMULATOR_HOST=localhost:8891",
			"BIGTABLE_EMULATOR_HOST=localhost:8892",
			"PUBSUB_EMULATOR_HOST=localhost:8893",
			"FIRESTORE_EMULATOR_HOST=localhost:8894",
		})
		defer func() {
			if _, err := exec.RunCwd(ctx, infraDir, runEmulators, "stop"); err != nil {
				td.Fatal(ctx, err)
			}
		}()
	}

	// Print Go info.
	goExc, goVer, err := golang.Info(ctx)
	if err != nil {
		td.Fatal(ctx, err)
	}
	sklog.Infof("Using Go from %s", goExc)
	sklog.Infof("Go version %s", goVer)

	// Sync dependencies.
	if err := golang.ModDownload(ctx, infraDir); err != nil {
		td.Fatal(ctx, err)
	}
	if err := golang.InstallCommonDeps(ctx, infraDir); err != nil {
		td.Fatal(ctx, err)
	}

	// Run the tests.
	cmd := []string{"run", "./run_unittests.go", "--alsologtostderr"}
	if strings.Contains(*taskName, "Race") {
		cmd = append(cmd, "--race", "--large", "--medium", "--small")
	} else if strings.Contains(*taskName, "Large") {
		cmd = append(cmd, "--large")
	} else if strings.Contains(*taskName, "Medium") {
		cmd = append(cmd, "--medium")
	} else {
		cmd = append(cmd, "--small")
	}
	if _, err := golang.Go(ctx, infraDir, cmd...); err != nil {
		td.Fatal(ctx, err)
	}

	// Sanity check; none of the above should have modified the go.mod file.
	if _, err := exec.RunCwd(ctx, infraDir, "git", "diff", "--no-ext-diff", "--exit-code", "go.mod"); err != nil {
		td.Fatal(ctx, err)
	}
}
