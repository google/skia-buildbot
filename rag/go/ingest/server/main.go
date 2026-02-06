package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/urfave/cli/v2"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/go/sklog/stdlogging"
	"go.skia.org/infra/go/urfavecli"
	"go.skia.org/infra/rag/go/config"
	"go.skia.org/infra/rag/go/genai"
	"go.skia.org/infra/rag/go/tracing"
)

const (
	geminiApiKeyEnvVar   = "GEMINI_API_KEY"
	geminiProjectEnvVar  = "GEMINI_PROJECT"
	geminiLocationEnvVar = "GEMINI_LOCATION"
)

// IngesterFlags defines the commandline flags to start the ingester.
type IngesterFlags struct {
	ConfigFilename string
	Local          bool
	PromPort       string
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
		&cli.BoolFlag{
			Destination: &flags.Local,
			Name:        "local",
			Value:       false,
			Usage:       "Set to true if running in non-production environment",
		},
		&cli.StringFlag{
			Destination: &flags.PromPort,
			Name:        "prom_port",
			Value:       ":20000",
			Usage:       "Prometheus metrics port",
		},
	}
}

func main() {

	var flags IngesterFlags
	cli.MarkdownDocTemplate = urfavecli.MarkdownDocTemplate
	cliApp := &cli.App{
		Name:  "RAG ingest",
		Usage: "Command line tool that runs the RAG ingester subscribing to a pubsub.",
		Before: func(c *cli.Context) error {
			// Log to stdout.
			sklogimpl.SetLogger(stdlogging.New(os.Stdout))
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:        "topics",
				Usage:       "The rag topics ingester service",
				Description: "Runs the process that runs the RAG topics ingester.",
				Flags:       (&flags).AsCliFlags(),
				Action: func(c *cli.Context) error {
					urfavecli.LogFlags(c)

					metrics2.InitPrometheus(flags.PromPort)
					err := tracing.Init(flags.Local, "historyrag-ingester", 0.1)
					if err != nil {
						sklog.Errorf("Error initializing tracing: %v", err)
						return err
					}
					config, err := config.NewApiServerConfigFromFile(flags.ConfigFilename)
					if err != nil {
						sklog.Errorf("Error reading config file %s: %v", flags.ConfigFilename, err)
						return err
					}

					var genAiClient genai.GenAIClient
					apiKey := os.Getenv(geminiApiKeyEnvVar)
					if apiKey != "" {
						sklog.Infof("Gemini api key specified in the environment, creating a local client.")
						genAiClient, err = genai.NewLocalGeminiClient(c.Context, apiKey)
					} else {
						projectId := os.Getenv(geminiProjectEnvVar)
						location := os.Getenv(geminiLocationEnvVar)
						if projectId != "" && location != "" {
							sklog.Infof("Creating a new Gemini client for project %s and location %s", projectId, location)
							genAiClient, err = genai.NewGeminiClient(c.Context, projectId, location)
						}
					}
					// Exit if there was an error setting up the gemini client.
					if err != nil {
						sklog.Fatalf("Error creating new gemini client: %v", err)
					}

					subscriber, err := NewIngestionSubscriber(c.Context, *config, genAiClient)
					if err != nil {
						return err
					}
					sklog.Infof("Starting subscriber")
					var wg sync.WaitGroup
					wg.Add(1)
					subscriber.Start(c.Context, &wg)
					wg.Wait()
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
