package config

import (
	"encoding/json"
	"fmt"
	"io"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	// MAX_SAMPLE_TRACES_PER_CLUSTER  is the maximum number of traces stored in a
	// ClusterSummary.
	MAX_SAMPLE_TRACES_PER_CLUSTER = 50

	// MIN_STDDEV is the smallest standard deviation we will normalize, smaller
	// than this and we presume it's a standard deviation of zero.
	MIN_STDDEV = 0.001

	// GOTO_RANGE is the number of commits on either side of a target
	// commit we will display when going through the goto redirector.
	GOTO_RANGE = 10

	CONSTRUCTOR_NANO        = "nano"
	CONSTRUCTOR_NANO_TRYBOT = "nano-trybot"
)

// DataStoreType determines what type of datastore to build. Applies to
// tracestore.Store, alerts.Store, regression.Store, and shortcut.Store.
type DataStoreType string

const (
	// GCPDataStoreType is for datastores in a Google Cloud Project, i.e.
	// BigTable for tracestore.Store, and the rest in Cloud Datastore..
	GCPDataStoreType DataStoreType = "gcp"

	// SQLite3DataStoreType is for storing all data in an SQLite3 database.
	SQLite3DataStoreType DataStoreType = "sqlite3"

	// CockroachDBDataStoreType is for storing all data in a CockroachDB database.
	CockroachDBDataStoreType DataStoreType = "cockroachdb"
)

// DataStoreConfig is the configuration for how Perf stores data.
type DataStoreConfig struct {
	// DataStoreType determines what type of datastore to build. This value will
	// determine how the rest of the DataStoreConfig values are interpreted.
	DataStoreType DataStoreType `json:"datastore_type"`

	// ConnectionString is only used for datastores of the type 'sqlite3' and
	// 'cockroachdb'.
	//
	// If the datastore type is 'sqlite3' this value is a filename of the
	// database.
	//
	// If the datastore type is 'cockroachdb' then this value is a connection
	// string of the form "postgres://...". See
	// https://www.cockroachlabs.com/docs/stable/connection-parameters.html for
	// more details.
	//
	// In addition, for 'cockroachdb' databases, the database name given in the
	// connection string must exist and the user given in the connection string
	// must have rights to create, delete, and alter tables as Perf will do
	// database migrations on startup.
	ConnectionString string `json:"connection_string"`

	// TileSize is the size of each tile in commits. This value is used for all
	// datastore types.
	TileSize int32 `json:"tile_size"`

	// Project is the Google Cloud Project name. This value is only used for
	// 'gcp' datastore types.
	Project string `json:"project"`

	// Instance is the name of the BigTable instance. This value is only used
	// for 'gcp' datastore types.
	Instance string `json:"instance"`

	// Table is the name of the table in BigTable to use. This value is only
	// used for 'gcp' datastore types.
	Table string `json:"table"`

	// Shards is the number of shards to break up all trace data into.
	Shards int32 `json:"shards"`

	// Namespace is the Google Cloud Datastore namespace that alerts,
	// regressions, and shortcuts should use. This value is only used for 'gcp'
	// datastore types.
	Namespace string `json:"namespace"`
}

// SourceType determines what type of file.Source to build from a SourceConfig.
type SourceType string

const (
	// GCSSourceType is for Google Cloud Storage.
	GCSSourceType SourceType = "gcs"

	// DirSourceType is for a local filesystem directory and is only appropriate
	// for tests and demo mode.
	DirSourceType SourceType = "dir"
)

// SourceConfig is the config for where ingestable files come from.
type SourceConfig struct {
	// SourceType is the type of file.Source to use. This value will determine
	// how the rest of the SourceConfig values are interpreted.
	SourceType SourceType `json:"source_type"`

	// Project is the Google Cloud Project name. Only used for source of type
	// "gcs".
	Project string `json:"project"`

	// Topic is the PubSub topic when new files arrive to be ingested. Only used
	// for source of type "gcs".
	Topic string `json:"topic"`

	// Sources is the list of sources of data files. For a source of "gcs" this
	// is a list of Google Cloud Storage URLs, e.g.
	// "gs://skia-perf/nano-json-v1". For a source of type "dir" is must only
	// have a single entry and be populated with a local filesystem directory
	// name.
	Sources []string `json:"sources"`
}

