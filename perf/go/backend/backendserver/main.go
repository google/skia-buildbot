package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/go/sklog/stdlogging"
	"go.skia.org/infra/go/urfavecli"
	"go.skia.org/infra/perf/go/backend"
	"go.skia.org/infra/perf/go/config"
)

func main() {
	var backendFlags config.BackendFlags
	cli.MarkdownDocTemplate = urfavecli.MarkdownDocTemplate

	cliApp := &cli.App{
		Name:  "backend",
		Usage: "Command line tool that runs the backend service for Perf.",
		Before: func(c *cli.Context) error {
			// Log to stdout.
			sklogimpl.SetLogger(stdlogging.New(os.Stdout))

			return nil
		},
		Commands: []*cli.Command{
			{
				Name:        "run",
				Usage:       "The backend service",
				Description: "Runs the process that hosts the backend service.",
				Flags:       (&backendFlags).AsCliFlags(),
				Action: func(c *cli.Context) error {
					urfavecli.LogFlags(c)
					b, err := backend.New(&backendFlags, nil, nil, nil, nil, nil)
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
