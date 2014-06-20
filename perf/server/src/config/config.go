package config

import "time"

const (
	// JSON doesn't support NaN or +/- Inf, so we need a valid float
	// to signal missing data that also has a compact JSON representation.
	MISSING_DATA_SENTINEL = 1e100

	// Don't consider data before this time. May be due to schema changes, etc.
	// Note that the limit is exclusive, this date does not contain good data.
	// TODO(jcgregorio) Make into a flag.
	BEGINNING_OF_TIME = "20140614"

	// Limit the number of commits we hold in memory and do bulk analysis on.
	MAX_COMMITS_IN_MEMORY = 30

	// How often data is refreshed from BigQuery.
	// TODO(jcgregorio) Move to push once it's feasible.
	REFRESH_PERIOD = time.Minute * 30
)