// IngestionConfig is the configuration for how source files are ingested into
// being traces in a TraceStore.
type IngestionConfig struct {
	// SourceConfig is the config for where files to ingest come from.
	SourceConfig SourceConfig `json:"source_config"`

	// Branches, if populated then restrict to ingesting just these branches.
	Branches []string `json:"branches"`

	// FileIngestionTopicName is the PubSub topic name we should use if doing
	// event driven regression detection. The ingesters use this to know where
	// to emit events to, and the clusterers use this to know where to make a
	// subscription.
	//
	// Should only be turned on for instances that have a huge amount of data,
	// i.e. >500k traces, and that have sparse data.
	//
	// This should really go away, IngestionConfig should be used to build
	// an interface that ingests files and optionally provides a channel
	// of events when a file is ingested.
	FileIngestionTopicName string `json:"file_ingestion_pubsub_topic_name"`
}

// GitRepoConfig is the config for the git repo.
type GitRepoConfig struct {
	// URL that the Git repo is fetched from.
	URL string `json:"url"`

	// The directory into which the repo should be checked out.
	Dir string `json:"dir"`

	// DebouceCommitURL signals if a link to a Git commit needs to be specially
	// dereferenced. That is, some repos are synthetic and just contain a single
	// file that changes, with a commit message that is a URL that points to the
	// true source of information. If this value is true then links to commits
	// need to be debounced and use the commit message instead.
	DebouceCommitURL bool `json:"debounce_commit_url"`
}

// InstanceConfig contains all the info needed by btts.BigTableTraceStore.
//
// May eventually move to a separate config file.
type InstanceConfig struct {
	// URL is the root URL at which this instance is available, for example: "https://example.com".
	URL string `json:"URL"`

	DataStoreConfig DataStoreConfig `json:"data_store_config"`
	IngestionConfig IngestionConfig `json:"ingestion_config"`
	GitRepoConfig   GitRepoConfig   `json:"git_repo_config"`
}

// InstanceConfigFromFile returns the deserialized JSON of an InstanceConfig found in filename.
func InstanceConfigFromFile(filename string) (*InstanceConfig, error) {
	var instanceConfig InstanceConfig

	err := util.WithReadFile(filename, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&instanceConfig)
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &instanceConfig, nil
}

const (
	NANO         = "nano"
	ANDROID_PROD = "android-prod"
	CT_PROD      = "ct-prod"
	ANDROID_X    = "android-x"
	FLUTTER      = "flutter"
)

