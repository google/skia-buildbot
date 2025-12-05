package main

import (
	"context"
	"fmt"
	"strings"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/task_driver/go/lib/git_steps"
	"go.skia.org/infra/task_driver/go/td"
)

type containerToBuild struct {
	bazelTarget string
	imageURI    string
}

func build(ctx context.Context, commit, repo, workspace, username, email string, targets []string, extraArgs []string) error {
	ctx = td.StartStep(ctx, td.Props("Build Images"))
	defer td.EndStep(ctx)

	// Initialize git authentication.
	ts, err := git_steps.Init(ctx, true)
	if err != nil {
		return td.FailStep(ctx, err)
	}
	if _, err := gitauth.New(ctx, ts, "/tmp/.gitcookies", true, email); err != nil {
		return td.FailStep(ctx, err)
	}

	toBuild := make([]containerToBuild, 0, len(targets))
nextTarget:
	for _, target := range targets {
		targetSplit := strings.Split(target, ":")
		if len(targetSplit) != 3 {
			return td.FailStep(ctx, skerr.Fmt("Invalid target specification %q; expected \"//bazel-target:bazel-target:gcr.io/image/path\"", target))
		}
		bazelTarget := strings.Join(targetSplit[:2], ":")
		imagePath := targetSplit[2]

		c := containerToBuild{bazelTarget: bazelTarget, imageURI: imagePath}
		// Make sure we don't build the same target twice
		for _, x := range toBuild {
			if c == x {
				continue nextTarget
			}
		}
		toBuild = append(toBuild, c)
	}

	// Create a shallow clone of the repo.
	checkoutDir, err := shallowClone(ctx, repo, commit)
	if err != nil {
		return td.FailStep(ctx, err)
	}

	// Create the timestamped Docker image tag.
	timestamp := now.Now(ctx).UTC().Format("2006-01-02T15_04_05Z")
	imageTag := fmt.Sprintf("%s-%s-%s-%s", timestamp, username, commit[:7], "clean")

	// Perform the builds.
	imageInfo := &buildImagesJSON{
		Images: make([]*SingleImageInfo, 0, len(toBuild)),
	}
	for _, c := range toBuild {
		bazelTarget, imagePath := c.bazelTarget, c.imageURI
		tags := []string{
			fmt.Sprintf("louhi_ws/%s:%s", imagePath, imageTag),
			fmt.Sprintf("louhi_ws/%s:git-%s", imagePath, commit),
			fmt.Sprintf("louhi_ws/%s:latest", imagePath),
		}
		imageInfo.Images = append(imageInfo.Images, &SingleImageInfo{
			Image: imagePath,
			Tag:   imageTag,
		})
		if err := bazelRun(ctx, checkoutDir, bazelTarget, imagePath, tags, extraArgs); err != nil {
			return td.FailStep(ctx, err)
		}
	}
	return writeBuildImagesJSON(ctx, workspace, imageInfo)
}

// bazelRun executes `bazel run` for the given target and applies the given tag
// to the resulting image.
func bazelRun(ctx context.Context, cwd, target, imageURI string, tags []string, extraArgs []string) error {
	ctx = td.StartStep(ctx, td.Props(fmt.Sprintf("Build %s", target)))
	defer td.EndStep(ctx)

	cmd := []string{"bazelisk", "run"}
	if extraArgs != nil {
		cmd = append(cmd, extraArgs...)
	}
	cmd = append(cmd, target)
	if _, err := exec.RunCwd(ctx, cwd, cmd...); err != nil {
		return td.FailStep(ctx, err)
	}
	srcTag := fmt.Sprintf("%s:latest", imageURI)
	for _, tag := range tags {
		if _, err := exec.RunCwd(ctx, cwd, "docker", "tag", srcTag, tag); err != nil {
			return td.FailStep(ctx, err)
		}
	}
	return nil
}
