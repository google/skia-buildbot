package main

import (
	"context"
	"flag"
	"fmt"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
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
	projectID       = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskID          = flag.String("task_id", "", "ID of this task.")
	taskName        = flag.String("task_name", "", "Name of the task.")
	cachePath       = flag.String("cache-path", "", "Path to the pre-built cache.")
	repoPath        = flag.String("repo-path", ".", "Path to the repo containing the main module.")
	image           = flag.String("image", "", "Docker image to use to compile.")
	outputPath      = flag.String("output-path", "", "Write binaries to this path.")
	platforms       = common.NewMultiStringFlag("platform", nil, "Platform(s) to test, eg. \"linux-amd64\"")
	outputWhitelist = common.NewMultiStringFlag("output-whitelist", nil, "Include these binaries in output-path.")

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
	if *cachePath == "" {
		td.Fatalf(ctx, "--cache-path is required")
	}
	hostRepoPath, err := filepath.Abs(*repoPath)
	if err != nil {
		td.Fatal(ctx, err)
	}
	hostCachePath, err := filepath.Abs(*cachePath)
	if err != nil {
		td.Fatal(ctx, err)
	}
	hostOutPath, err := filepath.Abs(*outputPath)
	if err != nil {
		td.Fatal(ctx, err)
	}
	userInfo, err := user.Current()
	if err != nil {
		td.Fatalf(ctx, "Failed to retrieve current user: %s", err)
	}
	dockerUser := fmt.Sprintf("%s:%s", userInfo.Uid, userInfo.Gid)

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

	// For reasons I don't understand, this script only has the executable bit
	// set for the current user when we receive it via isolate. Make it
	// universally executable before trying to run it inside the Docker
	// container. An alternative is to create a user in the container with the
	// same user and group IDs as the host, but I found that to be fiddly due to
	// the need to run the commands as root.
	hostCopyTestPath := filepath.Join(hostRepoPath, "go_deps", "copy_test.sh")
	if err := os_steps.Chmod(ctx, hostCopyTestPath, 0755); err != nil {
		td.Fatal(ctx, err)
	}

	// Pull the Docker image and create the container.
	if err := d.Pull(ctx, *image); err != nil {
		td.Fatal(ctx, err)
	}

	// Set up the Docker container.
	dockerRepoPath := "/repo"
	dockerOutPath := "/out"
	dockerCachePath := "/gopath"
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
			"CGO_ENABLED": "0",
			"GOOS":        goos,
			"GOARCH":      goarch,
			"GOPATH":      dockerCachePath,
			"GOCACHE":     path.Join(dockerCachePath, "cache"),
		}
		props := &docker.ContainerProps{
			ContainerRunProps: docker.ContainerRunProps{
				Env:     env,
				User:    dockerUser,
				Workdir: dockerRepoPath,
			},
			Image: *image,
			Mounts: []*docker.Mount{
				{
					Type:        docker.MountTypeBind,
					Source:      hostRepoPath,
					Destination: dockerRepoPath,
					Readonly:    true,
				},
				{
					Type:        docker.MountTypeBind,
					Source:      hostOutPath,
					Destination: dockerOutPath,
					Readonly:    false,
				},
				{
					Type:        docker.MountTypeBind,
					Source:      hostCachePath,
					Destination: dockerCachePath,
					Readonly:    false,
				},
			},
		}
		hostPlatformOutPath := filepath.Join(hostOutPath, platform)
		dockerPlatformOutPath := path.Join(dockerOutPath, platform)
		platformErrGroup.Go(platform, func() error {
			return td.Do(ctx, td.Props(fmt.Sprintf("Build %s", platform)), func(ctx context.Context) error {
				hostBinTmpPath := filepath.Join(hostPlatformOutPath, "bin-tmp")
				if err := os_steps.MkdirAll(ctx, hostBinTmpPath); err != nil {
					return err
				}
				dockerBinTmpPath := path.Join(dockerPlatformOutPath, "bin-tmp")
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
					/*if _, err := c.Run(ctx, "addgroup", "-S", "-g", userInfo.Gid, userInfo.Username); err != nil {
						return err
					}
					if _, err := c.Run(ctx, "adduser", "-S", "-G", userInfo.Username, "--uid", userInfo.Uid, userInfo.Username); err != nil {
						return err
					}*/
					if _, err := exec.RunCwd(ctx, ".", "ls", "-alh", filepath.Join(hostRepoPath, "go_deps")); err != nil {
						return err
					}
					if _, err := c.Run(ctx, "ls", "-alh", path.Join(dockerRepoPath, "go_deps")); err != nil {
						return err
					}
					// Build all non-test binaries for this platform.
					if _, err := c.Run(ctx, "go", "build", "-v", "--trimpath", "-o", dockerBinTmpPath, "./..."); err != nil {
						return err
					}

					// Copy the whitelisted regular binaries.
					if err := td.Do(ctx, td.Props("Copy binaries"), func(ctx context.Context) error {
						hostBinOutPath := filepath.Join(hostPlatformOutPath, "bin")
						if *outputWhitelist == nil {
							if err := os_steps.Rename(ctx, hostBinTmpPath, hostBinOutPath); err != nil {
								return err
							}
						} else {
							if err := os_steps.MkdirAll(ctx, hostBinOutPath); err != nil {
								return err
							}
							for _, wl := range *outputWhitelist {
								src := filepath.Join(hostBinTmpPath, wl)
								dst := filepath.Join(hostBinOutPath, wl)
								if err := os_steps.Rename(ctx, src, dst); err != nil {
									return err
								}
							}
						}
						return os_steps.RemoveAll(ctx, hostBinTmpPath)
					}); err != nil {
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
					output, err := c.Run(ctx, "go", "test", "-vet=off", "-json", "--trimpath", "-exec", execCmd, "./...")
					if err != nil {
						return err
					}
					return td.Do(ctx, td.Props("Copy test binaries"), func(ctx context.Context) error {
						lines := strings.Split(strings.TrimSpace(output), "\n")
						for _, line := range lines {
							ev, err := test2json.ParseEvent(line)
							if err != nil {
								sklog.Errorf("Failed to parse event:")
								sklog.Error(err)
								sklog.Error("from:")
								sklog.Error(line)
								continue
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
