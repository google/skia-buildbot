package gitstore

import (
	"context"
	"math"
	"time"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	// EV_NEW_GIT_COMMIT is the event that is fired when a previously unseen Git commit is available.
	EV_NEW_GIT_COMMIT = "gitstore:new-git-commit"
)

func startVCSTracker(gitStore GitStore, interval time.Duration, evt eventbus.EventBus, branch string, nCommits int) context.CancelFunc {
	ctx, cancelFn := context.WithCancel(context.Background())
	// Keep track of commits.
	var prevCommits []*vcsinfo.IndexCommit
	go util.RepeatCtx(interval, ctx, func() {
		ctx := context.TODO()
		allBranches, err := gitStore.GetBranches(ctx)
		if err != nil {
			sklog.Errorf("Error retrieving branches: %s", err)
			return
		}

		branchInfo, ok := allBranches[branch]
		if !ok {
			sklog.Errorf("Branch %s not found in gitstore", branch)
			return
		}

		startIdx := util.MaxInt(0, branchInfo.Index+1-nCommits)
		commits, err := gitStore.RangeN(ctx, startIdx, int(math.MaxInt32), branch)
		if err != nil {
			sklog.Errorf("Error getting last %d commits: %s", nCommits, err)
			return
		}

		// If we received new commits then publish an event and save them for the next round.
		if len(prevCommits) != len(commits) || commits[len(commits)-1].Index > prevCommits[len(prevCommits)-1].Index {
			prevCommits = commits
			cpCommits := append([]*vcsinfo.IndexCommit{}, commits...)
			evt.Publish(EV_NEW_GIT_COMMIT, cpCommits, false)
		}
	})
	return cancelFn
}
