package dsconst

import "go.skia.org/infra/go/ds"

// One const for each Datastore Kind.
const (
	// FAILURES are the failures gathered from swarming.
	FAILURES ds.Kind = "Failures"

	// FLAKY_RANGES are the time ranges each bot was flagged as flaky.
	FLAKY_RANGES ds.Kind = "FlakyRanges"
)
