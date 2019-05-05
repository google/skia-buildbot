package baseline

import (
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/types"
)

// CommitableBaseline captures the data necessary to verify test results on the
// commit queue.  A baseline is essentially the expectations for a set of
// tests in a given commit range.
// TODO(kjlubick): Maybe make a version of this that handles a single commit/CL
// called ExpectationsDelta or ChangelistExpectations or something like that.
type CommitableBaseline struct {
	// StartCommit covered by these baselines.
	StartCommit *tiling.Commit `json:"startCommit"`

	// EndCommit is the commit for which this baseline was collected.
	EndCommit *tiling.Commit `json:"endCommit"`

	// CommitDelta is the difference in index within the commits of a tile.
	// TODO(kjlubick) Appears to be unread and unwritten to.
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

// DeepCopyBaseline returns a copy of the given instance of CommitableBaseline.
// Note: It assumes all members except for Baseline to be immutable, thus only
// Baseline is "deep" copied.
func (c *CommitableBaseline) DeepCopyBaseline() *CommitableBaseline {
	ret := &CommitableBaseline{}
	*ret = *c
	ret.Baseline = c.Baseline.DeepCopy()
	return ret
}

// EmptyBaseline returns an instance of CommitableBaseline with the provided commits and nil
// values in all other fields. The Baseline field contains an empty instance of types.TestExp.
func EmptyBaseline(startCommit, endCommit *tiling.Commit) *CommitableBaseline {
	return &CommitableBaseline{
		StartCommit: startCommit,
		EndCommit:   endCommit,
		Baseline:    types.TestExp{},
		MD5:         md5SumEmptyExp,
	}
}

// Baseliner is an interface wrapping the functionality to save and fetch baselines.
type Baseliner interface {
	// CanWriteBaseline returns true if this instance was configured to write baseline files.
	CanWriteBaseline() bool

	// PushMasterBaselines writes the baselines for the master branch to GCS.
	// If tileInfo is nil the tile of the last call to PushMasterBaselines is used. If the
	// function was never called before and tileInfo is nil, an error is returned.
	// If targetHash != "" we also return the baseline for corresponding commit as the first return
	// value. Otherwise the first return value is nil.
	// It is assumed that the target commit is one of the commits that are written as part of
	// this call.
	PushMasterBaselines(tileInfo TileInfo, targetHash string) (*CommitableBaseline, error)

	// PushIssueBaseline writes the baseline for a Gerrit issue to GCS.
	PushIssueBaseline(issueID int64, tileInfo TileInfo, dCounter digest_counter.DigestCounter) error

	// FetchBaseline fetches the complete baseline for the given Gerrit issue by
	// loading the master baseline and the issue baseline from GCS and combining
	// them. If either of them doesn't exist an empty baseline is assumed.
	// If issueOnly is true and issueID > 0 then only the expectations attached to the issue are
	// returned (omitting the baselines of the master branch). This is primarily used for debugging.
	FetchBaseline(commitHash string, issueID int64, patchsetID int64, issueOnly bool) (*CommitableBaseline, error)
}

// TileInfo is an interface around a subset of the functionality given by types.ComplexTile.
// Specifically, Baseliner needs a way to get information about what commits in the tile we
// are considering.
type TileInfo interface {
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
