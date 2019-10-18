package mocks

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.skia.org/infra/go/vcsinfo"
)

// TODO(kjlubick): replace usages of this with the mockery based versions.

type MockVCSImpl struct {
	commits        []*vcsinfo.LongCommit
	depsFileMap    map[string]string
	pathContentMap map[string]string
}

// MockVCS returns an instance of VCS that returns the commits passed as
// arguments.
// To control the GetFile function use these two parameters:
//    depsContentMap maps commits from a hash to a dependency file.
//    pathContentMap maps file names to string content.
// Currently the GetFile function will only consider the fileName or the hash
// but not a combination of both. The fileName has priority.
func DeprecatedMockVCS(commits []*vcsinfo.LongCommit, depsContentMap map[string]string, pathContentMap map[string]string) vcsinfo.VCS {
	return MockVCSImpl{
		commits:        commits,
		depsFileMap:    depsContentMap,
		pathContentMap: pathContentMap,
	}
}

func (m MockVCSImpl) GetBranch() string                                        { return "" }
func (m MockVCSImpl) Update(ctx context.Context, pull, allBranches bool) error { return nil }
func (m MockVCSImpl) LastNIndex(N int) []*vcsinfo.IndexCommit                  { return nil }
func (m MockVCSImpl) Range(begin, end time.Time) []*vcsinfo.IndexCommit        { return nil }
func (m MockVCSImpl) IndexOf(ctx context.Context, hash string) (int, error) {
	return 0, nil
}
func (m MockVCSImpl) From(start time.Time) []string {
	idx := sort.Search(len(m.commits), func(i int) bool { return m.commits[i].Timestamp.Unix() >= start.Unix() })

	ret := make([]string, 0, len(m.commits)-idx)
	for _, commit := range m.commits[idx:] {
		ret = append(ret, commit.Hash)
	}
	return ret
}

func (m MockVCSImpl) Details(ctx context.Context, hash string, getBranches bool) (*vcsinfo.LongCommit, error) {
	for _, commit := range m.commits {
		if commit.Hash == hash {
			return commit, nil
		}
	}
	return nil, fmt.Errorf("Unable to find commit")
}

func (m MockVCSImpl) DetailsMulti(ctx context.Context, hashes []string, getBranches bool) ([]*vcsinfo.LongCommit, error) {
	return nil, nil
}

func (m MockVCSImpl) ByIndex(ctx context.Context, N int) (*vcsinfo.LongCommit, error) {
	return nil, nil
}

func (m MockVCSImpl) GetFile(ctx context.Context, fileName, commitHash string) (string, error) {
	// fileName must be non-empty to be considered.
	if ret, ok := m.pathContentMap[fileName]; (fileName != "") && ok {
		return ret, nil
	}
	// gitHash must be non-empty to be considered.
	if ret, ok := m.depsFileMap[commitHash]; (commitHash != "") && ok {
		return ret, nil
	}
	return "", fmt.Errorf("Unable to find file '%s' for commit '%s'", fileName, commitHash)
}
