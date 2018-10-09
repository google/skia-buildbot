// summary summarizes the current state of triaging.
package summary

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/types"
)

// Summary contains rolled up metrics for one test.
// It is not thread safe. The client of this package needs to make sure there
// are no conflicts.
type Summary struct {
	Name      string                 `json:"name"`
	Diameter  int                    `json:"diameter"`
	Pos       int                    `json:"pos"`
	Neg       int                    `json:"neg"`
	Untriaged int                    `json:"untriaged"`
	UntHashes []string               `json:"untHashes"`
	Num       int                    `json:"num"`
	Corpus    string                 `json:"corpus"`
	Blame     []*blame.WeightedBlame `json:"blame"`
}

// clone creates a copy of the summary.
func (s *Summary) clone() *Summary {
	ret := &Summary{}
	*ret = *s
	ret.UntHashes = append([]string(nil), s.UntHashes...)
	ret.Blame = append([]*blame.WeightedBlame(nil), s.Blame...)
	for idx, b := range s.Blame {
		ret.Blame[idx] = &blame.WeightedBlame{}
		*ret.Blame[idx] = *b
	}
	return ret
}

// Summaries contains a Summary for each test.
//
// It also updates itself when Tallies have been updated.
type Summaries struct {
	storages  *storage.Storage
	tallies   *tally.Tallies
	blamer    *blame.Blamer
	summaries map[string]*Summary
}

// New creates a new instance of Summaries.
func New(storages *storage.Storage) *Summaries {
	return &Summaries{
		storages: storages,
	}
}

// Clone creates a deep copy of the Summaries instance.
func (s *Summaries) Clone() *Summaries {
	copied := make(map[string]*Summary, len(s.summaries))
	for k, v := range s.summaries {
		copied[k] = v.clone()
	}

	return &Summaries{
		storages:  s.storages,
		tallies:   s.tallies,
		blamer:    s.blamer,
		summaries: copied,
	}
}

// Calculate sets the summaries based on the given tile. If testNames is empty
// (or nil) the entire tile will be calculated. Otherwise only the given
// test names will be updated.
func (s *Summaries) Calculate(tile *tiling.Tile, testNames []string, tallies *tally.Tallies, blamer *blame.Blamer) error {
	s.tallies = tallies
	s.blamer = blamer

	summaries, err := s.CalcSummaries(tile, testNames, nil, true)
	if err != nil {
		return fmt.Errorf("Failed to calculate summaries in Calculate: %s", err)
	}

	// If testNames were given, we copy the partially updated results.
	if testNames == nil {
		s.summaries = summaries
	} else {
		for k, v := range summaries {
			s.summaries[k] = v
		}
	}
	return nil
}

// Get returns the summaries keyed by the test names.
func (s *Summaries) Get() map[string]*Summary {
	return s.summaries
}

// TraceID is used to hold traces, along with their ids.
type TraceID struct {
	id string
	tr tiling.Trace
}

// CalcSummaries returns a Summary for each test that matches the given input filters.
//
// testNames
//   If not nil or empty then restrict the results to only tests that appear in this slice.
// query
//   URL encoded paramset to use for filtering.
// head
//   Only consider digests at head if true.
//
func (s *Summaries) CalcSummaries(tile *tiling.Tile, testNames []string, query url.Values, head bool) (map[string]*Summary, error) {
	defer timer.New("CalcSummaries").Stop()
	sklog.Infof("CalcSummaries: head %v", head)

	ret := map[string]*Summary{}

	t := timer.New("CalcSummaries:Expectations")
	e, err := s.storages.ExpectationsStore.Get()
	t.Stop()
	if err != nil {
		return nil, fmt.Errorf("Couldn't get expectations: %s", err)
	}

	// Filter down to just the traces we are interested in, based on query.
	filtered := map[string][]*TraceID{}
	t = timer.New("Filter Traces")
	for id, tr := range tile.Traces {
		name := tr.Params()[types.PRIMARY_KEY_FIELD]
		if len(testNames) > 0 && !util.In(name, testNames) {
			continue
		}
		if tiling.Matches(tr, query) {
			if slice, ok := filtered[name]; ok {
				filtered[name] = append(slice, &TraceID{tr: tr, id: id})
			} else {
				filtered[name] = []*TraceID{{tr: tr, id: id}}
			}
		}
	}
	t.Stop()

	traceTally := s.tallies.ByTrace()

	// Now create summaries for each test using the filtered set of traces.
	t = timer.New("Tally up the filtered traces")
	lastCommitIndex := tile.LastCommitIndex()
	for name, traces := range filtered {
		digests := util.NewStringSet()
		corpus := ""
		for _, trid := range traces {
			corpus = trid.tr.Params()[types.CORPUS_FIELD]
			if head {
				// Find the last non-missing value in the trace.
				for i := lastCommitIndex; i >= 0; i-- {
					if trid.tr.IsMissing(i) {
						continue
					} else {
						digests[trid.tr.(*types.GoldenTrace).Values[i]] = true
						break
					}
				}
			} else {
				// Use the traceTally if available, otherwise just inspect the trace.
				if t, ok := traceTally[trid.id]; ok {
					for k := range t {
						digests[k] = true
					}
				} else {
					for i := lastCommitIndex; i >= 0; i-- {
						if !trid.tr.IsMissing(i) {
							digests[trid.tr.(*types.GoldenTrace).Values[i]] = true
						}
					}
				}
			}
		}
		ret[name] = s.makeSummary(name, e, s.storages.DiffStore, corpus, digests.Keys())
	}
	t.Stop()

	return ret, nil
}

