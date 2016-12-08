package stats

import (
	"bytes"
	"fmt"
	"sync"

	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

type Statistician struct {
	currTileStats *TileStats
	mutex         sync.RWMutex
}

func NewStatistician() *Statistician {
	return &Statistician{
		currTileStats: &TileStats{},
	}
}

func (s *Statistician) CalculateTileStats(tile *tiling.Tile) error {
	commits := tile.Commits
	uniqueDigestSet := util.StringSet{}
	totalDigests := 0
	digestsPerCommit := make([]int, len(commits))
	perCommitSets := make([]util.StringSet, len(commits))

	for idx := range commits {
		perCommitSets[idx] = util.StringSet{}
	}

	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		for idx, val := range gTrace.Values {
			if val != types.MISSING_DIGEST {
				uniqueDigestSet[val] = true
				totalDigests++
				digestsPerCommit[idx]++
				perCommitSets[idx][val] = true
			}
		}
	}

	uniquesPerCommit := make([]int, len(commits))
	for idx, digestSet := range perCommitSets {
		uniquesPerCommit[idx] = len(digestSet)
	}

	ret := &TileStats{
		DigestsPerCommit:       digestsPerCommit,
		UniqueDigestsPerCommit: uniquesPerCommit,
		Digests:                totalDigests,
		UniqueDigests:          len(uniqueDigestSet),
		TotalCells:             len(commits) * len(tile.Traces),
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.currTileStats = ret
	return nil
}

func (s *Statistician) GetTileStats() *TileStats {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.currTileStats
}

type TileStats struct {
	DigestsPerCommit       []int `json:"digestsPerCommit"`
	UniqueDigestsPerCommit []int `json:"uniqueDigestsPerCommit"`
	Digests                int   `json:"digests"`
	UniqueDigests          int   `json:"uniqueDigests"`
	TotalCells             int   `json:"totalCells"`
}

func (t *TileStats) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Total Cells               : %d\n", t.TotalCells)
	fmt.Fprintf(&buf, "Total Digests             : %d\n", t.Digests)
	fmt.Fprintf(&buf, "Unique Digests            : %d\n", t.UniqueDigests)
	fmt.Fprintf(&buf, "Per Commit Digests        : ")
	intsToString(&buf, t.DigestsPerCommit, 8)
	fmt.Fprintf(&buf, "Per Commit Unique Digests : ")
	intsToString(&buf, t.UniqueDigestsPerCommit, 8)
	return buf.String()
}

func intsToString(buf *bytes.Buffer, arr []int, spaces int) {
	fmtStr := fmt.Sprintf("%%%dd ", spaces)
	for _, i := range arr {
		fmt.Fprintf(buf, fmtStr, i)
	}
	buf.WriteString("\n")
}
