package sharedconfig

import (
	"go.skia.org/infra/go/config"
)

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
	// TODO(kjlubick): remove GitRepoDir after Skia is off of a local checkout.
	GitRepoDir       string // Directory location for the repo. Deprecated.
	GitRepoURL       string // Git URL of the repo.
	SecondaryRepoURL string // URL of the secondary repo that has above as a dependency.
	SecondaryRepoDir string // Directory location for the secondary repo.
	SecondaryRegEx   string // Regular expression to extract the commit hash from the DEPS file.
	EventTopic       string // PubSub topic on which global events are sent.
	Ingesters        map[string]*IngesterConfig
}

// ConfigFromJson5File parses a JSON5 file into a Config struct.
func ConfigFromJson5File(path string) (*Config, error) {
	ret := &Config{}
	if err := config.ParseConfigFile(path, "", ret); err != nil {
		return nil, err
	}
	return ret, nil
}
