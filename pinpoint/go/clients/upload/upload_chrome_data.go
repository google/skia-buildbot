package upload

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/backends"
)

// uploadChromeDataClient implements UploadClient for Chrome.
type uploadChromeDataClient struct {
	Project   string
	DatasetID string
	TableName string
	client    backends.BigQueryClient
}

// newUploadChromeDataClient returns a configured version of uploadChromeDataClient.
func newUploadChromeDataClient(ctx context.Context, cfg *UploadClientConfig) (*uploadChromeDataClient, error) {
	bqClient, err := backends.NewBigQueryClient(ctx, cfg.Project)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create client for uploading Chrome data")
	}

	return &uploadChromeDataClient{
		Project:   cfg.Project,
		DatasetID: cfg.DatasetID,
		TableName: cfg.TableName,
		client:    bqClient,
	}, nil
}

// CreateTableFromStruct creates the table for Chrome according to the definition provided.
func (u *uploadChromeDataClient) CreateTableFromStruct(ctx context.Context, req *CreateTableRequest) error {
	return u.client.CreateTable(ctx, u.DatasetID, u.TableName, req.Definition)
}

// Insert injects data defined in the InsertRequest to BigQuery.
func (u *uploadChromeDataClient) Insert(ctx context.Context, req *InsertRequest) error {
	return u.client.Insert(ctx, u.DatasetID, u.TableName, req.Items)
}

var _ UploadClient = (*uploadChromeDataClient)(nil)
