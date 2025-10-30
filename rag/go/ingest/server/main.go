package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/go/sklog/stdlogging"
	"go.skia.org/infra/go/urfavecli"
	"go.skia.org/infra/rag/go/config"
)

// IngesterFlags defines the commandline flags to start the ingester.
type IngesterFlags struct {
	ConfigFilename string
}

// AsCliFlags returns the cli flags for the ingester.
func (flags *IngesterFlags) AsCliFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Destination: &flags.ConfigFilename,
			Name:        "config_filename",
			Value:       "./configs/demo.json",
			Usage:       "The name of the config file to use.",
		},
	}
}

func main() {

	var flags IngesterFlags
	cli.MarkdownDocTemplate = urfavecli.MarkdownDocTemplate
	cliApp := &cli.App{
		Name:  "RAG ingest",
		Usage: "Command line tool that runs the RAG ingester from local disk.",
		Before: func(c *cli.Context) error {
			// Log to stdout.
			sklogimpl.SetLogger(stdlogging.New(os.Stdout))
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:        "blames",
				Usage:       "The rag blames ingester service",
				Description: "Runs the process that runs the RAG blames ingester.",
				Flags:       (&flags).AsCliFlags(),
				Action: func(c *cli.Context) error {
					urfavecli.LogFlags(c)
					config, err := config.NewApiServerConfigFromFile(flags.ConfigFilename)
					if err != nil {
						sklog.Errorf("Error reading config file %s: %v", flags.ConfigFilename, err)
						return err
					}
					subscriber, err := NewIngestionSubscriber(c.Context, *config)
					if err != nil {
						return err
					}
					subscriber.Start(c.Context)
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
