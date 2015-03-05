package blame

import (
	"fmt"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/shared"
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

	storage *shared.Storage
	mutex   sync.Mutex
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
func New(storage *shared.Storage) (*Blamer, error) {
	ret := &Blamer{
		testBlameLists: map[string]map[string]*BlameDistribution{},
		storage:        storage,
	}

	// Process the first tile, schedule a background process.
	return ret, ret.processTileStream()
}

// processTileStream processes the first tile instantly and starts a background
// process to recalculate the blame lists as tiles and expectations change.
func (b *Blamer) processTileStream() error {
	// Get the tile stream and build the first blame lists synchronously.
	tileStreamCh := shared.GetTileStreamNow(b.storage.TileStore, 2*time.Minute)
	if err := b.updateBlame(tileStreamCh); err != nil {
		return err
	}

	// Schedule a background process to keep updating the blame lists.
	go func() {
		for {
			if err := b.updateBlame(tileStreamCh); err != nil {
				glog.Errorf("Error updating blame lists: %s", err)
			}
		}
	}()

	return nil
}

// GetBlameList returns a blame list for the given test.
func (b *Blamer) GetBlameList(testName string) (map[string]*BlameDistribution, []*ptypes.Commit) {
	b.mutex.Lock()
	blameLists, commits := b.testBlameLists, b.commits
	b.mutex.Unlock()

	if ret, ok := blameLists[testName]; ok {
		return ret, commits
	}

	glog.Warningf("Unable to find blame lists for test: %s", testName)
	return map[string]*BlameDistribution{}, commits
}

// updateBlame reads from the provided tileStream and updates the current
// blame lists.
func (b *Blamer) updateBlame(tileStreamCh <-chan *ptypes.Tile) error {
	// Read from the tile stream.
	tile := <-tileStreamCh
	if tile == nil {
		return fmt.Errorf("Unable to retrieve a tile.")
	}

	exp, err := b.storage.ExpectationsStore.Get()
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
			status := exp.Classification(testName, digest)
			if (status == types.UNTRIAGED) && !found[digest] {
				found[digest] = true

				// If this digest was first seen outside the current tile
				// we cannot calculate a blamelist and set the commit range
				// to nil.
				var commitRange []int
				digestInfo := b.storage.DigestStore.GetDigestInfo(testName, digest)
				if digestInfo.First < firstCommit.CommitTime {
					commitRange = nil
				} else {
					commitRange = []int{lastIdx + 1, idx}
				}
				if blameStartFound, ok := blameStart[testName]; !ok {
					blameStart[testName] = map[string]int{digest: lastIdx + 1}
					blameEnd[testName] = map[string]int{digest: idx}
					blameRange[testName] = map[string][][]int{digest: [][]int{commitRange}}
				} else if currentStart, ok := blameStartFound[digest]; !ok {
					blameStart[testName][digest] = lastIdx + 1
					blameEnd[testName][digest] = idx
					blameRange[testName][digest] = [][]int{commitRange}
				} else {
					blameStart[testName][digest] = util.MinInt(currentStart, idx)
					blameEnd[testName][digest] = util.MinInt(blameEnd[testName][digest], idx)
					blameRange[testName][digest] = append(blameRange[testName][digest], commitRange)
				}
			} else {
				lastIdx = idx
			}
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
				for i := commitRange[0]; i <= commitRange[1]; i++ {
					if i > end {
						break
					}
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
