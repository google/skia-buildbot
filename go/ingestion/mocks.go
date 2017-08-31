package ingestion

import (
	"fmt"
	"sort"
	"time"

	"go.skia.org/infra/go/vcsinfo"
)

type mockVCS []*vcsinfo.LongCommit

// MockVCS returns an instance of VCS that returns the commits passed as
// arguments.
func MockVCS(commits []*vcsinfo.LongCommit) vcsinfo.VCS {
	return mockVCS(commits)
}

func (m mockVCS) Update(pull, allBranches bool) error               { return nil }
func (m mockVCS) LastNIndex(N int) []*vcsinfo.IndexCommit           { return nil }
func (m mockVCS) Range(begin, end time.Time) []*vcsinfo.IndexCommit { return nil }
func (m mockVCS) IndexOf(hash string) (int, error) {
	return 0, nil
}
func (m mockVCS) From(start time.Time) []string {
	idx := sort.Search(len(m), func(i int) bool { return m[i].Timestamp.Unix() >= start.Unix() })

	ret := make([]string, 0, len(m)-idx)
	for _, commit := range m[idx:] {
		ret = append(ret, commit.Hash)
	}
	return ret
}

func (m mockVCS) Details(hash string, getBranches bool) (*vcsinfo.LongCommit, error) {
	for _, commit := range m {
		if commit.Hash == hash {
			return commit, nil
		}
	}
	return nil, fmt.Errorf("Unable to find commit")
}

func (m mockVCS) ByIndex(N int) (*vcsinfo.LongCommit, error) {
	return nil, nil
}
