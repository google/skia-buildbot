package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"cloud.google.com/go/spanner"
	"github.com/urfave/cli/v2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/go/sklog/stdlogging"
	"go.skia.org/infra/go/urfavecli"
	"go.skia.org/infra/rag/go/blamestore"
	"go.skia.org/infra/rag/go/config"
	"go.skia.org/infra/rag/go/ingest/history"
	"go.skia.org/infra/rag/go/ingest/sources"
	"go.skia.org/infra/rag/go/topicstore"
)

const (
	embeddingFileName = "embeddings.npy"
	indexFileName     = "index.pkl"
	topicsDirName     = "topics"
)

// LocalIngesterFlags defines the commandline flags to start the local ingester.
type LocalIngesterFlags struct {
	ConfigFilename string
	DirectoryPath  string
}

func (flags *LocalIngesterFlags) AsCliFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Destination: &flags.ConfigFilename,
			Name:        "config_filename",
			Value:       "./configs/demo.json",
			Usage:       "The name of the config file to use.",
		},
		&cli.StringFlag{
			Name:        "directory",
			Value:       ".",
			Usage:       "The path to the directory to ingest.",
			Destination: &flags.DirectoryPath,
		},
	}
}

func main() {
	var flags LocalIngesterFlags
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
				Usage:       "The rag api service",
				Description: "Runs the process that hosts the RAG api service.",
				Flags:       (&flags).AsCliFlags(),
				Action: func(c *cli.Context) error {
					urfavecli.LogFlags(c)
					sklog.Infof("Ingesting directory %s with config %s", flags.DirectoryPath, flags.ConfigFilename)
					config, err := config.NewApiServerConfigFromFile(flags.ConfigFilename)
					if err != nil {
						sklog.Errorf("Error reading config file %s: %v", flags.ConfigFilename, err)
						return err
					}

					// Generate the database identifier string and create the spanner client.
					databaseName := fmt.Sprintf("projects/%s/instances/%s/databases/%s", config.SpannerConfig.ProjectID, config.SpannerConfig.InstanceID, config.SpannerConfig.DatabaseID)
					spannerClient, err := spanner.NewClient(c.Context, databaseName)
					if err != nil {
						sklog.Errorf("Error creating a spanner client")
						return err
					}

					sklog.Infof("Creating a new blamestore instance")
					blamestore := blamestore.New(spannerClient)
					topicstore := topicstore.New(spannerClient)
					sklog.Infof("Creating a new history ingester.")
					ingester := history.New(blamestore, topicstore)

					return filepath.WalkDir(flags.DirectoryPath, func(path string, d fs.DirEntry, err error) error {
						if err != nil {
							return err
						}
						if d.IsDir() {
							return nil
						}

						fileInfo, err := d.Info()
						if err != nil {
							return err
						}

						extension := filepath.Ext(fileInfo.Name())
						if extension != ".json" {
							return nil
						}
						return ingestFile(c.Context, ingester, path)
					})
				},
			},
			{
				Name:        "topics",
				Usage:       "The rag api service",
				Description: "Runs the topic ingestion",
				Flags:       (&flags).AsCliFlags(),
				Action: func(c *cli.Context) error {
					urfavecli.LogFlags(c)
					sklog.Infof("Ingesting directory %s with config %s", flags.DirectoryPath, flags.ConfigFilename)
					config, err := config.NewApiServerConfigFromFile(flags.ConfigFilename)
					if err != nil {
						sklog.Errorf("Error reading config file %s: %v", flags.ConfigFilename, err)
						return err
					}

					// Generate the database identifier string and create the spanner client.
					databaseName := fmt.Sprintf("projects/%s/instances/%s/databases/%s", config.SpannerConfig.ProjectID, config.SpannerConfig.InstanceID, config.SpannerConfig.DatabaseID)
					spannerClient, err := spanner.NewClient(c.Context, databaseName)
					if err != nil {
						sklog.Errorf("Error creating a spanner client")
						return err
					}

					sklog.Infof("Creating a new blamestore instance")
					blamestore := blamestore.New(spannerClient)
					topicStore := topicstore.New(spannerClient)
					sklog.Infof("Creating a new history ingester.")
					ingester := history.New(blamestore, topicStore)
					embeddingFilePath := filepath.Join(flags.DirectoryPath, embeddingFileName)
					indexFilePath := filepath.Join(flags.DirectoryPath, indexFileName)
					topicsDirPath := filepath.Join(flags.DirectoryPath, topicsDirName)
					return ingester.IngestTopics(c.Context, topicsDirPath, embeddingFilePath, indexFilePath)
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

func ingestFile(ctx context.Context, ingester *history.HistoryIngester, filePath string) error {
	sklog.Infof("Ingesting file %s", filePath)
	fileSource := sources.NewFileSource(filePath, ingester)
	return fileSource.Ingest(ctx)
}
