package config

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

// PerfBigTableConfig contains all the info needed by btts.BigTableTraceStore.
//
// May eventually move to a separate config file.
type PerfBigTableConfig struct {
	TileSize     int32
	Project      string
	Instance     string
	Table        string
	Topic        string
	GitUrl       string
	Subscription string
	Shards       int32
}

const (
	NANO = "nano"
)

var (
	PERF_BIGTABLE_CONFIGS = map[string]*PerfBigTableConfig{
		NANO: &PerfBigTableConfig{
			TileSize:     50,
			Project:      "skia-public",
			Instance:     "perf-bt",
			Table:        "skia",
			Topic:        "perf-ingestion-skia",
			GitUrl:       "https://skia.googlesource.com/skia",
			Subscription: "perf-ingestion-skia",
			Shards:       8,
		},
	}
)
