// digest_counter returns counts of digests for various views on a Tile.
package digest_counter

import (
	"net/url"

	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/types"
)

// DigestCount maps a digest to a count. These counts are the number
// of times a digest was seen in a given scenario.
type DigestCount map[types.Digest]int

// DigestCounter allows querying for digest counts for a digest in different ways.
// For example, how many times did this tile see a given digest in a given
// test or a given trace? The results can be further filtered using
// ByQuery.
// It is not thread safe. The client of this package needs to make
// sure there are no conflicts.
type DigestCounter interface {
	// Calculate computes the counts for a given tile.
	// It must be called before any other method, or
	// data may be outdated or wrong.
	Calculate(tile *tiling.Tile)

	// ByTest returns a map of test_name -> DigestCount
	ByTest() map[types.TestName]DigestCount

	// ByTrace returns a map of trace_id -> DigestCount
	ByTrace() map[tiling.TraceId]DigestCount

	// MaxDigestsByTest returns a map of all tests seen
	// in the tile mapped to the digest that showed up
	// the most in that test.
	MaxDigestsByTest() map[types.TestName]types.DigestSet

	// ByQuery returns a DigestCount of all the digests that match the given query in
	// the provided tile.
	ByQuery(tile *tiling.Tile, query url.Values) DigestCount
}

// Counter implements DigestCounter
type Counter struct {
	traceDigestCount map[tiling.TraceId]DigestCount
	testDigestCount  map[types.TestName]DigestCount
	maxCountsByTest  map[types.TestName]types.DigestSet
}

// New creates a new Counter object.
func New() *Counter {
	return &Counter{}
}

// Calculate implements the DigestCounter interface.
func (t *Counter) Calculate(tile *tiling.Tile) {
	trace, test, maxCountsByTest := calculate(tile)
	t.traceDigestCount = trace
	t.testDigestCount = test
	t.maxCountsByTest = maxCountsByTest
}

// ByTest implements the DigestCounter interface.
func (t *Counter) ByTest() map[types.TestName]DigestCount {
	return t.testDigestCount
}

// ByTrace implements the DigestCounter interface.
func (t *Counter) ByTrace() map[tiling.TraceId]DigestCount {
	return t.traceDigestCount
}

// MaxDigestsByTest implements the DigestCounter interface.
func (t *Counter) MaxDigestsByTest() map[types.TestName]types.DigestSet {
	return t.maxCountsByTest
}

// ByQuery returns a DigestCount of all the digests that match the given query in
// the provided tile.
func (t *Counter) ByQuery(tile *tiling.Tile, query url.Values) DigestCount {
	return countByQuery(tile, t.traceDigestCount, query)
}

// countByQuery does the actual work of ByQuery.
func countByQuery(tile *tiling.Tile, traceDigestCount map[tiling.TraceId]DigestCount, query url.Values) DigestCount {
	ret := DigestCount{}
	for k, tr := range tile.Traces {
		if tiling.Matches(tr, query) {
			if _, ok := traceDigestCount[k]; !ok {
				continue
			}
			for digest, n := range traceDigestCount[k] {
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

// calculate computes a map[tracename]DigestCount and map[testname]DigestCount from the given Tile.
func calculate(tile *tiling.Tile) (map[tiling.TraceId]DigestCount, map[types.TestName]DigestCount, map[types.TestName]types.DigestSet) {
	defer shared.NewMetricsTimer("digest_counter_calculate").Stop()
	traceDigestCount := map[tiling.TraceId]DigestCount{}
	testDigestCount := map[types.TestName]DigestCount{}
	for k, tr := range tile.Traces {
		gtr := tr.(*types.GoldenTrace)
		dCount := DigestCount{}
		for _, d := range gtr.Digests {
			if d == types.MISSING_DIGEST {
				continue
			}
			if n, ok := dCount[d]; ok {
				dCount[d] = n + 1
			} else {
				dCount[d] = 1
			}
		}
		traceDigestCount[k] = dCount
		testName := gtr.TestName()
		if t, ok := testDigestCount[testName]; ok {
			for digest, n := range dCount {
				if _, ok := t[digest]; ok {
					t[digest] += n
				} else {
					t[digest] = n
				}
			}
		} else {
			cp := DigestCount{}
			for k, v := range dCount {
				cp[k] = v
			}
			testDigestCount[testName] = cp
		}
	}

	maxCountsByTest := make(map[types.TestName]types.DigestSet, len(testDigestCount))
	for testName, dCount := range testDigestCount {
		maxCount := 0
		for _, count := range dCount {
			if count > maxCount {
				maxCount = count
			}
		}
		maxCountsByTest[testName] = types.DigestSet{}
		for digest, count := range dCount {
			if count == maxCount {
				maxCountsByTest[testName][digest] = true
			}
		}
	}

	return traceDigestCount, testDigestCount, maxCountsByTest
}

// Make sure Counter fulfills the DigestCounter Interface
var _ DigestCounter = (*Counter)(nil)
