package main

/*
	docker-build is a thin wrapper around "docker build" and "docker push".
*/

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"

	"go.skia.org/infra/docker_experiment/go/config"
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

	// Build arguments.
	baseImage      = flag.String("base", "", "Base image. Must be specified using sha256 sum or be present in one of the --input-manifests.")
	inputFlags     = common.NewMultiStringFlag("input", nil, "Docker images used as input for this task. Must be specified using sha256 sum or be present in one of the --input-manifests.  Format is <source dir>:<dest dir>:<image>")
	buildArgs      = common.NewMultiStringFlag("build-arg", nil, "Set build-time variables in \"<name>=<value>\" format.")
	buildDir       = flag.String("build-dir", ".", "Directory to build.")
	buildFile      = flag.String("build-file", "", "Path to the Dockerfile to build.")
	buildOutputs   = common.NewMultiStringFlag("output", nil, "Images to build in \"[<build target>=]<image name>\" format.")
	inputManifests = common.NewMultiStringFlag("input-manifest", nil, "Docker image manifest file, with lines in \"<image>@sha256<sum>\" format.")
	outputManifest = flag.String("output-manifest", "", "Write this file containing a mapping of image name to sha256 sum.")

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
	var resolvedImage string
	if err := td.Do(ctx, td.Props("Setup").Infra(), func(ctx context.Context) error {
		// Parse the inputs.
		var inputs []*config.DockerBuildInput
		if inputFlags != nil {
			inputs = make([]*config.DockerBuildInput, 0, len(*inputFlags))
			for _, a := range *inputFlags {
				split := strings.Split(a, ":")
				if len(split) < 3 {
					return fmt.Errorf("Invalid format for --input; expected \"<image>:<source>:<mount>\" but got %q", a)
				}

				inputs = append(inputs, &config.DockerBuildInput{
					Image:  strings.Join(split[:len(split)-2], ":"),
					Source: split[len(split)-2],
					Dest:   split[len(split)-1],
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
		resolvedImage, err = resolve(*baseImage)
		if err != nil {
			return err
		}
		for _, a := range inputs {
			resolved, err := resolve(a.Image)
			if err != nil {
				return err
			}
			a.Image = resolved
		}

		// Parse build outputs.
		if buildOutputs != nil {
			outputs = make(map[string]string, len(*buildOutputs))
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
			return td.FailStep(ctx, err)
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

		// Download and extract the inputs.
		if err := os_steps.MkdirAll(ctx, *buildDir); err != nil {
			return err
		}
		for _, inp := range inputs {
			dest := filepath.Join(*buildDir, inp.Dest)
			if err := d.Extract(ctx, inp.Image, inp.Source, dest); err != nil {
				return err
			}
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
	var results []string
	if err := td.Do(ctx, td.Props("Build"), func(ctx context.Context) error {
		for target, image := range outputs {
			sha256, err := build(ctx, d, tmp, target, image)
			if err != nil {
				return err
			}
			results = append(results, fmt.Sprintf("%s@%s", image, sha256))
		}
		if *outputManifest != "" {
			if err := os_steps.WriteFile(ctx, *outputManifest, []byte(strings.Join(results, "\n")), os.ModePerm); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}
}

func build(ctx context.Context, d *docker.Docker, tmp, target, imageName string) (string, error) {
	name := "Build"
	if target != "" {
		name += " " + target
	}
	var rv string
	return rv, td.Do(ctx, td.Props(name), func(ctx context.Context) error {
		// Run "docker build".
		tmpImageTag := imageName + ":temp"
		args := []string{*buildDir, "--tag", tmpImageTag, "--build-arg", fmt.Sprintf("base=%s", *baseImage)}
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

		// Get the ChainID of the layers of the image. The ChainID is
		// based on only the filesystem layers and not metadata (eg.
		// build timestamp) like the image ID. We need a content-based
		// ID to support deduplication when the image contents don't
		// change.
		img, err := d.Inspect(ctx, tmpImageTag)
		if err != nil {
			return err
		}
		sklog.Infof("Got %s", spew.Sdump(img))
		diffIds := make([]layer.DiffID, 0, len(img.RootFS.Layers))
		for _, l := range img.RootFS.Layers {
			diffIds = append(diffIds, layer.DiffID(l))
		}
		cid := string((&image.RootFS{
			DiffIDs: diffIds,
		}).ChainID())
		if cid == "" {
			return fmt.Errorf("Failed to obtain ChainID from RootFS: %+v", img.RootFS)
		}
		tag := "content-" + strings.TrimPrefix(cid, "sha256:")
		imageTag := imageName + ":" + tag

		// Attempt to pull the image with this ChainID. If that fails,
		// then nobody has yet uploaded an image with this ChainID, so
		// we need to upload. Otherwise we don't upload, since we want
		// to avoid clobbering the existing one.
		// TODO(borenet): Is there a better way to determine whether a
		// particular image:tag exists?
		if err := d.Pull(ctx, imageTag); err == nil {
			// Already exists. Grab the sha256 sum of the existing
			// image and return that rather than the one we just
			// built.
			img, err := d.Inspect(ctx, imageTag)
			if err != nil {
				return err
			}
			rv = img.ID
			return nil
		}

		// Tag and push the image.
		if err := d.Tag(ctx, img.ID, imageTag); err != nil {
			return err
		}
		sha256, err := d.Push(ctx, imageTag)
		if err != nil {
			return err
		}
		rv = sha256
		return nil
	})
}
