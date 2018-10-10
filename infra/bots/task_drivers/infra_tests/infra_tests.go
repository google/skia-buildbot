package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/task_driver"
	"go.skia.org/infra/go/task_driver/lib/checkout"
	"go.skia.org/infra/go/task_driver/lib/os_steps"
	"go.skia.org/infra/task_scheduler/go/db"
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

func main() {
	// Setup.
	s := task_driver.MustInit(projectId, taskId, taskName, output, local)
	defer s.Done(nil)
	if *repo == "" {
		sklog.Fatal("--repo is required.")
	}
	if *revision == "" {
		sklog.Fatal("--revision is required.")
	}

	// Check out code.
	var err error
	*workdir, err = filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}
	goPath := path.Join(*workdir, "go_deps")
	goSrc := path.Join(goPath, "src")
	if err := os_steps.MkdirAll(s, goSrc); err != nil {
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
	if _, err = checkout.EnsureGitCheckout(s, infraDir, repoState); err != nil {
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
	s = s.Step().Env(env).Start()
	defer s.Done(nil)
	out, err := s.RunCwd(".", "which", "go")
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Using Go from %s", out)
	out, err = s.RunCwd(".", "go", "version")
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Go version %s", out)

	// More prerequisites.
	if !strings.Contains(*taskName, "Race") {
		if _, err := s.RunCwd(".", "sudo", "npm", "i", "-g", "bower@1.8.2"); err != nil {
			sklog.Fatal(err)
		}
	}
	/*if _, err := s.Step().Name("setup database").RunCwd(path.Join(infraDir, "go", "database"), "./setup_test_db"); err != nil {
		sklog.Fatal(err)
	}*/

	// For Large/Race, start the Cloud Datastore emulator.
	if strings.Contains(*taskName, "Large") || strings.Contains(*taskName, "Race") {
		d := path.Join(infraDir, "go", "ds", "emulator")
		if err := s.RunCwd(d, "./run_emulator", "start"); err != nil {
			sklog.Fatal(err)
		}
		env = append(env, "DATASTORE_EMULATOR_HOST=localhost:8891")
		env = append(env, "BIGTABLE_EMULATOR_HOST=localhost:8892")
		env = append(env, "PUBSUB_EMULATOR_HOST=localhost:8893")
		defer func() {
			if err := s.RunCwd(d, "./run_emulator", "stop"); err != nil {
				sklog.Fatal(err)
			}
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
	if _, err := s.RunCwd(infraDir, cmd...); err != nil {
		sklog.Fatal(err)
	}
}
