package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"cloud.google.com/go/spanner"
	"github.com/urfave/cli/v2"
	"go.skia.org/infra/go/skerr"
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
	ZipFile        string
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
		&cli.StringFlag{
			Name:        "index_zip",
			Value:       "",
			Usage:       "The path to the zip file containing topic data to ingest.",
			Destination: &flags.ZipFile,
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
					var topicStore topicstore.TopicStore
					if config.UseRepositoryTopics {
						topicStore = topicstore.NewRepositoryTopicStore(spannerClient)
					} else {
						topicStore = topicstore.New(spannerClient)
					}
					sklog.Infof("Creating a new history ingester.")
					ingester := history.New(blamestore, topicStore, config.OutputDimensionality, config.UseRepositoryTopics, config.DefaultRepoName)

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
					directoryPath := flags.DirectoryPath
					if flags.ZipFile != "" {
						sklog.Infof("Extracting zip index archive.")
						zipFileName := filepath.Base(flags.ZipFile)
						tempDir, err := os.MkdirTemp("", "index-"+zipFileName)
						if err != nil {
							return err
						}
						defer func() {
							err := os.RemoveAll(tempDir)
							if err != nil {
								sklog.Errorf("Error removing temp directory %s: %v", tempDir, err)
							}
						}()
						err = extractZip(flags.ZipFile, tempDir)
						if err != nil {
							return err
						}
						directoryPath = tempDir
						sklog.Infof("Zip file extracted to %s", directoryPath)
					}
					sklog.Infof("Ingesting directory %s with config %s", directoryPath, flags.ConfigFilename)
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
					var topicStore topicstore.TopicStore
					if config.UseRepositoryTopics {
						topicStore = topicstore.NewRepositoryTopicStore(spannerClient)
					} else {
						topicStore = topicstore.New(spannerClient)
					}
					sklog.Infof("Creating a new history ingester.")
					ingester := history.New(blamestore, topicStore, config.OutputDimensionality, config.UseRepositoryTopics, config.DefaultRepoName)
					embeddingFilePath := filepath.Join(directoryPath, embeddingFileName)
					indexFilePath := filepath.Join(directoryPath, indexFileName)
					topicsDirPath := filepath.Join(directoryPath, topicsDirName)
					return ingester.IngestTopics(c.Context, topicsDirPath, embeddingFilePath, indexFilePath, "")
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

func extractZip(zipFile string, dest string) error {
	r, err := zip.OpenReader(zipFile)
	if err != nil {
		// If opening fails, clean up the temp directory that was created.
		return skerr.Fmt("failed to open zip file: %w", err)
	}
	defer r.Close()

	extractFile := func(f *zip.File) error {
		// Construct the full path where the file will be extracted
		extractedPath := filepath.Join(dest, f.Name)

		// Handle Directory Entries
		if f.FileInfo().IsDir() {
			log.Printf("Creating directory: %s", extractedPath)
			if err := os.MkdirAll(extractedPath, f.Mode()); err != nil {
				return skerr.Fmt("failed to create directory %s: %w", extractedPath, err)
			}
			return nil
		}

		// Open the file in the archive
		rc, err := f.Open()
		if err != nil {
			return skerr.Fmt("failed to open file in zip: %w", err)
		}
		defer rc.Close()

		// Ensure the parent directory exists before creating the file
		if err := os.MkdirAll(filepath.Dir(extractedPath), 0755); err != nil {
			return skerr.Fmt("failed to create parent dir: %w", err)
		}

		// Create the output file
		outFile, err := os.OpenFile(extractedPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return skerr.Fmt("failed to create output file %s: %w", extractedPath, err)
		}
		defer outFile.Close()

		// Copy file contents
		if _, err = io.Copy(outFile, rc); err != nil {
			return skerr.Fmt("failed to copy file contents: %w", err)
		}
		return nil
	}
	for _, f := range r.File {
		err := extractFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}
