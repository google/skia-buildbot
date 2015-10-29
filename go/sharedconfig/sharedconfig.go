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

// DataSourc is a single ingestion source. Currently we use the convention
// that if bucket is empty, we assume a source on the local file system.
type DataSource struct {
	Bucket string // Bucket in Google storage. If empty local storage is assumed.
	Dir    string // Root directory of the data to ingest.
}

type CommonConfig struct {
	DoOAuth               bool   // Run through the OAuth 2.0 flow on startup, otherwise use a GCE service account.
	GitRepoDir            string // Directory location for the Skia repo.
	GraphiteServer        string // Where the Graphite metrics ingestion server is running.
	Local                 bool   // Running locally if true. As opposed to in production.
	OAuthCacheFile        string // Path to the file where to cache cache the oauth credentials.
	OAuthClientSecretFile string // Path to the file with the oauth client secret.
}

type IngesterConfig struct {
	RunEvery        TomlDuration      // How often the ingester should pull data from Google Storage.
	NCommits        int               // Minimum number of commits that should be ingested.
	MinDays         int               // Minimum number of days that should be covered by the ingested commits.
	StatusDir       string            // Path where the ingest process keeps its status between restarts.
	MetricName      string            // What to call this ingester's data when imported to Graphite
	ConstructorName string            // Named constructor for this ingester (defaults to the dataset name).
	DBHost          string            // Hostname of the MySQL database server used by this ingester.
	DBName          string            // Name of the MySQL database used by this ingester.
	DBPort          int               // Port number of the MySQL database used by this ingester.
	Sources         []*DataSource     // Input sources where the ingester reads from.
	ExtraParams     map[string]string // Any additional needed parameters (ingester specific)
}

// Appconfig is a configuration structure that can be shared by multiple
// applications. Currently it is only used by ingestion. In the future it will
// be used by any app related to ingestion. i.e. gold.
type AppConfig struct {
	Common    CommonConfig
	Ingesters map[string]*IngesterConfig
}

func ConfigFromTomlFile(path string) (*AppConfig, error) {
	ret := &AppConfig{}
	if _, err := toml.DecodeFile(path, ret); err != nil {
		return nil, err
	}
	return ret, nil
}
