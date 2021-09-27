package tiling

import (
	"go.skia.org/infra/golden/go/types"
)

// Trace represents all the Digests of a single test across a series
// of Commits.
type Trace struct {
	// Digests represents the images seen over the last N commits. Index 0 is the oldest data, index
	// len-1 is the newest data.
	Digests []types.Digest
	// keys describe how the digest was produced. These keys and values contribute to the trace id
	// (that is, they contribute to uniqueness).
	keys map[string]string
	// options describe other parameters. These do not contribute to the trace id, but are searchable.
	// It is strongly recommended to not have the same string be a key in both keys and options.
	// Doing so could lead to unspecified behavior.
	options map[string]string
	// OptionsID is the md5 hash of the options map.
	OptionsID []byte

	// cache these values so as not to incur the non-zero map lookup cost (~15 ns) repeatedly.
	testName types.TestName
	corpus   string
}

// NewEmptyTrace allocates a new Trace set up for the given number of samples.
//
// The Trace Digests are pre-filled in with the missing data sentinel since not
// all tests will be run on all commits.
func NewEmptyTrace(numDigests int, keys, options map[string]string) *Trace {
	g := &Trace{
		Digests: make([]types.Digest, numDigests),
		keys:    keys,
		options: options,

		// Prefetch these now, while we have the chance.
		testName: types.TestName(keys[types.PrimaryKeyField]),
		corpus:   keys[types.CorpusField],
	}
	for i := range g.Digests {
		g.Digests[i] = MissingDigest
	}
	return g
}

// TraceID helps document when strings should represent ids of traces
// This form of id is a comma separated listed of key-value pairs.
type TraceID string

// TraceIDV2 helps document when strings should represent ids of traces
// This form of is is a hex-encoded MD5 hash. The hash is of a JSON representation of the keys.
type TraceIDV2 string

const (
	// MissingDigest is a sentinel value meaning no digest is available at the given commit.
	MissingDigest = types.Digest("")
)
