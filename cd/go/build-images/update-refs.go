package main

import (
	"context"
	"fmt"
	"io/fs"
	"io/ioutil"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"go.skia.org/infra/cd/go/cd"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/task_driver/go/lib/git_steps"
	"go.skia.org/infra/task_driver/go/td"
)

func updateRefs(ctx context.Context, repo, workspace, username, email, louhiPubsubProject, executionID, srcRepo, srcCommit string) error {
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
		image.Sha256 = strings.TrimSuffix(strings.TrimPrefix(split[1], "sha256:"), "'")
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
	if err := td.Do(ctx, td.Props("Update Image References"), func(ctx context.Context) error {
		imageRegexes := make([]*regexp.Regexp, 0, len(imageInfo.Images))
		imageReplace := make([]string, 0, len(imageInfo.Images))
		for _, image := range imageInfo.Images {
			// Update instances of "image/path@sha256:digest"
			imageRegexes = append(imageRegexes, regexp.MustCompile(fmt.Sprintf(`%s@sha256:[a-f0-9]+`, image.Image)))
			imageReplace = append(imageReplace, fmt.Sprintf("%s@sha256:%s", image.Image, image.Sha256))

			// Replace Bazel container_pull specifications.
			bazelRegex, bazelReplace := bazelRegexAndReplaceForImage(image)
			imageRegexes = append(imageRegexes, bazelRegex)
			imageReplace = append(imageReplace, bazelReplace)
		}
		return filepath.WalkDir(checkoutDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			} else if d.IsDir() {
				if d.Name() == ".git" {
					return fs.SkipDir
				} else {
					return nil
				}
			}
			// Read the file.
			contents, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			contentsStr := string(contents)

			// Replace all instances of the old image specification with the new.
			for idx, re := range imageRegexes {
				contentsStr = re.ReplaceAllString(contentsStr, imageReplace[idx])
			}

			// Write out the updated file.
			contents = []byte(contentsStr)
			if err := ioutil.WriteFile(path, contents, d.Type().Perm()); err != nil {
				return err
			}
			return nil
		})
	}); err != nil {
		return td.FailStep(ctx, err)
	}

	// Upload a CL.
	imageList := make([]string, 0, len(imageInfo.Images))
	for _, image := range imageInfo.Images {
		imageList = append(imageList, path.Base(image.Image))
	}
	commitSubject := fmt.Sprintf("Update %s", strings.Join(imageList, ", "))
	return cd.MaybeUploadCL(ctx, checkoutDir, commitSubject, srcRepo, srcCommit, louhiPubsubProject, executionID)
}

func bazelRegexAndReplaceForImage(image *SingleImageInfo) (*regexp.Regexp, string) {
	const regexTmpl = `(container_pull\(\s*name\s*=\s*"%s",\s*digest\s*=\s*)"sha256:[a-f0-9]+",`
	regex := regexp.MustCompile(fmt.Sprintf(regexTmpl, path.Base(image.Image)))

	const replTmpl = `$1"%s",`
	replace := fmt.Sprintf(replTmpl, image.Sha256)
	return regex, replace
}
