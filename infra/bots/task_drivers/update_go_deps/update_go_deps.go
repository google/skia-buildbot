package main

import (
	"flag"
	"path"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/gerrit_steps"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	gerritProject = flag.String("gerrit_project", "", "Gerrit project name.")
	gerritUrl     = flag.String("gerrit_url", "", "URL of the Gerrit server.")
	projectId     = flag.String("project_id", "", "ID of the Google Cloud project.")
	reviewers     = flag.String("reviewers", "", "Comma-separated list of emails to review the CL.")
	taskId        = flag.String("task_id", "", "ID of this task.")
	taskName      = flag.String("task_name", "", "Name of the task.")
	workdir       = flag.String("workdir", ".", "Working directory")

	checkoutFlags = checkout.SetupFlags(nil)

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	rs, err := checkout.GetRepoState(checkoutFlags)
	if err != nil {
		td.Fatal(ctx, err)
	}
	if *gerritProject == "" {
		td.Fatalf(ctx, "--gerrit_project is required.")
	}
	if *gerritUrl == "" {
		td.Fatalf(ctx, "--gerrit_url is required.")
	}

	wd, err := os_steps.Abs(ctx, *workdir)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Check out the code.
	co, err := checkout.EnsureGitCheckout(ctx, path.Join(wd, "repo"), rs)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Setup go.
	ctx = golang.WithEnv(ctx, wd)

	// Perform steps to update the dependencies.
	// By default, the Go env includes GOFLAGS=-mod=readonly, which prevents
	// commands from modifying go.mod; in this case, we want to modify it,
	// so unset that variable.
	ctx = td.WithEnv(ctx, []string{"GOFLAGS="})
	if _, err := golang.Go(ctx, co.Dir(), "get", "-u"); err != nil {
		td.Fatal(ctx, err)
	}

	// Install some tool dependencies.
	if err := golang.InstallCommonDeps(ctx, co.Dir()); err != nil {
		td.Fatal(ctx, err)
	}

	// These commands may also update dependencies, or their results may
	// change based on the updated dependencies.
	if _, err := golang.Go(ctx, co.Dir(), "build", "./..."); err != nil {
		td.Fatal(ctx, err)
	}
	// Setting -exec=echo causes the tests to not actually run; therefore
	// this compiles the tests but doesn't run them.
	if _, err := golang.Go(ctx, co.Dir(), "test", "-exec=echo", "./..."); err != nil {
		td.Fatal(ctx, err)
	}
	if _, err := golang.Go(ctx, co.Dir(), "generate", "./..."); err != nil {
		td.Fatal(ctx, err)
	}
	// Regenerate the licenses file.
	if _, err := exec.RunCwd(ctx, filepath.Join(co.Dir(), "licenses"), "make", "regenerate"); err != nil {
		td.Fatal(ctx, err)
	}

	// If we changed anything, upload a CL.
	g, err := gerrit_steps.Init(ctx, *local, *gerritUrl)
	if err != nil {
		td.Fatal(ctx, err)
	}
	isTryJob := *local || rs.Issue != ""
	if err := gerrit_steps.UploadCL(ctx, g, co, *gerritProject, "master", rs.Revision, "Update Go Deps", strings.Split(*reviewers, ","), isTryJob); err != nil {
		td.Fatal(ctx, err)
	}
}
