package sharedconfig

import (
	"time"

	"github.com/BurntSushi/toml"
)

// tomlDuration is a simple struct wrapper to allow us to parse strings as durations
// from the incoming toml file (e.g,. RunEvery = "5m")
type TomlDuration struct {
	time.Duration
}

func (d *TomlDuration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

// DataSource is a single ingestion source. Currently we use the convention
// that if 'bucket' is empty, we assume a source on the local file system.
type DataSource struct {
	Bucket string // Bucket in Google storage. If empty local storage is assumed.
	Dir    string // Root directory of the data to ingest.
}

type IngesterConfig struct {
	RunEvery    TomlDuration      // How often the ingester should pull data from Google Storage.
	NCommits    int               // Minimum number of commits that should be ingested.
	MinDays     int               // Minimum number of days that should be covered by the ingested commits.
	StatusDir   string            // Path where the ingest process keeps its status between restarts.
	MetricName  string            // What to call this ingester's data when imported to Graphite
	Sources     []*DataSource     // Input sources where the ingester reads from.
	ExtraParams map[string]string // Any additional needed parameters (ingester specific)
}

// Config is a struct to configure multiple ingesters.
type Config struct {
	GitRepoDir string // Directory location for the repo.
	Ingesters  map[string]*IngesterConfig
}

// ConfigFromTomlFile parses a TOML file into a Config struct.
func ConfigFromTomlFile(path string) (*Config, error) {
	ret := &Config{}
	if _, err := toml.DecodeFile(path, ret); err != nil {
		return nil, err
	}
	return ret, nil
}
