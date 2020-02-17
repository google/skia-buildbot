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

// InstanceConfig contains all the info needed by btts.BigTableTraceStore.
//
// May eventually move to a separate config file.
type InstanceConfig struct {
	TileSize int32
	Project  string
	Instance string
	Table    string
	Topic    string
	GitUrl   string
	Shards   int32
	Sources  []string // List of gs: locations.
	Branches []string // If populated then restrict to ingesting just these branches.

	// Some repos are synthetic and just contain a single file that changes,
	// with a commit message that is a URL that points to the true source of
	// information. If this value is true then links to commits need to be
	// debounced and use the commit message instead.
	DebouceCommitURL bool

	// FileIngestionTopicName is the PubSub topic name we should use if doing
	// event driven regression detection. The ingesters use this to know where
	// to emit events to, and the clusterers use this to know where to make a
	// subscription.
	//
	// Should only be turned on for instances that have a huge amount of data,
	// i.e. >500k traces, and that have sparse data.
	FileIngestionTopicName string
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
			TileSize:               256,
			Project:                "skia-public",
			Instance:               "production",
			Table:                  "perf-skia",
			Topic:                  "perf-ingestion-skia-production",
			GitUrl:                 "https://skia.googlesource.com/skia",
			Shards:                 8,
			Sources:                []string{"gs://skia-perf/nano-json-v1", "gs://skia-perf/task-duration", "gs://skia-perf/buildstats-json-v1"},
			Branches:               []string{},
			FileIngestionTopicName: "",
		},
		ANDROID_PROD: {
			TileSize:               8192,
			Project:                "skia-public",
			Instance:               "production",
			Table:                  "perf-android",
			Topic:                  "perf-ingestion-android-production",
			GitUrl:                 "https://skia.googlesource.com/perf-buildid/android-master",
			Shards:                 8,
			Sources:                []string{"gs://skia-perf/android-master-ingest"},
			Branches:               []string{},
			DebouceCommitURL:       true,
			FileIngestionTopicName: "perf-ingestion-complete-android-production",
		},
		CT_PROD: {
			TileSize:               256,
			Project:                "skia-public",
			Instance:               "production",
			Table:                  "perf-ct",
			Topic:                  "perf-ingestion-ct-production",
			GitUrl:                 "https://skia.googlesource.com/perf-ct",
			Shards:                 8,
			Sources:                []string{"gs://cluster-telemetry-perf/ingest"},
			Branches:               []string{},
			FileIngestionTopicName: "",
		},
		ANDROID_X: { // https://bug.skia.org/9315
			TileSize:               512,
			Project:                "skia-public",
			Instance:               "production",
			Table:                  "perf-android-x",
			Topic:                  "perf-ingestion-android-x-production",
			GitUrl:                 "https://skia.googlesource.com/perf-buildid/android-master",
			Shards:                 8,
			Sources:                []string{"gs://skia-perf/android-master-ingest"},
			Branches:               []string{"aosp-androidx-master-dev"},
			DebouceCommitURL:       true,
			FileIngestionTopicName: "",
		},
		FLUTTER: {
			TileSize:               256,
			Project:                "skia-public",
			Instance:               "production",
			Table:                  "perf-flutter",
			Topic:                  "perf-ingestion-flutter",
			GitUrl:                 "https://github.com/flutter/engine",
			Shards:                 8,
			Sources:                []string{"gs://flutter-skia-perf/flutter-engine"},
			Branches:               []string{},
			FileIngestionTopicName: "",
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
