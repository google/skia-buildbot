package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectId   = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId      = flag.String("task_id", "", "ID of this task.")
	taskName    = flag.String("task_name", "", "Name of the task.")
	repo        = flag.String("repo", "", "URL of the repo.")
	revision    = flag.String("revision", "", "Git revision to test.")
	patchIssue  = flag.String("patch_issue", "", "Issue ID, required if this is a try job.")
	patchSet    = flag.String("patch_set", "", "Patch Set ID, required if this is a try job.")
	patchServer = flag.String("patch_server", "", "Code review server, required if this is a try job.")

	// Optional flags.
	workdir = flag.String("workdir", ".", "Working directory")
	local   = flag.Bool("local", false, "True if running locally (as opposed to in production)")
	output  = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

// goVars returns the target directory for the infra repo and the full set of
// environment variables which should be used for running Go commands.
func goVars(ctx context.Context, workdir string) (string, []string) {
	goPath := path.Join(workdir, "gopath")
	goRoot := path.Join(workdir, "go", "go")
	goBin := path.Join(goRoot, "bin")
	infraDir := path.Join(workdir, "buildbot")

	depotToolsDir := path.Join(workdir, "depot_tools")
	PATH := strings.Join([]string{
		goBin,
		path.Join(goPath, "bin"),
		path.Join(workdir, "gcloud_linux", "bin"),
		path.Join(workdir, "protoc", "bin"),
		path.Join(workdir, "node", "node", "bin"),
		os.Getenv("PATH"),
		depotToolsDir,
	}, string(os.PathListSeparator))
	env := []string{
		"CHROME_HEADLESS=1",
		fmt.Sprintf("GOCACHE=%s", path.Join(workdir, "cache", "go_cache")),
		"GOFLAGS=-mod=readonly", // Prohibit builds from modifying go.mod.
		fmt.Sprintf("GOROOT=%s", goRoot),
		fmt.Sprintf("GOPATH=%s", goPath),
		"GIT_USER_AGENT=git/1.9.1", // I don't think this version matters.
		fmt.Sprintf("PATH=%s", PATH),
		fmt.Sprintf("SKIABOT_TEST_DEPOT_TOOLS=%s", depotToolsDir),
	}
	return infraDir, env
}

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)
	if *repo == "" {
		td.Fatalf(ctx, "--repo is required.")
	}
	if *revision == "" {
		td.Fatalf(ctx, "--revision is required.")
	}

	// Setup Go.
	wd, err := os_steps.Abs(ctx, *workdir)
	if err != nil {
		td.Fatal(ctx, err)
	}
	infraDir, goEnv := goVars(ctx, wd)

	// Initialize the Git repo. We receive the code via Isolate, but it
	// doesn't include the .git dir.
	// with cwd = infraDir:
	gd := git.GitDir(infraDir)
	if _, err := gd.Git(ctx, "init"); err != nil {
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
		d := path.Join(infraDir, "go", "ds", "emulator")
		if _, err := exec.RunCwd(ctx, d, "./run_emulator", "start"); err != nil {
			td.Fatal(ctx, err)
		}
		goEnv = append(goEnv, "DATASTORE_EMULATOR_HOST=localhost:8891")
		goEnv = append(goEnv, "BIGTABLE_EMULATOR_HOST=localhost:8892")
		goEnv = append(goEnv, "PUBSUB_EMULATOR_HOST=localhost:8893")
		defer func() {
			if _, err := exec.RunCwd(ctx, d, "./run_emulator", "stop"); err != nil {
				td.Fatal(ctx, err)
			}
		}()
	}

	// The remainder of the steps want the Go environment variables.
	if err := td.Do(ctx, td.Props("Set Go Environment").Env(goEnv), func(ctx context.Context) (rvErr error) {
		// Print Go info.
		out, err := exec.RunCwd(ctx, ".", "which", "go")
		if err != nil {
			return err
		}
		sklog.Infof("Using Go from %s", out)
		out, err = exec.RunCwd(ctx, ".", "go", "version")
		if err != nil {
			return err
		}
		sklog.Infof("Go version %s", out)

		// Sync dependencies.
		if _, err := exec.RunCwd(ctx, infraDir, "go", "mod", "download"); err != nil {
			return err
		}
		installTargets := []string{
			"github.com/golang/protobuf/protoc-gen-go",
			"github.com/kisielk/errcheck",
			"golang.org/x/tools/cmd/goimports",
			"golang.org/x/tools/cmd/stringer",
		}
		for _, target := range installTargets {
			if _, err := exec.RunCwd(ctx, infraDir, "go", "install", "-v", target); err != nil {
				return err
			}
		}

		// More prerequisites.
		if !strings.Contains(*taskName, "Race") {
			if _, err := exec.RunCwd(ctx, ".", "sudo", "npm", "i", "-g", "bower@1.8.2"); err != nil {
				return err
			}
		}
		if !strings.Contains(*taskName, "Build") {
			if _, err := exec.RunCwd(ctx, path.Join(infraDir, "go", "database"), "./setup_test_db"); err != nil {
				return err
			}
		}

		// Run the tests.
		cmd := []string{"go", "run", "./run_unittests.go", "--alsologtostderr"}
		if strings.Contains(*taskName, "Race") {
			cmd = append(cmd, "--race", "--large", "--medium", "--small")
		} else if strings.Contains(*taskName, "Large") {
			cmd = append(cmd, "--large")
		} else if strings.Contains(*taskName, "Medium") {
			cmd = append(cmd, "--medium")
		} else {
			cmd = append(cmd, "--small")
		}
		if _, err := exec.RunCwd(ctx, infraDir, cmd...); err != nil {
			return err
		}

		// Sanity check; none of the above should have modified the go.mod file.
		if _, err := exec.RunCwd(ctx, infraDir, "git", "diff", "--no-ext-diff", "--exit-code", "go.mod"); err != nil {
			return err
		}

		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}
}
