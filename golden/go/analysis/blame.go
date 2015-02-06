package analysis

import (
	"sync"

	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/golden/go/types"
	ptypes "skia.googlesource.com/buildbot.git/perf/go/types"
)

// GUIBlameLists contains the blame lists for tests and their
// untriaged digests.
type GUIBlameLists struct {
	// Commits of the currently labeled tile.
	Commits []*ptypes.Commit `json:"commits"`

	// Blames are the blame distributions keyed by test name.
	Blames map[string][]*BlameDistribution `json:"blames"`
}

// BlameDistribution contains a rough estimation of the probabilities that
// a commit was responsible for the contained digest.
type BlameDistribution struct {
	// Digest under consideration.
	Digest string

	// Freq contains likelihood counts that a commit was responsible
	// for the observed digest. It starts with the first possible commitId
	// that could have caused the digest and spans to the end the commit range.
	Freq []int
}

type jobResult struct {
	testName      string
	distributions []*BlameDistribution
}

// getBlameLists calculates the blame lists for the given labeled tile in
// parallel.
func getBlameLists(labeledTile *LabeledTile) *GUIBlameLists {
	var wg sync.WaitGroup
	results := make(chan jobResult, len(labeledTile.Traces))
	for testName, traces := range labeledTile.Traces {

		wg.Add(1)
		go func(testName string, traces []*LabeledTrace) {
			blameCommitRanges := map[string][][]int{}
			blameStartCommitIds := map[string]int{}
			blameEndCommitIds := map[string]int{}
			for _, t := range traces {
				first := -1
				found := map[string]bool{}
				for idx, digest := range t.Digests {
					if t.Labels[idx] == types.UNTRIAGED {
						if _, ok := found[digest]; !ok {
							// We have not seen this digest in this trace before.
							found[digest] = true
							if currentEnd, ok := blameEndCommitIds[digest]; !ok {
								blameEndCommitIds[digest] = t.CommitIds[idx]
								blameStartCommitIds[digest] = first + 1
								blameCommitRanges[digest] = [][]int{[]int{first + 1, t.CommitIds[idx]}}
							} else {
								blameEndCommitIds[digest] = util.MinInt(currentEnd, t.CommitIds[idx])
								blameStartCommitIds[digest] = util.MinInt(blameStartCommitIds[digest], first+1)
								blameCommitRanges[digest] = append(blameCommitRanges[digest], []int{first + 1, t.CommitIds[idx]})
							}
						} else {
							first = t.CommitIds[idx]
						}
					} else {
						first = t.CommitIds[idx]
					}
				}
			}

			results <- jobResult{testName, calcBlameDistributions(labeledTile.Commits, blameCommitRanges, blameStartCommitIds, blameEndCommitIds)}
			wg.Done()
		}(testName, traces)
	}
	wg.Wait()
	close(results)

	blames := make(map[string][]*BlameDistribution, len(labeledTile.Traces))
	for r := range results {
		blames[r.testName] = r.distributions
	}

	return &GUIBlameLists{
		Commits: labeledTile.Commits,
		Blames:  blames,
	}
}

// calcBlameDistributions returns the accumulated counts for the commits in a blamelist.
func calcBlameDistributions(commits []*ptypes.Commit, blameCommitRanges map[string][][]int, blameStartCommitIds map[string]int, blameEndCommitIds map[string]int) []*BlameDistribution {
	result := make([]*BlameDistribution, 0, len(blameCommitRanges))
	for digest, commitRanges := range blameCommitRanges {
		startCommitId := blameStartCommitIds[digest]
		endCommitId := blameEndCommitIds[digest]
		freq := make([]int, len(commits)-startCommitId)
		for _, oneCommitRange := range commitRanges {
			for i := oneCommitRange[0]; i <= oneCommitRange[1]; i++ {
				if i <= endCommitId {
					freq[i-startCommitId]++
				} else {
					break
				}
			}
		}
		result = append(result, &BlameDistribution{
			Digest: digest,
			Freq:   freq,
		})
	}
	return result
}
