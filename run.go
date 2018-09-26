package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/test_automation"
	"go.skia.org/infra/task_scheduler/go/db"
)

var (
	output   = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
	taskName = flag.String("task_name", "", "Name of the task.")
	workdir  = flag.String("workdir", "", "Working directory")
	t        *test_automation.Step
)

func main() {
	// Setup.
	common.Init()
	s, err := test_automation.New(*output)
	if err != nil {
		sklog.Fatal(err)
	}
	defer s.Done()
	if *taskName == "" {
		sklog.Fatal("--task_name is required.")
	}
	if *workdir == "" {
		sklog.Fatal("--workdir is required.")
	}

	// Check out code.
	goPath := path.Join(*workdir, "gopath")
	goSrc := path.Join(goPath, "src")
	if err := test_automation.MkdirAll(s, goSrc); err != nil {
		sklog.Fatal(err)
	}
	goRoot := path.Join(*workdir, "go", "go")
	goBin := path.Join(*workdir, "go", "bin")
	checkoutRoot := path.Join(goSrc, "go.skia.org")
	infraDir := path.Join(checkoutRoot, "infra")

	repoState := db.RepoState{
		// TODO(borenet): Plumb through properties.
		Repo:     common.REPO_SKIA_INFRA,
		Revision: "fbaf36f34690c6e55f920b2610c3553247fc4a9d",
		Patch: db.Patch{
			Issue:    "156860",
			Patchset: "3",
			Server:   "https://skia-review.googlesource.com",
		},
	}
	if _, err = test_automation.EnsureGitCheckout(s, infraDir, repoState); err != nil {
		sklog.Fatal(err)
	}

	// Setup environment.
	goEnv := []string{
		"CHROME_HEADLESS=1",
		fmt.Sprintf("GOROOT=%s", goRoot),
		fmt.Sprintf("GOPATH=%s", goPath),
		"GIT_USER_AGENT=git/1.9.1", // I don't think this version matters.
		fmt.Sprintf("PATH=%s", strings.Join([]string{
			goBin,
			path.Join(goPath, "bin"),
			path.Join(*workdir, "gcloud_linux", "bin"),
			path.Join(*workdir, "protoc", "bin"),
			path.Join(*workdir, "node", "node", "bin"),
			os.Getenv("PATH"),
		}, string(os.PathSeparator))),
	}
	// TODO(borenet): There needs to be a better way to do this.
	for _, v := range goEnv {
		split := strings.SplitN(v, "=", 2)
		if len(split) != 2 {
			panic(v)
		}
		if err := os.Setenv(split[0], split[1]); err != nil {
			sklog.Fatal(err)
		}
	}

	// Print Go info.
	out, err := s.Step().Infra().Name("which go").RunCwd(".", "which", "go")
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Using Go from %s", out)
	out, err = s.Step().Infra().Name("go version").RunCwd(".", "go", "version")
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Go version %s", out)

	// Update Go deps. This will undo local changes.
	if _, err := s.Step().Name("update deps").RunCwd(infraDir, "go", "get", "-u", "-t", "./..."); err != nil {
		sklog.Fatal(err)
	}

	// Checkout AGAIN to undo whatever changes were just made.
	if _, err = test_automation.EnsureGitCheckout(s, infraDir, repoState); err != nil {
		sklog.Fatal(err)
	}

	// More prerequisites.
	if !strings.Contains(*taskName, "Race") {
		if _, err := s.Step().Name("install bower").RunCwd(".", "sudo", "npm", "i", "-g", "bower@1.8.2"); err != nil {
			sklog.Fatal(err)
		}
		if _, err := s.Step().Name("install go deps").RunCwd(".", "./scripts/install_go_deps.sh"); err != nil {
			sklog.Fatal(err)
		}
	}
	if _, err := s.Step().Name("setup database").RunCwd(path.Join(infraDir, "go", "database"), "./setup_test_db"); err != nil {
		sklog.Fatal(err)
	}
}
