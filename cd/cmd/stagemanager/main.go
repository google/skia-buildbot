package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"go.skia.org/infra/cd/go/stages"
	"go.skia.org/infra/go/docker"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vfs"
)

func main() {
	var sm *stages.StageManager
	app := &cli.App{
		Name:        "stagemanager",
		Description: `stagemanager provides tools for managing release stages.`,
		Before: func(ctx *cli.Context) error {
			dockerClient, err := docker.NewClient(ctx.Context)
			if err != nil {
				return skerr.Wrap(err)
			}
			fs, err := vfs.InRepoRoot()
			if err != nil {
				return skerr.Wrap(err)
			}
			sm = stages.NewStageManager(ctx.Context, fs, dockerClient)
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
						return skerr.Fmt("Expected exactly three positional arguments.")
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
							if len(args) != 1 || len(args) != 2 {
								return skerr.Fmt("Expected one or two positional arguments.")
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
								return skerr.Fmt("Expected exactly one positional argument.")
							}
							return sm.RemoveImage(ctx.Context, args[0])
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
								return skerr.Fmt("Expected exactly three positional arguments.")
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
							if len(args) != 1 {
								return skerr.Fmt("Expected exactly two positional arguments.")
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
