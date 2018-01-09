package dsconst

import "go.skia.org/infra/go/ds"

// One const for each Datastore Kind.
const (
	FAILURES     ds.Kind = "Failures"
	FLAKY_RANGES ds.Kind = "FlakyRanges"
)
