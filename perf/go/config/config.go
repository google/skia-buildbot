package config

import "fmt"

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
	TileSize int32
	Project  string
	Instance string
	Table    string
	Topic    string
	Shards   int32
}

// IngestionConfig is the configuration for how source files are ingested into
// being traces in a TraceStore.
type IngestionConfig struct {

	// Sources is the list of sources of data files, i.e. gs:// locations.
	Sources []string

	// Branches, if populated then restrict to ingesting just these branches.
	Branches []string

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
	FileIngestionTopicName string
}

type GitRepoConfig struct {
	// GitUrl is the URL the Git repo is fetched from.
	GitUrl string

	// DebouceCommitURL signals if a link to a Git commit needs to be specially
	// dereferenced. That is, some repos are synthetic and just contain a single
	// file that changes, with a commit message that is a URL that points to the
	// true source of information. If this value is true then links to commits
	// need to be debounced and use the commit message instead.
	DebouceCommitURL bool
}

// InstanceConfig contains all the info needed by btts.BigTableTraceStore.
//
// May eventually move to a separate config file.
type InstanceConfig struct {
	DataStoreConfig DataStoreConfig
	IngestionConfig IngestionConfig
	GitRepoConfig   GitRepoConfig
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
			DataStoreConfig: DataStoreConfig{
				TileSize: 256,
				Project:  "skia-public",
				Instance: "production",
				Table:    "perf-skia",
				Topic:    "perf-ingestion-skia-production",
				Shards:   8,
			},
			IngestionConfig: IngestionConfig{
				Sources:                []string{"gs://skia-perf/nano-json-v1", "gs://skia-perf/task-duration", "gs://skia-perf/buildstats-json-v1"},
				Branches:               []string{},
				FileIngestionTopicName: "",
			},
			GitRepoConfig: GitRepoConfig{
				GitUrl: "https://skia.googlesource.com/skia",
			},
		},
		ANDROID_PROD: {
			DataStoreConfig: DataStoreConfig{
				TileSize: 8192,
				Project:  "skia-public",
				Instance: "production",
				Table:    "perf-android",
				Topic:    "perf-ingestion-android-production",
				Shards:   8,
			},
			IngestionConfig: IngestionConfig{
				Sources:                []string{"gs://skia-perf/android-master-ingest"},
				Branches:               []string{},
				FileIngestionTopicName: "perf-ingestion-complete-android-production",
			},
			GitRepoConfig: GitRepoConfig{
				GitUrl:           "https://skia.googlesource.com/perf-buildid/android-master",
				DebouceCommitURL: true,
			},
		},
		CT_PROD: {
			DataStoreConfig: DataStoreConfig{
				TileSize: 256,
				Project:  "skia-public",
				Instance: "production",
				Table:    "perf-ct",
				Topic:    "perf-ingestion-ct-production",
				Shards:   8,
			},
			IngestionConfig: IngestionConfig{
				Sources:                []string{"gs://cluster-telemetry-perf/ingest"},
				Branches:               []string{},
				FileIngestionTopicName: "",
			},
			GitRepoConfig: GitRepoConfig{
				GitUrl: "https://skia.googlesource.com/perf-ct",
			},
		},
		ANDROID_X: { // https://bug.skia.org/9315
			DataStoreConfig: DataStoreConfig{
				TileSize: 512,
				Project:  "skia-public",
				Instance: "production",
				Table:    "perf-android-x",
				Topic:    "perf-ingestion-android-x-production",
				Shards:   8,
			},
			IngestionConfig: IngestionConfig{
				Sources:                []string{"gs://skia-perf/android-master-ingest"},
				Branches:               []string{"aosp-androidx-master-dev"},
				FileIngestionTopicName: "",
			},
			GitRepoConfig: GitRepoConfig{
				GitUrl:           "https://skia.googlesource.com/perf-buildid/android-master",
				DebouceCommitURL: true,
			},
		},
		FLUTTER: {
			DataStoreConfig: DataStoreConfig{
				TileSize: 256,
				Project:  "skia-public",
				Instance: "production",
				Table:    "perf-flutter",
				Topic:    "perf-ingestion-flutter",
				Shards:   8,
			},
			IngestionConfig: IngestionConfig{
				Sources:                []string{"gs://flutter-skia-perf/flutter-engine"},
				Branches:               []string{},
				FileIngestionTopicName: "",
			},
			GitRepoConfig: GitRepoConfig{
				GitUrl: "https://github.com/flutter/engine",
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
