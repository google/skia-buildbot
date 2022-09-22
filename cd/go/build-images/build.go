package main

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/task_driver/go/lib/git_steps"
	"go.skia.org/infra/task_driver/go/td"
	"golang.org/x/sync/errgroup"
)

func build(ctx context.Context, commit, repo, workspace, username, email string, targets []string, rbe bool) error {
	ctx = td.StartStep(ctx, td.Props("Build Images"))
	defer td.EndStep(ctx)

	// Initialize git authentication.
	ts, err := git_steps.Init(ctx, true)
	if err != nil {
		return td.FailStep(ctx, err)
	}
	if _, err := gitauth.New(ts, "/tmp/.gitcookies", true, email); err != nil {
		return td.FailStep(ctx, err)
	}

	bazelTargetToImagePath := make(map[string]string, len(targets))
	for _, target := range targets {
		targetSplit := strings.Split(target, ":")
		if len(targetSplit) != 3 {
			return td.FailStep(ctx, skerr.Fmt("Invalid target specification %q; expected \"//bazel-target:bazel-target:gcr.io/image/path\"", target))
		}
		bazelTarget := strings.Join(targetSplit[:2], ":")
		imagePath := targetSplit[2]
		bazelTargetToImagePath[bazelTarget] = imagePath
	}

	// Create a shallow clone of the repo.
	checkoutDir, err := shallowClone(ctx, repo, commit)
	if err != nil {
		return td.FailStep(ctx, err)
	}

	// Create the timestamped Docker image tag.
	timestamp := time.Now().UTC().Format("2006-01-02T15_04_05Z")
	imageTag := fmt.Sprintf("%s-%s-%s-%s", timestamp, username, commit[:7], "clean")

	// Perform the builds concurrently.
	imageInfo := &buildImagesJSON{
		Images: make([]SingleImageInfo, 0, len(bazelTargetToImagePath)),
	}
	eg, ctx := errgroup.WithContext(ctx)
	for bazelTarget, imagePath := range bazelTargetToImagePath {
		// https://golang.org/doc/faq#closures_and_goroutines
		bazelTarget := bazelTarget
		louhiImageTag := fmt.Sprintf("louhi_ws/%s:%s", imagePath, imageTag)
		imageInfo.Images = append(imageInfo.Images, SingleImageInfo{
			Image: imagePath,
			Tag:   imageTag,
		})
		eg.Go(func() error {
			return bazelRun(ctx, checkoutDir, bazelTarget, louhiImageTag, rbe)
		})
	}
	if err := eg.Wait(); err != nil {
		return td.FailStep(ctx, err)
	}
	return writeBuildImagesJSON(ctx, workspace, imageInfo)
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
