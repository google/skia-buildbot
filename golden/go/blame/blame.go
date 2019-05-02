package blame

import (
	"sort"
	"sync"

	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
)

// Blamer calculates blame lists by continuously loading tiles
// and changed expectations.
// It is not thread safe. The client of this package needs to make sure there
// are no conflicts.
type Blamer struct {
	// commits are the commits corresponding to the current blamelists.
	commits []*tiling.Commit

	// testBlameLists are the blamelists keyed by testName and digest.
	testBlameLists map[string]map[string]*BlameDistribution
	storages       *storage.Storage
	mutex          sync.Mutex
}

// BlameDistribution contains a rough estimation of the probabilities that
// a commit was responsible for the contained digest.
// We also use it as the output structure for the front end.
type BlameDistribution struct {
	// Freq contains likelihood counts that a commit was responsible
	// for the observed digest. The counts apply to the last len(Freq)
	// commits. When used as output structure in the GetBlame function
	// Freq contains the indices of commits.
	Freq []int `json:"freq"`

	// Old indicates whether the digest has been seen prior to the current
	// tile. In that case the blame might be unreliable.
	Old bool `json:"old"`
}

// WeightedBlame combines an authors name with a probability that she
// is on a blamelist this is aggregated over the digests of a test.
type WeightedBlame struct {
	Author string  `json:"author"`
	Prob   float64 `json:"prob"`
}

// Sorting wrapper around WeightedBlame.
type WeightedBlameSlice []*WeightedBlame

func (w WeightedBlameSlice) Len() int           { return len(w) }
func (w WeightedBlameSlice) Less(i, j int) bool { return w[i].Prob < w[j].Prob }
func (w WeightedBlameSlice) Swap(i, j int)      { w[i], w[j] = w[j], w[i] }

// New returns a new Blamer instance and error. The error is not
// nil if the first run of calculating the blame lists failed.
func New(storages *storage.Storage) *Blamer {
	return &Blamer{
		storages: storages,
	}
}

func (b *Blamer) GetAllBlameLists() (map[string]map[string]*BlameDistribution, []*tiling.Commit) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.testBlameLists, b.commits
}

// GetBlamesForTest returns the list of authors that have blame assigned to
// them for the given test.
func (b *Blamer) GetBlamesForTest(testName string) []*WeightedBlame {
	blameLists, commits := b.GetAllBlameLists()

	digestBlameList := blameLists[testName]
	total := 0.0
	blameMap := map[string]int{}
	for _, blameDistribution := range digestBlameList {
		commitIndices, maxCount := b.getBlame(blameDistribution, commits, commits)
		for _, commitIdx := range commitIndices {
			blameMap[commits[commitIdx].Author] += maxCount
		}
		total += float64(maxCount * len(commitIndices))
	}

	ret := make([]*WeightedBlame, 0, len(blameMap))
	for author, count := range blameMap {
		ret = append(ret, &WeightedBlame{author, float64(count) / total})
	}

	sort.Sort(sort.Reverse(WeightedBlameSlice(ret)))
	return ret
}

// TODO(stephana): Remove all public functions other than GetBlame
// once blame is working on the front-end and refactor BlameDistribution
// to be more obvious about the ways it is used (as intermediated and output
// format).

// GetBlame returns the indices of the provided list of commits that likely
// caused the given test name/digest pair. If the result is empty we are not
// able to determine blame, because the test name/digest appeared prior
// to the current tile.
func (b *Blamer) GetBlame(testName string, digest string, commits []*tiling.Commit) *BlameDistribution {
	blameLists, blameCommits := b.GetAllBlameLists()
	commitIndices, maxCount := b.getBlame(blameLists[testName][digest], blameCommits, commits)
	return &BlameDistribution{
		Freq: commitIndices,
		Old:  (maxCount != 0) && blameLists[testName][digest].Old,
	}
}

func (b *Blamer) getBlame(blameDistribution *BlameDistribution, blameCommits, commits []*tiling.Commit) ([]int, int) {
	if (blameDistribution == nil) || (len(blameDistribution.Freq) == 0) {
		return []int{}, 0
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

	return ret, maxCount
}

// Calculate calculates the blame list from the given tile.
func (b *Blamer) Calculate(tile *tiling.Tile) error {
	exp, err := b.storages.ExpectationsStore.Get()
	if err != nil {
		return err
	}

	defer timer.New("blame").Stop()

	// Note: blameStart and blameEnd are continuously updated to contain the
	// smallest start and end index of the ranges for a testName/digest pair.
	blameStart := map[string]map[string]int{}
	blameEnd := map[string]map[string]int{}

	// blameRange stores the candidate ranges for a testName/digest pair.
	blameRange := map[string]map[string][][]int{}
	tileLen := tile.LastCommitIndex() + 1
	ret := map[string]map[string]*BlameDistribution{}

	for _, trace := range tile.Traces {
		gtr := trace.(*types.GoldenTrace)
		testName := gtr.Params()[types.PRIMARY_KEY_FIELD]

		// lastIdx tracks the index of the last digest that is definitely
		// not in the blamelist.
		lastIdx := -1
		found := map[string]bool{}
		for idx, digest := range gtr.Digests[:tileLen] {
			if digest == types.MISSING_DIGEST {
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

				// Check if the digest was first seen outside the current tile.
				isOld := false
				commitRange := []int{startIdx, endIdx}
				if blameStartFound, ok := blameStart[testName]; !ok {
					blameStart[testName] = map[string]int{digest: startIdx}
					blameEnd[testName] = map[string]int{digest: endIdx}
					blameRange[testName] = map[string][][]int{digest: {commitRange}}
					ret[testName] = map[string]*BlameDistribution{digest: {Old: isOld}}
				} else if currentStart, ok := blameStartFound[digest]; !ok {
					blameStart[testName][digest] = startIdx
					blameEnd[testName][digest] = endIdx
					blameRange[testName][digest] = [][]int{commitRange}
					ret[testName][digest] = &BlameDistribution{Old: isOld}
				} else {
					blameStart[testName][digest] = util.MinInt(currentStart, startIdx)
					blameEnd[testName][digest] = util.MinInt(blameEnd[testName][digest], endIdx)
					blameRange[testName][digest] = append(blameRange[testName][digest], commitRange)
					ret[testName][digest].Old = isOld || ret[testName][digest].Old
				}
			}
			lastIdx = idx
		}
	}

	commits := tile.Commits[:tileLen]
	for testName, digests := range blameRange {
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
				idxEnd := util.MinInt(commitRange[1], end)
				for i := commitRange[0]; i <= idxEnd; i++ {
					freq[i-start]++
				}
			}

			ret[testName][digest].Freq = freq
		}
	}

	// Swap out the old blame lists for the new ones.
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.testBlameLists, b.commits = ret, commits
	return nil
}
