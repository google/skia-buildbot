package attic

import (
	"encoding/gob"
	"fmt"

	"skia.googlesource.com/buildbot.git/perf/go/config"
	ptypes "skia.googlesource.com/buildbot.git/perf/go/types"
)

// Digest is the calculated hash for an image.
type Digest string

const MISSING_DIGEST = ""

// GoldenTrace represents all the Digests of a single golden test across a series of Commits.
// GoldenTrace implements perf/types.Trace.
type GoldenTrace struct {
	Params_ map[string]string
	Values  []Digest
}

func (g *GoldenTrace) Params() map[string]string {
	return g.Params_
}

func (g *GoldenTrace) Len() int {
	return len(g.Values)
}

func (g *GoldenTrace) IsMissing(i int) bool {
	return g.Values[i] == MISSING_DIGEST
}

func (g *GoldenTrace) DeepCopy() ptypes.Trace {
	n := len(g.Values)
	cp := &GoldenTrace{
		Values:  make([]Digest, n, n),
		Params_: make(map[string]string),
	}
	copy(cp.Values, g.Values)
	for k, v := range g.Params_ {
		cp.Params_[k] = v
	}
	return cp
}

func (g *GoldenTrace) Merge(next ptypes.Trace) ptypes.Trace {
	nextGold := next.(*GoldenTrace)
	n := len(g.Values) + len(nextGold.Values)
	n1 := len(g.Values)

	merged := NewGoldenTraceN(n)
	merged.Params_ = g.Params_
	for k, v := range nextGold.Params_ {
		merged.Params_[k] = v
	}
	for i, v := range g.Values {
		merged.Values[i] = v
	}
	for i, v := range nextGold.Values {
		merged.Values[n1+i] = v
	}
	return merged
}

func (g *GoldenTrace) Grow(n int, fill ptypes.FillType) {
	if n < len(g.Values) {
		panic(fmt.Sprintf("Grow must take a value (%d) larger than the current Trace size: %d", n, len(g.Values)))
	}
	delta := n - len(g.Values)
	newValues := make([]Digest, n)

	if fill == ptypes.FILL_AFTER {
		copy(newValues, g.Values)
		for i := 0; i < delta; i++ {
			newValues[i+len(g.Values)] = MISSING_DIGEST
		}
	} else {
		for i := 0; i < delta; i++ {
			newValues[i] = MISSING_DIGEST
		}
		copy(newValues[delta:], g.Values)
	}
	g.Values = newValues
}

// NewGoldenTrace allocates a new Trace set up for the given number of samples.
//
// The Trace Values are pre-filled in with the missing data sentinel since not
// all tests will be run on all commits.
func NewGoldenTrace() *GoldenTrace {
	return NewGoldenTraceN(config.TILE_SIZE)
}

// NewGoldenTraceN allocates a new Trace set up for the given number of samples.
//
// The Trace Values are pre-filled in with the missing data sentinel since not
// all tests will be run on all commits.
func NewGoldenTraceN(n int) *GoldenTrace {
	g := &GoldenTrace{
		Values:  make([]Digest, n, n),
		Params_: make(map[string]string),
	}
	for i, _ := range g.Values {
		g.Values[i] = MISSING_DIGEST
	}
	return g
}

func init() {
	gob.Register(&GoldenTrace{})
}
