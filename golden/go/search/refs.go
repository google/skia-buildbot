package search

import (
	"math"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
)

const (
	REF_CLOSEST_POSTIVE  = "pos"
	REF_CLOSEST_NEGATIVE = "neg"
	REF_PREVIOUS_TRACE   = "trace"
)

type ReferenceFn func() map[string]*diff.DiffMetrics

func referenceDiffs(storage *storage.Storage) (string, map[string]*CTDiffMetrics) {
	return "", nil
}

type RefDiffer struct {
	exp       *expstorage.Expectations
	diffStore diff.DiffStore
	idx       *indexer.SearchIndex
}

func NewRefDiffer(exp *expstorage.Expectations, diffStore diff.DiffStore, idx *indexer.SearchIndex) *RefDiffer {
	return &RefDiffer{
		exp:       exp,
		diffStore: diffStore,
		idx:       idx,
	}
}

func (r *RefDiffer) GetRefDiffs(metric string, match []string, test, digest string, params paramtools.ParamSet, traces []tiling.Trace) (string, map[string]*CTDiffMetrics) {
	unavailableDigests := r.diffStore.UnavailableDigests()
	if _, ok := unavailableDigests[digest]; ok {
		return "", nil
	}

	paramsByDigest := r.idx.GetParamsetSummaryByTest(false)[test]
	posDigests := r.getDigestsWithLabel(test, match, params, paramsByDigest, unavailableDigests, types.POSITIVE)
	negDigests := r.getDigestsWithLabel(test, match, params, paramsByDigest, unavailableDigests, types.NEGATIVE)
	// traceDigests := findPreviousDigests(digest, traces)

	ret := make(map[string]*CTDiffMetrics, 3)
	ret[REF_CLOSEST_POSTIVE] = r.getClosestDiff(metric, digest, posDigests)
	ret[REF_CLOSEST_NEGATIVE] = r.getClosestDiff(metric, digest, negDigests)
	// ret[REF_PREVIOUS_TRACE] = r.getClosestDiff(metric, digest, traceDigests)

	// Find the minimum according to the diff metric.
	minKey := ""
	minDiff := float32(math.Inf(1))
	for key, val := range ret {
		if val.DiffMetrics.Diffs[metric] < minDiff {
			minKey = key
			minDiff = val.DiffMetrics.Diffs[metric]
		}
	}

	return minKey, ret
}

func (r *RefDiffer) getDigestsWithLabel(test string, match []string, params paramtools.ParamSet, paramsByDigest map[string]paramtools.ParamSet, unavailable map[string]*diff.DigestFailure, targetLabel types.Label) []string {
	ret := []string{}
	for d, digestParams := range paramsByDigest {
		if _, ok := unavailable[d]; ok && (r.exp.Classification(test, d) == targetLabel) && paramSetsMatch(match, params, digestParams) {
			ret = append(ret, d)
		}
	}
	return ret
}

func paramSetsMatch(match []string, p1, p2 paramtools.ParamSet) bool {
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

// func findPreviousDigests(digest string, traces []tiling.Trace) []string {
//   ret := []string{}
//   for _, t := range traces {
//
//   }
//   return ret
// }

func (r *RefDiffer) getClosestDiff(metric string, digest string, compDigests []string) *CTDiffMetrics {
	diffs, err := r.diffStore.Get(diff.PRIORITY_NOW, digest, compDigests)
	if err != nil {
		glog.Errorf("Error diffing %s %v: %s", digest, compDigests, err)
		return nil
	}

	if len(diffs) == 0 {
		return nil
	}

	minDiff := float32(math.Inf(1))
	minDigest := ""
	for resultDigest, diffInfo := range diffs {
		if diffInfo.Diffs[metric] < minDiff {
			minDiff = diffInfo.Diffs[metric]
			minDigest = resultDigest
		}
	}

	return &CTDiffMetrics{
		DiffMetrics: diffs[minDigest],
		CTDigestCount: CTDigestCount{
			Digest: minDigest,
		},
	}
}
