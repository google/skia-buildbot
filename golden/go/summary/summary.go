// Package summary summarizes the current state of triaging.
package summary

import (
	"context"
	"net/url"
	"sort"
	"sync"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
)

// DigestsForTest contains rolled up digest counts/blames for one test in one corpus.
// It is immutable and should be thread safe.
type DigestsForTest struct {
	Name      types.TestName        `json:"name"`
	Corpus    string                `json:"corpus"`
	Pos       int                   `json:"pos"`
	Neg       int                   `json:"neg"`
	Untriaged int                   `json:"untriaged"`
	Num       int                   `json:"num"`
	UntHashes types.DigestSlice     `json:"untHashes"`
	Blame     []blame.WeightedBlame `json:"blame"`
	// currently unused
	Diameter int `json:"diameter"`
}

// TODO(jcgregorio) Make diameter faster, and also make the actual diameter
//   metric better. Until then disable it.
const computeDiameter = false

// SummaryMapConfig is a helper struct for calculating SummaryMap.
type SummaryMapConfig struct {
	ExpectationsStore expstorage.ExpectationsStore
	DiffStore         diff.DiffStore // only needed if computeDiameter = true

	DigestCounter digest_counter.DigestCounter
	Blamer        blame.Blamer
}

// Calculate calculates a slice of DigestsForTest for the given data
func Calculate(smc SummaryMapConfig, tile *tiling.Tile, testNames types.TestNameSet, query url.Values, head bool) ([]*DigestsForTest, error) {
	return smc.calcSummaries(tile, testNames, query, head)
}

// OverwriteWithNew creates a new SummaryMap from this and the passed
// in map. The passed in map will "win" in the event there are tests
// in both. We assume that the existing slice is sorted by TestName,Corpus already.
func OverwriteWithNew(existing, newOnes []*DigestsForTest) []*DigestsForTest {
	ret := existing
	for _, f := range newOnes {
		i := sort.Search(len(existing), func(i int) bool {
			return existing[i].Name > f.Name || (existing[i].Name == f.Name && existing[i].Corpus >= f.Corpus)
		})
		if i < len(existing) && existing[i].Name == f.Name && existing[i].Corpus == f.Corpus {
			existing[i] = f
		} else {
			// Didn't find an existing one, just stick it on the end
			ret = append(ret, f)
		}
	}
	// Re-sort
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name || (ret[i].Name < ret[j].Name && ret[i].Corpus < ret[j].Corpus)
	})
	return ret
}

type testCorpusPair struct {
	test   types.TestName
	corpus string
}

// tracePair is used to hold traces, along with their ids.
type tracePair struct {
	id tiling.TraceId
	tr *types.GoldenTrace
}

