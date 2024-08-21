package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"go.skia.org/infra/go/coverage"
	"go.skia.org/infra/go/coverage/config"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/go/sklog/stdlogging"
	"go.skia.org/infra/go/urfavecli"
)

func main() {
	sklog.Debug("Running Server...")
	var coverageConfig config.CoverageConfig
	cli.MarkdownDocTemplate = urfavecli.MarkdownDocTemplate
	cliApp := &cli.App{
		Name:  "coverage",
		Usage: "Command line tool that runs the coverage service.",
		Before: func(c *cli.Context) error {
			// Log to stdout.
			sklogimpl.SetLogger(stdlogging.New(os.Stdout))
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:        "run",
				Usage:       "The coverage service",
				Description: "Runs the process that hosts the coverage service.",
				Flags:       (&coverageConfig).AsCliFlags(),
				Action: func(c *cli.Context) error {
					urfavecli.LogFlags(c)
					b, err := coverage.New(&coverageConfig, nil)
					if err != nil {
						return err
					}

					b.Serve()
					return nil
				},
			},
		},
	}
	err := cliApp.Run(os.Args)
	if err != nil {
		fmt.Printf("\nError: %s\n", err.Error())
		os.Exit(2)
	}
}
