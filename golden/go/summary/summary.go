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
	s.summaries, err = s.CalcSummaries(nil, "", false, false)
	if err != nil {
		return nil, fmt.Errorf("Failed to calculate summaries in New: %s", err)
	}

	// TODO(jcgregorio) Move to a channel for tallies and then combine
	// this and the expStore handling into a single switch statement.
	tallies.OnChange(func() {
		summaries, err := s.CalcSummaries(nil, "", false, false)
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
			partialSummaries, err := s.CalcSummaries(testNames, "", false, false)
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

// SortableTrace is used to hold traces, along with their ids, for sorting.
type SortableTrace struct {
	id string
	tr types.Trace
}

type SortableTraceSlice []SortableTrace

func (p SortableTraceSlice) Len() int { return len(p) }
func (p SortableTraceSlice) Less(i, j int) bool {
	return p[i].tr.Params()[gtypes.PRIMARY_KEY_FIELD] < p[j].tr.Params()[gtypes.PRIMARY_KEY_FIELD]
}
func (p SortableTraceSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// CalcSummaries returns a Summary for each test that matches the given input filters.
//
// testNames
//   If not nil or empty then restrict the results to only tests that appear in this slice.
// query
//   URL encoded paramset to use for filtering.
// include
//   Boolean, if true then include all digests in the results, including ones normally hidden
//   by the ignores list.
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
	glog.Infof("%#v", ignores)
	// The corpus each test belongs to.
	corpus := map[string]string{}
	for _, tr := range tile.Traces {
		if test, ok := tr.Params()[gtypes.PRIMARY_KEY_FIELD]; ok {
			if corpusName, ok := tr.Params()["source_type"]; ok {
				corpus[test] = corpusName
			}
		}
	}
	ret := map[string]*Summary{}

	e, err := s.storages.ExpectationsStore.Get()
	if err != nil {
		return nil, fmt.Errorf("Couldn't get expectations: %s", err)
	}

	// Filter the traces, then sort and sum.
	filtered := []SortableTrace{}
	for id, tr := range tile.Traces {
		name := tr.Params()[gtypes.PRIMARY_KEY_FIELD]
		if len(testNames) > 0 && !util.In(name, testNames) {
			continue
		}
		if _, ok := corpus[name]; !ok {
			continue
		}
		if types.MatchesWithIgnores(tr, q, ignores...) {
			filtered = append(filtered, SortableTrace{
				id: id,
				tr: tr,
			})
		}
	}
	glog.Infof("Found %d matches", len(filtered))
	sort.Sort(SortableTraceSlice(filtered))
	traceTally := s.tallies.ByTrace()

	lastCommitIndex := tile.LastCommitIndex()
	lastTest := ""
	digests := map[string]bool{}
	for _, st := range filtered {
		if st.tr.Params()[gtypes.PRIMARY_KEY_FIELD] != lastTest {
			if lastTest != "" {
				ret[lastTest] = makeSummary(lastTest, e, s.storages.DiffStore, corpus[lastTest], util.KeysOfStringSet(digests))
				lastTest = st.tr.Params()[gtypes.PRIMARY_KEY_FIELD]
				digests = map[string]bool{}
			}
			lastTest = st.tr.Params()[gtypes.PRIMARY_KEY_FIELD]
		}
		if head {
			for i := lastCommitIndex; i >= 0; i-- {
				if st.tr.IsMissing(i) {
					continue
				} else {
					digests[st.tr.(*types.GoldenTrace).Values[i]] = true
					break
				}
			}
		} else {
			if t, ok := traceTally[st.id]; !ok {
				continue
			} else {
				for k, _ := range *t {
					digests[k] = true
				}
			}
		}
	}
	if lastTest != "" {
		ret[lastTest] = makeSummary(lastTest, e, s.storages.DiffStore, corpus[lastTest], util.KeysOfStringSet(digests))
	}

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
		// TODO(jcgregorio) Make diameter faster with better thumbnailing, and also make
		// the actual diameter metric better. Until then disable it.
		// Diameter:  diameter(diamDigests, diffStore),
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
