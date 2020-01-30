package main

/*
	docker-build is a thin wrapper around "docker build" and "docker push".
*/

import (
	"context"
	"flag"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
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

	// Build arguments. If any is provided, we're performing a build.
	buildArgs    = common.NewMultiStringFlag("build-arg", nil, "Set build-time variables in \"<name>=<value>\" format.")
	buildDir     = flag.String("build-dir", ".", "Directory to build.")
	buildFile    = flag.String("build-file", "", "Path to the Dockerfile to build.")
	buildOutputs = common.NewMultiStringFlag("output", nil, "Images to build in \"[<build target>=]<image name>\" format.")

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	var outputs map[string]string // maps build target to image name.
	var d *docker.Docker
	var tmp string
	if err := td.Do(ctx, td.Props("Setup").Infra(), func(ctx context.Context) error {
		// Parse build outputs.
		if buildOutputs != nil {
			outputs := make(map[string]string, len(*buildOutputs))
			for _, out := range *buildOutputs {
				var image string
				var target string
				split := strings.SplitN(out, "=", 2)
				if len(split) == 1 {
					image = split[0]
				} else if len(split) == 2 {
					target = split[0]
					image = split[1]
				} else {
					td.Fatalf(ctx, "Incorrect format for --output; wanted [<build target>=]<image name> but got %q", out)
				}
				if exist, ok := outputs[target]; ok {
					if target == "" {
						td.Fatalf(ctx, "More than one output image specified without a target name: %s %s", exist, image)
					} else {
						td.Fatalf(ctx, "Multiple output images reference target %q: %s %s", target, exist, image)
					}
				}
				outputs[target] = image
			}
		}

		// Create token source with scope for cloud registry (storage).
		ts, err := auth_steps.Init(ctx, *local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_FULL_CONTROL)
		if err != nil {
			return err
		}

		// Initialize Docker.
		d, err = docker.New(ctx, ts)
		if err != nil {
			return err
		}

		// Create a scratch directory.
		tmp, err = os_steps.TempDir(ctx, "", "")
		if err != nil {
			td.Fatal(ctx, err)
		}
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}
	defer func() {
		// Cleanup.
		if err := td.Do(ctx, td.Props("Cleanup").Infra(), func(ctx context.Context) error {
			if err := os_steps.RemoveAll(ctx, tmp); err != nil {
				return err
			}
			if err := d.Cleanup(ctx); err != nil {
				return err
			}
			return nil
		}); err != nil {
			td.Fatal(ctx, err)
		}
	}()

	// Build stage.
	if len(outputs) == 0 {
		if err := build(ctx, d, tmp, "", ""); err != nil {
			td.Fatal(ctx, err)
		}
	} else {
		if err := td.Do(ctx, td.Props("Build"), func(ctx context.Context) error {
			for target, image := range outputs {
				if err := build(ctx, d, tmp, target, image); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			td.Fatal(ctx, err)
		}
	}
}

func build(ctx context.Context, d *docker.Docker, tmp, target, image string) error {
	name := "Build"
	if target != "" {
		name += " " + target
	}
	td.StartStep(ctx, td.Props(name))
	defer td.EndStep(ctx)

	// Run "docker build".
	iidFile := filepath.Join(tmp, "img_id")
	args := []string{"--iidfile", iidFile, *buildDir}
	if *buildArgs != nil {
		for _, arg := range *buildArgs {
			args = append(args, "--build-arg", arg)
		}
	}
	if *buildFile != "" {
		args = append(args, "--file", *buildFile)
	}
	if target != "" {
		args = append(args, "--target", target)
	}
	if err := d.Build(ctx, args...); err != nil {
		return err
	}

	// Read the ID of the image we built.
	contents, err := os_steps.ReadFile(ctx, iidFile)
	if err != nil {
		return err
	}
	imageId := strings.TrimSpace(string(contents))

	// Push the image, if it was named,
	if image != "" {
		if err := d.Tag(ctx, imageId, image); err != nil {
			return err
		}
		if err := d.Push(ctx, image); err != nil {
			return err
		}
	}
	return nil
}
