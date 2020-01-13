// update_go_deps modifies the go.mod and go.sum files to sync to the most
// recent versions of all listed dependencies.
//
// If the go.mod file is not being updated, check the recent runs of this Task
// Driver to verify that:
//
// 1. It is running at all. If not, there may be a bot capacity problem, or a
//    problem with the Task Scheduler.
// 2. It is succeeding. There are a number of reasons why it might fail, but the
//    most common is that a change has landed in one of the dependencies which
//    is not compatible with the current version of our code. Check the logs for
//    the failing step(s). Note that dependencies may be shared, and upstream
//    changes can result in a dependency graph which is impossible to satisfy.
//    In this case, you may need to fork a dependency to keep it at a working
//    revision, or disable this task until fixes propagate through the graph.
// 3. The CL uploaded by this task driver is passing the commit queue and
//    landing. This task driver does not run all of the tests and so the CL it
//    uploads may fail the commit queue for legitimate reasons. Look into the
//    failures and determine whether fixes need to be applied in this repo, a
//    dependency needs to be pinned to a different release, forked, etc.
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
