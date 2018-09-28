package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/test_automation"
	"go.skia.org/infra/task_scheduler/go/db"
)

var (
	// Required properties for this task.
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

func main() {
	// Setup.
	common.InitWithMust(
		"borenet-testing",
		common.CloudLoggingDefaultAuthOpt(local),
	)
	s, err := test_automation.New(*output)
	if err != nil {
		sklog.Fatal(err)
	}
	defer s.Done(nil)
	sklog.Infof("Environment:\n%s", strings.Join(os.Environ(), "\n"))
	if *taskName == "" {
		sklog.Fatal("--task_name is required.")
	}
	if *repo == "" {
		sklog.Fatal("--repo is required.")
	}
	if *revision == "" {
		sklog.Fatal("--revision is required.")
	}

	// Check out code.
	*workdir, err = filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}
	goPath := path.Join(*workdir, "gopath")
	goSrc := path.Join(goPath, "src")
	if err := test_automation.MkdirAll(s, goSrc); err != nil {
		sklog.Fatal(err)
	}
	goRoot := path.Join(*workdir, "go", "go")
	goBin := path.Join(goRoot, "bin")
	checkoutRoot := path.Join(goSrc, "go.skia.org")
	infraDir := path.Join(checkoutRoot, "infra")

	if strings.HasSuffix(*repo, ".git") {
		*repo = (*repo)[:len(*repo)-len(".git")]
	}
	repoState := db.RepoState{
		Repo:     *repo,
		Revision: *revision,
		Patch: db.Patch{
			Issue:    *patchIssue,
			Patchset: *patchSet,
			Server:   *patchServer,
		},
	}
	if _, err = test_automation.EnsureGitCheckout(s, infraDir, repoState); err != nil {
		sklog.Fatal(err)
	}

	// Setup environment.
	depotToolsDir := path.Join(*workdir, "depot_tools")
	PATH := strings.Join([]string{
		goBin,
		path.Join(goPath, "bin"),
		path.Join(*workdir, "gcloud_linux", "bin"),
		path.Join(*workdir, "protoc", "bin"),
		path.Join(*workdir, "node", "node", "bin"),
		os.Getenv("PATH"),
		depotToolsDir,
	}, string(os.PathListSeparator))
	// This is needed because exec.Command.Start() requires the executable
	// to be in this process's PATH.
	if err := os.Setenv("PATH", PATH); err != nil {
		sklog.Fatal(err)
	}
	env := []string{
		"CHROME_HEADLESS=1",
		fmt.Sprintf("GOROOT=%s", goRoot),
		fmt.Sprintf("GOPATH=%s", goPath),
		"GIT_USER_AGENT=git/1.9.1", // I don't think this version matters.
		fmt.Sprintf("PATH=%s", PATH),
		fmt.Sprintf("SKIABOT_TEST_DEPOT_TOOLS=%s", depotToolsDir),
		"TMPDIR=",
	}

	// Print Go info.
	out, err := s.Step().Infra().Name("which go").Env(env).RunCwd(".", "which", "go")
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Using Go from %s", out)
	out, err = s.Step().Infra().Name("go version").Env(env).RunCwd(".", "go", "version")
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Go version %s", out)

	// Update Go deps. This will undo local changes.
	if _, err := s.Step().Name("update deps").Env(env).RunCwd(infraDir, "go", "get", "-u", "-t", "./..."); err != nil {
		sklog.Fatal(err)
	}

	// Checkout AGAIN to undo whatever changes were just made.
	if _, err = test_automation.EnsureGitCheckout(s, infraDir, repoState); err != nil {
		sklog.Fatal(err)
	}

	// More prerequisites.
	/*if !strings.Contains(*taskName, "Race") {
		if _, err := s.Step().Name("install bower").RunCwd(".", "sudo", "npm", "i", "-g", "bower@1.8.2"); err != nil {
			sklog.Fatal(err)
		}
		if _, err := s.Step().Name("install go deps").Env(env).RunCwd(".", "./scripts/install_go_deps.sh"); err != nil {
			sklog.Fatal(err)
		}
	}
	if _, err := s.Step().Name("setup database").RunCwd(path.Join(infraDir, "go", "database"), "./setup_test_db"); err != nil {
		sklog.Fatal(err)
	}*/

	// For Large/Race, start the Cloud Datastore emulator.
	if strings.Contains(*taskName, "Large") || strings.Contains(*taskName, "Race") {
		d := path.Join(infraDir, "go", "ds", "emulator")
		s.Step().Name("Start DS Emulator").RunCwd(d, "./run_emulator", "start")
		env = append(env, "DATASTORE_EMULATOR_HOST=localhost:8891")
		env = append(env, "BIGTABLE_EMULATOR_HOST=localhost:8892")
		env = append(env, "PUBSUB_EMULATOR_HOST=localhost:8893")
		defer func() {
			s.Step().Name("Stop DS Emulator").RunCwd(d, "./run_emulator", "stop")
		}()
	}

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
	if _, err := s.Step().Name("Run Unit Tests").Env(env).RunCwd(".", cmd...); err != nil {
		sklog.Fatal(err)
	}
}
