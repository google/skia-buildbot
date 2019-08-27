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
	// REF_CLOSEST_POSTIVE identifies the diff to the closest positive digest.
	REF_CLOSEST_POSTIVE = "pos"

	// REF_CLOSEST_NEGATIVE identifies the diff to the closest negative digest.
	REF_CLOSEST_NEGATIVE = "neg"
)

// RefDiffer aggregates the helper objects needed to calculate reference diffs.
type RefDiffer struct {
	exp       ExpSlice
	diffStore diff.DiffStore
	idx       indexer.IndexSearcher
}

func NewRefDiffer(exp ExpSlice, diffStore diff.DiffStore, idx indexer.IndexSearcher) *RefDiffer {
	return &RefDiffer{
		exp:       exp,
		diffStore: diffStore,
		idx:       idx,
	}
}

// GetRefDiffs calculates the reference diffs between the given
// digest and the other digests in the same test based on the given
// metric. 'match' is the list of parameters that need to match between
// the digests that are compared, i.e. this allows to restrict comparison
// of gamma correct images to other digests that are also gamma correct.
func (r *RefDiffer) GetRefDiffs(metric string, match []string, test types.TestName, digest types.Digest, params paramtools.ParamSet, rhsQuery paramtools.ParamSet, is types.IgnoreState) (types.Digest, map[types.Digest]*SRDiffDigest) {
	unavailableDigests := r.diffStore.UnavailableDigests()
	if _, ok := unavailableDigests[digest]; ok {
		return "", nil
	}

	paramsByDigest := r.idx.GetParamsetSummaryByTest(types.ExcludeIgnoredTraces)[test]
	posDigests := r.getDigestsWithLabel(test, match, params, paramsByDigest, unavailableDigests, rhsQuery, types.POSITIVE)
	negDigests := r.getDigestsWithLabel(test, match, params, paramsByDigest, unavailableDigests, rhsQuery, types.NEGATIVE)

	ret := make(map[types.Digest]*SRDiffDigest, 3)
	ret[REF_CLOSEST_POSTIVE] = r.getClosestDiff(metric, digest, posDigests)
	ret[REF_CLOSEST_NEGATIVE] = r.getClosestDiff(metric, digest, negDigests)

	// TODO(stephana): Add a diff to the previous digest in the trace.

	// Find the minimum according to the diff metric.
	minDigest := types.Digest("")
	minDiff := float32(math.Inf(1))
	dCount := r.idx.DigestCountsByTest(is)[test]
	for digest, srdd := range ret {
		if srdd != nil {
			// Fill in the missing fields.
			srdd.Status = r.exp.Classification(test, srdd.Digest).String()
			srdd.ParamSet = paramsByDigest[srdd.Digest]
			srdd.N = dCount[srdd.Digest]

			// Find the minimum.
			if srdd.DiffMetrics.Diffs[metric] < minDiff {
				minDigest = digest
				minDiff = srdd.DiffMetrics.Diffs[metric]
			}
		}
	}

	return minDigest, ret
}

// getDigestsWithLabel return all digests within the given test that
// have the given label assigned to them and where the parameters
// listed in 'match' match.
func (r *RefDiffer) getDigestsWithLabel(test types.TestName, match []string, params paramtools.ParamSet, paramsByDigest map[types.Digest]paramtools.ParamSet, unavailable map[types.Digest]*diff.DigestFailure, rhsQuery paramtools.ParamSet, targetLabel types.Label) types.DigestSlice {
	ret := types.DigestSlice{}
	for d, digestParams := range paramsByDigest {
		// Accept all digests that are: available, in the set of allowed digests
		//                              match the target label and where the required
		//                              parameter fields match.
		_, ok := unavailable[d]
		if !ok &&
			(len(rhsQuery) == 0 || rhsQuery.Matches(digestParams)) &&
			(r.exp.Classification(test, d) == targetLabel) &&
			paramSetsMatch(match, params, digestParams) {
			ret = append(ret, d)
		}
	}
	return ret
}

// getClosestDiff returns the closest diff between a digest and a set of digest.
func (r *RefDiffer) getClosestDiff(metric string, digest types.Digest, compDigests types.DigestSlice) *SRDiffDigest {
	diffs, err := r.diffStore.Get(diff.PRIORITY_NOW, digest, compDigests)
	if err != nil {
		sklog.Errorf("Error diffing %s %v: %s", digest, compDigests, err)
		return nil
	}

	if len(diffs) == 0 {
		return nil
	}

	minDiff := float32(math.Inf(1))
	minDigest := types.Digest("")
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
