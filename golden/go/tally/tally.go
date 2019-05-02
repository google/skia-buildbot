// tally returns counts of digests for various views on a Tile.
package tally

import (
	"net/url"

	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

// Tally maps a digest to a count.
type Tally map[string]int

// Tallies allows querying for digest counts in different ways.
// It is not thread safe. The client of this package needs to make sure there
// are no conflicts.
type Tallies struct {
	traceTally      map[string]Tally
	testTally       map[string]Tally
	maxCountsByTest map[string]util.StringSet
}

// New creates a new Tallies object.
func New() *Tallies {
	return &Tallies{}
}

// Calculate sets the tallies for the given tile.
func (t *Tallies) Calculate(tile *tiling.Tile) {
	trace, test, maxCountsByTest := tallyTile(tile)
	t.traceTally = trace
	t.testTally = test
	t.maxCountsByTest = maxCountsByTest
}

// ByTest returns Tally's indexed by test name.
func (t *Tallies) ByTest() map[string]Tally {
	return t.testTally
}

func (t *Tallies) MaxDigestsByTest() map[string]util.StringSet {
	return t.maxCountsByTest
}

// ByTrace returns Tally's index by trace id.
func (t *Tallies) ByTrace() map[string]Tally {
	return t.traceTally
}

// ByQuery returns a Tally of all the digests that match the given query in
// the provided tile.
func (t *Tallies) ByQuery(tile *tiling.Tile, query url.Values) Tally {
	return tallyBy(tile, t.traceTally, query)
}

// tallyBy does the actual work of ByQuery.
func tallyBy(tile *tiling.Tile, traceTally map[string]Tally, query url.Values) Tally {
	ret := Tally{}
	for k, tr := range tile.Traces {
		if tiling.Matches(tr, query) {
			if _, ok := traceTally[k]; !ok {
				continue
			}
			for digest, n := range traceTally[k] {
				if _, ok := ret[digest]; ok {
					ret[digest] += n
				} else {
					ret[digest] = n
				}
			}
		}
	}
	return ret
}

// tallyTile computes a map[tracename]Tally and map[testname]Tally from the given Tile.
func tallyTile(tile *tiling.Tile) (map[string]Tally, map[string]Tally, map[string]util.StringSet) {
	defer timer.New("tally").Stop()
	traceTally := map[string]Tally{}
	testTally := map[string]Tally{}
	for k, tr := range tile.Traces {
		gtr := tr.(*types.GoldenTrace)
		tally := Tally{}
		for _, s := range gtr.Digests {
			if s == types.MISSING_DIGEST {
				continue
			}
			if n, ok := tally[s]; ok {
				tally[s] = n + 1
			} else {
				tally[s] = 1
			}
		}
		traceTally[k] = tally
		testName := tr.Params()[types.PRIMARY_KEY_FIELD]
		if t, ok := testTally[testName]; ok {
			for digest, n := range tally {
				if _, ok := t[digest]; ok {
					t[digest] += n
				} else {
					t[digest] = n
				}
			}
		} else {
			cp := Tally{}
			for k, v := range tally {
				cp[k] = v
			}
			testTally[testName] = cp
		}
	}

	maxCountsByTest := make(map[string]util.StringSet, len(testTally))
	for testName, tally := range testTally {
		maxCount := 0
		for _, count := range tally {
			if count > maxCount {
				maxCount = count
			}
		}
		maxCountsByTest[testName] = util.StringSet{}
		for digest, count := range tally {
			if count == maxCount {
				maxCountsByTest[testName][digest] = true
			}
		}
	}

	return traceTally, testTally, maxCountsByTest
}