// DigestInfo is test name and a digest found in that test. Returned from Search.
type DigestInfo struct {
	Test   string `json:"test"`
	Digest string `json:"digest"`
}

// TODO(stephana): search should probably be removed because it is not used
// anywhere.

// search returns a slice of DigestInfo with all the digests that match the given query parameters.
//
// Note that unlike CalcSummaries the results aren't restricted by test name.
// Also note that the result can include positive and negative digests.
func (s *Summaries) search(tile *tiling.Tile, query string, head bool, pos bool, neg bool, unt bool) ([]DigestInfo, error) {
	q, err := url.ParseQuery(query)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse Query in Search: %s", err)
	}

	t := timer.New("Search:Expectations")
	e, err := s.storages.ExpectationsStore.Get()
	t.Stop()
	if err != nil {
		return nil, fmt.Errorf("Couldn't get expectations: %s", err)
	}

	// Filter down to just the traces we are interested in, based on query.
	filtered := map[string]tiling.Trace{}
	t = timer.New("Filter Traces")
	for id, tr := range tile.Traces {
		if tiling.Matches(tr, q) {
			filtered[id] = tr
		}
	}
	t.Stop()

	traceTally := s.tallies.ByTrace()

	// Find all test:digest's in the filtered traces.
	matches := map[string]bool{}
	t = timer.New("Tally up the filtered traces")
	lastCommitIndex := tile.LastCommitIndex()
	for id, trace := range filtered {
		test := trace.Params()[types.PRIMARY_KEY_FIELD]
		if head {
			// Find the last non-missing value in the trace.
			for i := lastCommitIndex; i >= 0; i-- {
				if trace.IsMissing(i) {
					continue
				} else {
					matches[test+":"+trace.(*types.GoldenTrace).Values[i]] = true
					break
				}
			}
		} else {
			if t, ok := traceTally[id]; ok {
				for d := range t {
					matches[test+":"+d] = true
				}
			}
		}
	}
	t.Stop()

	// Now create DigestInfo for each test:digest found, filtering out
	// digests with that don't match the triage classification.
	ret := []DigestInfo{}
	for key := range matches {
		testDigest := strings.Split(key, ":")
		if len(testDigest) != 2 {
			sklog.Errorf("Invalid test name or digest value: %s", key)
			continue
		}
		test := testDigest[0]
		digest := testDigest[1]
		class := e.Classification(test, digest)
		switch {
		case class == types.NEGATIVE && !neg:
			continue
		case class == types.POSITIVE && !pos:
			continue
		case class == types.UNTRIAGED && !unt:
			continue
		}
		ret = append(ret, DigestInfo{
			Test:   test,
			Digest: digest,
		})
	}

	return ret, nil
}

// makeSummary returns a Summary for the given digests.
func (s *Summaries) makeSummary(name string, e types.Expectations, diffStore diff.DiffStore, corpus string, digests []string) *Summary {
	pos := 0
	neg := 0
	unt := 0
	diamDigests := []string{}
	untHashes := []string{}
	testExp := e.TestExp()
	if expectations, ok := testExp[name]; ok {
		for _, digest := range digests {
			if dtype, ok := expectations[digest]; ok {
				switch dtype {
				case types.UNTRIAGED:
					unt += 1
					diamDigests = append(diamDigests, digest)
					untHashes = append(untHashes, digest)
				case types.NEGATIVE:
					neg += 1
				case types.POSITIVE:
					pos += 1
					diamDigests = append(diamDigests, digest)
				}
			} else {
				unt += 1
				diamDigests = append(diamDigests, digest)
				untHashes = append(untHashes, digest)
			}
		}
	} else {
		unt += len(digests)
		diamDigests = digests
		untHashes = digests
	}
	sort.Strings(diamDigests)
	sort.Strings(untHashes)
	return &Summary{
		Name: name,
		// TODO(jcgregorio) Make diameter faster, and also make the actual diameter
		// metric better. Until then disable it.  Diameter:  diameter(diamDigests,
		// diffStore),
		Diameter:  0,
		Pos:       pos,
		Neg:       neg,
		Untriaged: unt,
		UntHashes: untHashes,
		Num:       pos + neg + unt,
		Corpus:    corpus,
		Blame:     s.blamer.GetBlamesForTest(name),
	}
}

func diameter(digests []string, diffStore diff.DiffStore) int {
	// TODO Parallelize.
	lock := sync.Mutex{}
	max := 0
	wg := sync.WaitGroup{}
	for {
		if len(digests) <= 2 {
			break
		}
		wg.Add(1)
		go func(d1 string, d2 []string) {
			defer wg.Done()
			dms, err := diffStore.Get(diff.PRIORITY_NOW, d1, d2)
			if err != nil {
				sklog.Errorf("Unable to get diff: %s", err)
				return
			}
			localMax := 0
			for _, dm := range dms {
				diffMetrics := dm.(*diff.DiffMetrics)
				if diffMetrics.NumDiffPixels > localMax {
					localMax = diffMetrics.NumDiffPixels
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
