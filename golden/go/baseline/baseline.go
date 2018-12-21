// package baseline contains functions to gather the current baseline and
// write them to GCS.
package baseline

import (
	"fmt"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
)

// CommitableBaseLine captures the data necessary to verify test results on the
// commit queue.
type CommitableBaseLine struct {
	// Start commit covered by these baselines.
	StartCommit *tiling.Commit `json:"startCommit"`

	// Commit is the commit for which this baseline was collected.
	EndCommit *tiling.Commit `json:"endCommit"`

	// Baseline captures the baseline of the current commit.
	Baseline types.TestExp `json:"master"`

	// Issue indicates the Gerrit issue of this baseline. 0 indicates the master branch.
	Issue int64
}

// GetBaselineForMaster calculates the master baseline for the given configuration of
// expectations and the given tile. The commit of the baseline is last commit
// in tile.
func GetBaselineForMaster(exps types.Expectations, tile *tiling.Tile) *CommitableBaseLine {
	commits := tile.Commits
	if len(tile.Commits) == 0 {
		return nil
	}

	// Set the start and end commit in case there are no traces
	var startCommit *tiling.Commit = tile.Commits[0]
	var endCommit *tiling.Commit = tile.Commits[len(tile.Commits)-1]

	masterBaseline := types.TestExp{}
	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		testName := gTrace.Params_[types.PRIMARY_KEY_FIELD]
		if idx := gTrace.LastIndex(); idx >= 0 {
			digest := gTrace.Values[idx]
			if digest != types.MISSING_DIGEST && exps.Classification(testName, digest) == types.POSITIVE {
				masterBaseline.AddDigest(testName, digest, types.POSITIVE)
			}

			startCommit = minCommit(startCommit, commits[idx])
			endCommit = maxCommit(endCommit, commits[idx])
		}
	}

	ret := &CommitableBaseLine{
		StartCommit: startCommit,
		EndCommit:   endCommit,
		Baseline:    masterBaseline,
		Issue:       0,
	}
	return ret
}

// GetBaselineForIssue returns the baseline for the given issue. This baseline
// contains all triaged digests that are not in the master tile.
func GetBaselineForIssue(issueID int64, tryjobs []*tryjobstore.Tryjob, tryjobResults [][]*tryjobstore.TryjobResult, exp types.Expectations, commits []*tiling.Commit, talliesByTest map[string]tally.Tally) *CommitableBaseLine {
	var startCommit *tiling.Commit = nil
	var endCommit *tiling.Commit = nil

	baseLine := types.TestExp{}
	for idx, tryjob := range tryjobs {
		for _, result := range tryjobResults[idx] {
			// Ignore all digests that appear in the master.
			if _, ok := talliesByTest[result.TestName][result.Digest]; ok {
				continue
			}

			if result.Digest != types.MISSING_DIGEST && exp.Classification(result.TestName, result.Digest) == types.POSITIVE {
				baseLine.AddDigest(result.TestName, result.Digest, types.POSITIVE)
			}

			_, c := tiling.FindCommit(commits, tryjob.MasterCommit)
			startCommit = minCommit(startCommit, c)
			endCommit = maxCommit(endCommit, c)
		}
	}
	ret := &CommitableBaseLine{
		StartCommit: startCommit,
		EndCommit:   endCommit,
		Baseline:    baseLine,
		Issue:       issueID,
	}
	return ret
}

// CommitIssueBaseline commits the expectations for the given issue to the master baseline.
func CommitIssueBaseline(issueID int64, user string, issueChanges types.TestExp, tryjobStore tryjobstore.TryjobStore, expStore expstorage.ExpectationsStore) error {
	if len(issueChanges) == 0 {
		return nil
	}

	syntheticUser := fmt.Sprintf("%s:%d", user, issueID)

	commitFn := func() error {
		if err := expStore.AddChange(issueChanges, syntheticUser); err != nil {
			return sklog.FmtErrorf("Unable to add expectations for issue %d: %s", issueID, err)
		}
		return nil
	}

	return tryjobStore.CommitIssueExp(issueID, commitFn)
}

// minCommit returns newCommit if it appears before current (or current is nil).
func minCommit(current *tiling.Commit, newCommit *tiling.Commit) *tiling.Commit {
	if current == nil || newCommit == nil || newCommit.CommitTime < current.CommitTime {
		return newCommit
	}
	return current
}

// maxCommit returns newCommit if it appears after current (or current is nil).
func maxCommit(current *tiling.Commit, newCommit *tiling.Commit) *tiling.Commit {
	if current == nil || newCommit == nil || newCommit.CommitTime > current.CommitTime {
		return newCommit
	}
	return current
}
