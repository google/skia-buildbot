package types

const (
	// PrimaryKeyField is the field that uniquely identifies a key.
	PrimaryKeyField = "name"

	// CorpusField is the field that contains the corpus identifier.
	CorpusField = "source_type"

	// MaximumNameLength is the maximum length in bytes a test name can be.
	MaximumNameLength = 256
)

// Strings are used a lot, so these type "aliases" can help document
// which are meant where. See also tiling.TraceID
// Of note, Digest exclusively means a unique image, identified by
// the MD5 hash of its pixels.
type Digest string
type TestName string

// The IgnoreState enum gives a human-readable way to determine if the
// tile or whatever is dealing with the full amount of information
// (IncludeIgnoredTraces) or the information with the ignore rules applied
// (ExcludeIgnoredTraces).
type IgnoreState int

const (
	ExcludeIgnoredTraces IgnoreState = iota
	IncludeIgnoredTraces             // i.e. all digests
)

var IgnoreStates = []IgnoreState{ExcludeIgnoredTraces, IncludeIgnoredTraces}
