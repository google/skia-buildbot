package tiling

import (
	"fmt"
	"strings"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
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

// NewTrace creates a new Trace with the given data.
func NewTrace(digests []types.Digest, keys, options map[string]string) *Trace {
	return &Trace{
		Digests: digests,
		keys:    keys,
		options: options,

		// Prefetch these now, while we have the chance.
		testName: types.TestName(keys[types.PrimaryKeyField]),
		corpus:   keys[types.CorpusField],
	}
}

// Keys returns the key value pairs associated with this trace.
func (t *Trace) Keys() map[string]string {
	return t.keys
}

// Options returns the optional key value pairs associated with this trace (those that don't affect
// the trace id).
func (t *Trace) Options() map[string]string {
	return t.options
}

// KeysAndOptions returns a combined copy of Keys() and Options().
func (t *Trace) KeysAndOptions() map[string]string {
	m := make(map[string]string, len(t.keys)+len(t.options))
	for k, v := range t.keys {
		m[k] = v
	}
	for k, v := range t.options {
		m[k] = v
	}
	return m
}

// Matches returns true if this trace matches the given ParamSet. To match, the trace must have
// each of the keys defined in the ParamSet in either the Keys() or the Options() and the associated
// value for each of those keys must be in the values from the ParamSet.
func (t *Trace) Matches(ps paramtools.ParamSet) bool {
	for k, values := range ps {
		if p, ok := t.keys[k]; ok && util.In(p, values) {
			continue
		}
		if len(t.options) > 0 {
			if p, ok := t.options[k]; ok && util.In(p, values) {
				continue
			}
		}
		// Not in keys nor options.
		return false
	}
	return true
}

// TestName is a helper for extracting just the test name for this
// trace, of which there should always be exactly one.
func (t *Trace) TestName() types.TestName {
	if t.testName == "" {
		t.testName = types.TestName(t.keys[types.PrimaryKeyField])
	}
	return t.testName
}

// Corpus is a helper for extracting just the corpus key for this
// trace, of which there should always be exactly one.
func (t *Trace) Corpus() string {
	if t.corpus == "" {
		t.corpus = t.keys[types.CorpusField]
	}
	return t.corpus
}

// Len returns how many digests are in this trace.
func (t *Trace) Len() int {
	return len(t.Digests)
}

// IsMissing implements the tiling.Trace interface.
func (t *Trace) IsMissing(i int) bool {
	return t.Digests[i] == MissingDigest
}

// Merge returns a new Trace that has the digests joined together and the keys and options
// combined (with the other Trace's values overriding this trace's values in the event of a
// collision).
func (t *Trace) Merge(other *Trace) *Trace {
	n := len(t.Digests) + len(other.Digests)

	merged := NewEmptyTrace(n, copyStringMap(t.keys), copyStringMap(t.options))
	for k, v := range other.keys {
		merged.keys[k] = v
	}
	for k, v := range other.options {
		merged.options[k] = v
	}
	// Combine the digests
	copy(merged.Digests, t.Digests)
	for i, v := range other.Digests {
		merged.Digests[len(t.Digests)+i] = v
	}
	return merged
}

// copyStringMap returns a copy of the given string map.
func copyStringMap(m map[string]string) map[string]string {
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// FillType is how filling in of missing values should be done in Trace.Grow().
type FillType int

const (
	FillBefore FillType = iota
	FillAfter
)

// Grow implements the tiling.Trace interface.
func (t *Trace) Grow(n int, fill FillType) {
	if n < len(t.Digests) {
		panic(fmt.Sprintf("Grow must take a value (%d) larger than the current Trace size: %d", n, len(t.Digests)))
	}
	delta := n - len(t.Digests)
	newDigests := make([]types.Digest, n)

	if fill == FillAfter {
		copy(newDigests, t.Digests)
		for i := 0; i < delta; i++ {
			newDigests[i+len(t.Digests)] = MissingDigest
		}
	} else {
		for i := 0; i < delta; i++ {
			newDigests[i] = MissingDigest
		}
		copy(newDigests[delta:], t.Digests)
	}
	t.Digests = newDigests
}

// AtHead returns the last digest in the trace (HEAD) or the empty string otherwise.
func (t *Trace) AtHead() types.Digest {
	if idx := t.LastIndex(); idx >= 0 {
		return t.Digests[idx]
	}
	return MissingDigest
}

// LastIndex returns the index of last non-empty value in this trace and -1 if
// if the entire trace is empty.
func (t *Trace) LastIndex() int {
	for i := len(t.Digests) - 1; i >= 0; i-- {
		if t.Digests[i] != MissingDigest {
			return i
		}
	}
	return -1
}

// String prints a human friendly version of this trace.
func (t *Trace) String() string {
	return fmt.Sprintf("Keys: %#v, Options: %#v, Digests: %q", t.keys, t.options, t.Digests)
}

// TraceIDFromParams deterministically returns a TraceID that uniquely encodes
// the given params. It follows the same convention as perf's trace ids, that
// is something like ",key1=value1,key2=value2,...," where the keys
// are in alphabetical order.
func TraceIDFromParams(params paramtools.Params) TraceID {
	// Clean up any params with , or =
	params = forceValid(params)
	s, err := query.MakeKeyFast(params)
	if err != nil {
		// this should never happen given that forceValid fixes them up.
		sklog.Warningf("Invalid params passed in for trace id %#v: %s", params, err)
		return "invalid_trace_id"
	}
	return TraceID(s)
}

// clean replaces any special runes (',', '=') in a string such that
// they can be turned into a trace id, which uses those special runes
// as dividers.
func clean(s string) string {
	// In most cases, traces will be valid, so check that first.
	// Allocating the string buffer and copying the runes can be expensive
	// when done for no reason.
	bad := false
	for _, c := range s {
		if c == ',' || c == '=' {
			bad = true
			break
		}
	}
	if !bad {
		return s
	}
	sb := strings.Builder{}
	sb.Grow(len(s))
	// Regexp doesn't handle being run from a large number of go routines
	// very well. See https://github.com/golang/go/issues/8232.
	for _, c := range s {
		if c == ',' || c == '=' {
			sb.WriteRune('_')
		} else {
			sb.WriteRune(c)
		}
	}
	return sb.String()
}

// forceValid ensures that the resulting map will make a valid structured key.
func forceValid(m map[string]string) map[string]string {
	ret := make(map[string]string, len(m))
	for key, value := range m {
		ret[clean(key)] = clean(value)
	}

	return ret
}
