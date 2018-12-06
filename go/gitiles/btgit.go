package gitiles

import (
	"time"

	"go.skia.org/infra/go/vcsinfo"
)

type GitStore interface {
	GetRangeCommits(start, end time.Time) ([]string, error)
	GetCommit(hash string) (*vcsinfo.LongCommit, error)
}

func BTGitStore(projectID, btInstanceID, tableID string) (GitStore, error) {
	return nil, nil
}
