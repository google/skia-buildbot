package main

/*
	build-images is used for building Skia Infrastructure Docker images using
	Bazel. It is intended to run inside of Louhi as part of a CD pipeline.
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"
	"golang.org/x/oauth2"

	"go.skia.org/infra/cd/go/cd"
	"go.skia.org/infra/go/docker"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/lib/git_steps"
	"go.skia.org/infra/task_driver/go/td"
)

func main() {
	const (
		flagCommit             = "commit"
		flagCommitSubject      = "commit-subject"
		flagEmail              = "email"
		flagLouhiExecutionID   = "louhi-execution-id"
		flagLouhiPubSubProject = "louhi-pubsub-project"
		flagRepo               = "repo"
		flagSourceRepo         = "source-repo"
		flagSourceCommit       = "source-commit"
		flagTarget             = "target"
		flagUser               = "user"
		flagWorkspace          = "workspace"
		flagExtraBazelArg      = "extra-bazel-arg"
	)
	app := &cli.App{
		Name:        "build-images",
		Description: `build-images is used for building Skia Infrastructure Docker images using Bazel. It is intended to run inside of Louhi as part of a CD pipeline.`,
		Commands: []*cli.Command{
			{
				Name:        "build",
				Description: "Build Docker images.",
				Usage:       "build-images <options>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     flagCommit,
						Usage:    "Commit at which to build the image(s).",
						Required: true,
					},
					&cli.StringFlag{
						Name:     flagRepo,
						Usage:    "URL of the repo to update.",
						Required: true,
					},
					&cli.StringFlag{
						Name:     flagWorkspace,
						Usage:    "Path to Louhi workspace.",
						Required: true,
					},
					&cli.StringSliceFlag{
						Name:     flagTarget,
						Usage:    "Bazel target + image path pairs in the form \"//bazel-package:bazel-target:gcr.io/image/path\".",
						Required: true,
					},
					&cli.MultiStringFlag{
						Target: &cli.StringSliceFlag{
							Name:  flagExtraBazelArg,
							Usage: "Extra argument(s) to pass to Bazel.",
						},
					},
					&cli.StringFlag{
						Name:     flagSourceCommit,
						Usage:    "Commit hash which triggered the build.",
						Required: false,
					},
					&cli.StringFlag{
						Name:     flagSourceRepo,
						Usage:    "URL of the repo which triggered the build.",
						Required: false,
					},
					&cli.StringFlag{
						Name:     flagUser,
						Usage:    "User name to attribute the build.",
						Required: true,
					},
					&cli.StringFlag{
						Name:     flagEmail,
						Usage:    "Email address to attribute the build.",
						Required: true,
					},
				},
				Action: func(cliCtx *cli.Context) error {
					ctx, _, err := initGitAuth(cliCtx.Context, cliCtx.String(flagEmail))
					if err != nil {
						return td.FailStep(ctx, err)
					}
					return build(ctx, cliCtx.String(flagCommit), cliCtx.String(flagRepo), cliCtx.String(flagWorkspace), cliCtx.String(flagUser), cliCtx.String(flagEmail), cliCtx.StringSlice(flagTarget), cliCtx.StringSlice(flagExtraBazelArg))
				},
			},
			{
				Name:        "update-references",
				Description: "Update references to the images we built.",
				Usage:       "update-references <options>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     flagRepo,
						Usage:    "Repository URL.",
						Required: true,
					},
					&cli.StringFlag{
						Name:     flagWorkspace,
						Usage:    "Path to Louhi workspace.",
						Required: true,
					},
					&cli.StringFlag{
						Name:     flagUser,
						Usage:    "User name to attribute the build.",
						Required: true,
					},
					&cli.StringFlag{
						Name:     flagEmail,
						Usage:    "Email address to attribute the build.",
						Required: true,
					},
					&cli.StringFlag{
						Name:     flagLouhiPubSubProject,
						Usage:    "GCP project used for sending Louhi pub/sub notifications.",
						Required: false,
					},
					&cli.StringFlag{
						Name:     flagLouhiExecutionID,
						Usage:    "Execution ID of the Louhi flow.",
						Required: false,
					},
					&cli.StringFlag{
						Name:     flagSourceCommit,
						Usage:    "Commit hash which triggered the build.",
						Required: false,
					},
					&cli.StringFlag{
						Name:     flagSourceRepo,
						Usage:    "URL of the repo which triggered the build.",
						Required: false,
					},
				},
				Action: func(cliCtx *cli.Context) error {
					dc, err := docker.NewClient(cliCtx.Context)
					if err != nil {
						return skerr.Wrap(err)
					}
					ctx, ts, err := initGitAuth(cliCtx.Context, cliCtx.String(flagEmail))
					if err != nil {
						return td.FailStep(ctx, err)
					}
					return updateRefs(ctx, ts, dc, cliCtx.String(flagRepo), cliCtx.String(flagWorkspace), cliCtx.String(flagEmail), cliCtx.String(flagLouhiPubSubProject), cliCtx.String(flagLouhiExecutionID), cliCtx.String(flagSourceRepo), cliCtx.String(flagSourceCommit))
				},
			},
			{
				Name:        "upload-cl",
				Description: "Upload a CL with any changes in the local checkout.",
				Usage:       "upload-cl <options>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     flagCommitSubject,
						Usage:    "Commit message subject line.",
						Required: true,
					},
					&cli.StringFlag{
						Name:     flagLouhiPubSubProject,
						Usage:    "GCP project used for sending Louhi pub/sub notifications.",
						Required: false,
					},
					&cli.StringFlag{
						Name:     flagLouhiExecutionID,
						Usage:    "Execution ID of the Louhi flow.",
						Required: false,
					},
					&cli.StringFlag{
						Name:     flagSourceCommit,
						Usage:    "Commit hash which triggered the build.",
						Required: false,
					},
					&cli.StringFlag{
						Name:     flagSourceRepo,
						Usage:    "URL of the repo which triggered the build.",
						Required: false,
					},
					&cli.StringFlag{
						Name:     flagEmail,
						Usage:    "Email address to attribute the build.",
						Required: true,
					},
				},
				Action: func(cliCtx *cli.Context) error {
					cwd, err := os.Getwd()
					if err != nil {
						return skerr.Wrap(err)
					}
					ctx, _, err := initGitAuth(cliCtx.Context, cliCtx.String(flagEmail))
					if err != nil {
						return td.FailStep(ctx, err)
					}
					return cd.MaybeUploadCL(ctx, cwd, cliCtx.String(flagCommitSubject), cliCtx.String(flagSourceRepo), cliCtx.String(flagSourceCommit), cliCtx.String(flagLouhiPubSubProject), cliCtx.String(flagLouhiExecutionID))
				},
			},
		},
		Usage: "build-images <subcommand>",
	}

	// We're using the task driver framework because it provides logging and
	// helpful insight into what's occurring as the program runs.
	fakeProjectId := ""
	fakeTaskId := ""
	fakeTaskName := ""
	output := "-"
	local := true
	ctx := td.StartRun(&fakeProjectId, &fakeTaskId, &fakeTaskName, &output, &local)
	defer td.EndRun(ctx)

	// Run the app.
	if err := app.RunContext(ctx, os.Args); err != nil {
		td.Fatal(ctx, err)
	}
}

const (
	// buildImagesJSONFile persists information about the images built by this
	// program between invocations. This is necessary because the push to the
	// image repository must be performed using the built-in Louhi stage in
	// order to create the attestation which can be used to verify the image.
	// After that is done, we invoke build-images again to obtain the sha256
	// sum for each image (which can only be done after the push) and update
	// the image references in Git.
	buildImagesJSONFile = "build-images.json"
)

// buildImagesJSON describes the structure of buildImagesJSONFile.
type buildImagesJSON struct {
	Images []*SingleImageInfo `json:"images"`
}

type SingleImageInfo struct {
	Image string `json:"image"`
	Tag   string `json:"tag"`
}

// readBuildImagesJSON reads the buildImagesJSONFile.
func readBuildImagesJSON(ctx context.Context, workspace string) (*buildImagesJSON, error) {
	ctx = td.StartStep(ctx, td.Props(fmt.Sprintf("Read %s", buildImagesJSONFile)))
	defer td.EndStep(ctx)

	f := filepath.Join(workspace, buildImagesJSONFile)
	var imageInfo buildImagesJSON
	if err := util.WithReadFile(f, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&imageInfo)
	}); err != nil {
		return nil, td.FailStep(ctx, err)
	}
	return &imageInfo, nil
}

// writeBuildImagesJSON writes the buildImagesJSONFile.
func writeBuildImagesJSON(ctx context.Context, workspace string, imageInfo *buildImagesJSON) error {
	ctx = td.StartStep(ctx, td.Props(fmt.Sprintf("Write %s", buildImagesJSONFile)))
	defer td.EndStep(ctx)

	f := filepath.Join(workspace, buildImagesJSONFile)
	if err := util.WithWriteFile(f, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(imageInfo)
	}); err != nil {
		return td.FailStep(ctx, err)
	}
	return nil
}

// initGitAuth initializes Git authentication.
func initGitAuth(ctx context.Context, email string) (context.Context, oauth2.TokenSource, error) {
	// Use a custom location for the global .gitconfig, since we may not be able
	// to write to $HOME.
	rvCtx := td.WithEnv(ctx, []string{"GIT_CONFIG_GLOBAL=/tmp/.gitconfig"})

	ctx = td.StartStep(rvCtx, td.Props("Init Git Auth"))
	defer td.EndStep(ctx)

	ts, err := git_steps.Init(ctx, true)
	if err != nil {
		return rvCtx, nil, td.FailStep(ctx, err)
	}
	if _, err := gitauth.New(ctx, ts, "/tmp/.gitcookies", true, email); err != nil {
		return rvCtx, nil, td.FailStep(ctx, err)
	}
	return rvCtx, ts, nil
}

// shallowClone creates a shallow clone of the given repo at the given commit.
// Returns the location of the checkout or any error which occurred.
func shallowClone(ctx context.Context, repoURL, commit string) (string, error) {
	ctx = td.StartStep(ctx, td.Props("Clone"))
	defer td.EndStep(ctx)

	checkoutDir, err := os.MkdirTemp("", "")
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
