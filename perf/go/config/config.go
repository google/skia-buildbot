package config

import (
	"fmt"
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

// DataStoreConfig is the configuration for how Perf stores data.
type DataStoreConfig struct {
	// TileSize is the size of each tile in commits.
	TileSize int32 `json:"tile_size"`

	// Project is the Google Cloud Project name.
	Project string `json:"project"`

	// Instance is the name of the BigTable instance.
	Instance string `json:"instance"`

	// Table is the name of the table in BigTable to use.
	Table string `json:"table"`

	// Shards is the number of shards to break up all trace data into.
	Shards int32 `json:"shards"`
}

// SourceConfig is the config for where ingestable files come from.
type SourceConfig struct {
	// Project is the Google Cloud Project name.
	Project string `json:"project"`

	// Topic is the PubSub topic when new files arrive to be ingested.
	Topic string `json:"topic"`

	// Sources is the list of sources of data files, i.e. gs:// locations.
	Sources []string `json:"sources"`
}

// IngestionConfig is the configuration for how source files are ingested into
// being traces in a TraceStore.
type IngestionConfig struct {
	// SourceConfig is the config for where files to ingest come from.
	SourceConfig SourceConfig

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
				TileSize: 256,
				Project:  "skia-public",
				Instance: "production",
				Table:    "perf-skia",
				Shards:   8,
			},
			IngestionConfig: IngestionConfig{
				SourceConfig: SourceConfig{
					Project: "skia-public",
					Topic:   "perf-ingestion-skia-production",
					Sources: []string{"gs://skia-perf/nano-json-v1", "gs://skia-perf/task-duration", "gs://skia-perf/buildstats-json-v1"},
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
				TileSize: 8192,
				Project:  "skia-public",
				Instance: "production",
				Table:    "perf-android",
				Shards:   8,
			},
			IngestionConfig: IngestionConfig{
				SourceConfig: SourceConfig{
					Project: "skia-public",
					Topic:   "perf-ingestion-android-production",
					Sources: []string{"gs://skia-perf/android-master-ingest"},
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
				TileSize: 256,
				Project:  "skia-public",
				Instance: "production",
				Table:    "perf-ct",
				Shards:   8,
			},
			IngestionConfig: IngestionConfig{
				SourceConfig: SourceConfig{
					Project: "skia-public",
					Topic:   "perf-ingestion-ct-production",
					Sources: []string{"gs://cluster-telemetry-perf/ingest"},
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
				TileSize: 512,
				Project:  "skia-public",
				Instance: "production",
				Table:    "perf-android-x",
				Shards:   8,
			},
			IngestionConfig: IngestionConfig{
				SourceConfig: SourceConfig{
					Project: "skia-public",
					Topic:   "perf-ingestion-android-x-production",
					Sources: []string{"gs://skia-perf/android-master-ingest"},
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
				TileSize: 256,
				Project:  "skia-public",
				Instance: "production",
				Table:    "perf-flutter",
				Shards:   8,
			},
			IngestionConfig: IngestionConfig{
				SourceConfig: SourceConfig{
					Project: "skia-public",
					Topic:   "perf-ingestion-flutter",
					Sources: []string{"gs://flutter-skia-perf/flutter-engine"},
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
