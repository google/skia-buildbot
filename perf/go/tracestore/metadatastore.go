package tracestore

import "context"

// MetadataStore provides an interface to perform Metadata operations.
type MetadataStore interface {
	// InsertMetadata inserts the metadata for the source file.
	InsertMetadata(ctx context.Context, sourceFileName string, links map[string]string) error

	// GetMetadata returns the metadata for the given source file.
	GetMetadata(ctx context.Context, sourceFileId int) (map[string]string, error)
}
