package analysis

import (
	"fmt"
	"time"

	"github.com/boltdb/bolt"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/vcsinfo"
)

var (
	beginningOfTime = time.Date(2014, time.June, 18, 0, 0, 0, 0, time.UTC)
)

type AnalysisStore interface {
	AddCommitInfo()
}

type analyzerStore struct {
	git            *gitinfo.GitInfo
	currentCommits []*vcsinfo.IndexCommit

	// Mappings
	mapGitHash IntStringMap
}

func newAnalyzerStore(baseDir string) (*analyzerStore, error) {
	git, err := gitinfo.NewGitInfo("", true, false)
	if err != nil {
		return nil, err
	}

	ret := &analyzerStore{
		git:            git,
		currentCommits: nil,
	}
	if err = ret.updateCommits(); err != nil {
		return nil, err
	}
	return ret, nil
}

func (a *analyzerStore) AddTile(tile *tiling.Tile) error {
	return nil
}

// updateCommits makes sure that currentCommits is up to date.
func (a *analyzerStore) updateCommits() error {
	startTime := beginningOfTime
	if a.currentCommits != nil {
		startTime = a.currentCommits[len(a.currentCommits)-1].Timestamp
	}
	newCommits := a.git.Range(startTime, time.Now())
	if len(newCommits) == 0 {
		return fmt.Errorf("Expected to find at least one commit.")
	}

	if (a.currentCommits != nil) && (newCommits[0].Hash != a.currentCommits[len(a.currentCommits)-1].Hash) {
		return fmt.Errorf("Previously found commits and new commits need to match up.")
	}

	// If we already have commits we don't need the first of the new commits.
	if a.currentCommits != nil {
		newCommits = newCommits[1:]
	}
	a.currentCommits = append(a.currentCommits, newCommits...)
	for _, commit := range newCommits {
		a.mapGitHash.IncAdd(commit.Hash)
	}
	return nil
}

type IntStringMap interface {
	Idx(string) uint32
	Val(uint32) string
	IncAdd(string) uint32
}

type boltIntStringMap struct {
}

func NewIntStringMap(db *bolt.DB, bucket []byte) (IntStringMap, error) {
	return &boltIntStringMap{}, nil
}

func (b *boltIntStringMap) Idx(string) uint32 {
	return 0
}

func (b *boltIntStringMap) Val(uint32) string {
	return ""
}

func (b *boltIntStringMap) IncAdd(string) uint32 {
	return 0
}
