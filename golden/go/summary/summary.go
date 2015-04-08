// summary summarizes the current state of triaging.
package summary

import (
	"fmt"
	"net/url"
	"sort"
	"sync"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tally"
	gtypes "go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/perf/go/types"
)

// Summary contains rolled up metrics for one test.
type Summary struct {
	Name      string `json:"name"`
	Diameter  int    `json:"diameter"`
	Pos       int    `json:"pos"`
	Neg       int    `json:"neg"`
	Untriaged int    `json:"untriaged"`
	Num       int    `json:"num"`
	Corpus    string `json:"corpus"`
}

// Summaries contains a Summary for each test.
//
// It also updates itself when Tallies have been updated.
type Summaries struct {
	storages  *storage.Storage
	mutex     sync.Mutex
	summaries map[string]*Summary
	tallies   *tally.Tallies
}

// New creates a new instance of Summaries.
func New(storages *storage.Storage, tallies *tally.Tallies) (*Summaries, error) {
	s := &Summaries{
		storages: storages,
		tallies:  tallies,
	}

	var err error
	s.summaries, err = s.CalcSummaries(nil, "", false, true)
	if err != nil {
		return nil, fmt.Errorf("Failed to calculate summaries in New: %s", err)
	}

	// TODO(jcgregorio) Move to a channel for tallies and then combine
	// this and the expStore handling into a single switch statement.
	tallies.OnChange(func() {
		summaries, err := s.CalcSummaries(nil, "", false, true)
		if err != nil {
			glog.Errorf("Failed to refresh summaries: %s", err)
			return
		}
		s.mutex.Lock()
		s.summaries = summaries
		s.mutex.Unlock()
	})

	ch := storages.ExpectationsStore.Changes()
	go func() {
		for {
			testNames := <-ch
			glog.Info("Updating summaries after expectations change.")
			partialSummaries, err := s.CalcSummaries(testNames, "", false, true)
			if err != nil {
				glog.Errorf("Failed to refresh summaries: %s", err)
				continue
			}
			s.mutex.Lock()
			for k, v := range partialSummaries {
				s.summaries[k] = v
			}
			s.mutex.Unlock()
		}
	}()
	return s, nil
}

func (s *Summaries) Get() map[string]*Summary {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.summaries
}

// TraceID is used to hold traces, along with their ids.
type TraceID struct {
	id string
	tr types.Trace
}

// CalcSummaries returns a Summary for each test that matches the given input filters.
//
// testNames
//   If not nil or empty then restrict the results to only tests that appear in this slice.
// query
//   URL encoded paramset to use for filtering.
// includeIgnores
//   Boolean, if true then include all digests in the results, including ones normally hidden
//   by the ignores list.
// head
//   Only consider digests at head if true.
//
func (s *Summaries) CalcSummaries(testNames []string, query string, includeIgnores bool, head bool) (map[string]*Summary, error) {
	defer timer.New("CalcSummaries").Stop()
	glog.Infof("CalcSummaries: includeIgnores %v head %v", includeIgnores, head)

	tile, err := s.storages.GetLastTileTrimmed()
	if err != nil {
		return nil, fmt.Errorf("Couldn't retrieve tile: %s", err)
	}
	q, err := url.ParseQuery(query)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse Query in CalcSummaries: %s", err)
	}

	// Decide the set of ignore filters we are using.
	t := timer.New("Gather Ignores")
	ignores := []url.Values{}
	if !includeIgnores {
		allIgnores, err := s.storages.IgnoreStore.List()
		if err != nil {
			return nil, fmt.Errorf("Failed to load ignores: %s", err)
		}
		for _, i := range allIgnores {
			q, _ := url.ParseQuery(i.Query)
			ignores = append(ignores, q)
		}
	}
	t.Stop()

	ret := map[string]*Summary{}

	e, err := s.storages.ExpectationsStore.Get()
	if err != nil {
		return nil, fmt.Errorf("Couldn't get expectations: %s", err)
	}

	// Filter down to just the traces we are interested in, based on query and ignores.
	filtered := map[string][]*TraceID{}
	t = timer.New("Filter Traces")
	for id, tr := range tile.Traces {
		name := tr.Params()[gtypes.PRIMARY_KEY_FIELD]
		if len(testNames) > 0 && !util.In(name, testNames) {
			continue
		}
		if types.MatchesWithIgnores(tr, q, ignores...) {
			if slice, ok := filtered[name]; ok {
				filtered[name] = append(slice, &TraceID{tr: tr, id: id})
			} else {
				filtered[name] = []*TraceID{&TraceID{tr: tr, id: id}}
			}
		}
	}
	t.Stop()

	traceTally := s.tallies.ByTrace()

	// Now create summaries for each test using the filtered set of traces.
	t = timer.New("Tally up the filtered traces")
	lastCommitIndex := tile.LastCommitIndex()
	for name, traces := range filtered {
		digests := map[string]bool{}
		corpus := ""
		for _, trid := range traces {
			corpus = trid.tr.Params()["source_type"]
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
					for k, _ := range *t {
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
		ret[name] = makeSummary(name, e, s.storages.DiffStore, corpus, util.KeysOfStringSet(digests))
	}
	t.Stop()

	return ret, nil
}

// makeSummary returns a Summary for the given digests.
func makeSummary(name string, e *expstorage.Expectations, diffStore diff.DiffStore, corpus string, digests []string) *Summary {
	pos := 0
	neg := 0
	unt := 0
	diamDigests := []string{}
	if expectations, ok := e.Tests[name]; ok {
		for _, digest := range digests {
			if dtype, ok := expectations[digest]; ok {
				switch dtype {
				case gtypes.UNTRIAGED:
					unt += 1
					diamDigests = append(diamDigests, digest)
				case gtypes.NEGATIVE:
					neg += 1
				case gtypes.POSITIVE:
					pos += 1
					diamDigests = append(diamDigests, digest)
				}
			} else {
				unt += 1
				diamDigests = append(diamDigests, digest)
			}
		}
	} else {
		unt += len(digests)
		diamDigests = digests
	}
	sort.Strings(diamDigests)
	return &Summary{
		Name: name,
		// TODO(jcgregorio) Make diameter faster, and also make the actual diameter
		// metric better. Until then disable it.  Diameter:  diameter(diamDigests,
		// diffStore),
		Diameter:  0,
		Pos:       pos,
		Neg:       neg,
		Untriaged: unt,
		Num:       pos + neg + unt,
		Corpus:    corpus,
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
			dms, err := diffStore.Get(d1, d2)
			if err != nil {
				glog.Errorf("Unable to get diff: %s", err)
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
