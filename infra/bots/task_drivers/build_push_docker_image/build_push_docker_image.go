package main

import (
	"flag"
	"fmt"
	"path"
	"path/filepath"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/docker"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	gerritProject = flag.String("gerrit_project", "", "Gerrit project name.")
	gerritUrl     = flag.String("gerrit_url", "", "URL of the Gerrit server.")
	projectId     = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId        = flag.String("task_id", "", "ID of this task.")
	taskName      = flag.String("task_name", "", "Name of the task.")
	workdir       = flag.String("workdir", ".", "Working directory")

	dockerfileDir = flag.String("dockerfile_dir", "", "Directory that contains the Dockerfile that should be built and pushed.")
	imageName     = flag.String("image_name", "", "Name of the image to build and push to docker. Eg: gcr.io/skia-public/infra")

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
	if *imageName == "" {
		td.Fatalf(ctx, "--image_name is required.")
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

	// TODO(rmistry): Needed?
	// Setup go.
	ctx = golang.WithEnv(ctx, wd)

	// Figure out which tag to use.
	tag := rs.Revision
	if rs.Issue != "" && rs.Patchset != "" {
		tag = fmt.Sprintf("%s_%s", rs.Issue, rs.Patchset)
	}
	imageWithTag := fmt.Sprintf("%s:%s", *imageName, tag)

	// Build docker image.
	if err := docker.Build(ctx, filepath.Join(co.Dir(), *dockerfileDir), imageWithTag); err != nil {
		td.Fatal(ctx, err)
	}

	// Login to docker (required to push to docker).
	ts, err := auth_steps.Init(ctx, *local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_FULL_CONTROL)
	if err != nil {
		td.Fatal(ctx, err)
	}
	token, err := ts.Token()
	if err != nil {
		td.Fatal(ctx, err)
	}
	if err := docker.Login(ctx, token.AccessToken, *imageName); err != nil {
		td.Fatal(ctx, err)
	}

	// Push to docker.
	if err := docker.Push(ctx, imageWithTag); err != nil {
		td.Fatal(ctx, err)
	}
}
