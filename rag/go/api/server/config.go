package main

import (
	"encoding/json"
	"io"

	"github.com/urfave/cli/v2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// ApiServerFlags defines the commandline flags to start the api server.
type ApiServerFlags struct {
	ConfigFilename string
	GrpcPort       string
	HttpPort       string
	PromPort       string
	Services       cli.StringSlice
}

// AsCliFlags returns a slice of cli.Flag.
func (flags *ApiServerFlags) AsCliFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Destination: &flags.ConfigFilename,
			Name:        "config_filename",
			Value:       "./configs/demo.json",
			Usage:       "The name of the config file to use.",
		},
		&cli.StringSliceFlag{
			Name:        "services",
			Value:       cli.NewStringSlice("history"),
			Usage:       "This list of RAG services to host on the api.",
			Destination: &flags.Services,
		},
		&cli.StringFlag{
			Destination: &flags.GrpcPort,
			Name:        "grpc_port",
			Value:       ":8000",
			Usage:       "The port number to use for grpc server.",
		},
		&cli.StringFlag{
			Destination: &flags.HttpPort,
			Name:        "http_port",
			Value:       ":8002",
			Usage:       "The port number to use for http server.",
		},
		&cli.StringFlag{
			Destination: &flags.PromPort,
			Name:        "prom_port",
			Value:       ":20000",
			Usage:       "Metrics service address (e.g., ':10110')",
		},
	}
}

// ApiServerConfig defines a struct to hold the config information.
type ApiServerConfig struct {
	// Spanner database configuration.
	SpannerConfig SpannerConfig `json:"spanner_config"`
}

// SpannerConfig defines a struct to hold the spanner database configuration.
type SpannerConfig struct {
	// ID of the GCP project.
	ProjectID string `json:"project_id"`
	// ID of the spanner instance.
	InstanceID string `json:"instance_id"`
	// ID of the database.
	DatabaseID string `json:"database_id"`
}

// NewApiServerConfigFromFile returns a new config object based on the file content.
func NewApiServerConfigFromFile(filename string) (*ApiServerConfig, error) {
	var config ApiServerConfig
	sklog.Infof("Reading config file: %s", filename)
	err := util.WithReadFile(filename, func(r io.Reader) error {
		b, err := io.ReadAll(r)
		if err != nil {
			return skerr.Wrapf(err, "failed to read bytes")
		}

		return json.Unmarshal(b, &config)
	})

	if err != nil {
		return nil, skerr.Wrapf(err, "Filename: %s", filename)
	}
	return &config, nil
}
