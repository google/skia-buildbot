package upload

import "context"

// UploadClientConfig contains configurations required for client setup.
type UploadClientConfig struct {
	Project   string
	DatasetID string
	TableName string
}

// CreateTableRequest contains all information to create a database table.
// Definition is usually a struct with some sort of tagging.
//
// For example, for Chrome, if the database is BigQuery:
//
//	type TestResult struct {
//	      ID     string `bigquery:"id"`
//	      Result string `bigquery:"result"`
//	}
type CreateTableRequest struct {
	Definition interface{}
}

// InsertRequest contains all information to upload.
type InsertRequest struct {
	Items interface{}
}

// UploadClient defines an interface for uploading data to some source.
type UploadClient interface {
	// CreateTableFromStruct creates a table based on the provided struct definition.
	// The struct should define some tags that can be utilized for table definition.
	CreateTableFromStruct(ctx context.Context, req *CreateTableRequest) error

	// Insert uploads one or more rows.
	Insert(ctx context.Context, req *InsertRequest) error
}

// NewUploadClient returns, at the moment, a Chrome implementation for uploading data.
func NewUploadClient(ctx context.Context, cfg *UploadClientConfig) (UploadClient, error) {
	return newUploadChromeDataClient(ctx, cfg)
}
