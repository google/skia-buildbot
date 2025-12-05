package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"go.skia.org/infra/cd/go/cd"
	"go.skia.org/infra/cd/go/stages"
	"go.skia.org/infra/go/docker"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vfs"
	"go.skia.org/infra/task_driver/go/lib/git_steps"
	"go.skia.org/infra/task_driver/go/td"
)

func updateRefs(ctx context.Context, dockerClient docker.Client, repo, workspace, email, louhiPubsubProject, executionID, srcRepo, srcCommit string) error {
	ctx = td.StartStep(ctx, td.Props("Update References"))
	defer td.EndStep(ctx)

	// Initialize git authentication.
	ts, err := git_steps.Init(ctx, true)
	if err != nil {
		return td.FailStep(ctx, err)
	}
	if _, err := gitauth.New(ctx, ts, "/tmp/.gitcookies", true, email); err != nil {
		return td.FailStep(ctx, err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

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

	// Read the stage file from the repo.
	stageFile, err := stages.DecodeFile(filepath.Join(checkoutDir, stages.StageFilePath))
	if err != nil {
		if os.IsNotExist(skerr.Unwrap(err)) {
			stageFile = &stages.StageFile{
				Images: map[string]*stages.Image{},
			}
		} else {
			return td.FailStep(ctx, err)
		}
	}

	// Read the information about the images we built.
	imageInfo, err := readBuildImagesJSON(ctx, workspace)
	if err != nil {
		return td.FailStep(ctx, err)
	}

	// Find-and-replace each of the image references.
	if err := td.Do(ctx, td.Props("Update Image References"), func(ctx context.Context) error {
		imageRegexes := make([]*regexp.Regexp, 0, len(imageInfo.Images))
		imageReplace := make([]string, 0, len(imageInfo.Images))
		for _, image := range imageInfo.Images {
			registry, repository, _, err := docker.SplitImage(image.Image)
			if err != nil {
				return err
			}
			if _, ok := stageFile.Images[image.Image]; ok {
				// Use the stagemanager to update the image references.
				sm := stages.NewStageManager(ctx, vfs.Local(checkoutDir), dockerClient, stages.GitilesCommitResolver(httpClient))
				if err := sm.SetStage(ctx, image.Image, "latest", image.Tag); err != nil {
					return err
				}
			} else {
				// Retrieve the digest and perform a simple find-and-replace.
				manifest, err := dockerClient.GetManifest(ctx, registry, repository, image.Tag)
				if err != nil {
					return err
				}

				newReg, newRepl := findRegexesAndReplaces(image, manifest.Digest)
				imageRegexes = append(imageRegexes, newReg...)
				imageReplace = append(imageReplace, newRepl...)
			}
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
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			existingStats, err := os.Stat(path)
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
			if err := os.WriteFile(path, contents, existingStats.Mode()); err != nil {
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

func findRegexesAndReplaces(image *SingleImageInfo, digest string) ([]*regexp.Regexp, []string) {
	// Update instances of "image/path@sha256:digest"
	ymlRegex := regexp.MustCompile(fmt.Sprintf(`%s@sha256:[a-f0-9]+`, image.Image))
	ymlReplace := fmt.Sprintf("%s@%s", image.Image, digest)

	ymlTagRegex := regexp.MustCompile(fmt.Sprintf(`%s@tag:[a-zA-Z0-9_\-.]+`, image.Image))
	ymlTagReplace := fmt.Sprintf("%s@tag:%s", image.Image, image.Tag)

	// Replace Bazel container_pull specifications.
	cpRegex, cpReplace := bazelRegexAndReplaceForContainerPull(image, digest)
	ociRegex, ociReplace := bazelRegexAndReplaceForOCIPull(image, digest)
	return []*regexp.Regexp{ymlRegex, ymlTagRegex, cpRegex, ociRegex}, []string{ymlReplace, ymlTagReplace, cpReplace, ociReplace}
}

func bazelRegexAndReplaceForContainerPull(image *SingleImageInfo, digest string) (*regexp.Regexp, string) {
	const regexTmpl = `(container_pull\(\s*name\s*=\s*"%s",\s*digest\s*=\s*)"sha256:[a-f0-9]+",`
	regex := regexp.MustCompile(fmt.Sprintf(regexTmpl, path.Base(image.Image)))

	const replTmpl = `$1"%s",`
	replace := fmt.Sprintf(replTmpl, digest)
	return regex, replace
}

func bazelRegexAndReplaceForOCIPull(image *SingleImageInfo, digest string) (*regexp.Regexp, string) {
	const regexTmpl = `(oci\.pull\(\s*name\s*=\s*"%s",\s*digest\s*=\s*)"sha256:[a-f0-9]+",`
	regex := regexp.MustCompile(fmt.Sprintf(regexTmpl, path.Base(image.Image)))

	const replTmpl = `$1"%s",`
	replace := fmt.Sprintf(replTmpl, digest)
	return regex, replace
}
