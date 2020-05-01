package tiling

import (
	"fmt"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

// GoldenTrace represents all the Digests of a single test across a series
// of Commits. GoldenTrace implements the Trace interface.
type GoldenTrace struct {
	// The JSON keys are named this way to maintain backwards compatibility
	// with JSON already written to disk.
	Keys    map[string]string `json:"Params_"`
	Digests []types.Digest    `json:"Values"`

	// cache these values so as not to incur the non-zero map lookup cost (~15 ns) repeatedly.
	testName types.TestName
	corpus   string
}

// NewEmptyGoldenTrace allocates a new Trace set up for the given number of samples.
//
// The Trace Digests are pre-filled in with the missing data sentinel since not
// all tests will be run on all commits.
func NewEmptyGoldenTrace(n int, keys map[string]string) *GoldenTrace {
	g := &GoldenTrace{
		Digests: make([]types.Digest, n),
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

// NewGoldenTrace creates a new GoldenTrace with the given data.
func NewGoldenTrace(digests []types.Digest, keys map[string]string) *GoldenTrace {
	return &GoldenTrace{
		Digests: digests,
		Keys:    keys,

		// Prefetch these now, while we have the chance.
		testName: types.TestName(keys[types.PrimaryKeyField]),
		corpus:   keys[types.CorpusField],
	}
}

// Params implements the tiling.Trace interface.
func (g *GoldenTrace) Params() map[string]string {
	return g.Keys
}

// Matches returns true if the given Trace matches the given query.
func (g *GoldenTrace) Matches(query paramtools.ParamSet) bool {
	for k, values := range query {
		if p, ok := g.Params()[k]; !ok || !util.In(p, values) {
			return false
		}
	}
	return true
}

// TestName is a helper for extracting just the test name for this
// trace, of which there should always be exactly one.
func (g *GoldenTrace) TestName() types.TestName {
	if g.testName == "" {
		g.testName = types.TestName(g.Keys[types.PrimaryKeyField])
	}
	return g.testName
}

// Corpus is a helper for extracting just the corpus key for this
// trace, of which there should always be exactly one.
func (g *GoldenTrace) Corpus() string {
	if g.corpus == "" {
		g.corpus = g.Keys[types.CorpusField]
	}
	return g.corpus
}

// Len implements the tiling.Trace interface.
func (g *GoldenTrace) Len() int {
	return len(g.Digests)
}

// IsMissing implements the tiling.Trace interface.
func (g *GoldenTrace) IsMissing(i int) bool {
	return g.Digests[i] == MissingDigest
}

// DeepCopy implements the tiling.Trace interface.
func (g *GoldenTrace) DeepCopy() *GoldenTrace {
	nd := make([]types.Digest, len(g.Digests))
	copy(nd, g.Digests)
	nk := make(map[string]string, len(g.Keys))
	for k, v := range g.Keys {
		nk[k] = v
	}
	return NewGoldenTrace(nd, nk)
}

// Merge implements the tiling.Trace interface.
func (g *GoldenTrace) Merge(next *GoldenTrace) *GoldenTrace {
	n := len(g.Digests) + len(next.Digests)
	n1 := len(g.Digests)

	merged := NewEmptyGoldenTrace(n, g.Keys)
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
func (g *GoldenTrace) Grow(n int, fill FillType) {
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
func (g *GoldenTrace) Trim(begin, end int) error {
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
func (g *GoldenTrace) AtHead() types.Digest {
	if idx := g.LastIndex(); idx >= 0 {
		return g.Digests[idx]
	}
	return MissingDigest
}

// LastIndex returns the index of last non-empty value in this trace and -1 if
// if the entire trace is empty.
func (g *GoldenTrace) LastIndex() int {
	for i := len(g.Digests) - 1; i >= 0; i-- {
		if g.Digests[i] != MissingDigest {
			return i
		}
	}
	return -1
}

// String prints a human friendly version of this trace.
func (g *GoldenTrace) String() string {
	return fmt.Sprintf("Keys: %#v, Digests: %q", g.Keys, g.Digests)
}
