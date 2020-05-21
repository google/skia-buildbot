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
	// The JSON keys are named this way to maintain backwards compatibility
	// with JSON already written to disk.
	Keys    map[string]string `json:"Params_"`
	Digests []types.Digest    `json:"Values"`

	// cache these values so as not to incur the non-zero map lookup cost (~15 ns) repeatedly.
	testName types.TestName
	corpus   string
}

// NewEmptyTrace allocates a new Trace set up for the given number of samples.
//
// The Trace Digests are pre-filled in with the missing data sentinel since not
// all tests will be run on all commits.
func NewEmptyTrace(numDigests int, keys map[string]string) *Trace {
	g := &Trace{
		Digests: make([]types.Digest, numDigests),
		Keys:    keys,

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
func NewTrace(digests []types.Digest, keys map[string]string) *Trace {
	return &Trace{
		Digests: digests,
		Keys:    keys,

		// Prefetch these now, while we have the chance.
		testName: types.TestName(keys[types.PrimaryKeyField]),
		corpus:   keys[types.CorpusField],
	}
}

// Params implements the tiling.Trace interface.
func (g *Trace) Params() map[string]string {
	return g.Keys
}

// Matches returns true if the given Trace matches the given query.
func (g *Trace) Matches(query paramtools.ParamSet) bool {
	for k, values := range query {
		if p, ok := g.Params()[k]; !ok || !util.In(p, values) {
			return false
		}
	}
	return true
}

// TestName is a helper for extracting just the test name for this
// trace, of which there should always be exactly one.
func (g *Trace) TestName() types.TestName {
	if g.testName == "" {
		g.testName = types.TestName(g.Keys[types.PrimaryKeyField])
	}
	return g.testName
}

// Corpus is a helper for extracting just the corpus key for this
// trace, of which there should always be exactly one.
func (g *Trace) Corpus() string {
	if g.corpus == "" {
		g.corpus = g.Keys[types.CorpusField]
	}
	return g.corpus
}

// Len implements the tiling.Trace interface.
func (g *Trace) Len() int {
	return len(g.Digests)
}

// IsMissing implements the tiling.Trace interface.
func (g *Trace) IsMissing(i int) bool {
	return g.Digests[i] == MissingDigest
}

// DeepCopy implements the tiling.Trace interface.
func (g *Trace) DeepCopy() *Trace {
	nd := make([]types.Digest, len(g.Digests))
	copy(nd, g.Digests)
	nk := make(map[string]string, len(g.Keys))
	for k, v := range g.Keys {
		nk[k] = v
	}
	return NewTrace(nd, nk)
}

// Merge implements the tiling.Trace interface.
func (g *Trace) Merge(next *Trace) *Trace {
	n := len(g.Digests) + len(next.Digests)
	n1 := len(g.Digests)

	merged := NewEmptyTrace(n, g.Keys)
	for k, v := range next.Keys {
		merged.Keys[k] = v
	}
	copy(merged.Digests, g.Digests)

	for i, v := range next.Digests {
		merged.Digests[n1+i] = v
	}
	return merged
}

// FillType is how filling in of missing values should be done in Trace.Grow().
type FillType int

const (
	FillBefore FillType = iota
	FillAfter
)

// Grow implements the tiling.Trace interface.
func (g *Trace) Grow(n int, fill FillType) {
	if n < len(g.Digests) {
		panic(fmt.Sprintf("Grow must take a value (%d) larger than the current Trace size: %d", n, len(g.Digests)))
	}
	delta := n - len(g.Digests)
	newDigests := make([]types.Digest, n)

	if fill == FillAfter {
		copy(newDigests, g.Digests)
		for i := 0; i < delta; i++ {
			newDigests[i+len(g.Digests)] = MissingDigest
		}
	} else {
		for i := 0; i < delta; i++ {
			newDigests[i] = MissingDigest
		}
		copy(newDigests[delta:], g.Digests)
	}
	g.Digests = newDigests
}

// Trim implements the tiling.Trace interface.
func (g *Trace) Trim(begin, end int) error {
	if end < begin || end > g.Len() || begin < 0 {
		return fmt.Errorf("Invalid Trim range [%d, %d) of [0, %d]", begin, end, g.Len())
	}
	n := end - begin
	newDigests := make([]types.Digest, n)

	for i := 0; i < n; i++ {
		newDigests[i] = g.Digests[i+begin]
	}
	g.Digests = newDigests
	return nil
}

// AtHead returns the last digest in the trace (HEAD) or the empty string otherwise.
func (g *Trace) AtHead() types.Digest {
	if idx := g.LastIndex(); idx >= 0 {
		return g.Digests[idx]
	}
	return MissingDigest
}

// LastIndex returns the index of last non-empty value in this trace and -1 if
// if the entire trace is empty.
func (g *Trace) LastIndex() int {
	for i := len(g.Digests) - 1; i >= 0; i-- {
		if g.Digests[i] != MissingDigest {
			return i
		}
	}
	return -1
}

// String prints a human friendly version of this trace.
func (g *Trace) String() string {
	return fmt.Sprintf("Keys: %#v, Digests: %q", g.Keys, g.Digests)
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
