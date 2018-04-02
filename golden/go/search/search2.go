package search

import (
	"context"
	"sync"

	"go.skia.org/infra/golden/go/tryjobstore"

	"go.opencensus.io/trace"
	"go.skia.org/infra/golden/go/gtile"
)

// Search queries the current tile based on the parameters specified in
// the instance of Query.
func (s *SearchAPI) Search2(ctx context.Context, q *Query) (*NewSearchResponse, error) {
	ctx, span := trace.StartSpan(ctx, "search/Search2")
	defer span.End()

	/*
		// get the gtile
		// run the lhs - query and the rhs query in parallel (omit rhs query for now )
		//   => list of traces with int values intermediate rep
		//
		//

		gTile := idx.GTile(includeIgnores)
	*/

	// Keep track if we are including reference diffs. This is going to be true
	// for the majority of queries.
	getRefDiffs := !q.NoDiff

	isTryjobSearch := q.Issue > 0

	// Get the expectations and the current index, which we assume constant
	// for the duration of this query.
	exp, err := s.getExpectationsFromQuery(q)
	if err != nil {
		return nil, err
	}
	idx := s.ixr.GetIndex()

	var inter srInterMap = nil
	var issue *tryjobstore.Issue = nil

	// Find the digests (left hand side) we are interested in.
	if isTryjobSearch {
		// Search the tryjob results for the issue at hand.
		inter, issue, err = s.queryIssue(ctx, q, s.storages.WhiteListQuery, idx, exp)
	} else {
		// Iterate through the tile and get an intermediate
		// representation that contains all the traces matching the queries.
		inter, err = s.filterTile(ctx, q, exp, idx)
	}
	if err != nil {
		return nil, err
	}

	var ret []*SRDigest

	// Get reference diffs unless it was specifically disabled.
	if getRefDiffs {
		gTile := idx.GTile(q.IncludeIgnores)

		// Diff stage: Compare all digests found in the previous stages and find
		// reference points (positive, negative etc.) for each digest.
		s.getDiffs(ctx, inter, q.Metric, q.Match, gTile)
		if err != nil {
			return nil, err
		}
		ret = s.getDigestRecs(inter, exp)

		// Post-diff stage: Apply all filters that are relevant once we have
		// diff values for the digests.
		ret = s.afterDiffResultFilter(ctx, ret, q)
	} else {
		ret = s.getDigestRecs(inter, exp)
	}

	// Sort the digests and fill the ones that are going to be displayed with
	// additional data. Note we are returning all digests found, so we can do
	// bulk triage, but only the digests that are going to be shown are padded
	// with additional information.
	displayRet, offset := s.sortAndLimitDigests(ctx, q, ret, int(q.Offset), int(q.Limit))
	s.addParamsAndTraces(ctx, displayRet, inter, exp, idx)

	// Return all digests with the selected offset within the result set.
	return &NewSearchResponse{
		Digests: ret,
		Offset:  offset,
		Size:    len(displayRet),
		Commits: idx.GetTile(false).Commits,
		Issue:   issue,
	}, nil
}

// getReferenceDiffs compares all digests collected in the intermediate representation
// and compares them to the other known results for the test at hand.
func (s *SearchAPI) getDiffs(ctx context.Context, inter srInterMap, metric string, match []string, gTile *gtile.GTile) {
	ctx, span := trace.StartSpan(ctx, "search/getDiffs")
	defer span.End()

	var wg sync.WaitGroup
	for testName, digests := range inter {
		wg.Add(1)
		go func(testName string, digests map[string]*srIntermediate) {
			s.findRefDiff(testName, digests, metric, match, gTile)
			wg.Done()
		}(testName, digests)
	}
	wg.Wait()
}

func (s *SearchAPI) findRefDiff(testName string, digests map[string]*srIntermediate, metric string, match []string, gTile *gtile.GTile) {
	// // // Get the subquery for this test
	// bLine := gTile.BaselineForTest(testName)
	// dmCache := s.storages.Cache
	// diffStore := s.storages.DiffStore

	// for digest, intermediate := range digests {
	// 	// Insert rhs query here that is common to the entire test.
	// 	// Build the query on the stack and get all digests in the baseline to consider.
	// 	compDigests := bLine.Filter(digest, match, intermediate.params, metric) // digest contains positive and negatives.
	// 	diffs := make([]*DiffMetrics, len(compDigests))
	// 	for idx, compDigest := range compDigests {
	// 		func(idx int, d1, d2 string) {
	// 			go func() {
	// 				diffs[idx] = dmCache.Get(d1, d2)
	// 				if diffs[idx] == nil {
	// 					diffMap, err := diffStore.Get(diff.PRIORITY_NOW, 1, []string{d2})
	// 					if err != nil {
	// 						sklog.Errorf("Unable to get diff for %s:%s. Got error: %s", err)
	// 						return
	// 					}
	// 					if len(diffMap) == 1 {
	// 						diffs[idx] = diffMap[d2]
	// 						dmCache.Put(d1, d2, diffs[idx])
	// 					}
	// 				}
	// 			}()
	// 		}(idx, digest, compDigest)
	// 	}
	// }
}
