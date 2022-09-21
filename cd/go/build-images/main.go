package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os/user"
	"path"
	"strings"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/task_driver/go/td"
	"golang.org/x/sync/errgroup"
)

/*
	build-images is used for building Skia Infrastructure Docker images using
	Bazel. It is intended to run inside of Louhi as part of a CD pipeline.
*/

func main() {
	// Setup.
	commit := flag.String("commit", "", "Commit at which to build the image.")
	repo := flag.String("repo", "", "Repository URL.")
	workspace := flag.String("workspace", "", "Path to Louhi workspace.")
	targets := common.NewMultiStringFlag("target", nil, "Bazel target + image path pairs in the form \"//bazel-package:bazel-target:gcr.io/image/path\". ")
	rbe := flag.Bool("rbe", false, "Whether or not to use Bazel RBE.")
	username := flag.String("user", "", "User name to attribute the build. If not specified, attempt to determine automatically.")

	// We're using the task driver framework because it provides logging and
	// helpful insight into what's occurring as the program runs.
	fakeProjectId := ""
	fakeTaskId := ""
	fakeTaskName := ""
	output := "-"
	local := true
	ctx := td.StartRun(&fakeProjectId, &fakeTaskId, &fakeTaskName, &output, &local)
	defer td.EndRun(ctx)

	if *commit == "" {
		td.Fatalf(ctx, "--commit is required.")
	}
	if *repo == "" {
		td.Fatalf(ctx, "--repo is required.")
	}
	if *workspace == "" {
		td.Fatalf(ctx, "--workspace is required.")
	}
	if len(*targets) == 0 {
		td.Fatalf(ctx, "At least one --target is required.")
	}
	bazelTargetToImagePath := make(map[string]string, len(*targets))
	for _, target := range *targets {
		targetSplit := strings.Split(target, ":")
		if len(targetSplit) != 3 {
			td.Fatalf(ctx, "Invalid target specification %q; expected \"//bazel-target:bazel-target:gcr.io/image/path\"", target)
		}
		bazelTarget := strings.Join(targetSplit[:2], ":")
		imagePath := targetSplit[2]
		bazelTargetToImagePath[bazelTarget] = imagePath
	}

	// Create a shallow clone of the repo.
	checkoutDir, err := shallowClone(ctx, *repo, *commit)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Create the timestamped Docker image tag.
	// 2022-09-21T13_13_46Z-louhi-02b6ac9-clean
	ts := time.Now().UTC().Format("2006-01-02T15_04_05Z")
	if *username == "" {
		userObj, err := user.Current() // TODO(borenet): Will this work in Louhi?
		if err != nil {
			td.Fatal(ctx, err)
		}
		*username = userObj.Username
	}
	imageTag := fmt.Sprintf("%s-%s-%s-%s", ts, *username, (*commit)[:7], "clean")

	// Perform the builds concurrently.
	eg, ctx := errgroup.WithContext(ctx)
	for bazelTarget, imagePath := range bazelTargetToImagePath {
		// https://golang.org/doc/faq#closures_and_goroutines
		bazelTarget := bazelTarget
		louhiImageTag := fmt.Sprintf("louhi_ws/%s:%s", imagePath, imageTag)
		eg.Go(func() error {
			return bazelRun(ctx, checkoutDir, bazelTarget, louhiImageTag, *rbe)
		})
	}
	if err := eg.Wait(); err != nil {
		td.Fatal(ctx, err)
	}
}

// shallowClone creates a shallow clone of the given repo at the given commit.
// Returns the location of the checkout or any error which occurred.
func shallowClone(ctx context.Context, repoURL, commit string) (string, error) {
	ctx = td.StartStep(ctx, td.Props("Clone"))
	defer td.EndStep(ctx)

	checkoutDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", td.FailStep(ctx, err)
	}
	git, err := git.Executable(ctx)
	if err != nil {
		return "", td.FailStep(ctx, err)
	}
	if _, err := exec.RunCwd(ctx, checkoutDir, git, "init"); err != nil {
		return "", td.FailStep(ctx, err)
	}
	if _, err := exec.RunCwd(ctx, checkoutDir, git, "remote", "add", "origin", repoURL); err != nil {
		return "", td.FailStep(ctx, err)
	}
	if _, err := exec.RunCwd(ctx, checkoutDir, git, "fetch", "--depth=1", "origin", commit); err != nil {
		return "", td.FailStep(ctx, err)
	}
	if _, err := exec.RunCwd(ctx, checkoutDir, git, "checkout", "FETCH_HEAD"); err != nil {
		return "", td.FailStep(ctx, err)
	}
	return checkoutDir, nil
}

// bazelTargetToDockerTag converts a Bazel target specification to a Docker
// image tag which is applied to the image during the Bazel build.
func bazelTargetToDockerTag(target string) string {
	return path.Join("bazel", target)
}

// bazelRun executes `bazel run` for the given target and applies the given tag
// to the resulting image.
func bazelRun(ctx context.Context, cwd, target, louhiImageTag string, rbe bool) error {
	ctx = td.StartStep(ctx, td.Props(fmt.Sprintf("Build %s", target)))
	defer td.EndStep(ctx)

	cmd := []string{"bazelisk", "run"}
	if rbe {
		cmd = append(cmd, "--config=remote", "--google_default_credentials")
	}
	cmd = append(cmd, target)
	if _, err := exec.RunCwd(ctx, cwd, cmd...); err != nil {
		return td.FailStep(ctx, err)
	}
	if _, err := exec.RunCwd(ctx, cwd, "docker", "tag", bazelTargetToDockerTag(target), louhiImageTag); err != nil {
		return td.FailStep(ctx, err)
	}
	return nil
}
