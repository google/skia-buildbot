package main

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"

	"go.skia.org/infra/cd/go/cd"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/task_driver/go/td"
	"golang.org/x/oauth2/google"
)

func updateRefs(ctx context.Context, repoURL, workspace, username, email, louhiPubsubProject, louhiExecutionID, srcRepo, srcCommit string) error {
	ctx = td.StartStep(ctx, td.Props("Update References"))
	defer td.EndStep(ctx)

	// Create the git repo.
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, gerrit.AuthScope)
	if err != nil {
		return td.FailStep(ctx, err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	repo := gitiles.NewRepo(repoURL, client)

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
		image.Sha256 = strings.TrimSuffix(strings.TrimPrefix(split[1], "sha256:"), "'")
	}

	// Obtain the current contents of all files in the repo.
	baseCommit, err := repo.ResolveRef(ctx, git.DefaultRef)
	if err != nil {
		return td.FailStep(ctx, err)
	}
	oldFiles, err := repo.ListFilesRecursiveAtRef(ctx, ".", baseCommit)
	if err != nil {
		return td.FailStep(ctx, err)
	}
	oldContents := map[string][]byte{}
	for _, f := range oldFiles {
		contents, err := repo.ReadFileAtRef(ctx, f, baseCommit)
		if err != nil {
			return td.FailStep(ctx, err)
		}
		oldContents[f] = contents
	}

	// Create regexes for each of the images.
	imageRegexes := make([]*regexp.Regexp, 0, len(imageInfo.Images))
	imageReplace := make([]string, 0, len(imageInfo.Images))
	for _, image := range imageInfo.Images {
		imageRegexes = append(imageRegexes, regexp.MustCompile(fmt.Sprintf(`%s@sha256:[a-f0-9]+`, image.Image)))
		imageReplace = append(imageReplace, fmt.Sprintf("%s@sha256:%s", image.Image, image.Sha256))
	}

	// Find-and-replace each of the image references.
	changes := map[string]string{}
	for f, oldFileContents := range oldContents {
		// Replace all instances of the old image specification with the new.
		contentsStr := string(oldFileContents)
		for idx, re := range imageRegexes {
			contentsStr = re.ReplaceAllString(contentsStr, imageReplace[idx])
		}

		// Write out the updated file.
		newFileContents := contentsStr
		if string(oldFileContents) != newFileContents {
			changes[f] = newFileContents
		}
	}

	// Upload a CL.
	if len(changes) > 0 {
		imageList := make([]string, 0, len(imageInfo.Images))
		for _, image := range imageInfo.Images {
			imageList = append(imageList, path.Base(image.Image))
		}
		commitSubject := fmt.Sprintf("Update %s", strings.Join(imageList, ", "))
		return cd.UploadCL(ctx, changes, repoURL, baseCommit, commitSubject, srcRepo, srcCommit, louhiPubsubProject, louhiExecutionID)
	}
	return nil
}
