package baseline

import (
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/types"
)

// CommitableBaseLine captures the data necessary to verify test results on the
// commit queue.  A baseline is essentially the expectations for a set of
// tests in a given commit range.
type CommitableBaseLine struct {
	// StartCommit covered by these baselines.
	StartCommit *tiling.Commit `json:"startCommit"`

	// EndCommit is the commit for which this baseline was collected.
	EndCommit *tiling.Commit `json:"endCommit"`

	// CommitDelta is the difference in index within the commits of a tile.
	// Appears to be unread and unwritten to.
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

type Baseliner interface {
	// CanWriteBaseline returns true if this instance was configured to write baseline files.
	CanWriteBaseline() bool

	// PushMasterBaselines writes the baselines for the master branch to GCS.
	// If cpxTile is nil the tile of the last call to PushMasterBaselines is used. If the function
	// was never called before and cpxTile is nil, an error is returned.
	// If targetHash != "" we also return the baseline for corresponding commit as the first return
	// value. Otherwise the first return value is nil.
	// It is assumed that the target commit is one of the commits that are written as part of this call.
	PushMasterBaselines(cpxTile *types.ComplexTile, targetHash string) (*CommitableBaseLine, error)

	// PushIssueBaseline writes the baseline for a Gerrit issue to GCS.
	PushIssueBaseline(issueID int64, cpxTile *types.ComplexTile, tallies *tally.Tallies) error

	// FetchBaseline fetches the complete baseline for the given Gerrit issue by
	// loading the master baseline and the issue baseline from GCS and combining
	// them. If either of them doesn't exist an empty baseline is assumed.
	// If issueOnly is true and issueID > 0 then only the expectations attached to the issue are
	// returned (omitting the baselines of the master branch). This is primarily used for debugging.
	FetchBaseline(commitHash string, issueID int64, patchsetID int64, issueOnly bool) (*CommitableBaseLine, error)
}
