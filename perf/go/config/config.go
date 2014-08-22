package config

import (
	"time"
)

// QuerySince holds the start time we have data since.
// Don't consider data before this time. May be due to schema changes, etc.
// Note that the limit is exclusive, this date does not contain good data.
type QuerySince time.Time

// Date returns QuerySince in the YearMonDay format.
func (b QuerySince) Date() string {
	return time.Time(b).Format("20060102")
}

// GitHashColumn returns QuerySince in the format of SQL table TIMESTAMP
// column.
func (b QuerySince) SqlTsColumn() string {
	return time.Time(b).Format("2006-01-02 15:04:05")
}

// Unix returns the unix timestamp.
func (b QuerySince) Unix() int64 {
	return time.Time(b).Unix()
}

func NewQuerySince(t time.Time) QuerySince {
	return QuerySince(t)
}

const (
	// TILE_SCALE The number of points to subsample when moving one level of scaling. I.e.
	// a tile at scale 1 will contain every 4th point of the tiles at scale 0.
	TILE_SCALE = 4

	// The number of samples per trace in a tile, i.e. the number of git hashes that have data
	// in a single tile.
	TILE_SIZE = 128

	// JSON doesn't support NaN or +/- Inf, so we need a valid float
	// to signal missing data that also has a compact JSON representation.
	MISSING_DATA_SENTINEL = 1e100

	// Limit the number of commits we hold in memory and do bulk analysis on.
	MAX_COMMITS_IN_MEMORY = 32

	// Limit the number of times the ingester tries to get a file before giving up.
	MAX_URI_GET_TRIES = 4

	// MAX_SAMPLE_TRACES_PER_CLUSTER  is the maximum number of traces stored in a
	// ClusterSummary.
	MAX_SAMPLE_TRACES_PER_CLUSTER = 5

	RECLUSTER_DURATION = 15 * time.Minute
)

type DatasetName string

const (
	DATASET_SKP   DatasetName = "skps"
	DATASET_MICRO DatasetName = "micro"
)

var (
	// TODO(jcgregorio) Make into a flag.
	BEGINNING_OF_TIME = QuerySince(time.Date(2014, time.June, 18, 0, 0, 0, 0, time.UTC))
)
