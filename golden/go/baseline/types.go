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

	// Filled is the number of tests that had at least one positive digest at EndCommit.
	Filled int `json:"filled"`

	// MD5 is the hash of the Baseline field.
	MD5 string `json:"md5"`

	// Baseline captures the baseline of the current commit.
	Baseline types.TestExp `json:"master"`

	// Issue indicates the Gerrit issue of this baseline. 0 indicates the master branch.
	Issue int64
}

// Baseliner is an interface wrapping a functionality to save and fetch baselines.
type Baseliner interface {
	// CanWriteBaseline returns true if this instance was configured to write baseline files.
	CanWriteBaseline() bool

	// PushMasterBaselines writes the baselines for the master branch to GCS.
	// If commitSource is nil the tile of the last call to PushMasterBaselines is used. If the
	// function was never called before and commitSource is nil, an error is returned.
	// If targetHash != "" we also return the baseline for corresponding commit as the first return
	// value. Otherwise the first return value is nil.
	// It is assumed that the target commit is one of the commits that are written as part of
	// this call.
	PushMasterBaselines(commitSource CommitSource, targetHash string) (*CommitableBaseLine, error)

	// PushIssueBaseline writes the baseline for a Gerrit issue to GCS.
	PushIssueBaseline(issueID int64, commitSource CommitSource, tallies *tally.Tallies) error

	// FetchBaseline fetches the complete baseline for the given Gerrit issue by
	// loading the master baseline and the issue baseline from GCS and combining
	// them. If either of them doesn't exist an empty baseline is assumed.
	// If issueOnly is true and issueID > 0 then only the expectations attached to the issue are
	// returned (omitting the baselines of the master branch). This is primarily used for debugging.
	FetchBaseline(commitHash string, issueID int64, patchsetID int64, issueOnly bool) (*CommitableBaseLine, error)
}

// CommitSource is an interface around a subset of the functionality given by types.ComplexTile.
// Specifically, Baseliner needs a way to get information about what commits we are considering.
type CommitSource interface {
	// AllCommits returns all commits that were processed to get the data commits.
	// Its first commit should match the first commit returned when calling DataCommits.
	AllCommits() []*tiling.Commit

	// DataCommits returns all commits that contain data. In some busy repos, there are commits
	// that don't get tested directly because the commits are batched in with others.
	// DataCommits is a way to get just the commits where some data has been ingested.
	DataCommits() []*tiling.Commit

	// GetTile returns a simple tile either with or without ignored traces depending on
	// the argument.
	GetTile(includeIgnores bool) *tiling.Tile
}