// calcSummaries returns a DigestsForTest of the given tile. If testNames is not empty,
// then restrict the results to only tests with those names. If query is not empty,
// it will be used as an additional filter. Finally, if head is true, only consider
// the single most recent digest per trace.
func (s *SummaryMapConfig) calcSummaries(tile *tiling.Tile, testNames types.TestNameSet, query url.Values, head bool) ([]*DigestsForTest, error) {
	defer shared.NewMetricsTimer("calc_summaries_total").Stop()
	sklog.Infof("CalcSummaries: head %v", head)

	var ret []*DigestsForTest
	e, err := s.ExpectationsStore.Get()
	if err != nil {
		return nil, skerr.Wrapf(err, "getting expectations")
	}

	// Filter down to just the traces we are interested in, based on query.
	filtered := map[testCorpusPair][]*tracePair{}
	t := shared.NewMetricsTimer("calc_summaries_filter_traces")
	for id, tr := range tile.Traces {
		gt := tr.(*types.GoldenTrace)
		if len(testNames) > 0 && !testNames[gt.TestName()] {
			continue
		}
		if tiling.Matches(tr, query) {
			k := testCorpusPair{test: gt.TestName(), corpus: gt.Corpus()}
			if slice, ok := filtered[k]; ok {
				filtered[k] = append(slice, &tracePair{tr: gt, id: id})
			} else {
				filtered[k] = []*tracePair{{tr: gt, id: id}}
			}
		}
	}
	t.Stop()

	digestsByTrace := s.DigestCounter.ByTrace()

	// Now create summaries for each test using the filtered set of traces.
	t = shared.NewMetricsTimer("calc_summaries_tally")
	lastCommitIndex := tile.LastCommitIndex()
	for k, traces := range filtered {
		digestMap := types.DigestSet{}
		for _, pair := range traces {
			if head {
				// Find the last non-missing value in the trace.
				for i := lastCommitIndex; i >= 0; i-- {
					if pair.tr.IsMissing(i) {
						continue
					} else {
						digestMap[pair.tr.Digests[i]] = true
						break
					}
				}
			} else {
				// Use the digestsByTrace if available, otherwise just inspect the trace.
				if t, ok := digestsByTrace[pair.id]; ok {
					for d := range t {
						digestMap[d] = true
					}
				} else {
					for i := lastCommitIndex; i >= 0; i-- {
						if !pair.tr.IsMissing(i) {
							digestMap[pair.tr.Digests[i]] = true
						}
					}
				}
			}
		}
		ret = append(ret, s.makeSummary(k.test, e, k.corpus, digestMap.Keys()))
	}
	t.Stop()

	// Sort for determinism and to allow clients to use binary search.
	t = shared.NewMetricsTimer("calc_summaries_sort")
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name || (ret[i].Name == ret[j].Name && ret[i].Corpus < ret[j].Corpus)
	})
	t.Stop()

	return ret, nil
}

// makeSummary returns a DigestsForTest for the given digests.
func (s *SummaryMapConfig) makeSummary(name types.TestName, exp expectations.ReadOnly, corpus string, digests types.DigestSlice) *DigestsForTest {
	pos := 0
	neg := 0
	unt := 0
	diamDigests := types.DigestSlice{}
	untHashes := types.DigestSlice{}
	for _, digest := range digests {
		switch exp.Classification(name, digest) {
		case expectations.Untriaged:
			unt += 1
			diamDigests = append(diamDigests, digest)
			untHashes = append(untHashes, digest)
		case expectations.Negative:
			neg += 1
		case expectations.Positive:
			pos += 1
			diamDigests = append(diamDigests, digest)
		}
	}

	sort.Sort(diamDigests)
	sort.Sort(untHashes)

	d := 0
	if computeDiameter {
		d = diameter(diamDigests, s.DiffStore)
	}
	return &DigestsForTest{
		Name:      name,
		Diameter:  d,
		Pos:       pos,
		Neg:       neg,
		Untriaged: unt,
		UntHashes: untHashes,
		Num:       pos + neg + unt,
		Corpus:    corpus,
		Blame:     s.Blamer.GetBlamesForTest(name),
	}
}

func diameter(digests types.DigestSlice, diffStore diff.DiffStore) int {
	// TODO Parallelize.
	lock := sync.Mutex{}
	max := 0
	wg := sync.WaitGroup{}
	for {
		if len(digests) <= 2 {
			break
		}
		wg.Add(1)
		go func(d1 types.Digest, d2 types.DigestSlice) {
			defer wg.Done()
			dms, err := diffStore.Get(context.TODO(), d1, d2)
			if err != nil {
				sklog.Errorf("Unable to get diff: %s", err)
				return
			}
			localMax := 0
			for _, dm := range dms {
				if dm.NumDiffPixels > localMax {
					localMax = dm.NumDiffPixels
				}
			}
			lock.Lock()
			defer lock.Unlock()
			if localMax > max {
				max = localMax
			}
		}(digests[0], digests[1:2])
		digests = digests[1:]
	}
	wg.Wait()
	return max
}
