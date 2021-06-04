package blame

import (
	"context"
	"sort"

	"go.opencensus.io/trace"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

// Blamer provides the results of blame calculations from a given
// tile and set of expectations. Specifically, blame is trying to identify
// who is responsible for Untriaged digests showing up (it essentially
// ignores positive/negative digests).
// A Blamer should be immutable after creation.
type Blamer interface {
	// GetBlamesForTest returns the list of WeightedBlame for the given test.
	GetBlamesForTest(testName types.TestName) []WeightedBlame

	// GetBlame returns the indices of the provided list of commits that likely
	// caused the given test name/digest pair. If the result is empty we are not
	// able to determine blame, because the test name/digest appeared prior
	// to the current tile.
	GetBlame(testName types.TestName, digest types.Digest, commits []tiling.Commit) BlameDistribution
}

// BlamerImpl implements the Blamer interface.
type BlamerImpl struct {
	// commits are the commits corresponding to the current blamelists.
	commits []tiling.Commit

	// blameLists are the blamelists keyed by testName and digest.
	blameLists map[types.TestName]map[types.Digest]blameCounts
}

// BlameDistribution contains the data about which commits
// might have introduced Untriaged digests.
// TODO(kjlubick): This type might not make it to the frontend at all, in which
// case, it should be deleted. Otherwise, perhaps we can directly return
// []tiling.Commit.
type BlameDistribution struct {
	// Freq contains the indices of commits that are to blame for this
	// Test producing the specified digest.
	Freq []int `json:"freq"`
}

// blameCounts contains likelihood counts that a commit was responsible
// for the observed digest. The counts apply to the last len(Freq)
// commits (i.e. the tail of the commits).
// Put another way,  it counts for how many traces
// the given digest was first seen at a particular commit.
type blameCounts []int

func (b *BlameDistribution) IsEmpty() bool {
	return len(b.Freq) == 0
}

// WeightedBlame combines an authors name with a probability that they
// are on a blamelist. This is aggregated over the digests of a test.
type WeightedBlame struct {
	Author string  `json:"author"`
	Prob   float64 `json:"prob"`
}

// Sorting wrapper around WeightedBlame.
type WeightedBlameSlice []WeightedBlame

func (w WeightedBlameSlice) Len() int { return len(w) }
func (w WeightedBlameSlice) Less(i, j int) bool {
	// Use author on tiebreaks (for determinism in tests)
	return w[i].Prob < w[j].Prob || (w[i].Prob == w[j].Prob && w[i].Author < w[j].Author)
}
func (w WeightedBlameSlice) Swap(i, j int) { w[i], w[j] = w[j], w[i] }

// New returns a new Blamer instance and error. The error is not
// nil if the first run of calculating the blame lists failed.
func New(tile *tiling.Tile, exp expectations.ReadOnly) (*BlamerImpl, error) {
	b := &BlamerImpl{}
	return b, b.calculate(tile, exp)
}

// GetBlamesForTest fulfills the Blamer interface.
func (b *BlamerImpl) GetBlamesForTest(testName types.TestName) []WeightedBlame {
	digestBlameList := b.blameLists[testName]
	total := 0.0
	blameMap := map[string]int{}
	for _, blameDistribution := range digestBlameList {
		commitIndices, maxCount := b.getBlame(blameDistribution, b.commits, b.commits)
		for _, commitIdx := range commitIndices {
			blameMap[b.commits[commitIdx].Author] += maxCount
		}
		total += float64(maxCount * len(commitIndices))
	}

	ret := make([]WeightedBlame, 0, len(blameMap))
	for author, count := range blameMap {
		ret = append(ret, WeightedBlame{
			Author: author,
			Prob:   float64(count) / total,
		})
	}

	sort.Sort(sort.Reverse(WeightedBlameSlice(ret)))
	return ret
}

// GetBlame fulfills the Blamer interface.
func (b *BlamerImpl) GetBlame(testName types.TestName, digest types.Digest, commits []tiling.Commit) BlameDistribution {
	commitIndices, _ := b.getBlame(b.blameLists[testName][digest], b.commits, commits)
	return BlameDistribution{
		Freq: commitIndices,
	}
}

func (b *BlamerImpl) getBlame(freq blameCounts, blameCommits, commits []tiling.Commit) ([]int, int) {
	if len(freq) == 0 {
		return []int{}, 0
	}

	// We have a blamelist. Let's find the indices relative to the given
	// list of commits.
	ret := make([]int, 0, len(freq))
	maxCount := util.MaxInt(freq...)

	// Find the first element in the list and align the commits.
	idx := 0
	for freq[idx] < maxCount {
		idx++
	}
	tgtCommit := blameCommits[len(blameCommits)-len(freq)+idx]
	commitIdx := sort.Search(len(commits), func(i int) bool {
		return commits[i].CommitTime.After(tgtCommit.CommitTime) || commits[i].CommitTime.Equal(tgtCommit.CommitTime)
	})
	for (idx < len(freq)) && (freq[idx] > 0) && (commitIdx < len(commits)) {
		ret = append(ret, commitIdx)
		idx++
		commitIdx++
	}

	return ret, maxCount
}

func (b *BlamerImpl) calculate(tile *tiling.Tile, exp expectations.ReadOnly) error {
	_, span := trace.StartSpan(context.TODO(), "blame_calculate")
	defer span.End()

	if len(tile.Commits) == 0 {
		return nil
	}

	// Note: blameStart and blameEnd are continuously updated to contain the
	// smallest start and end index of the ranges for a testName/digest pair.
	blameStart := map[types.TestName]map[types.Digest]int{}
	blameEnd := map[types.TestName]map[types.Digest]int{}

	// blameRange stores the candidate ranges for a testName/digest pair.
	blameRange := map[types.TestName]map[types.Digest][][]int{}
	tileLen := tile.LastCommitIndex() + 1
	ret := map[types.TestName]map[types.Digest]blameCounts{}

	for _, tr := range tile.Traces {
		testName := tr.TestName()

		// lastIdx tracks the index of the last digest that is definitely
		// not in the blamelist.
		lastIdx := -1
		found := types.DigestSet{}
		for idx, digest := range tr.Digests[:tileLen] {
			if digest == tiling.MissingDigest {
				continue
			}

			status := exp.Classification(testName, digest)
			if (status == expectations.Untriaged) && !found[digest] {
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
				commitRange := []int{startIdx, endIdx}
				if blameStartFound, ok := blameStart[testName]; !ok {
					blameStart[testName] = map[types.Digest]int{digest: startIdx}
					blameEnd[testName] = map[types.Digest]int{digest: endIdx}
					blameRange[testName] = map[types.Digest][][]int{digest: {commitRange}}
					ret[testName] = map[types.Digest]blameCounts{
						digest: nil,
					}
				} else if currentStart, ok := blameStartFound[digest]; !ok {
					blameStart[testName][digest] = startIdx
					blameEnd[testName][digest] = endIdx
					blameRange[testName][digest] = [][]int{commitRange}
					ret[testName][digest] = nil
				} else {
					blameStart[testName][digest] = util.MinInt(currentStart, startIdx)
					blameEnd[testName][digest] = util.MinInt(blameEnd[testName][digest], endIdx)
					blameRange[testName][digest] = append(blameRange[testName][digest], commitRange)
				}
			}
			lastIdx = idx
		}
	}

	// make a copy of the commits we hold onto, so as not to hold a reference
	// to the tile, preventing GC.
	commits := append([]tiling.Commit{}, tile.Commits[:tileLen]...)
	for testName, digests := range blameRange {
		for digest, commitRanges := range digests {
			start := blameStart[testName][digest]
			end := blameEnd[testName][digest]

			freq := make(blameCounts, len(commits)-start)
			for _, commitRange := range commitRanges {
				// If the commit range is nil, we cannot calculate the a
				// blamelist.
				if commitRange == nil {
					freq = blameCounts{}
					break
				}

				// Calculate the blame.
				idxEnd := util.MinInt(commitRange[1], end)
				for i := commitRange[0]; i <= idxEnd; i++ {
					freq[i-start]++
				}
			}
			ret[testName][digest] = freq
		}
	}

	// store the computations in the struct to be used by
	// the query methods (see Blamer interface).
	b.blameLists, b.commits = ret, commits
	return nil
}

// Make sure BlamerImpl fulfills the Blamer Interface
var _ Blamer = (*BlamerImpl)(nil)
