// This package contains routines to calculate reference diffs for digests.
package ref_differ

import (
	"math"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/types"
)

// RefDiffer is an interface for calculating the ReferenceDiffs for a given test+digest pair.
// A 'diff' captures the difference between two digests.
// To deal with digests in a human friendly way we calculate reference
// diffs for each digest, most notably the closest negative and
// positive digest. But other reference diffs might make sense and
// should be added here.
type RefDiffer interface {
	// GetRefDiffs returns the closest negative and positive images (digests) for a given
	// Digest and TestName. It uses "metric" to determine "closeness". If match is non-nil,
	// it only returns those digest that match the given Digest's params in that slice. If
	// rhsQuery is not empty, it only compares against digests that match rhsQuery.
	// TODO(kjlubick) bundle t, d, pSet together (maybe *frontend.SRDigest?)
	GetRefDiffs(metric string, match []string, t types.TestName, d types.Digest, pSet paramtools.ParamSet,
		rhsQuery paramtools.ParamSet, is types.IgnoreState) (common.RefClosest, map[common.RefClosest]*frontend.SRDiffDigest)
}

// DiffImpl aggregates the helper objects needed to calculate reference diffs.
type DiffImpl struct {
	exp       common.ExpSlice
	diffStore diff.DiffStore
	idx       indexer.IndexSearcher
}

// New returns a *DiffImpl using given types.
func New(exp common.ExpSlice, diffStore diff.DiffStore, idx indexer.IndexSearcher) *DiffImpl {
	return &DiffImpl{
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
func (r *DiffImpl) GetRefDiffs(metric string, match []string, test types.TestName, digest types.Digest, params paramtools.ParamSet, rhsQuery paramtools.ParamSet, is types.IgnoreState) (common.RefClosest, map[common.RefClosest]*frontend.SRDiffDigest) {
	unavailableDigests := r.diffStore.UnavailableDigests()
	if _, ok := unavailableDigests[digest]; ok {
		return "", nil
	}

	paramsByDigest := r.idx.GetParamsetSummaryByTest(types.ExcludeIgnoredTraces)[test]

	posDigests := r.getDigestsWithLabel(test, match, params, paramsByDigest, unavailableDigests, rhsQuery, types.POSITIVE)
	negDigests := r.getDigestsWithLabel(test, match, params, paramsByDigest, unavailableDigests, rhsQuery, types.NEGATIVE)

	ret := make(map[common.RefClosest]*frontend.SRDiffDigest, 3)
	ret[common.PositiveRef] = r.getClosestDiff(metric, digest, posDigests)
	ret[common.NegativeRef] = r.getClosestDiff(metric, digest, negDigests)

	// TODO(stephana): Add a diff to the previous digest in the trace.

	// Find the minimum according to the diff metric.
	closest := common.RefClosest("")
	minDiff := float32(math.Inf(1))
	dCount := r.idx.DigestCountsByTest(is)[test]
	for ref, srdd := range ret {
		if srdd != nil {
			// Fill in the missing fields.
			srdd.Status = r.exp.Classification(test, srdd.Digest).String()
			srdd.ParamSet = paramsByDigest[srdd.Digest]
			srdd.ParamSet.Normalize()
			srdd.OccurrencesInTile = dCount[srdd.Digest]

			// Find the minimum.
			if srdd.DiffMetrics.Diffs[metric] < minDiff {
				closest = ref
				minDiff = srdd.DiffMetrics.Diffs[metric]
			}
		}
	}

	return closest, ret
}

// getDigestsWithLabel return all digests within the given test that
// have the given label assigned to them and where the parameters
// listed in 'match' match.
func (r *DiffImpl) getDigestsWithLabel(test types.TestName, match []string, params paramtools.ParamSet, paramsByDigest map[types.Digest]paramtools.ParamSet, unavailable map[types.Digest]*diff.DigestFailure, rhsQuery paramtools.ParamSet, targetLabel types.Label) types.DigestSlice {
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
func (r *DiffImpl) getClosestDiff(metric string, digest types.Digest, compDigests types.DigestSlice) *frontend.SRDiffDigest {
	if len(compDigests) == 0 {
		return nil
	}

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
		diffMetrics, ok := diffInfo.(*diff.DiffMetrics)
		if !ok {
			sklog.Warningf("unexpected diffmetric type: %#v", diffInfo)
			continue
		}
		if diffMetrics.Diffs[metric] < minDiff {
			minDiff = diffMetrics.Diffs[metric]
			minDigest = resultDigest
		}
	}

	return &frontend.SRDiffDigest{
		DiffMetrics: diffs[minDigest].(*diff.DiffMetrics),
		Digest:      minDigest,
	}
}

// paramSetsMatch returns true if the two param sets have matching
// values for the parameters listed in 'match'. If one of them is nil
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

// Make sure DiffImpl fulfills the RefDiffer interface.
var _ RefDiffer = (*DiffImpl)(nil)
