// digest_counter returns counts of digests for various views on a Tile.
package digest_counter

import (
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

// DigestCount maps a digest to a count. These counts are the number
// of times a digest was seen in a given scenario.
type DigestCount map[types.Digest]int

// DigestCounter allows querying for digest counts for a digest in different ways.
// For example, how many times did this tile see a given digest in a given
// test or a given trace? The results can be further filtered using
// ByQuery.
// It should be immutable once created and therefore thread-safe.
type DigestCounter interface {
	// ByTest returns a map of test_name -> DigestCount
	ByTest() map[types.TestName]DigestCount

	// ByTrace returns a map of trace_id -> DigestCount
	ByTrace() map[tiling.TraceID]DigestCount

	// MaxDigestsByTest returns a map of all tests seen
	// in the tile mapped to the digest that showed up
	// the most in that test.
	MaxDigestsByTest() map[types.TestName]types.DigestSet

	// ByQuery returns a DigestCount of all the digests that match the given query in
	// the provided tile. Note that this will recompute the digests across the entire tile.
	ByQuery(tile *tiling.Tile, query paramtools.ParamSet) DigestCount
}

// Counter implements DigestCounter
type Counter struct {
	traceDigestCount map[tiling.TraceID]DigestCount
	testDigestCount  map[types.TestName]DigestCount
	maxCountsByTest  map[types.TestName]types.DigestSet
}

// New creates a new Counter object.
func New(tile *tiling.Tile) *Counter {
	trace, test, maxCountsByTest := calculate(tile)
	return &Counter{
		traceDigestCount: trace,
		testDigestCount:  test,
		maxCountsByTest:  maxCountsByTest,
	}
}

// ByTest implements the DigestCounter interface.
func (t *Counter) ByTest() map[types.TestName]DigestCount {
	return t.testDigestCount
}

// ByTrace implements the DigestCounter interface.
func (t *Counter) ByTrace() map[tiling.TraceID]DigestCount {
	return t.traceDigestCount
}

// MaxDigestsByTest implements the DigestCounter interface.
func (t *Counter) MaxDigestsByTest() map[types.TestName]types.DigestSet {
	return t.maxCountsByTest
}

// ByQuery returns a DigestCount of all the digests that match the given query in
// the provided tile.
func (t *Counter) ByQuery(tile *tiling.Tile, query paramtools.ParamSet) DigestCount {
	return countByQuery(tile, t.traceDigestCount, query)
}

// countByQuery does the actual work of ByQuery.
func countByQuery(tile *tiling.Tile, traceDigestCount map[tiling.TraceID]DigestCount, query paramtools.ParamSet) DigestCount {
	ret := DigestCount{}
	for k, tr := range tile.Traces {
		if tr.Matches(query) {
			if _, ok := traceDigestCount[k]; !ok {
				continue
			}
			for digest, n := range traceDigestCount[k] {
				ret[digest] += n
			}
		}
	}
	return ret
}

// calculate computes the counts by trace id and test name from the given Tile.
func calculate(tile *tiling.Tile) (map[tiling.TraceID]DigestCount, map[types.TestName]DigestCount, map[types.TestName]types.DigestSet) {
	defer shared.NewMetricsTimer("digest_counter_calculate").Stop()
	traceDigestCount := map[tiling.TraceID]DigestCount{}
	testDigestCount := map[types.TestName]DigestCount{}
	for k, trace := range tile.Traces {
		dCount := DigestCount{}
		for _, d := range trace.Digests {
			if d == tiling.MissingDigest {
				continue
			}
			if n, ok := dCount[d]; ok {
				dCount[d] = n + 1
			} else {
				dCount[d] = 1
			}
		}
		traceDigestCount[k] = dCount
		testName := trace.TestName()
		if t, ok := testDigestCount[testName]; ok {
			for digest, n := range dCount {
				t[digest] += n
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
