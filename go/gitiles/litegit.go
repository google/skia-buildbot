package gitiles

import (
	"context"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

type liteVCS struct {
	repo     *Repo
	gitStore GitStore
}

func NewGitInfo(repo *Repo, gitstore GitStore) (vcsinfo.VCS, error) {
	return nil, nil
}

func (li *liteVCS) Update(ctx context.Context, pull, allBranches bool) error {
	return nil
}

//
func (li *liteVCS) From(start time.Time) []string {
	now := time.Now()
	commits, err := li.gitStore.GetRangeCommits(start, now)
	if err != nil {
		sklog.Errorf("Error retrieving commits for range %s to %s. Got: %s", start, now, err)
		return []string{}
	}

	// Lazily load the logs that we don't have in RAM yet.
	return commits
}

func (li *liteVCS) Details(ctx context.Context, hash string, includeBranchInfo bool) (*vcsinfo.LongCommit, error) {
	return li.gitStore.GetCommit(hash)
}

func (li *liteVCS) LastNIndex(N int) []*vcsinfo.IndexCommit {
	return li.gitStore.RangeN(true)
}

func (li *liteVCS) Range(begin, end time.Time) []*vcsinfo.IndexCommit { return nil }

func (li *liteVCS) IndexOf(ctx context.Context, hash string) (int, error) { return 0, nil }

func (li *liteVCS) ByIndex(ctx context.Context, N int) (*vcsinfo.LongCommit, error) { return nil, nil }

func (li *liteVCS) GetFile(ctx context.Context, fileName, commitHash string) (string, error) {
	return "", nil
}

func (li *liteVCS) ResolveCommit(ctx context.Context, commitHash string) (string, error) {
	return "", nil
}

var _ vcsinfo.VCS = (*Repo)(nil)
