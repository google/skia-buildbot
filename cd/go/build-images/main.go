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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/td"
)

func main() {
	const (
		flagCommit             = "commit"
		flagEmail              = "email"
		flagLouhiPubSubProject = "louhi-pubsub-project"
		flagRBE                = "rbe"
		flagRepo               = "repo"
		flagTarget             = "target"
		flagUser               = "user"
		flagWorkspace          = "workspace"
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
						Usage:    "Repository URL.",
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
					&cli.BoolFlag{
						Name:  flagRBE,
						Usage: "Whether or not to use Bazel RBE",
						Value: false,
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
				Action: func(ctx *cli.Context) error {
					return build(ctx.Context, ctx.String(flagCommit), ctx.String(flagRepo), ctx.String(flagWorkspace), ctx.String(flagUser), ctx.String(flagEmail), ctx.StringSlice(flagTarget), ctx.Bool(flagRBE))
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
						Required: true,
					},
				},
				Action: func(ctx *cli.Context) error {
					return updateRefs(ctx.Context, ctx.String(flagRepo), ctx.String(flagWorkspace), ctx.String(flagUser), ctx.String(flagEmail), ctx.String(flagLouhiPubSubProject))
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
	Image  string `json:"image"`
	Tag    string `json:"tag"`
	Sha256 string `json:"sha256"`
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
