package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/urfave/cli/v2"
	"go.skia.org/infra/cd/go/cd"
	"go.skia.org/infra/cd/go/stages"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/docker"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/term"
	"go.skia.org/infra/go/vfs"
	"golang.org/x/oauth2/google"
)

func main() {
	var sm *stages.StageManager
	var dockerClient docker.Client
	var httpClient *http.Client

	imageListLimitFlag := &cli.IntFlag{
		Name:    "limit",
		Aliases: []string{"n"},
		Usage:   "Number of versions to display.",
		Value:   100,
	}
	app := &cli.App{
		Name:        "stagemanager",
		Description: `stagemanager provides tools for managing release stages.`,
		Before: func(ctx *cli.Context) error {
			var err error
			dockerClient, err = docker.NewClient(ctx.Context)
			if err != nil {
				return skerr.Wrap(err)
			}
			fs, err := vfs.InRepoRoot()
			if err != nil {
				return skerr.Wrap(err)
			}
			ts, err := google.DefaultTokenSource(ctx.Context, auth.ScopeGerrit, auth.ScopeUserinfoEmail)
			if err != nil {
				return skerr.Wrap(err)
			}
			httpClient = httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
			sm = stages.NewStageManager(ctx.Context, fs, dockerClient, stages.GitilesCommitResolver(httpClient))
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:        "promote",
				Description: "Update a stage to equal another.",
				Usage:       "promote <image path> <stage to match> <stage to update>",
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 3 {
						return skerr.Fmt("Expected exactly three positional arguments, but got %d.", len(args))
					}
					return sm.PromoteStage(ctx.Context, args[0], args[1], args[2])
				},
			},
			{
				Name:        "apply",
				Description: "Update all config files to conform to the stage file.",
				Usage:       "apply",
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 0 {
						return skerr.Fmt("Expected no positional arguments.")
					}
					return sm.Apply(ctx.Context)
				},
			},
			{
				Name:        "images",
				Description: "Manage images.",
				Usage:       "images <subcommand>",
				Subcommands: []*cli.Command{
					{
						Name:        "add",
						Description: "Add a new image.",
						Usage:       "add <image path> [non-default git repo]",
						Action: func(ctx *cli.Context) error {
							args := ctx.Args().Slice()
							if len(args) != 1 && len(args) != 2 {
								return skerr.Fmt("Expected one or two positional arguments, but got %d", len(args))
							}
							gitRepo := ""
							if len(args) == 2 {
								gitRepo = args[1]
							}
							return sm.AddImage(ctx.Context, args[0], gitRepo)
						},
					},
					{
						Name:        "rm",
						Description: "Remove an image.",
						Usage:       "rm <image path>",
						Action: func(ctx *cli.Context) error {
							args := ctx.Args().Slice()
							if len(args) != 1 {
								return skerr.Fmt("Expected exactly one positional argument, but got %d", len(args))
							}
							return sm.RemoveImage(ctx.Context, args[0])
						},
					},
					{
						Name:        "list",
						Description: "List available versions of an image.",
						Usage:       "list <image path>",
						Flags: []cli.Flag{
							imageListLimitFlag,
						},
						Action: func(ctx *cli.Context) error {
							args := ctx.Args().Slice()
							if len(args) != 1 {
								return skerr.Fmt("Expected exactly one positional argument, but got %d", len(args))
							}
							image := args[0]
							table, err := imagesList(ctx.Context, sm, httpClient, dockerClient, image, ctx.Int(imageListLimitFlag.Name))
							if err != nil {
								return skerr.Wrap(err)
							}
							fmt.Println(table)
							return nil
						},
					},
				},
			},
			{
				Name:        "stages",
				Description: "Manage stages.",
				Usage:       "stages <subcommand>",
				Subcommands: []*cli.Command{
					{
						Name:        "set",
						Description: "Add or update a stage.",
						Usage:       "set <image path> <stage name> <git revision | digest>",
						Action: func(ctx *cli.Context) error {
							args := ctx.Args().Slice()
							if len(args) != 3 {
								return skerr.Fmt("Expected exactly three positional arguments, but got %d", len(args))
							}
							return sm.SetStage(ctx.Context, args[0], args[1], args[2])
						},
					},
					{
						Name:        "rm",
						Description: "Remove a stage.",
						Usage:       "rm <image path> <stage name>",
						Action: func(ctx *cli.Context) error {
							args := ctx.Args().Slice()
							if len(args) != 2 {
								return skerr.Fmt("Expected exactly two positional arguments, but got %d", len(args))
							}
							return sm.RemoveStage(ctx.Context, args[0], args[1])
						},
					},
				},
			},
			{
				Name:  "markdown",
				Usage: "Generates markdown help for stagemanager.",
				Action: func(c *cli.Context) error {
					body, err := c.App.ToMarkdown()
					if err != nil {
						return skerr.Wrap(err)
					}
					fmt.Println(body)
					return nil
				},
			},
		},
		Usage: "stagemanager <subcommand>",
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func imagesList(ctx context.Context, sm *stages.StageManager, httpClient *http.Client, dockerClient docker.Client, image string, limit int) (string, error) {
	sf, err := sm.ReadStageFile(ctx)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	repoURL, err := sf.GitRepoForImage(image)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	repo := gitiles.NewRepo(repoURL, httpClient)
	stagesByDigest := map[string][]string{}
	for stageName, stage := range sf.Images[image].Stages {
		stagesByDigest[stage.Digest] = append(stagesByDigest[stage.Digest], stageName)
	}

	instances, err := cd.MatchDockerImagesToGitCommits(ctx, dockerClient, repo, image, limit)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	type displayRow struct {
		Commit  string
		Stages  []string
		Time    time.Time
		Subject string
	}
	data := make([]displayRow, 0, len(instances))
	for _, instance := range instances {
		data = append(data, displayRow{
			Commit:  instance.Commit.Hash[:7],
			Stages:  stagesByDigest[instance.Digest],
			Time:    instance.Commit.Timestamp,
			Subject: instance.Commit.Subject,
		})
	}
	table, err := term.TableConfig{
		GetTerminalWidth:      term.GetTerminalWidthFunc(100),
		IncludeHeader:         true,
		TimeAsDiffs:           true,
		EmptyCollectionsBlank: true,
	}.Structs(ctx, data)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return table, nil
}
