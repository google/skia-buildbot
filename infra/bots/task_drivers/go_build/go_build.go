package main

import (
	"context"
	"flag"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/test2json"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"go.skia.org/infra/task_driver/go/lib/docker"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
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

var (
	// copyTestPathRegex matches the output of //go_deps/copy_test.sh. It needs
	// To be kept in sync.
	copyTestPathRegex = regexp.MustCompile(`Wrote test executable (.+)`)
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
	workdirAbs, err := filepath.Abs(*workdir)
	if err != nil {
		td.Fatal(ctx, err)
	}
	hostOutPath, err := filepath.Abs(*outputPath)
	if err != nil {
		td.Fatal(ctx, err)
	}

	ts, err := auth_steps.Init(ctx, *local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_FULL_CONTROL)
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
	dockerRepoPath := "/repo"
	dockerOutPath := "/out"
	props := &docker.ContainerProps{
		Image: *image,
		Mounts: []*docker.Mount{
			{
				Type:        docker.MountTypeBind,
				Source:      workdirAbs,
				Destination: dockerRepoPath,
				Readonly:    true,
			},
			{
				Type:        docker.MountTypeBind,
				Source:      hostOutPath,
				Destination: dockerOutPath,
				Readonly:    false,
			},
		},
		Command:         []string{"/bin/sh"},
		DisallowNetwork: true,
	}
	platformErrGroup := util.NewNamedErrGroup()
	for _, platform := range *platforms {
		// Set up the environment for this platform.
		platform := platform
		split := strings.Split(platform, "-")
		if len(split) != 2 {
			td.Fatalf(ctx, "Bad format for platform; expected \"os-arch\" but got: %s", platform)
		}
		goos := split[0]
		goarch := split[1]
		env := map[string]string{
			"GOOS":   goos,
			"GOARCH": goarch,
		}
		hostPlatformOutPath := filepath.Join(hostOutPath, platform)
		dockerPlatformOutPath := path.Join(dockerOutPath, platform)
		platformErrGroup.Go(platform, func() error {
			return td.Do(ctx, td.Props(fmt.Sprintf("Build %s", platform)), func(ctx context.Context) error {
				hostBinOutPath := filepath.Join(hostPlatformOutPath, "bin")
				if err := os_steps.MkdirAll(ctx, hostBinOutPath); err != nil {
					return err
				}
				dockerBinOutPath := path.Join(dockerPlatformOutPath, "bin")
				hostTestTmpPath := filepath.Join(hostPlatformOutPath, "test-tmp")
				if err := os_steps.MkdirAll(ctx, hostTestTmpPath); err != nil {
					return err
				}
				dockerTestTmpPath := path.Join(dockerPlatformOutPath, "test-tmp")
				hostTestPath := filepath.Join(hostPlatformOutPath, "test")
				if err := os_steps.MkdirAll(ctx, hostTestPath); err != nil {
					return err
				}
				return d.RunInContainer(ctx, props, func(ctx context.Context, c *docker.Container) error {
					// Build all non-test binaries for this platform.
					if _, err := c.Run(ctx, dockerRepoPath, env, "go", "build", "-v", "--trimpath", "-o", dockerBinOutPath, "./..."); err != nil {
						return err
					}

					// Build the tests. Unfortunately, there is no command which
					// builds all of the tests at once and saves the binaries
					// for future use[1], so we use the -exec flag to "go test",
					// using a script which copies the executable to the output
					// directory. Note that the test executables passed to this
					// script only contain the base name of the package, eg.
					// "types" => "types.test". To prevent conflicts, the script
					// appends a random number to the file name, and we use the
					// JSON output from "go test" to match the resulting files
					// name back to the full package import paths and then
					// rename them accordingly.
					//
					// As an alternative, I tried using "go list" to find the
					// packages with test files and then running "go test -c"
					// for each of those. Doing so sequentially took about 10
					// minutes; running in parallel resulted in timeouts after a
					// much longer time, presumably due to contention of some
					// kind.
					//
					// [1] https://github.com/golang/go/issues/15513
					dockerCopyTestPath := path.Join(dockerRepoPath, "go_deps", "copy_test.sh")
					execCmd := fmt.Sprintf("%s %s", dockerCopyTestPath, dockerTestTmpPath)
					// Note: We're skipping vet here because we want to compile
					// all of the tests and run them regardless of whether vet
					// passes. We'll need to explicitly run it somewhere else.
					output, err := c.Run(ctx, dockerRepoPath, env, "go", "test", "-vet=off", "-json", "--trimpath", "-exec", execCmd, "./...")
					if err != nil {
						return err
					}
					return td.Do(ctx, td.Props("Copy test binaries"), func(ctx context.Context) error {
						lines := strings.Split(strings.TrimSpace(output), "\n")
						for _, line := range lines {
							ev, err := test2json.ParseEvent(line)
							if err != nil {
								return err
							}
							m := copyTestPathRegex.FindStringSubmatch(ev.Output)
							if len(m) == 2 {
								// Get the host-side path to the test binary.
								testSrc := strings.ReplaceAll(m[1], dockerTestTmpPath, hostTestTmpPath)
								testDst := filepath.Join(hostTestPath, ev.Package+".test")
								if err := os_steps.MkdirAll(ctx, filepath.Dir(testDst)); err != nil {
									return err
								}
								if err := os_steps.Rename(ctx, testSrc, testDst); err != nil {
									return err
								}
							}
						}
						return os_steps.RemoveAll(ctx, hostTestTmpPath)
					})
				})
			})
		})
	}
	if err := platformErrGroup.Wait(); err != nil {
		td.Fatal(ctx, err)
	}
}
