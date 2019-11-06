// Package summary summarizes the current state of triaging.
package summary

import (
	"context"
	"net/url"
	"sort"
	"sync"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
)

// TriageStatus contains rolled up digest counts/blames for one test in one corpus.
// It is immutable and should be thread safe.
type TriageStatus struct {
	// TODO(kjlubick) Change Name/Corpus to be a more generic "Grouping"
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

// MergeSorted creates a new []*TriageStatus from this and the passed
// in slices. The passed in data will "win" in the event there are tests
// in both. We assume that the two passed in slices are sorted by TestName,Corpus already.
func MergeSorted(existing, newOnes []*TriageStatus) []*TriageStatus {
	ret := make([]*TriageStatus, 0, len(existing)+len(newOnes))

	// Basic algorithm for merging two sorted arrays, with a small tweak to have
	// the second one win for an exact match on Name and Corpus.
	i, j := 0, 0
	for i < len(existing) && j < len(newOnes) {
		e, n := existing[i], newOnes[j]
		if e.Name == n.Name && e.Corpus == n.Corpus {
			ret = append(ret, n)
			i++
			j++
		} else if e.Name > n.Name || (e.Name == n.Name && e.Corpus > n.Corpus) {
			ret = append(ret, n)
			j++
		} else {
			ret = append(ret, e)
			i++
		}
	}
	// Only one of these will actually append something, since j or i are at the end.
	ret = append(ret, newOnes[j:]...)
	ret = append(ret, existing[i:]...)

	return ret
}

// TODO(jcgregorio) Make diameter faster, and also make the actual diameter
//   metric better. Until then disable it.
const computeDiameter = false

// Data is a helper struct containing the data that goes into computing a summary.
type Data struct {
	Traces       map[tiling.TraceId]tiling.Trace
	Expectations expectations.ReadOnly
	// ByTrace maps all traces in Trace to the counts of digests that appeared
	// in those traces.
	ByTrace map[tiling.TraceId]digest_counter.DigestCount
	Blamer  blame.Blamer

	DiffStore diff.DiffStore // only needed if computeDiameter = true
}

// Calculate calculates a slice of TriageStatus for the given data and query options. It will
// summarize only those traces that match the given testNames (if any), the query (if any), and
// optionally be only for those digests at head. At head means just the non-empty digests that
// appear in the most recent commit. The return value will have its entries sorted by TestName
// first, then sorted by Corpus in the event of a tie.
// TODO(kjlubick): make CalculateWithDiameter its own function (needs context.Context too)
func (s *Data) Calculate(testNames types.TestNameSet, query url.Values, head bool) []*TriageStatus {
	defer shared.NewMetricsTimer("calc_summaries_total").Stop()
	sklog.Infof("CalcSummaries: head %v", head)

	var ret []*TriageStatus

	// Filter down to just the traces we are interested in, based on query.
	filtered := map[grouping][]*tracePair{}
	t := shared.NewMetricsTimer("calc_summaries_filter_traces")
	for id, tr := range s.Traces {
		gt := tr.(*types.GoldenTrace)
		if len(testNames) > 0 && !testNames[gt.TestName()] {
			continue
		}
		if tiling.Matches(tr, query) {
			k := grouping{test: gt.TestName(), corpus: gt.Corpus()}
			if slice, ok := filtered[k]; ok {
				filtered[k] = append(slice, &tracePair{tr: gt, id: id})
			} else {
				filtered[k] = []*tracePair{{tr: gt, id: id}}
			}
		}
	}
	t.Stop()

	// Now create summaries for each test using the filtered set of traces.
	t = shared.NewMetricsTimer("calc_summaries_tally")
	for k, traces := range filtered {
		digestMap := types.DigestSet{}
		for _, pair := range traces {
			if head {
				// Find the last non-missing value in the trace.
				for i := len(pair.tr.Digests) - 1; i >= 0; i-- {
					if pair.tr.IsMissing(i) {
						continue
					} else {
						digestMap[pair.tr.Digests[i]] = true
						break
					}
				}
			} else {
				// Use the digests by trace if available, otherwise just inspect the trace.
				if t, ok := s.ByTrace[pair.id]; ok {
					for d := range t {
						digestMap[d] = true
					}
				} else {
					for i := len(pair.tr.Digests) - 1; i >= 0; i-- {
						if !pair.tr.IsMissing(i) {
							digestMap[pair.tr.Digests[i]] = true
						}
					}
				}
			}
		}
		ret = append(ret, s.makeSummary(k.test, k.corpus, digestMap.Keys()))
	}
	t.Stop()

	// Sort for determinism and to allow clients to use binary search.
	t = shared.NewMetricsTimer("calc_summaries_sort")
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name || (ret[i].Name == ret[j].Name && ret[i].Corpus < ret[j].Corpus)
	})
	t.Stop()

	return ret
}

// grouping is the set of information used to combine like traces.
type grouping struct {
	test   types.TestName
	corpus string
}

// tracePair is used to hold traces, along with their ids.
type tracePair struct {
	id tiling.TraceId
	tr *types.GoldenTrace
}

// makeSummary returns a TriageStatus for the given digests.
func (s *Data) makeSummary(name types.TestName, corpus string, digests types.DigestSlice) *TriageStatus {
	pos := 0
	neg := 0
	unt := 0
	diamDigests := types.DigestSlice{}
	untHashes := types.DigestSlice{}
	for _, digest := range digests {
		switch s.Expectations.Classification(name, digest) {
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
	return &TriageStatus{
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
