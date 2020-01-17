package main

/*
	docker_build is a thin wrapper around "docker build" and "docker push".
*/

import (
	"context"
	"flag"
	"fmt"
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
	buildArgs   = common.NewMultiStringFlag("build-arg", nil, "Set build-time variables in \"<name>=<value>\" format.")
	buildDir    = flag.String("build-dir", ".", "Directory to build.")
	buildFile   = flag.String("build-file", "", "Path to the Dockerfile to build.")
	buildLabels = common.NewMultiStringFlag("build-label", nil, "Set metadata for an image.")
	buildTags   = common.NewMultiStringFlag("build-tag", nil, "Name and optionally a tag in \"<name>:<tag>\" format.")
	buildTarget = flag.String("build-target", "", "Set the target build stage to build.")
	pushAs      = flag.String("push-as", "", "Push the image with the given name and tag.")

	// Run arguments. If any is provided, we're running a command.
	runEnv    = common.NewMultiStringFlag("run-env", nil, "Environment variables in \"<key>=<value>\" format.")
	runVolume = common.NewMultiStringFlag("run-volume", nil, "Volumenes to mount in \"<src>:<dst>\" format.")
	runImage  = flag.String("run-image", "", "Image to run; mutually exclusive with --build-*")

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

	// Build stage.
	if *runImage == "" {
		if err := td.Do(ctx, td.Props("Build"), func(ctx context.Context) error {
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
			if *buildLabels != nil {
				for _, label := range *buildLabels {
					args = append(args, "--label", label)
				}
			}
			if *buildTags != nil {
				for _, tag := range *buildTags {
					args = append(args, "--tag", tag)
				}
			}
			if *buildTarget != "" {
				args = append(args, "--target", *buildTarget)
			}
			if err := d.Build(ctx, args...); err != nil {
				return err
			}

			// Read the ID of the image we built.
			contents, err := os_steps.ReadFile(ctx, iidFile)
			if err != nil {
				return err
			}
			*runImage = strings.TrimSpace(string(contents))

			// Run "docker push" if requested.
			if *pushAs != "" {
				if err := d.Tag(ctx, *runImage, *pushAs); err != nil {
					return err
				}
				if err := d.Push(ctx, *pushAs); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			td.Fatal(ctx, err)
		}
	}

	// Run stage.
	runCmd := flag.Args()
	if len(runCmd) != 0 {
		// TODO(borenet): Seems like Run() should just take a slice of
		// environment variables.
		var env map[string]string
		if runEnv != nil {
			env = make(map[string]string, len(*runEnv))
			for _, e := range *runEnv {
				split := strings.SplitN(e, ":", 2)
				if len(split) != 2 {
					td.Fatal(ctx, fmt.Errorf("Expected environment variables in the form \"<key>=<value>\"; not %q", e))
				}
				env[split[0]] = env[split[1]]
			}
		}
		if err := d.Run(ctx, *runImage, runCmd, *runVolume, env); err != nil {
			td.Fatal(ctx, err)
		}
	}
}
