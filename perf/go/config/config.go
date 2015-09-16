package config

import (
	"time"

	"go.skia.org/infra/go/tiling"
)

const (
	// JSON doesn't support NaN or +/- Inf, so we need a valid float
	// to signal missing data that also has a compact JSON representation.
	MISSING_DATA_SENTINEL = 1e100

	// Limit the number of commits we hold in memory and do bulk analysis on.
	MAX_COMMITS_IN_MEMORY = 32

	// MAX_SAMPLE_TRACES_PER_CLUSTER  is the maximum number of traces stored in a
	// ClusterSummary.
	MAX_SAMPLE_TRACES_PER_CLUSTER = 1

	RECLUSTER_DURATION = 5 * time.Minute

	// CLUSTER_COMMITS is the number of commits to use when clustering.
	MAX_CLUSTER_COMMITS = tiling.TILE_SIZE

	// MIN_CLUSTER_STEP_COMMITS is minimum number of commits that we need on either leg
	// of a step function.
	MIN_CLUSTER_STEP_COMMITS = 5

	// MIN_STDDEV is the smallest standard deviation we will normalize, smaller
	// than this and we presume it's a standard deviation of zero.
	MIN_STDDEV = 0.001
)

const (
	// Different datasets that are stored in tiles.
	DATASET_NANO = "nano"

	// Constructor names that are used to instantiate an ingester.
	// Note that, e.g. 'android-gold' has a different ingester, but writes
	// to the gold dataset.
	CONSTRUCTOR_NANO        = DATASET_NANO
	CONSTRUCTOR_NANO_TRYBOT = "nano-trybot"
)
