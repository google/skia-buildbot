package sources

import "context"

// Source defines an interface for data source provider.
type Source interface {
	// Ingest performs the ingestion of the provided data.
	Ingest(ctx context.Context) error
}