var (
	PERF_BIGTABLE_CONFIGS = map[string]*InstanceConfig{
		NANO: {
			URL: "https://perf.skia.org",
			DataStoreConfig: DataStoreConfig{
				DataStoreType: GCPDataStoreType,
				TileSize:      256,
				Project:       "skia-public",
				Instance:      "production",
				Table:         "perf-skia",
				Shards:        8,
				Namespace:     "perf",
			},
			IngestionConfig: IngestionConfig{
				SourceConfig: SourceConfig{
					SourceType: GCSSourceType,
					Project:    "skia-public",
					Topic:      "perf-ingestion-skia-production",
					Sources:    []string{"gs://skia-perf/nano-json-v1", "gs://skia-perf/task-duration", "gs://skia-perf/buildstats-json-v1"},
				},
				Branches:               []string{},
				FileIngestionTopicName: "",
			},
			GitRepoConfig: GitRepoConfig{
				URL: "https://skia.googlesource.com/skia",
				Dir: "/tmp/repo",
			},
		},
		ANDROID_PROD: {
			URL: "https://android-master-perf.skia.org",
			DataStoreConfig: DataStoreConfig{
				DataStoreType: GCPDataStoreType,
				TileSize:      8192,
				Project:       "skia-public",
				Instance:      "production",
				Table:         "perf-android",
				Shards:        8,
				Namespace:     "perf-androidmaster",
			},
			IngestionConfig: IngestionConfig{
				SourceConfig: SourceConfig{
					SourceType: GCSSourceType,
					Project:    "skia-public",
					Topic:      "perf-ingestion-android-production",
					Sources:    []string{"gs://skia-perf/android-master-ingest"},
				},
				Branches:               []string{},
				FileIngestionTopicName: "perf-ingestion-complete-android-production",
			},
			GitRepoConfig: GitRepoConfig{
				URL:              "https://skia.googlesource.com/perf-buildid/android-master",
				Dir:              "/tmp/repo",
				DebouceCommitURL: true,
			},
		},
		CT_PROD: {
			URL: "https://ct-perf.skia.org",
			DataStoreConfig: DataStoreConfig{
				DataStoreType: GCPDataStoreType,
				TileSize:      256,
				Project:       "skia-public",
				Instance:      "production",
				Table:         "perf-ct",
				Shards:        8,
				Namespace:     "perf-ct",
			},
			IngestionConfig: IngestionConfig{
				SourceConfig: SourceConfig{
					SourceType: GCSSourceType,
					Project:    "skia-public",
					Topic:      "perf-ingestion-ct-production",
					Sources:    []string{"gs://cluster-telemetry-perf/ingest"},
				},
				Branches:               []string{},
				FileIngestionTopicName: "",
			},
			GitRepoConfig: GitRepoConfig{
				URL: "https://skia.googlesource.com/perf-ct",
				Dir: "/tmp/repo",
			},
		},
		ANDROID_X: { // https://bug.skia.org/9315
			URL: "https://androidx-perf.skia.org/",
			DataStoreConfig: DataStoreConfig{
				DataStoreType: GCPDataStoreType,
				TileSize:      512,
				Project:       "skia-public",
				Instance:      "production",
				Table:         "perf-android-x",
				Shards:        8,
				Namespace:     "perf-android-x",
			},
			IngestionConfig: IngestionConfig{
				SourceConfig: SourceConfig{
					SourceType: GCSSourceType,
					Project:    "skia-public",
					Topic:      "perf-ingestion-android-x-production",
					Sources:    []string{"gs://skia-perf/android-master-ingest"},
				},
				Branches:               []string{"aosp-androidx-master-dev"},
				FileIngestionTopicName: "",
			},
			GitRepoConfig: GitRepoConfig{
				URL:              "https://skia.googlesource.com/perf-buildid/android-master",
				Dir:              "/tmp/repo",
				DebouceCommitURL: true,
			},
		},
		FLUTTER: { // https://bug.skia.org/9789
			URL: "https://flutter-perf.skia.org/",
			DataStoreConfig: DataStoreConfig{
				DataStoreType: GCPDataStoreType,
				TileSize:      256,
				Project:       "skia-public",
				Instance:      "production",
				Table:         "perf-flutter",
				Shards:        8,
				Namespace:     "perf-flutter",
			},
			IngestionConfig: IngestionConfig{
				SourceConfig: SourceConfig{
					SourceType: GCSSourceType,
					Project:    "skia-public",
					Topic:      "perf-ingestion-flutter",
					Sources:    []string{"gs://flutter-skia-perf/flutter-engine"},
				},
				Branches:               []string{},
				FileIngestionTopicName: "",
			},
			GitRepoConfig: GitRepoConfig{
				URL: "https://github.com/flutter/engine",
				Dir: "/tmp/repo",
			},
		},
	}
)

// Config is the currently running config.
var Config *InstanceConfig

// Init loads the selected config by name.
func Init(configName string) error {
	cfg, ok := PERF_BIGTABLE_CONFIGS[configName]
	if !ok {
		return fmt.Errorf("Invalid config name: %q", configName)
	}
	Config = cfg
	return nil
}
