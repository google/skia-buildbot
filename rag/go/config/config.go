package config

import (
	"encoding/json"
	"io"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// ApiServerConfig defines a struct to hold the config information.
type ApiServerConfig struct {
	// Spanner database configuration.
	SpannerConfig SpannerConfig `json:"spanner_config"`

	// Ingestion configuration
	IngestionConfig IngestionConfig `json:"ingestion_config"`

	// The embedding model to use for embedding the input query.
	QueryEmbeddingModel string `json:"query_embedding_model"`

	// The output dimensionality to use for input query embedding.
	OutputDimensionality int `json:"output_dimensionality"`

	// The name of the instance.
	InstanceName string `json:"instance_name"`

	// The URL of the image to display in the header.
	HeaderIconUrl string `json:"header_icon_url"`
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

// IngestionConfig provides a struct to contain ingestion config data.
type IngestionConfig struct {
	Topic        string `json:"topic"`
	Subscription string `json:"subscription"`
	Project      string `json:"project"`
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
