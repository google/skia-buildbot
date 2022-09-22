package main

import (
	"context"
	"fmt"
	"strings"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit/rubberstamper"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/task_driver/go/lib/git_steps"
	"go.skia.org/infra/task_driver/go/td"
)

func updateRefs(ctx context.Context, repo, workspace, username, email string) error {
	ctx = td.StartStep(ctx, td.Props("Update References"))
	defer td.EndStep(ctx)

	// Initialize git authentication.
	ts, err := git_steps.Init(ctx, true)
	if err != nil {
		return td.FailStep(ctx, err)
	}
	if _, err := gitauth.New(ts, "/tmp/.gitcookies", true, email); err != nil {
		return td.FailStep(ctx, err)
	}

	imageInfo, err := readBuildImagesJSON(ctx, workspace)
	if err != nil {
		return td.FailStep(ctx, err)
	}

	// First, obtain the sha256 sums for the images.
	for _, image := range imageInfo.Images {
		imageAndTag := fmt.Sprintf("%s:%s", image.Image, image.Tag)
		if _, err := exec.RunCwd(ctx, ".", "docker", "pull", imageAndTag); err != nil {
			return td.FailStep(ctx, err)
		}
		output, err := exec.RunCwd(ctx, ".", "docker", "inspect", "--format='{{index .RepoDigests 0}}'", imageAndTag)
		if err != nil {
			return td.FailStep(ctx, err)
		}
		split := strings.Split(strings.TrimSpace(output), "@")
		if len(split) != 2 {
			return td.FailStep(ctx, skerr.Fmt("Failed to obtain sha256 sum for %s; expected <image>@<sha256> but got %q", image.Image, output))
		}
		image.Sha256 = split[1]
	}

	// Create a shallow clone of the repo.
	checkoutDir, err := shallowClone(ctx, repo, git.DefaultRef)
	if err != nil {
		return td.FailStep(ctx, err)
	}

	// Create a branch.
	gitExec, err := git.Executable(ctx)
	if err != nil {
		return td.FailStep(ctx, err)
	}
	if _, err := exec.RunCwd(ctx, checkoutDir, gitExec, "checkout", "-b", "update", "-t", git.DefaultRemoteBranch); err != nil {
		return td.FailStep(ctx, err)
	}

	// Find-and-replace each of the image references.
	for _, image := range imageInfo.Images {
		if _, err := exec.RunCwd(ctx, checkoutDir, "find", "./", "-type", "f", "-exec", "sed", "-r", "-i", fmt.Sprintf("s;%s@sha256:[a-f0-9]+;%s@sha256:%s;g", image.Image, image.Image, image.Sha256), "{}", "\\;"); err != nil {
			return td.FailStep(ctx, err)
		}
	}

	// Did we change anything?
	if _, err := exec.RunCwd(ctx, checkoutDir, gitExec, "diff", "--exit-code"); err != nil {
		// If so, create a CL.
		imageList := make([]string, 0, len(imageInfo.Images))
		for _, image := range imageInfo.Images {
			imageList = append(imageList, image.Image)
		}
		commitMsg := fmt.Sprintf(`Update %s

%s`, strings.Join(imageList, ", "), rubberstamper.RandomChangeID())
		if _, err := exec.RunCwd(ctx, checkoutDir, gitExec, "commit", "-a", "-m", commitMsg); err != nil {
			return td.FailStep(ctx, err)
		}
		if _, err := exec.RunCwd(ctx, checkoutDir, gitExec, "push", git.DefaultRemote, rubberstamper.PushRequestAutoSubmit); err != nil {
			return td.FailStep(ctx, err)
		}
	}

	return nil
}
