// Package ref_differ contains routines to calculate reference diffs for digests.
package ref_differ

import (
	"context"
	"math"
	"sort"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/types"
)

// RefDiffer is an interface for calculating the ReferenceDiffs for a given test+digest pair.
// A 'diff' captures the difference between two digests.
// To deal with digests in a human friendly way we calculate reference
// diffs for each digest, most notably the closest negative and
// positive digest. But other reference diffs might make sense and
// should be added here.
type RefDiffer interface {
	// FillRefDiffs fills in d with the closest negative and positive images (digests) to it.
	// It uses "metric" to determine "closeness". If match is non-nil, it only returns those
	// digests that match d's params for the keys in match. If rhsQuery is not empty, it only
	// compares against digests that match rhsQuery.
	FillRefDiffs(ctx context.Context, d *frontend.SearchResult, metric string, match []string, rhsQuery paramtools.ParamSet, is types.IgnoreState) error
}

// DiffImpl aggregates the helper objects needed to calculate reference diffs.
type DiffImpl struct {
	exp       expectations.Classifier
	diffStore diff.DiffStore
	idx       indexer.IndexSearcher
}

// New returns a *DiffImpl using given types.
func New(exp expectations.Classifier, diffStore diff.DiffStore, idx indexer.IndexSearcher) *DiffImpl {
	return &DiffImpl{
		exp:       exp,
		diffStore: diffStore,
		idx:       idx,
	}
}

// FillRefDiffs implements the RefDiffer interface.
func (r *DiffImpl) FillRefDiffs(ctx context.Context, d *frontend.SearchResult, metric string, match []string, rhsQuery paramtools.ParamSet, iState types.IgnoreState) error {
	paramsByDigest := r.idx.GetParamsetSummaryByTest(iState)[d.Test]

	// TODO(kjlubick) maybe make this use an errgroup
	posDigests := r.getDigestsWithLabel(d, match, paramsByDigest, rhsQuery, expectations.Positive)
	negDigests := r.getDigestsWithLabel(d, match, paramsByDigest, rhsQuery, expectations.Negative)

	var err error
	ret := make(map[common.RefClosest]*frontend.SRDiffDigest, 2)
	ret[common.PositiveRef], err = r.getClosestDiff(ctx, metric, d.Digest, posDigests)
	if err != nil {
		return skerr.Wrapf(err, "fetching positive diffs")
	}
	ret[common.NegativeRef], err = r.getClosestDiff(ctx, metric, d.Digest, negDigests)
	if err != nil {
		return skerr.Wrapf(err, "fetching negative diffs")
	}

	// Find the minimum according to the diff metric.
	closest := common.NoRef
	minDiff := float32(math.Inf(1))
	dCount := r.idx.DigestCountsByTest(iState)[d.Test]
	for ref, srdd := range ret {
		if srdd != nil {
			// Fill in the missing fields.
			srdd.Status = r.exp.Classification(d.Test, srdd.Digest)
			srdd.ParamSet = paramsByDigest[srdd.Digest]
			srdd.OccurrencesInTile = dCount[srdd.Digest]

			// Find the minimum.
			if srdd.QueryMetric < minDiff {
				closest = ref
				minDiff = srdd.QueryMetric
			}
		}
	}
	d.ClosestRef = closest
	d.RefDiffs = ret
	return nil
}

func getMetric(d *diff.DiffMetrics, metric string) float32 {
	switch metric {
	case query.CombinedMetric:
		return d.CombinedMetric
	case query.PercentMetric:
		return d.PixelDiffPercent
	case query.PixelMetric:
		return float32(d.NumDiffPixels)
	}
	return d.CombinedMetric
}

// getDigestsWithLabel return all digests within the given test that
// have the given label assigned to them and where the parameters
// listed in 'match' match.
func (r *DiffImpl) getDigestsWithLabel(s *frontend.SearchResult, match []string, paramsByDigest map[types.Digest]paramtools.ParamSet, rhsQuery paramtools.ParamSet, targetLabel expectations.Label) types.DigestSlice {
	ret := types.DigestSlice{}
	for d, digestParams := range paramsByDigest {
		// Accept all digests that are:  in the set of allowed digests
		//                              match the target label and where the required
		//                              parameter fields match.
		if (len(rhsQuery) == 0 || rhsQuery.Matches(digestParams)) &&
			(r.exp.Classification(s.Test, d) == targetLabel) &&
			paramSetsMatch(match, s.ParamSet, digestParams) {
			ret = append(ret, d)
		}
	}
	// Sort for determinism
	sort.Sort(ret)
	return ret
}

// getClosestDiff returns the closest diff between a digest and a set of digest.
func (r *DiffImpl) getClosestDiff(ctx context.Context, metric string, digest types.Digest, compDigests types.DigestSlice) (*frontend.SRDiffDigest, error) {
	if len(compDigests) == 0 {
		return nil, nil
	}

	diffs, err := r.diffStore.Get(ctx, digest, compDigests)
	if err != nil {
		return nil, skerr.Wrapf(err, "diffing digest %s with %d other digests", digest, len(compDigests))
	}

	if len(diffs) == 0 {
		return nil, nil
	}

	minDiff := float32(math.Inf(1))
	minDigest := types.Digest("")
	for resultDigest, diffMetrics := range diffs {
		if m := getMetric(diffMetrics, metric); m < minDiff {
			minDiff = m
			minDigest = resultDigest
		}
	}
	d := diffs[minDigest]
	return &frontend.SRDiffDigest{
		NumDiffPixels:    d.NumDiffPixels,
		CombinedMetric:   d.CombinedMetric,
		PixelDiffPercent: d.PixelDiffPercent,
		MaxRGBADiffs:     d.MaxRGBADiffs,
		DimDiffer:        d.DimDiffer,
		QueryMetric:      minDiff,
		Digest:           minDigest,
	}, nil
}

// paramSetsMatch returns true if the two param sets have matching values for the parameters listed
// in 'match'. If one of them is nil or completely empty there is always a match.
func paramSetsMatch(match []string, p1, p2 paramtools.ParamSet) bool {
	if len(p1) == 0 || len(p2) == 0 {
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
