package blame

import (
	"sort"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
	ptypes "go.skia.org/infra/perf/go/types"
)

// Blamer calculates blame lists by continously loading tiles
// and changed expectations.
type Blamer struct {
	// commits are the commits corresponding to the current blamelists.
	commits []*ptypes.Commit

	// testBlameLists are the blamelists keyed by testName and digest.
	testBlameLists map[string]map[string]*BlameDistribution

	storages *storage.Storage
	mutex    sync.Mutex
}

// BlameDistribution contains a rough estimation of the probabilities that
// a commit was responsible for the contained digest.
type BlameDistribution struct {
	// Freq contains likelihood counts that a commit was responsible
	// for the observed digest. The counts apply to the last len(Freq)
	// commits. An empty Freq slice means we cannot calculate a blame list.
	Freq []int `json:"freq"`
}

// New returns a new Blamer instance and error. The error is not
// nil if the first run of calculating the blame lists failed.
func New(storages *storage.Storage) (*Blamer, error) {
	ret := &Blamer{
		testBlameLists: map[string]map[string]*BlameDistribution{},
		storages:       storages,
	}

	// Process the first tile, schedule a background process.
	return ret, ret.processTileStream()
}

// processTileStream processes the first tile instantly and starts a background
// process to recalculate the blame lists as tiles and expectations change.
func (b *Blamer) processTileStream() error {
	expChanges := make(chan []string)
	b.storages.EventBus.SubscribeAsync(expstorage.EV_EXPSTORAGE_CHANGED, func(e interface{}) {
		expChanges <- e.([]string)
	})
	tileStream := b.storages.GetTileStreamNow(2*time.Minute, true)

	lastTile := <-tileStream
	if err := b.updateBlame(lastTile); err != nil {
		return err
	}

	// Schedule a background process to keep updating the blame lists.
	go func() {
		for {
			select {
			case tile := <-tileStream:
				if err := b.updateBlame(tile); err != nil {
					glog.Errorf("Error updating blame lists: %s", err)
				} else {
					lastTile = tile
				}
			case <-expChanges:
				storage.DrainChangeChannel(expChanges)
				if err := b.updateBlame(lastTile); err != nil {
					glog.Errorf("Error updating blame lists: %s", err)
				}
			}
		}
	}()

	return nil
}

func (b *Blamer) GetAllBlameLists() (map[string]map[string]*BlameDistribution, []*ptypes.Commit) {
	b.mutex.Lock()
	blameLists, commits := b.testBlameLists, b.commits
	b.mutex.Unlock()
	return blameLists, commits
}

// GetBlameList returns a blame list for the given test.
func (b *Blamer) GetBlameList(testName string) (map[string]*BlameDistribution, []*ptypes.Commit) {
	blameLists, commits := b.GetAllBlameLists()

	if ret, ok := blameLists[testName]; ok {
		return ret, commits
	}

	glog.Warningf("Unable to find blame lists for test: %s", testName)
	return map[string]*BlameDistribution{}, commits
}

// TODO(stephana): Remove all public functions other than GetBlame
// once blame is working on the front-end.

// GetBlame returns the indices of the provided list of commits that likely
// caused the given test name/digest pair. If the result is empty we are not
// able to determine blame, because the test name/digest appeared prior
// to the current tile.
func (b *Blamer) GetBlame(testName string, digest string, commits []*ptypes.Commit) []int {
	blameLists, blameCommits := b.GetAllBlameLists()

	blameDistribution, ok := blameLists[testName][digest]
	if !ok || (len(blameDistribution.Freq) == 0) {
		return []int{}
	}

	// We have a blamelist. Let's find the indices relative to the given
	// list of commits.
	freq := blameDistribution.Freq
	ret := make([]int, 0, len(freq))
	maxCount := util.MaxInt(freq...)

	// Find the first element in the list and align the commits.
	idx := 0
	for freq[idx] < maxCount {
		idx++
	}
	tgtCommit := blameCommits[len(blameCommits)-len(freq)+idx]
	commitIdx := sort.Search(len(commits), func(i int) bool { return commits[i].CommitTime >= tgtCommit.CommitTime })
	for (idx < len(freq)) && (freq[idx] > 0) && (commitIdx < len(commits)) {
		ret = append(ret, commitIdx)
		idx++
		commitIdx++
	}

	return ret
}

