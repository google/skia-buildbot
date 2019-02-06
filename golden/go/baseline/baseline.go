// package baseline contains functions to gather the current baseline and
// write them to GCS.
package baseline

import (
	"fmt"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
	"golang.org/x/sync/errgroup"
)

// md5SumEmptyExp is the MD5 sum of an empty expectation.
var md5SumEmptyExp = fileutil.Must(util.MD5Sum(types.TestExp{}))

// CommitableBaseLine captures the data necessary to verify test results on the
// commit queue.
type CommitableBaseLine struct {
	// StartCommit covered by these baselines.
	StartCommit *tiling.Commit `json:"startCommit"`

	// EncCommit is the commit for which this baseline was collected.
	EndCommit *tiling.Commit `json:"endCommit"`

	// CommitDelta is the difference in index within the commits of a tile.
	CommitDelta int `json:"commitDelta"`

	// Total is the total number of traces that were iterated when generating the baseline.
	Total int `json:"total"`

	// Filled is the number of traces that had non-empty values at EndCommit.
	Filled int `json:"filled"`

	// MD5 is the hash of the Baseline field.
	MD5 string `json:"md5"`

	// Baseline captures the baseline of the current commit.
	Baseline types.TestExp `json:"master"`

	// Issue indicates the Gerrit issue of this baseline. 0 indicates the master branch.
	Issue int64
}

// TODO(stephana): Add tests for GetBaselinePerCommit.

// GetBaselinesPerCommit calculates the baselines for each commit in the tile.
func GetBaselinesPerCommit(exps types.Expectations, tile *tiling.Tile) (map[string]*CommitableBaseLine, error) {
	commits := tile.Commits
	if len(tile.Commits) == 0 {
		return map[string]*CommitableBaseLine{}, nil
	}

	perCommitBaselines := make([]*CommitableBaseLine, len(tile.Commits))
	var egroup errgroup.Group

	for cIdx := range commits {
		func(cIdx int) {
			egroup.Go(func() error {
				startCommitIdx := cIdx
				masterBaseline := types.TestExp{}
				filled := 0
				for _, trace := range tile.Traces {
					gTrace := trace.(*types.GoldenTrace)
					testName := gTrace.Params_[types.PRIMARY_KEY_FIELD]
					digest := types.MISSING_DIGEST
					idx := util.MinInt(cIdx, gTrace.LastIndex())
					for ; idx >= 0; idx-- {
						digest = gTrace.Values[idx]
						if digest != types.MISSING_DIGEST {
							break
						}
					}

					if digest != types.MISSING_DIGEST && exps.Classification(testName, digest) == types.POSITIVE {
						masterBaseline.AddDigest(testName, digest, types.POSITIVE)
					}

					if idx == cIdx {
						filled++
					}
					startCommitIdx = util.MaxInt(0, util.MinInt(startCommitIdx, idx))
				}

				md5Sum, err := util.MD5Sum(masterBaseline)
				if err != nil {
					return skerr.Fmt("Error calculating MD5 sum: %s", err)
				}

				perCommitBaselines[cIdx] = &CommitableBaseLine{
					StartCommit: commits[startCommitIdx],
					EndCommit:   commits[cIdx],
					CommitDelta: cIdx - startCommitIdx,
					Total:       len(tile.Traces),
					Filled:      filled,
					Baseline:    masterBaseline,
					Issue:       0,
					MD5:         md5Sum,
				}
				return nil
			})
		}(cIdx)
	}
	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	ret := make(map[string]*CommitableBaseLine, len(commits))
	for idx, bLine := range perCommitBaselines {
		ret[commits[idx].Hash] = bLine
	}
	return ret, nil
}

// EmptyBaseline returns an instance of CommitableBaseline with the provided commits and nil
// values in all other fields. The Baseline field contains an empty instance of types.TestExp.
func EmptyBaseline(startCommit, endCommit *tiling.Commit) *CommitableBaseLine {
	return &CommitableBaseLine{
		StartCommit: startCommit,
		EndCommit:   endCommit,
		Baseline:    types.TestExp{},
		MD5:         md5SumEmptyExp,
	}
}

// GetBaselineForIssue returns the baseline for the given issue. This baseline
// contains all triaged digests that are not in the master tile.
func GetBaselineForIssue(issueID int64, tryjobs []*tryjobstore.Tryjob, tryjobResults [][]*tryjobstore.TryjobResult, exp types.Expectations, commits []*tiling.Commit, talliesByTest map[string]tally.Tally) (*CommitableBaseLine, error) {
	var startCommit *tiling.Commit = commits[len(commits)-1]
	var endCommit *tiling.Commit = commits[len(commits)-1]

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

	md5Sum, err := util.MD5Sum(baseLine)
	if err != nil {
		return nil, skerr.Fmt("Error calculating MD5 sum: %s", err)
	}

	// Note: CommitDelta, Total and Filled are not relevant for an issue baseline since there
	// are not traces and commits directly related to this.

	// TODO(stephana): Review whether StartCommit and EndCommit are useful here.

	ret := &CommitableBaseLine{
		StartCommit: startCommit,
		EndCommit:   endCommit,
		Baseline:    baseLine,
		Issue:       issueID,
		MD5:         md5Sum,
	}
	return ret, nil
}

// CommitIssueBaseline commits the expectations for the given issue to the master baseline.
func CommitIssueBaseline(issueID int64, user string, issueChanges types.TestExp, tryjobStore tryjobstore.TryjobStore, expStore expstorage.ExpectationsStore) error {
	if len(issueChanges) == 0 {
		return nil
	}

	syntheticUser := fmt.Sprintf("%s:%d", user, issueID)

	commitFn := func() error {
		if err := expStore.AddChange(issueChanges, syntheticUser); err != nil {
			return skerr.Fmt("Unable to add expectations for issue %d: %s", issueID, err)
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
