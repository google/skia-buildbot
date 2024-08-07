package backends

import (
	"context"

	"cloud.google.com/go/bigquery"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// BigQueryClient interfaces interactions to BigQuery
type BigQueryClient interface {
	// CreateTable creates the dataset and table for the project that the client was instantiated with.
	// schema is expected to be a struct with bigquery tags defined for column definitions.
	CreateTable(ctx context.Context, datasetID, tableName string, schema interface{}) error

	// Insert implements BigQuery PUT.
	Insert(ctx context.Context, datasetID, tableName string, rows interface{}) error
}

// bigQueryClient implements BigQueryClient
type bigQueryClient struct {
	client *bigquery.Client
}

// NewBigQueryClient returns a BigQueryClient for the provided project.
func NewBigQueryClient(ctx context.Context, project string) (*bigQueryClient, error) {
	bqClient, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create BigQuery client")
	}
	return &bigQueryClient{
		client: bqClient,
	}, nil
}

// CreateTable creates the dataset and the table in the project defined by the client.
// src is expected to be a struct with bigquery tags defined.
func (b *bigQueryClient) CreateTable(ctx context.Context, datasetID, tableName string, src interface{}) error {
	if datasetID == "" || tableName == "" {
		return skerr.Fmt("Cannot create table with undefined dataset or table name")
	}
	schema, err := bigquery.InferSchema(src)
	if err != nil {
		return skerr.Wrapf(err, "Unable to infer schema while creating table")
	}

	ds := b.client.Dataset(datasetID)
	// Try creating the dataset to ensure it exists
	if err := ds.Create(ctx, nil); err != nil {
		// It only throws an error if it already exists.
		sklog.Infof("%s dataset already exists.", datasetID)
	}

	table := ds.Table(tableName)
	if err := table.Create(ctx, &bigquery.TableMetadata{Name: tableName, Schema: schema}); err != nil {
		return skerr.Wrapf(err, "Failed to create table")
	}

	return nil
}

// Insert wraps BigQuery PUT.
func (b *bigQueryClient) Insert(ctx context.Context, datasetID, tableName string, rows interface{}) error {
	tableIns := b.client.Dataset(datasetID).Table(tableName).Inserter()
	return tableIns.Put(ctx, rows)
}

var _ BigQueryClient = (*bigQueryClient)(nil)
