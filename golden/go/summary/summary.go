// summary summarizes the current state of triaging.
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

// TODO(kjlubick) This data type does not do well if multiple corpora have the same test name.
//   Additionally, in all the uses of this (poorly named) object, we just iterate over everything.
//   Therefore, it should be straight forward enough to remove this type and use []Summary
//   everywhere.
type SummaryMap map[types.TestName]*Summary

// Summary contains rolled up metrics for one test.
// It is immutable and should be thread safe.
type Summary struct {
	Name      types.TestName        `json:"name"`
	Diameter  int                   `json:"diameter"`
	Pos       int                   `json:"pos"`
	Neg       int                   `json:"neg"`
	Untriaged int                   `json:"untriaged"`
	UntHashes types.DigestSlice     `json:"untHashes"`
	Num       int                   `json:"num"`
	Corpus    string                `json:"corpus"`
	Blame     []blame.WeightedBlame `json:"blame"`
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

// NewSummaryMap creates a new instance of Summaries.
func NewSummaryMap(smc SummaryMapConfig, tile *tiling.Tile, testNames types.TestNameSet, query url.Values, head bool) (SummaryMap, error) {
	return smc.calcSummaries(tile, testNames, query, head)
}

// Combine creates a new SummaryMap from this and the passed
// in map. The passed in map will "win" in the event there are tests
// in both.
func (s SummaryMap) Combine(other SummaryMap) SummaryMap {
	copied := make(SummaryMap, len(s))
	for k, v := range s {
		copied[k] = v
	}

	for k, v := range other {
		copied[k] = v
	}
	return copied
}

// tracePair is used to hold traces, along with their ids.
type tracePair struct {
	id tiling.TraceId
	tr tiling.Trace
}

// calcSummaries returns a Summary of the given tile. If testNames is not empty,
// then restrict the results to only tests with those names. If query is not empty,
// it will be used as an additional filter. Finally, if head is true, only consider
// the single most recent digest per trace.
func (s *SummaryMapConfig) calcSummaries(tile *tiling.Tile, testNames types.TestNameSet, query url.Values, head bool) (SummaryMap, error) {
	defer shared.NewMetricsTimer("calc_summaries_total").Stop()
	sklog.Infof("CalcSummaries: head %v", head)

	ret := SummaryMap{}
	e, err := s.ExpectationsStore.Get()
	if err != nil {
		return nil, skerr.Wrapf(err, "getting expectations")
	}

	// Filter down to just the traces we are interested in, based on query.
	filtered := map[types.TestName][]*tracePair{}
	t := shared.NewMetricsTimer("calc_summaries_filter_traces")
	for id, tr := range tile.Traces {
		name := types.TestName(tr.Params()[types.PRIMARY_KEY_FIELD])
		if len(testNames) > 0 && !testNames[name] {
			continue
		}
		if tiling.Matches(tr, query) {
			if slice, ok := filtered[name]; ok {
				filtered[name] = append(slice, &tracePair{tr: tr, id: id})
			} else {
				filtered[name] = []*tracePair{{tr: tr, id: id}}
			}
		}
	}
	t.Stop()

	digestsByTrace := s.DigestCounter.ByTrace()

	// Now create summaries for each test using the filtered set of traces.
	t = shared.NewMetricsTimer("calc_summaries_tally")
	lastCommitIndex := tile.LastCommitIndex()
	for name, traces := range filtered {
		digestMap := types.DigestSet{}
		corpus := ""
		for _, trid := range traces {
			corpus = trid.tr.Params()[types.CORPUS_FIELD]
			if head {
				// Find the last non-missing value in the trace.
				for i := lastCommitIndex; i >= 0; i-- {
					if trid.tr.IsMissing(i) {
						continue
					} else {
						digestMap[trid.tr.(*types.GoldenTrace).Digests[i]] = true
						break
					}
				}
			} else {
				// Use the digestsByTrace if available, otherwise just inspect the trace.
				if t, ok := digestsByTrace[trid.id]; ok {
					for k := range t {
						digestMap[k] = true
					}
				} else {
					for i := lastCommitIndex; i >= 0; i-- {
						if !trid.tr.IsMissing(i) {
							digestMap[trid.tr.(*types.GoldenTrace).Digests[i]] = true
						}
					}
				}
			}
		}
		ret[name] = s.makeSummary(name, e, corpus, digestMap.Keys())
	}
	t.Stop()

	return ret, nil
}

// DigestInfo is test name and a digest found in that test. Returned from Search.
type DigestInfo struct {
	Test   types.TestName `json:"test"`
	Digest types.Digest   `json:"digest"`
}

// makeSummary returns a Summary for the given digests.
func (s *SummaryMapConfig) makeSummary(name types.TestName, exp expectations.ReadOnly, corpus string, digests types.DigestSlice) *Summary {
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
	return &Summary{
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