// updateBlame reads from the provided tileStream and updates the current
// blame lists.
func (b *Blamer) updateBlame(tile *ptypes.Tile) error {
	exp, err := b.storages.ExpectationsStore.Get()
	if err != nil {
		return err
	}

	defer timer.New("blame").Stop()

	// Note: blameStart and blameEnd are continously updated to contain the
	// smalles start and end index of the ranges for a testName/digest pair.
	blameStart := map[string]map[string]int{}
	blameEnd := map[string]map[string]int{}

	// blameRange stores the candidate ranges for a testName/digest pair.
	blameRange := map[string]map[string][][]int{}
	firstCommit := tile.Commits[0]

	tileLen := tile.LastCommitIndex() + 1
	for _, trace := range tile.Traces {
		gtr := trace.(*ptypes.GoldenTrace)
		testName := gtr.Params()[types.PRIMARY_KEY_FIELD]

		// lastIdx tracks the index of the last digest that is definitely
		// not in the blamelist.
		lastIdx := -1
		found := map[string]bool{}
		for idx, digest := range gtr.Values[:tileLen] {
			if digest == ptypes.MISSING_DIGEST {
				continue
			}

			status := exp.Classification(testName, digest)
			if (status == types.UNTRIAGED) && !found[digest] {
				found[digest] = true

				var startIdx int
				endIdx := idx

				// If we have only seen empty digests, then we do not
				// consider any digest before the current one.
				if lastIdx == -1 {
					startIdx = idx
				} else {
					startIdx = lastIdx + 1
				}

				// Get the info about this digest.
				digestInfo, err := b.storages.GetOrUpdateDigestInfo(testName, digest, tile.Commits[idx])
				if err != nil {
					return err
				}

				// If this digest was first seen outside the current tile
				// we cannot calculate a blamelist and set the commit range
				// to nil.
				var commitRange []int

				if digestInfo.First < firstCommit.CommitTime {
					commitRange = nil
				} else {
					commitRange = []int{startIdx, endIdx}
				}
				if blameStartFound, ok := blameStart[testName]; !ok {
					blameStart[testName] = map[string]int{digest: startIdx}
					blameEnd[testName] = map[string]int{digest: endIdx}
					blameRange[testName] = map[string][][]int{digest: [][]int{commitRange}}
				} else if currentStart, ok := blameStartFound[digest]; !ok {
					blameStart[testName][digest] = startIdx
					blameEnd[testName][digest] = endIdx
					blameRange[testName][digest] = [][]int{commitRange}
				} else {
					blameStart[testName][digest] = util.MinInt(currentStart, startIdx)
					blameEnd[testName][digest] = util.MinInt(blameEnd[testName][digest], endIdx)
					blameRange[testName][digest] = append(blameRange[testName][digest], commitRange)
				}
			}
			lastIdx = idx
		}
	}

	commits := tile.Commits[:tileLen]
	ret := make(map[string]map[string]*BlameDistribution, len(blameStart))

	for testName, digests := range blameRange {
		ret[testName] = make(map[string]*BlameDistribution, len(digests))
		for digest, commitRanges := range digests {
			start := blameStart[testName][digest]
			end := blameEnd[testName][digest]

			freq := make([]int, len(commits)-start)
			for _, commitRange := range commitRanges {
				// If the commit range is nil, we cannot calculate the a
				// blamelist.
				if commitRange == nil {
					freq = []int{}
					break
				}

				// Calculate the blame.
				idxEnd := util.MinInt(commitRange[0], end)
				for i := commitRange[0]; i <= idxEnd; i++ {
					freq[i-start]++
				}
			}

			ret[testName][digest] = &BlameDistribution{
				Freq: freq,
			}
		}
	}

	// Swap out the old blame lists for the new ones.
	b.mutex.Lock()
	b.testBlameLists, b.commits = ret, commits
	b.mutex.Unlock()
	return nil
}
