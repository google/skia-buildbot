package ingestion

import (
	"context"
	"errors"
	"time"

	"go.skia.org/infra/go/config"
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
	// As of 2019, the primary way to ingest data is event-driven. That is, when
	// new files are put into a GCS Bucket, PubSub fires an event and that is the
	// primary way for an ingester to be notified about a file.
	// The four parameters below configure the manual polling of the source, which
	// is a backup way to ingest data in the unlikely case that a PubSub event is
	// dropped (PubSub will try and re-try to send events for up to seven days by default).
	// If MinDays and MinHours are both 0, polling will not happen.
	// If MinDays and MinHours are both specified, the two will be added.

	// How often the ingester should pull data from Google Storage.
	RunEvery config.Duration `json:"backup_poll_every"`

	// Minimum number of commits that should be ingested.
	NCommits int `json:"backup_poll_last_n_commits" optional:"true"`

	// Minimum number of days the commits polled should span.
	MinDays int `json:"backup_poll_last_n_days" optional:"true"`

	// Minimum number of hours the commits polled should span.
	MinHours int `json:"backup_poll_last_n_hours" optional:"true"`

	// Input sources where the ingester reads from.
	// TODO(kjlubick) we only really need one source.
	Sources []GCSSourceConfig `json:"gcs_sources"`

	// Any additional needed parameters (ingester specific)
	ExtraParams map[string]string `json:"extra_configuration"`
}

type GCSSourceConfig struct {
	Bucket string `json:"bucket"`
	Prefix string `json:"dir"`
}
