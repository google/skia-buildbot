package search

import (
	"math"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/types"
)

// This file contains routines to calculate reference diffs for digests.
// 'digest' is synonymous with image, because the digests are
// hashes of the pixel buffer when the image was generated.
// A 'diff' captures the difference between two digests.
// To deal with digests in a human friendly way we calculate reference
// diffs for each digest, most notably the closest positive and closest
// negative digest. But other reference diffs might make sense and
// should be added here.

const (
	// REF_CLOSEST_POSITIVE identifies the diff to the closest positive digest.
	REF_CLOSEST_POSITIVE = "pos"

	// REF_CLOSEST_NEGATIVE identifies the diff to the closest negative digest.
	REF_CLOSEST_NEGATIVE = "neg"
)

// RefDiffer aggregates the helper objects needed to calculate reference diffs.
// One instance of RefDiffer is created per query, so this can contain all the information
// for that query and that does not change when calculating a specific diff.
type RefDiffer struct {
	metric            string
	match             []string
	rhsQuery          paramtools.ParamSet
	includeIgnores    bool
	issueParamsByTest map[string]map[string]paramtools.ParamSet
	exp               ExpSlice
	diffStore         diff.DiffStore
	idx               *indexer.SearchIndex
}

// GetRefDiffs calculates the reference diffs between the given
// digest and the other digests in the same test based on the given
// metric. 'match' is the list of parameters that need to match between
// the digests that are compared, i.e. this allows to restrict comparison
// of gamma correct images to other digests that are also gamma correct.
func (r *RefDiffer) GetRefDiffs(test, digest string, params paramtools.ParamSet) (string, map[string]*SRDiffDigest) {
	unavailableDigests := r.diffStore.UnavailableDigests()
	if _, ok := unavailableDigests[digest]; ok {
		return "", nil
	}

	mParamsByDigest := make(MultiParamsByDigest, 0, 2)
	if r.issueParamsByTest != nil && len(r.issueParamsByTest[test]) > 0 {
		mParamsByDigest = append(mParamsByDigest, r.issueParamsByTest[test])
	}
	mParamsByDigest = append(mParamsByDigest, r.idx.GetParamsetSummaryByTest(false)[test])
	posDigests := r.getDigestsWithLabel(test, params, mParamsByDigest, unavailableDigests, types.POSITIVE)
	negDigests := r.getDigestsWithLabel(test, params, mParamsByDigest, unavailableDigests, types.NEGATIVE)

	ret := make(map[string]*SRDiffDigest, 3)
	ret[REF_CLOSEST_POSITIVE] = r.getClosestDiff(r.metric, digest, posDigests)
	ret[REF_CLOSEST_NEGATIVE] = r.getClosestDiff(r.metric, digest, negDigests)

	// TODO(stephana): Add a diff to the previous digest in the trace.

	// Find the minimum according to the diff metric.
	minKey := ""
	minDiff := float32(math.Inf(1))
	tally := r.idx.TalliesByTest(r.includeIgnores)[test]
	for key, val := range ret {
		if val != nil {
			// Fill in the missing fields.
			val.Status = r.exp.Classification(test, val.Digest).String()
			val.ParamSet = mParamsByDigest.Get(val.Digest)
			val.N = tally[val.Digest]

			// Find the minimum.
			if val.DiffMetrics.Diffs[r.metric] < minDiff {
				minKey = key
				minDiff = val.DiffMetrics.Diffs[r.metric]
			}
		}
	}

	return minKey, ret
}

type MultiParamsByDigest []map[string]paramtools.ParamSet

func (m MultiParamsByDigest) Get(d string) paramtools.ParamSet {
	for _, paramSet := range m {
		if ret, ok := paramSet[d]; ok {
			return ret
		}
	}
	return paramtools.ParamSet{}
}

// getDigestsWithLabel return all digests within the given test that
// have the given label assigned to them and where the parameters
// listed in 'match' match.
func (r *RefDiffer) getDigestsWithLabel(test string, params paramtools.ParamSet, multiParamsByDigest MultiParamsByDigest, unavailable map[string]*diff.DigestFailure, targetLabel types.Label) []string {
	ret := util.StringSet{}

	for _, paramsByDigest := range multiParamsByDigest {
		for d, digestParams := range paramsByDigest {
			// Accept all digests that are: available, in the set of allowed digests
			//                              match the target label and where the required
			//                              parameter fields match.
			_, ok := unavailable[d]
			if !ok &&
				(len(r.rhsQuery) == 0 || r.rhsQuery.Matches(digestParams)) &&
				(r.exp.Classification(test, d) == targetLabel) &&
				paramSetsMatch(r.match, params, digestParams) {
				ret[d] = true
			}
		}
	}
	return ret.Keys()
}

// getClosestDiff returns the closest diff between a digest and a set of digest.
func (r *RefDiffer) getClosestDiff(metric, digest string, compDigests []string) *SRDiffDigest {
	diffs, err := r.diffStore.Get(diff.PRIORITY_NOW, digest, compDigests)
	if err != nil {
		sklog.Errorf("Error diffing %s %v: %s", digest, compDigests, err)
		return nil
	}

	if len(diffs) == 0 {
		return nil
	}

	minDiff := float32(math.Inf(1))
	minDigest := ""
	for resultDigest, diffInfo := range diffs {
		diffMetrics := diffInfo.(*diff.DiffMetrics)
		if diffMetrics.Diffs[metric] < minDiff {
			minDiff = diffMetrics.Diffs[metric]
			minDigest = resultDigest
		}
	}

	return &SRDiffDigest{
		DiffMetrics: diffs[minDigest].(*diff.DiffMetrics),
		Digest:      minDigest,
	}
}

// paramSetsMatch returns true if the two param sets have matching
// values for the parameters listed in 'match'. If one of the is nil
// there is always a match.
func paramSetsMatch(match []string, p1, p2 paramtools.ParamSet) bool {
	if (p1 == nil) || (p2 == nil) {
		return true
	}

	for _, paramKey := range match {
		vals1, ok1 := p1[paramKey]
		vals2, ok2 := p2[paramKey]
		if !ok1 || !ok2 {
			return false
		}
		found := false
		for _, v := range vals1 {
			if util.In(v, vals2) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
