package main

/*
	docker-build is a thin wrapper around "docker build" and "docker push".
*/

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"go.skia.org/infra/docker_experiment/go/config"
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

	// Run arguments.
	inputFlags     = common.NewMultiStringFlag("input", nil, "Docker images used as input for this task. Must be specified using sha256 sum or be present in one of the --input-manifests.  Format is <source dir>:<mount location>:<image>")
	envs           = common.NewMultiStringFlag("env", nil, "Environment variables in \"<key>=<value>\" format.")
	volumeFlags    = common.NewMultiStringFlag("volume", nil, "Volumenes to mount in \"<src>:<dst>\" format.")
	image          = flag.String("image", "", "Image to run.")
	inputManifests = common.NewMultiStringFlag("input-manifest", nil, "Docker image manifest file, with lines in \"<image>@sha256<sum>\" format.")
	authDir        = flag.String("auth-dir", "", "Host-side directory used for auth.")
	mountDir       = flag.String("mount-dir", "", "Host-to-container mapping used for mounting volumes into the sub-container.")

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	if *image == "" {
		td.Fatalf(ctx, "--image is required.")
	}

	var volumes []string
	if *authDir != "" {
		volumes = append(volumes, fmt.Sprintf("%s:/auth", *authDir))
	}
	if volumeFlags != nil {
		for _, v := range *volumeFlags {
			volumes = append(volumes, v)
		}
	}
	var d *docker.Docker
	var tmp string
	var resolvedImage string
	if err := td.Do(ctx, td.Props("Setup").Infra(), func(ctx context.Context) error {
		// Parse the inputs.
		var inputs []*config.DockerRunInput
		if inputFlags != nil {
			inputs = make([]*config.DockerRunInput, 0, len(*inputFlags))
			for _, a := range *inputFlags {
				split := strings.Split(a, ":")
				if len(split) < 3 {
					return fmt.Errorf("Invalid format for --input; expected \"<image>:<source>:<mount>\" but got %q", a)
				}

				inputs = append(inputs, &config.DockerRunInput{
					Image:  strings.Join(split[:len(split)-2], ":"),
					Source: split[len(split)-2],
					Mount:  split[len(split)-1],
				})
			}
		}

		// Read the input manifests.
		inputToSha256 := map[string]string{}
		for _, m := range *inputManifests {
			content, err := os_steps.ReadFile(ctx, m)
			if err != nil {
				return err
			}
			for _, line := range strings.Split(string(content), "\n") {
				if line == "" {
					continue
				}
				split := strings.Split(line, "@sha256:")
				if len(split) != 2 {
					return fmt.Errorf("Invalid input manifest format in %s; expected \"<image>@sha256:<sum>\" but got %q", m, line)
				}
				inputToSha256[split[0]] = split[1]
			}
		}

		// Resolve the base image and inputs.
		resolve := func(a string) (string, error) {
			if len(strings.Split(a, "@sha256:")) == 2 {
				return a, nil
			}
			sha256, ok := inputToSha256[a]
			if ok {
				return fmt.Sprintf("%s@sha256:%s", a, sha256), nil
			}
			return "", fmt.Errorf("Failed to resolve %q; not found in any manifest.", a)
		}
		var err error
		resolvedImage, err = resolve(*image)
		if err != nil {
			return err
		}
		for _, inp := range inputs {
			resolved, err := resolve(inp.Image)
			if err != nil {
				return err
			}
			inp.Image = resolved
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
			return err
		}

		// Download and extract the inputs.
		mountSplit := strings.SplitN(*mountDir, ":", 2)
		if len(mountSplit) != 2 {
			return fmt.Errorf("Expected format \"<host path>:<container path>\" for --mount-dir")
		}
		mountHost := mountSplit[0]
		mountLocal := mountSplit[1]
		if err := os_steps.MkdirAll(ctx, mountLocal); err != nil {
			return err
		}
		for idx, inp := range inputs {
			dest := filepath.Join(mountLocal, fmt.Sprintf("input_%d", idx))
			if err := d.Extract(ctx, inp.Image, inp.Source, dest); err != nil {
				return err
			}
			volumes = append(volumes, fmt.Sprintf("%s/input_%d:%s", mountHost, idx, inp.Mount))
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

	// Docker run.
	cmd := flag.Args()
	// TODO(borenet): Seems like Run() should just take a slice of
	// environment variables.
	var env []string
	if envs != nil {
		env = *envs
	}
	if err := d.Run(ctx, resolvedImage, cmd, volumes, env); err != nil {
		td.Fatal(ctx, err)
	}
}
