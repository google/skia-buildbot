// package baseline contains functions to gather the current baseline and
// write them to GCS.
package baseline

import (
	"fmt"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
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

// DeepCopyBaseline returns a copy of the given instance of CommitableBaseline.
// Note: It assumes all members except for BaseLine to be immutable, thus only
// BaseLine is "deep" copied.
func (c *CommitableBaseLine) DeepCopyBaseline() *CommitableBaseLine {
	ret := &CommitableBaseLine{}
	*ret = *c
	ret.Baseline = c.Baseline.DeepCopy()
	return ret
}

// TODO(stephana): Add tests for GetBaselinePerCommit.

// GetBaselinesPerCommit calculates the baselines for each commit in the tile.
// If extraCommits is not empty they are assumed to be commits immediately following the commits
// in the given complex tile and BaseLines for these should be essentially copies of the last
// commit in the tile. This covers the case when the current tile is slightly behind all commits
// that have already been added to the repository.
func GetBaselinesPerCommit(exps types.Expectations, cpxTile *types.ComplexTile, extraCommits []*tiling.Commit) (map[string]*CommitableBaseLine, error) {
	allCommits := cpxTile.AllCommits()
	if len(allCommits) == 0 {
		return map[string]*CommitableBaseLine{}, nil
	}

	// Get the baselines for all commits for which we have data and that are not ignored.
	denseTile := cpxTile.GetTile(false)
	denseCommits := cpxTile.DataCommits()
	denseBaseLines := make(map[string]types.TestExp, len(denseCommits))

	// Initialize the expectations for all data commits
	for _, commit := range denseCommits {
		denseBaseLines[commit.Hash] = make(types.TestExp, len(denseTile.Traces))
	}

	// Sweep the tile and calculate the baselines.
	// For each trace we keep a running tally of triaged digests and for each
	// commit on the trace we add that tally to the baseline.
	for _, trace := range denseTile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		currDigests := map[string]types.Label{}
		testName := gTrace.Params_[types.PRIMARY_KEY_FIELD]
		for idx := 0; idx < len(denseCommits); idx++ {
			digest := gTrace.Values[idx]

			// If the digest is not missing then add the digest to the running list of digests.
			if digest != types.MISSING_DIGEST {
				if _, ok := currDigests[digest]; !ok {
					if cl := exps.Classification(testName, digest); cl == types.POSITIVE {
						currDigests[digest] = cl
					}
				}
			}

			if len(currDigests) > 0 {
				denseBaseLines[denseCommits[idx].Hash].AddDigests(testName, currDigests)
			}
		}
	}

	// Iterate over all commits. If the tile is sparse we substitute the expectations with the
	// expectations of the closest ancestor that has expecations. We also add the commits that
	// have landed already, but are not captured in the current tile.
	combined := allCommits
	if len(extraCommits) > 0 {
		combined = make([]*tiling.Commit, 0, len(allCommits)+len(extraCommits))
		combined = append(combined, allCommits...)
		combined = append(combined, extraCommits...)
	}

	ret := make(map[string]*CommitableBaseLine, len(combined))
	var currBL *CommitableBaseLine = nil
	for _, commit := range combined {
		bl, ok := denseBaseLines[commit.Hash]
		if ok {
			md5Sum, err := util.MD5Sum(bl)
			if err != nil {
				return nil, skerr.Fmt("Error calculating MD5 sum: %s", err)
			}

			ret[commit.Hash] = &CommitableBaseLine{
				StartCommit: commit,
				EndCommit:   commit,
				Total:       len(denseTile.Traces),
				Filled:      len(bl),
				Baseline:    bl,
				MD5:         md5Sum,
			}
			currBL = ret[commit.Hash]
		} else if currBL != nil {
			// Make a copy of the baseline of the previous commit and update the commit information.
			cpBL := *currBL
			cpBL.StartCommit = commit
			cpBL.EndCommit = commit
			ret[commit.Hash] = &cpBL
		} else {
			// Reaching this point means the first in the dense tile does not align with the first
			// commit of the sparse portion of the tile. This a sanity test and should only happen
			// in the presence of a programming error or data corruption.
			sklog.Errorf("Unable to get baseline for commit %s. It has not commits for immediate ancestors in tile.", commit.Hash)
		}
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
func GetBaselineForIssue(issueID int64, tryjobs []*tryjobstore.Tryjob, tryjobResults [][]*tryjobstore.TryjobResult, exp types.Expectations, commits []*tiling.Commit) (*CommitableBaseLine, error) {
	var startCommit *tiling.Commit = commits[len(commits)-1]
	var endCommit *tiling.Commit = commits[len(commits)-1]

	baseLine := types.TestExp{}
	for idx, tryjob := range tryjobs {
		for _, result := range tryjobResults[idx] {
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

// commitIssueBaseline commits the expectations for the given issue to the master baseline.
func commitIssueBaseline(issueID int64, user string, issueChanges types.TestExp, tryjobStore tryjobstore.TryjobStore, expStore expstorage.ExpectationsStore) error {
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
