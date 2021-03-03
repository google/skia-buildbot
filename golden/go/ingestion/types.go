package ingestion

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrRetryable can be returned to indicate the input file was valid, but couldn't be
	// processed due to temporary issues, like a bad HTTP connection.
	ErrRetryable = errors.New("error may be resolved with retry")
)

// Processor is the core of an Ingester. It reads in the files that are given to it and stores
// the relevant data.
type Processor interface {
	// HandlesFile returns true if this processor is configured to handle this file.
	HandlesFile(name string) bool
	// Process ingests a single result file.
	Process(ctx context.Context, filename string) error
}

// Store keeps track of files being ingested based on their MD5 hashes.
type Store interface {
	// SetIngested indicates that we have ingested the given filename. Implementations may make
	// use of the ingested timestamp.
	SetIngested(ctx context.Context, fileName string, ts time.Time) error

	// WasIngested returns true if the provided file has been ingested previously.
	WasIngested(ctx context.Context, fileName string) (bool, error)
}

// Config is the configuration for a single ingester.
type Config struct {
	// Input sources where the ingester reads from.
	// TODO(kjlubick) we only really need one source.
	Sources []GCSSourceConfig `json:"gcs_sources"`

	// Any additional needed parameters (ingester specific)
	ExtraParams map[string]string `json:"extra_configuration"`
}

// GCSSourceConfig is the configuration needed to ingest from files in a GCS bucket.
type GCSSourceConfig struct {
	Bucket string `json:"bucket"`
	Prefix string `json:"prefix"`
}
