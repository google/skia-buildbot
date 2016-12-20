package search

import (
	"sort"

	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/storage"
)

// TODO: main return type from new search
type SRDigest struct {
	Test       string                    `json:"test"`
	Digest     string                    `json:"digest"`
	Status     string                    `json:"status"`
	ParamSet   map[string][]string       `json:"paramset"`
	Traces     *Traces                   `json:"traces"`
	ClosestRef string                    `json:"closestRef"`
	RefDiffs   map[string]*CTDiffMetrics `json:"refDiffs"`
	Blame      *blame.BlameDistribution  `json:"blame"`
}

func (s *SRDigest) diffVal(diffMetric string) float32 {
	return s.RefDiffs[s.ClosestRef].Diffs[diffMetric]
}

type NewSearchResponse struct {
	Digests []*SRDigest
	Total   int
	Commits []*tiling.Commit
}

type SearchAPI struct {
	storages *storage.Storage
	ixr      *indexer.Indexer
}

func NewSearchAPI(storages *storage.Storage, ixr *indexer.Indexer) (*SearchAPI, error) {
	return &SearchAPI{
		storages: storages,
		ixr:      ixr,
	}, nil
}

func (s *SearchAPI) Search(q *Query) (*NewSearchResponse, error) {
	// Add digest/trace to the result.
	inter := map[string]map[string]*intermediate{}
	addFn := func(test, digest, traceID string, trace tiling.Trace, accptRet interface{}) {
		var testMap map[string]*intermediate
		var ok bool
		if testMap, ok = inter[test]; !ok {
			inter[test] = map[string]*intermediate{digest: newIntermediate(test, digest, traceID, trace)}
		} else if entry, ok := testMap[digest]; !ok {
			testMap[digest] = newIntermediate(test, digest, traceID, trace)
		} else {
			entry.addTrace(traceID, trace)
		}
	}

	idx := s.ixr.GetIndex()
	if err := iterTile(q, addFn, nil, s.storages, idx); err != nil {
		return nil, err
	}

	exp, err := s.storages.ExpectationsStore.Get()
	if err != nil {
		return nil, err
	}

	// Filter early for anything that does not involve diffs.
	if (q.Filter != nil) && (q.Filter.GroupTest == GROUP_FILTER_MAX_COUNT) {
		talliesByTest := idx.TalliesByTest()
		newInter := make(map[string]map[string]*intermediate, len(inter))
		for testName, digestInfo := range inter {
			maxCount := -1
			maxDigest := ""
			for digest := range digestInfo {
				if talliesByTest[testName][digest] > maxCount {
					maxCount = talliesByTest[testName][digest]
					maxDigest = digest
				}
			}
			newInter[testName] = map[string]*intermediate{maxDigest: inter[testName][maxDigest]}
		}
		inter = newInter
	}

	// Now loop over all the intermediates and build a Digest for each one.
	tile := idx.GetTile(q.IncludeIgnores)
	retDigests := make([]*SRDigest, 0, len(inter))
	for test, testDigests := range inter {
		for digest, i := range testDigests {
			retDigests = append(retDigests, newDigestWithDiff(test, digest, i))
		}
	}

	// Filter over diff values.
	if q.Filter != nil {
		// Iterate over the results
		for i, digest := range retDigests {
			// Filter all digests where MaxRGBA is above a certain threshold.
			if (q.Filter.RGBAMax > 0) && (util.MaxInt(digest.RefDiffs[digest.ClosestRef].MaxRGBADiffs...) >= q.Filter.RGBAMax) {
				retDigests[i] = nil
			}
		}
	}

	// Sort the digests

	//

	// retDigests = append(retDigests, digestFromIntermediate(test, digest, i, exp, tile, idx, s.storages.DiffStore, q.IncludeIgnores))
	sort.Sort(SRDigestSlice(retDigests))
	fullLength := len(retDigests)
	if fullLength > q.Limit {
		retDigests = retDigests[0:q.Limit]
	}

	return &NewSearchResponse{
		Digests: retDigests,
		Total:   fullLength,
		Commits: tile.Commits,
	}, nil
}

func newDigestWithDiff(test, digest string, inter *intermediate) *SRDigest {
	return nil
}

// DigestSlice is a utility type for sorting slices of Digest by their max diff.
type SRDigestSlice []*SRDigest

func (s SRDigestSlice) Len() int           { return len(s) }
func (s SRDigestSlice) Less(i, j int) bool { return s[i].diffVal("") < s[j].diffVal("") }
func (s SRDigestSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
