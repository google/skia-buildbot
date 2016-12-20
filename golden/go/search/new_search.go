package search

import (
	"sort"

	"go.skia.org/infra/go/paramtools"
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

type srIntermediate struct {
	traces []tiling.Trace
	params paramtools.ParamSet
}

func newSrIntermediate(test, digest, traceID string, trace tiling.Trace) *srIntermediate {
	ret := &srIntermediate{}
	ret.Add(traceID, trace)
	return ret
}

func (s *srIntermediate) Add(traceID string, trace tiling.Trace) {
	s.traces = append(s.traces, trace)
}

func (s *SearchAPI) filterTile(q *Query, idx *indexer.SearchIndex) (map[string]map[string]*srIntermediate, error) {
	// Add digest/trace to the result.
	ret := map[string]map[string]*srIntermediate{}
	addFn := func(test, digest, traceID string, trace tiling.Trace, accptRet interface{}) {
		if testMap, ok := ret[test]; !ok {
			ret[test] = map[string]*srIntermediate{digest: newSrIntermediate(test, digest, traceID, trace)}
		} else if entry, ok := testMap[digest]; !ok {
			testMap[digest] = newSrIntermediate(test, digest, traceID, trace)
		} else {
			entry.Add(traceID, trace)
		}
	}

	if err := iterTile(q, addFn, nil, s.storages, idx); err != nil {
		return nil, err
	}
	return ret, nil
}

// TODO(stephana)
func (s *SearchAPI) preDiffResultFilter(qf *Filter, inter map[string]map[string]*srIntermediate, idx *indexer.SearchIndex) {
	// Group by tests and find the one with the maximum count.
	if (qf != nil) && (qf.GroupTest == GROUP_TEST_MAX_COUNT) {
		talliesByTest := idx.TalliesByTest()

		for testName, digestInfo := range inter {
			maxCount := -1
			maxDigest := ""
			for digest := range digestInfo {
				if talliesByTest[testName][digest] > maxCount {
					maxCount = talliesByTest[testName][digest]
					maxDigest = digest
				}
			}
			inter[testName] = map[string]*srIntermediate{maxDigest: inter[testName][maxDigest]}
		}
	}
}

// TODO(stephana)
func (s *SearchAPI) getReferenceDiffs(q *Query, inter map[string]map[string]*srIntermediate, idx *indexer.SearchIndex) ([]*SRDigest, error) {
	retDigests := make([]*SRDigest, 0, len(inter))
	for test, testDigests := range inter {
		for digest, i := range testDigests {
			retDigests = append(retDigests, newSRDigestWithDiff(test, digest, i))
		}
	}
	return retDigests, nil
}

// TODO(stephana) Filter over diff values.
func (s *SearchAPI) postDiffResultFilter(digestInfo []*SRDigest, qf *Filter) []*SRDigest {
	if qf == nil {
		return digestInfo
	}

	newDigestInfo := make([]*SRDigest, 0, len(digestInfo))
	for _, digest := range digestInfo {
		// Filter all digests where MaxRGBA is above a certain threshold.
		if (qf.RGBAMax > 0) && (util.MaxInt(digest.RefDiffs[digest.ClosestRef].MaxRGBADiffs...) >= qf.RGBAMax) {
			continue
		}
		newDigestInfo = append(newDigestInfo, digest)
	}
	return newDigestInfo
}

// TODO(stephana)
func (s *SearchAPI) sortAndLimitDigests(digestInfo []*SRDigest, offset, limit int) []*SRDigest {
	if offset >= len(digestInfo) {
		return []*SRDigest{}
	}

	// retDigests = append(retDigests, digestFromIntermediate(test, digest, i, exp, tile, idx, s.storages.DiffStore, q.IncludeIgnores))
	sort.Sort(SRDigestSlice(digestInfo))

	// Fill in the extra information for the traces we are interested in.
	fullLength := len(digestInfo)
	if fullLength > limit {
		return digestInfo[0:limit]
	}
	return digestInfo
}

func (s *SearchAPI) Search(q *Query) (*NewSearchResponse, error) {
	idx := s.ixr.GetIndex()

	// Filter the tiles.
	inter, err := s.filterTile(q, idx)

	// Filter early for anything that does not involve diffs.
	s.preDiffResultFilter(q.Filter, inter, idx)

	// exp, err := s.storages.ExpectationsStore.Get()
	// if err != nil {
	// 	return nil, err
	// }

	// Get the reference points (closet postive, negative etc. )
	ret, err := s.getReferenceDiffs(q, inter, idx)
	if err != nil {
		return nil, err
	}

	// Filter by diff values.
	ret = s.postDiffResultFilter(ret, q.Filter)

	// Sort the digests and fill the ones we are interested in with more traces.
	total := len(ret)
	ret = s.sortAndLimitDigests(ret, q.Offset, q.Limit)

	return &NewSearchResponse{
		Digests: ret,
		Total:   total,
		Commits: idx.GetTile(false).Commits,
	}, nil
}

func newSRDigestWithDiff(test, digest string, inter *srIntermediate) *SRDigest {
	return nil
}

// DigestSlice is a utility type for sorting slices of Digest by their max diff.
type SRDigestSlice []*SRDigest

func (s SRDigestSlice) Len() int           { return len(s) }
func (s SRDigestSlice) Less(i, j int) bool { return s[i].diffVal("") < s[j].diffVal("") }
func (s SRDigestSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
