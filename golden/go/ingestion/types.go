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
	// TODO(kjlubick) add context.Context to this interface
	SetResultFileHash(ctx context.Context, fileName, md5 string) error

	// ContainsResultFileHash returns true if the provided file and md5 hash
	// were previously set with SetResultFileHash.
	ContainsResultFileHash(ctx context.Context, fileName, md5 string) (bool, error)
}

// DataSource is a single ingestion source. Currently we use the convention
// that if 'bucket' is empty, we assume a source on the local file system.
type DataSource struct {
	Bucket string // Bucket in Google storage. If empty local storage is assumed.
	Dir    string // Root directory of the data to ingest.
}

type IngesterConfig struct {
	// As of 2019, the primary way to ingest data is event-driven. That is, when
	// new files are put into a GCS bucket, PubSub fires an event and that is the
	// primary way for an ingester to be notified about a file.
	// The four parameters below configure the manual polling of the source, which
	// is a backup way to ingest data in the unlikely case that a PubSub event is
	// dropped (PubSub will try and re-try to send events for up to seven days by default).
	// If MinDays and MinHours are both 0, polling will not happen.
	// If MinDays and MinHours are both specified, the two will be added.
	RunEvery config.Duration // How often the ingester should pull data from Google Storage.
	NCommits int             // Minimum number of commits that should be ingested.
	MinDays  int             // Minimum number of days the commits polled should span.
	MinHours int             // Minimum number of hours the commits polled should span.

	MetricName  string            // What to call this ingester's data when imported to Graphite
	Sources     []*DataSource     // Input sources where the ingester reads from.
	ExtraParams map[string]string // Any additional needed parameters (ingester specific)
}

// Config is a struct to configure multiple ingesters.
type Config struct {
	GitRepoURL       string // Git URL of the repo.
	SecondaryRepoURL string // URL of the secondary repo that has above as a dependency.
	SecondaryRepoDir string // Directory location for the secondary repo.
	SecondaryRegEx   string // Regular expression to extract the commit hash from the DEPS file.
	EventTopic       string // PubSub topic on which global events are sent.
	Ingesters        map[string]*IngesterConfig
}

// ConfigFromJson5File parses a JSON5 file into a Config struct.
// TODO(kjlubick) replace this with golden/go/config
func ConfigFromJson5File(path string) (*Config, error) {
	ret := &Config{}
	if err := config.ParseConfigFile(path, "", ret); err != nil {
		return nil, err
	}
	return ret, nil
}
