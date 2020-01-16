package main

/*
	docker_build is a thin wrapper around "docker build" and "docker push".
*/

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"go.skia.org/infra/task_driver/go/lib/docker"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectId = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId    = flag.String("task_id", "", "ID of this task.")
	taskName  = flag.String("task_name", "", "Name of the task.")

	buildArgs  = common.NewMultiStringFlag("build-arg", nil, "Set build-time variables in \"<name>=<value>\" format.")
	dir        = flag.String("dir", ".", "Directory to build.")
	dockerfile = flag.String("file", "", "Path to the Dockerfile to build.")
	labels     = common.NewMultiStringFlag("label", nil, "Set metadata for an image.")
	tags       = common.NewMultiStringFlag("tag", nil, "Name and optionally a tag in \"<name>:<tag>\" format.")
	target     = flag.String("target", "", "Set the target build stage to build.")

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	// Create token source with scope for cloud registry (storage).
	ts, err := auth_steps.Init(ctx, *local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_FULL_CONTROL)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Initialize Docker.
	fmt.Println("initializing docker")
	d, err := docker.New(ctx, ts)
	if err != nil {
		td.Fatal(ctx, err)
	}
	defer func() {
		fmt.Println("cleaning up docker")
		if err := d.Cleanup(ctx); err != nil {
			td.Fatal(ctx, err)
		}
		fmt.Println("done cleaning up docker")
	}()
	fmt.Println("initialized docker")

	// Create a scratch directory.
	tmp, err := os_steps.TempDir(ctx, "", "")
	if err != nil {
		td.Fatal(ctx, err)
	}
	defer func() {
		if err := os_steps.RemoveAll(ctx, tmp); err != nil {
			td.Fatal(ctx, err)
		}
	}()

	// Run "docker build".
	iidFile := filepath.Join(tmp, "img_id")
	args := []string{"--iidfile", iidFile, *dir}
	if *buildArgs != nil {
		for _, arg := range *buildArgs {
			args = append(args, "--build-arg", arg)
		}
	}
	if *dockerfile != "" {
		args = append(args, "--file", *dockerfile)
	}
	if *labels != nil {
		for _, label := range *labels {
			args = append(args, "--label", label)
		}
	}
	if *tags != nil {
		for _, tag := range *tags {
			args = append(args, "--tag", tag)
		}
	}
	if *target != "" {
		args = append(args, "--target", *target)
	}
	if err := d.Build(ctx, args...); err != nil {
		td.Fatal(ctx, err)
	}

	// Read the ID of the image we built.
	contents, err := os_steps.ReadFile(ctx, iidFile)
	if err != nil {
		td.Fatal(ctx, err)
	}
	sklog.Infof("Created: %s", strings.TrimSpace(string(contents)))
}
