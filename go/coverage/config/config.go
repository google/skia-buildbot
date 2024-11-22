package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	cli "github.com/urfave/cli/v2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	localHost string = "127.0.0.1:8006"
)

var schema []byte

// CoverageConfig provide commandline flags for the Coverage Service.
type CoverageConfig struct {
	ConfigFilename string
	DatabaseType   string `json:"database_type"`
	DatabaseName   string `json:"database_name"`
	DatabaseHost   string `json:"database_host"`
	DatabasePort   int    `json:"database_port"`
	ServiceHost    string `json:"service_host"`
	ServicePort    string `json:"service_port"`
	PromPort       string `json:"prom_port"`
}

var Config *CoverageConfig

// AsCliFlags returns a slice of cli.Flag.
func (config *CoverageConfig) AsCliFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Destination: &config.ConfigFilename,
			Name:        "config_filename",
			Value:       "coverage.json",
			Usage:       "The name of the config file to use.",
		},
	}
}

// InstanceConfigFromFile returns the deserialized JSON of an InstanceConfig
// found in filename.
//
// If there was an error loading the file a list of schema violations may be
// returned also.
func (config *CoverageConfig) LoadCoverageConfig(filename string) (*CoverageConfig, error) {
	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		cwd, err := os.Getwd()
		filename = filepath.Join(cwd, "config", filename)
		if err != nil {
			sklog.Fatalf("Could not get working dir: %s, %s", err, filename)
		}
	}

	// Validate config here.
	err := util.WithReadFile(filename, func(r io.Reader) error {
		c, err := io.ReadAll(r)
		if err != nil {
			return skerr.Wrapf(err, "failed to read bytes")
		}
		err = json.Unmarshal(c, &config)
		if err != nil {
			return skerr.Wrapf(err, "Failed to Unmarshal: %s", err)
		}
		sklog.Infof("Loaded Config: %v", config)
		return nil
	})
	if err != nil {
		return config, err
	}
	return config, nil
}

func (config *CoverageConfig) GetConnectionString() string {
	return fmt.Sprintf("postgresql://root@%s:%d/%s?sslmode=disable", config.DatabaseHost,
		config.DatabasePort, config.DatabaseName)
}

func (config *CoverageConfig) GetServiceHost() string {
	if config == nil {
		return localHost
	}
	return config.ServiceHost
}

func (config *CoverageConfig) GetDatabaseName() string {
	return config.DatabaseName
}
