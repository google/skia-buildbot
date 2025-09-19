package tracestore

import "context"

// MetadataStore provides an interface to perform Metadata operations.
type MetadataStore interface {
	// InsertMetadata inserts the metadata for the source file.
	InsertMetadata(ctx context.Context, sourceFileName string, links map[string]string) error

	// GetMetadata returns the metadata for the given source file.
	GetMetadata(ctx context.Context, sourceFileName string) (map[string]string, error)

	// GetMetadataMultiple returns the metadata for the list of sourceFiles.
	GetMetadataMultiple(ctx context.Context, sourceFileNames []string) (map[string]map[string]string, error)

	// GetMetadataForSourceFileIDs returns the metadata for the source files defined by the ids.
	GetMetadataForSourceFileIDs(ctx context.Context, sourceFileIDs []int64) (map[int64]map[string]string, error)
}
