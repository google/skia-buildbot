package ingestion

import (
	"context"
	"errors"
	"io"

	"go.skia.org/infra/go/config"
)

var (
	// IgnoreResultsFileErr can be returned by the Process function of a processor to
	// indicated that this file should be considered ignored. It is up to the processor
	// to write to the log.
	IgnoreResultsFileErr = errors.New("Ignore this file.")
)

// Source defines an ingestion source that returns lists of result files
// either through polling or in an event driven mode.
type Source interface {
	// ID returns a unique identifier for this source.
	ID() string

	// Poll returns a channel to read all the result files that originated between
	// the given timestamps in seconds since the epoch.
	Poll(startTime, endTime int64) <-chan ResultFileLocation

	// SetEventChannel configures storage events and sets up routines to send
	// new results to the given channel.
	SetEventChannel(resultCh chan<- ResultFileLocation) error
}

// ResultFileLocation is an abstract interface to a file like object that
// contains results that need to be ingested.
type ResultFileLocation interface {
	// Open returns a reader that allows to read the content of the file.
	Open(ctx context.Context) (io.ReadCloser, error)

	// Name returns the full path of the file. The last segment is usually the
	// the file name.
	Name() string

	// StorageIDs return the bucket and object ID for the given location.
	StorageIDs() (string, string)

	// MD5 returns the MD5 hash of the content of the file.
	MD5() string

	// TimeStamp returns the timestamp when the file was last updated.
	TimeStamp() int64

	// Content returns the content of the file if has been read or nil otherwise.
	Content() []byte
}

// Processor is the core of an Ingester. It takes instances of ResultFileLocation
// and ingests them. It is responsible for the storage of ingested data.
type Processor interface {
	// Process ingests a single result file.
	Process(ctx context.Context, resultsFile ResultFileLocation) error
}

// IngestionStore keeps track of files being ingested based on their MD5 hashes.
type IngestionStore interface {
	// SetResultFileHash indicates that we have ingested the given filename
	// with the given md5hash.
	SetResultFileHash(ctx context.Context, fileName, md5 string) error

	// ContainsResultFileHash returns true if the provided file and md5 hash
	// were previously set with SetResultFileHash.
	ContainsResultFileHash(ctx context.Context, fileName, md5 string) (bool, error)
}

// Config is the configuration for a single ingester.
type Config struct {
	// As of 2019, the primary way to ingest data is event-driven. That is, when
	// new files are put into a GCS bucket, PubSub fires an event and that is the
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
	Sources []GCSSource `json:"gcs_sources"`

	// Any additional needed parameters (ingester specific)
	ExtraParams map[string]string `json:"extra_configuration"`
}

// GCSSource is a single ingestion source of a given GCS bucket.
type GCSSource struct {
	// Bucket in Google storage. The reason this is specified here is that a single ingester could
	// be configured to read in data from multiple buckets (e.g. a public bucket and a private
	// bucket).
	Bucket string `json:"bucket"`

	// Root directory (aka prefix) of the data to ingest in the GCS bucket.
	Dir string `json:"prefix"`
}
