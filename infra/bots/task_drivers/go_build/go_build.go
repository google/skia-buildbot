package main

import (
	"context"
	"flag"
	"fmt"
	"path"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"go.skia.org/infra/task_driver/go/lib/docker"
	"go.skia.org/infra/task_driver/go/td"
)

/*
	Build all Golang binaries and tests for the given platform(s).
*/

var (
	// Required properties for this task.
	projectID  = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskID     = flag.String("task_id", "", "ID of this task.")
	taskName   = flag.String("task_name", "", "Name of the task.")
	workdir    = flag.String("workdir", ".", "Working directory")
	image      = flag.String("image", "", "Docker image to use to compile.")
	outputPath = flag.String("output-path", "", "Write binaries to this path.")
	platforms  = common.NewMultiStringFlag("platform", nil, "Platform(s) to test, eg. \"linux-amd64\"")

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	ctx := td.StartRun(projectID, taskID, taskName, output, local)
	defer td.EndRun(ctx)

	if *image == "" {
		td.Fatalf(ctx, "--image is required")
	}
	if *platforms == nil {
		td.Fatalf(ctx, "--platform is required")
	}
	if *outputPath == "" {
		td.Fatalf(ctx, "--output-path is required")
	}

	ts, err := auth_steps.Init(ctx, *local, auth.SCOPE_USERINFO_EMAIL)
	if err != nil {
		td.Fatal(ctx, err)
	}
	d, err := docker.New(ctx, ts)
	if err != nil {
		td.Fatal(ctx, err)
	}
	defer func() {
		if err := d.Cleanup(ctx); err != nil {
			td.Fatal(ctx, err)
		}
	}()

	// Pull the Docker image and create the container.
	if err := d.Pull(ctx, *image); err != nil {
		td.Fatal(ctx, err)
	}

	// Set up the Docker container.
	dockerOutPath := "/out"
	props := &docker.ContainerProps{
		Image: *image,
		Mounts: []*docker.Mount{
			{
				Type:        docker.MountTypeBind,
				Source:      *workdir,
				Destination: "/repo",
				Readonly:    true,
			},
			{
				Type:        docker.MountTypeBind,
				Source:      *outputPath,
				Destination: dockerOutPath,
				Readonly:    false,
			},
		},
		Command: []string{"/bin/sh"},
	}
	if err := d.RunInContainer(ctx, props, func(ctx context.Context, c *docker.Container) error {
		g := util.NewNamedErrGroup()
		for _, platform := range *platforms {
			// Set up the environment for this platform.
			platform := platform
			split := strings.Split(platform, "-")
			if len(split) != 2 {
				return fmt.Errorf("Bad format for platform; expected \"os-arch\" but got: %s", platform)
			}
			goos := split[0]
			goarch := split[1]
			env := map[string]string{
				"GOOS":   goos,
				"GOARCH": goarch,
			}
			platformOutPath := path.Join(dockerOutPath, platform)
			g.Go(platform, func() error {
				// Build all non-test binaries for this platform.
				if _, err := c.Run(env, "go", "build", "-o", path.Join(platformOutPath, "bin"), "./..."); err != nil {
					return err
				}

				// Determine which tests to build for this platform.
				out, err := c.Run(env, "go", "list", "-f", "{{if .TestGoFiles}}{{.ImportPath}}{{end}}", "./...")
				if err != nil {
					return err
				}
				testPackages := strings.Split(strings.TrimSpace(out), "\n")

				// Build the test binaries in parallel.
				testOutPath := path.Join(platformOutPath, "test")
				for _, pkg := range testPackages {
					pkg := strings.TrimPrefix(pkg, "go.skia.org/infra/")
					// TODO(borenet): Don't hard-code the repo.
					localPath := "./" + pkg
					dest := path.Join(testOutPath, pkg+".test")
					g.Go(pkg, func() error {
						// TODO(borenet): We do want to run "go vet" at some
						// point, but I don't think we want to block compilation
						// on it.
						_, err := c.Run(env, "go", "test", "-vet=off", "-c", "-o", dest, localPath)
						return err
					})
				}
				return nil
			})
		}
		return g.Wait()
	}); err != nil {
		td.Fatal(ctx, err)
	}
}
