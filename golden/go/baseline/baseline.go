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

// TODO(stephana): Baseline is similar to the expstorage.Expecations type and
// both are based on map[string]types.TestClassification
// These should all be merged into one type to handle expectations and
// expectation changes

// Baseline maps test names to a set of digests with their label.
// Test names are the names of individual tests, e.g. GMs and digests are
// hashes that uniquely identify an output image.
// This is used as the export format of baseline.
type Baseline map[string]types.TestClassification

// Merge merges the right baseline into the receiver and returns the receiver
func (b Baseline) Merge(right Baseline) Baseline {
	for testName, digests := range right {
		if found, ok := b[testName]; ok {
			for d, l := range digests {
				found[d] = l
			}
		} else {
			b[testName] = digests.DeepCopy()
		}
	}
	return b
}

// DeepCopy creates a copy of this this baseline.
func (b Baseline) DeepCopy() Baseline {
	ret := make(map[string]types.TestClassification, len(b))
	for testName, digests := range b {
		ret[testName] = digests.DeepCopy()
	}
	return ret
}

// add adds a test/digest pair to the baseline.
func (b Baseline) add(testName, digest string) {
	if (testName == "") || (digest == types.MISSING_DIGEST) {
		return
	}
	if found, ok := b[testName]; ok {
		found[digest] = types.POSITIVE
	} else {
		b[testName] = types.TestClassification{digest: types.POSITIVE}
	}
}

// CommitableBaseLine captures the data necessary to verify test results on the
// commit queue.
type CommitableBaseLine struct {
	// Start commit covered by these baselines.
	StartCommit *tiling.Commit `json:"startCommit"`

	// Commit is the commit for which this baseline was collected.
	EndCommit *tiling.Commit `json:"endCommit"`

	// Baseline captures the baseline of the current commit.
	Baseline Baseline `json:"master"`

	// Issue indicates the Gerrit issue of this baseline. 0 indicates the master branch.
	Issue int64
}

// GetBaselineForMaster calculates the master baseline for the given configuration of
// expectations and the given tile. The commit of the baseline is last commit
// in tile.
func GetBaselineForMaster(exps *expstorage.Expectations, tile *tiling.Tile) *CommitableBaseLine {
	commits := tile.Commits
	var startCommit *tiling.Commit = nil
	var endCommit *tiling.Commit = nil

	masterBaseline := Baseline{}
	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		testName := gTrace.Params_[types.PRIMARY_KEY_FIELD]
		if idx := gTrace.LastIndex(); idx >= 0 {
			digest := gTrace.Values[idx]
			if exps.Classification(testName, digest) == types.POSITIVE {
				masterBaseline.add(testName, digest)
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
func GetBaselineForIssue(issueID int64, tryjobs []*tryjobstore.Tryjob, tryjobResults [][]*tryjobstore.TryjobResult, exp *expstorage.Expectations, commits []*tiling.Commit, talliesByTest map[string]tally.Tally) *CommitableBaseLine {
	var startCommit *tiling.Commit = nil
	var endCommit *tiling.Commit = nil

	baseLine := Baseline{}
	for idx, tryjob := range tryjobs {
		for _, result := range tryjobResults[idx] {
			// Ignore all digests that appear in the master.
			if _, ok := talliesByTest[result.TestName][result.Digest]; ok {
				continue
			}

			if exp.Classification(result.TestName, result.Digest) == types.POSITIVE {
				baseLine.add(result.TestName, result.Digest)
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
func CommitIssueBaseline(issueID int64, user string, issueChanges map[string]types.TestClassification, tryjobStore tryjobstore.TryjobStore, expStore expstorage.ExpectationsStore) error {
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
