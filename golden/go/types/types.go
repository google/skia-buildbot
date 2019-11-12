package types

import (
	"encoding/gob"
	"fmt"

	"go.skia.org/infra/go/tiling"
)

func init() {
	// Register *GoldenTrace in gob so that it can be used as a
	// concrete type for Trace when writing and reading Tiles in gobs.
	// TODO(kjlubick) It does not appear we gob encode traces anymore.
	gob.Register(&GoldenTrace{})
}

const (
	// PRIMARY_KEY_FIELD is the field that uniquely identifies a key.
	PRIMARY_KEY_FIELD = "name"

	// CORPUS_FIELD is the field that contains the corpus identifier.
	CORPUS_FIELD = "source_type"

	// MAXIMUM_NAME_LENGTH is the maximum length in bytes a test name can be.
	MAXIMUM_NAME_LENGTH = 256
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

const (
	// No digest available.
	MISSING_DIGEST = Digest("")
)

// GoldenTrace represents all the Digests of a single test across a series
// of Commits. GoldenTrace implements the Trace interface.
type GoldenTrace struct {
	// The JSON keys are named this way to maintain backwards compatibility
	// with JSON already written to disk.
	Keys    map[string]string `json:"Params_"`
	Digests []Digest          `json:"Values"`

	// cache these values so as not to incur the non-zero map lookup cost (~15 ns) repeatedly.
	testName TestName
	corpus   string
}

// NewEmptyGoldenTrace allocates a new Trace set up for the given number of samples.
//
// The Trace Digests are pre-filled in with the missing data sentinel since not
// all tests will be run on all commits.
func NewEmptyGoldenTrace(n int, keys map[string]string) *GoldenTrace {
	g := &GoldenTrace{
		Digests: make([]Digest, n),
		Keys:    keys,

		// Prefetch these now, while we have the chance.
		testName: TestName(keys[PRIMARY_KEY_FIELD]),
		corpus:   keys[CORPUS_FIELD],
	}
	for i := range g.Digests {
		g.Digests[i] = MISSING_DIGEST
	}
	return g
}

// NewGoldenTrace creates a new GoldenTrace with the given data.
func NewGoldenTrace(digests []Digest, keys map[string]string) *GoldenTrace {
	return &GoldenTrace{
		Digests: digests,
		Keys:    keys,

		// Prefetch these now, while we have the chance.
		testName: TestName(keys[PRIMARY_KEY_FIELD]),
		corpus:   keys[CORPUS_FIELD],
	}
}

// Params implements the tiling.Trace interface.
func (g *GoldenTrace) Params() map[string]string {
	return g.Keys
}

// TestName is a helper for extracting just the test name for this
// trace, of which there should always be exactly one.
func (g *GoldenTrace) TestName() TestName {
	if g.testName == "" {
		g.testName = TestName(g.Keys[PRIMARY_KEY_FIELD])
	}
	return g.testName
}

// Corpus is a helper for extracting just the corpus key for this
// trace, of which there should always be exactly one.
func (g *GoldenTrace) Corpus() string {
	if g.corpus == "" {
		g.corpus = g.Keys[CORPUS_FIELD]
	}
	return g.corpus
}

// Len implements the tiling.Trace interface.
func (g *GoldenTrace) Len() int {
	return len(g.Digests)
}

// IsMissing implements the tiling.Trace interface.
func (g *GoldenTrace) IsMissing(i int) bool {
	return g.Digests[i] == MISSING_DIGEST
}

// DeepCopy implements the tiling.Trace interface.
func (g *GoldenTrace) DeepCopy() tiling.Trace {
	n := len(g.Digests)
	cp := &GoldenTrace{
		Digests: make([]Digest, n),
		Keys:    make(map[string]string),
	}
	copy(cp.Digests, g.Digests)
	for k, v := range g.Keys {
		cp.Keys[k] = v
	}
	return cp
}

// Merge implements the tiling.Trace interface.
func (g *GoldenTrace) Merge(next tiling.Trace) tiling.Trace {
	nextGold := next.(*GoldenTrace)
	n := len(g.Digests) + len(nextGold.Digests)
	n1 := len(g.Digests)

	merged := NewEmptyGoldenTrace(n, g.Keys)
	for k, v := range nextGold.Keys {
		merged.Keys[k] = v
	}
	copy(merged.Digests, g.Digests)

	for i, v := range nextGold.Digests {
		merged.Digests[n1+i] = v
	}
	return merged
}

// Grow implements the tiling.Trace interface.
func (g *GoldenTrace) Grow(n int, fill tiling.FillType) {
	if n < len(g.Digests) {
		panic(fmt.Sprintf("Grow must take a value (%d) larger than the current Trace size: %d", n, len(g.Digests)))
	}
	delta := n - len(g.Digests)
	newDigests := make([]Digest, n)

	if fill == tiling.FILL_AFTER {
		copy(newDigests, g.Digests)
		for i := 0; i < delta; i++ {
			newDigests[i+len(g.Digests)] = MISSING_DIGEST
		}
	} else {
		for i := 0; i < delta; i++ {
			newDigests[i] = MISSING_DIGEST
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
	newDigests := make([]Digest, n)

	for i := 0; i < n; i++ {
		newDigests[i] = g.Digests[i+begin]
	}
	g.Digests = newDigests
	return nil
}

// SetAt implements the tiling.Trace interface.
func (g *GoldenTrace) SetAt(index int, value []byte) error {
	if index < 0 || index > len(g.Digests) {
		return fmt.Errorf("Invalid index: %d", index)
	}
	g.Digests[index] = Digest(value)
	return nil
}

// LastDigest returns the last digest in the trace (HEAD) or the empty string otherwise.
func (g *GoldenTrace) LastDigest() Digest {
	if idx := g.LastIndex(); idx >= 0 {
		return g.Digests[idx]
	}
	return MISSING_DIGEST
}

// LastIndex returns the index of last non-empty value in this trace and -1 if
// if the entire trace is empty.
func (g *GoldenTrace) LastIndex() int {
	for i := len(g.Digests) - 1; i >= 0; i-- {
		if g.Digests[i] != MISSING_DIGEST {
			return i
		}
	}
	return -1
}

// String prints a human friendly version of this trace.
func (g *GoldenTrace) String() string {
	return fmt.Sprintf("Keys: %#v, Digests: %q", g.Keys, g.Digests)
}
