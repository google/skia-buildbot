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
	RunEvery    config.Duration   // How often the ingester should pull data from Google Storage.
	NCommits    int               // Minimum number of commits that should be ingested.
	MinDays     int               // Minimum number of days that should be covered by the ingested commits.
	MetricName  string            // What to call this ingester's data when imported to Graphite
	Sources     []*DataSource     // Input sources where the ingester reads from.
	ExtraParams map[string]string // Any additional needed parameters (ingester specific)
}

// Config is a struct to configure multiple ingesters.
type Config struct {
	GitRepoDir       string // Directory location for the repo.
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
