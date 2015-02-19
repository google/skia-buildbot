// summary summarizes the current state of triaging.
package summary

import (
	"fmt"
	"sort"
	"sync"

	"github.com/skia-dev/glog"
	"skia.googlesource.com/buildbot.git/go/timer"
	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/golden/go/diff"
	"skia.googlesource.com/buildbot.git/golden/go/expstorage"
	"skia.googlesource.com/buildbot.git/golden/go/tally"
	gtypes "skia.googlesource.com/buildbot.git/golden/go/types"
	"skia.googlesource.com/buildbot.git/perf/go/types"
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
	mutex     sync.Mutex
	summaries map[string]*Summary
	ts        types.TileStore
	expStore  expstorage.ExpectationsStore
	tallies   *tally.Tallies
	diffStore diff.DiffStore
}

func New(ts types.TileStore, expStore expstorage.ExpectationsStore, tallies *tally.Tallies, diffStore diff.DiffStore) (*Summaries, error) {
	summaries, err := calcSummaries(ts, expStore, tallies, diffStore, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to calculate summaries in New: %s", err)
	}
	s := &Summaries{
		summaries: summaries,
		ts:        ts,
		expStore:  expStore,
		tallies:   tallies,
		diffStore: diffStore,
	}
	// TODO(jcgregorio) Move to a channel for tallies and then combine
	// this and the expStore handling into a single switch statement.
	tallies.OnChange(func() {
		summaries, err := calcSummaries(ts, expStore, tallies, diffStore, nil)
		if err != nil {
			glog.Errorf("Failed to refresh summaries: %s", err)
			return
		}
		s.mutex.Lock()
		s.summaries = summaries
		s.mutex.Unlock()
	})

	ch := expStore.Changes()
	go func() {
		for {
			testNames := <-ch
			glog.Info("Updating summaries after expectations change.")
			partialSummaries, err := calcSummaries(ts, expStore, tallies, diffStore, testNames)
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

func calcSummaries(ts types.TileStore, expStore expstorage.ExpectationsStore, tallies *tally.Tallies, diffStore diff.DiffStore, testNames []string) (map[string]*Summary, error) {
	defer timer.New("calcSummaries").Stop()

	tile, err := ts.Get(0, -1)
	if err != nil {
		return nil, fmt.Errorf("Couldn't retrieve tile: %s", err)
	}

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

	e, err := expStore.Get()
	if err != nil {
		return nil, fmt.Errorf("Couldn't get expectations: %s", err)
	}

	testTally := tallies.ByTest()
	for _, name := range tile.ParamSet[gtypes.PRIMARY_KEY_FIELD] {
		if testNames != nil && !util.In(name, testNames) {
			continue
		}
		digests := make([]string, 0, len(*(testTally[name])))

		pos := 0
		neg := 0
		unt := 0
		expectations, ok := e.Tests[name]
		if ok {
			for digest, _ := range *(testTally[name]) {
				if dtype, ok := expectations[digest]; ok {
					switch dtype {
					case gtypes.UNTRIAGED:
						unt += 1
						digests = append(digests, digest)
					case gtypes.NEGATIVE:
						neg += 1
					case gtypes.POSITIVE:
						pos += 1
						digests = append(digests, digest)
					}
				} else {
					unt += 1
					digests = append(digests, digest)
				}
			}
		} else {
			unt += len(*(testTally[name]))
			for digest, _ := range *(testTally[name]) {
				digests = append(digests, digest)
			}
		}
		sort.Strings(digests)
		ret[name] = &Summary{
			Name:      name,
			Diameter:  diameter(digests, diffStore),
			Pos:       pos,
			Neg:       neg,
			Untriaged: unt,
			Num:       pos + neg + unt,
			Corpus:    corpus[name],
		}
	}

	return ret, nil
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
