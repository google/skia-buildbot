package blame

import (
	"sort"

	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/types"
)

// Blamer provides the results of blame calculations from a given
// tile and set of expectations. Specifically, blame is trying to identify
// who is responsible for UNTRIAGED digests showing up (it essentially
// ignores positive/negative digests).
// A Blamer should be immutable after creation.
type Blamer interface {
	// GetAllBlameLists returns all BlameLists that have been computed.
	GetAllBlameLists() (map[types.TestName]map[types.Digest]BlameDistribution, []*tiling.Commit)

	// GetBlamesForTest returns the list of WeightedBlame for the given test.
	GetBlamesForTest(testName types.TestName) []WeightedBlame

	// GetBlame returns the indices of the provided list of commits that likely
	// caused the given test name/digest pair. If the result is empty we are not
	// able to determine blame, because the test name/digest appeared prior
	// to the current tile.
	GetBlame(testName types.TestName, digest types.Digest, commits []*tiling.Commit) BlameDistribution
}

// BlamerImpl implements the Blamer interface.
type BlamerImpl struct {
	// commits are the commits corresponding to the current blamelists.
	commits []*tiling.Commit

	// testBlameLists are the blamelists keyed by testName and digest.
	testBlameLists map[types.TestName]map[types.Digest]BlameDistribution
}

// BlameDistribution contains a rough estimation of the probabilities that
// a commit was responsible for the contained digest.
// We also use it as the output structure for the front end.
type BlameDistribution struct {
	// Freq contains likelihood counts that a commit was responsible
	// for the observed digest. The counts apply to the last len(Freq)
	// commits. When used as output structure in the GetBlame function
	// Freq contains the indices of commits.
	// TODO(kjlubick): What does this actually represent?  The comment
	// above says two conflicting things. Can freq be anything other than
	// length 0 or 1?
	// dogben says: "When Freq contains counts, the array lines up with the
	// tail of the associated slice of commits. It counts for how many traces
	// the given digest was first seen at a particular commit.
	// When Freq contains commit indices, it's essentially just a slice of
	// commits. Any of those commits might be to blame, and there's no
	// probability associated with them."
	// TODO(kjlubick): refactor this into two structs, one for each condition
	// mentioned by dogben.
	Freq []int `json:"freq"`

	// Old indicates whether the digest has been seen prior to the current
	// tile. In that case the blame might be unreliable.
	Old bool `json:"old"`
}

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

func (w WeightedBlameSlice) Len() int           { return len(w) }
func (w WeightedBlameSlice) Less(i, j int) bool { return w[i].Prob < w[j].Prob }
func (w WeightedBlameSlice) Swap(i, j int)      { w[i], w[j] = w[j], w[i] }

// New returns a new Blamer instance and error. The error is not
// nil if the first run of calculating the blame lists failed.
func New(tile *tiling.Tile, exp types.Expectations) (*BlamerImpl, error) {
	b := &BlamerImpl{}
	return b, b.calculate(tile, exp)
}

// GetAllBlameLists fulfills the Blamer interface.
func (b *BlamerImpl) GetAllBlameLists() (map[types.TestName]map[types.Digest]BlameDistribution, []*tiling.Commit) {
	return b.testBlameLists, b.commits
}

// GetBlamesForTest fulfills the Blamer interface.
func (b *BlamerImpl) GetBlamesForTest(testName types.TestName) []WeightedBlame {
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

// TODO(stephana): Remove all public functions other than GetBlame
// once blame is working on the front-end and refactor BlameDistribution
// to be more obvious about the ways it is used (as intermediated and output
// format).

// GetBlame fulfills the Blamer interface.
func (b *BlamerImpl) GetBlame(testName types.TestName, digest types.Digest, commits []*tiling.Commit) BlameDistribution {
	blameLists, blameCommits := b.GetAllBlameLists()
	commitIndices, maxCount := b.getBlame(blameLists[testName][digest], blameCommits, commits)
	return BlameDistribution{
		Freq: commitIndices,
		Old:  (maxCount != 0) && blameLists[testName][digest].Old,
	}
}

func (b *BlamerImpl) getBlame(blameDistribution BlameDistribution, blameCommits, commits []*tiling.Commit) ([]int, int) {
	if len(blameDistribution.Freq) == 0 {
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

func (b *BlamerImpl) calculate(tile *tiling.Tile, exp types.Expectations) error {
	defer shared.NewMetricsTimer("blame_calculate").Stop()

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
	ret := map[types.TestName]map[types.Digest]BlameDistribution{}

	for _, trace := range tile.Traces {
		gtr := trace.(*types.GoldenTrace)
		testName := gtr.TestName()

		// lastIdx tracks the index of the last digest that is definitely
		// not in the blamelist.
		lastIdx := -1
		found := types.DigestSet{}
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
					blameStart[testName] = map[types.Digest]int{digest: startIdx}
					blameEnd[testName] = map[types.Digest]int{digest: endIdx}
					blameRange[testName] = map[types.Digest][][]int{digest: {commitRange}}
					ret[testName] = map[types.Digest]BlameDistribution{digest: {Old: isOld}}
				} else if currentStart, ok := blameStartFound[digest]; !ok {
					blameStart[testName][digest] = startIdx
					blameEnd[testName][digest] = endIdx
					blameRange[testName][digest] = [][]int{commitRange}
					ret[testName][digest] = BlameDistribution{Old: isOld}
				} else {
					blameStart[testName][digest] = util.MinInt(currentStart, startIdx)
					blameEnd[testName][digest] = util.MinInt(blameEnd[testName][digest], endIdx)
					blameRange[testName][digest] = append(blameRange[testName][digest], commitRange)
					bd := ret[testName][digest]
					bd.Old = isOld || bd.Old
					ret[testName][digest] = bd
				}
			}
			lastIdx = idx
		}
	}

	// make a copy of the commits we hold onto, so as not to hold a reference
	// to the tile, preventing GC.
	commits := append([]*tiling.Commit{}, tile.Commits[:tileLen]...)
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
			bd := ret[testName][digest]
			bd.Freq = freq
			ret[testName][digest] = bd
		}
	}

	// store the computations in the struct to be used by
	// the query methods (see Blamer interface).
	b.testBlameLists, b.commits = ret, commits
	return nil
}

// Make sure BlamerImpl fulfills the Blamer Interface
var _ Blamer = (*BlamerImpl)(nil)
