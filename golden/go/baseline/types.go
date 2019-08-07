package baseline

import (
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/types"
)

// Baseline captures the data necessary to verify test results on the
// commit queue. A baseline is essentially the expectations for a set of
// tests in a given commit range.
type Baseline struct {
	// MD5 is the hash of the Expectations field.
	MD5 string `json:"md5"`

	// Expectations captures the "baseline expectations", that is, the Expectations
	// with only the positive digests of the current commit.
	Expectations types.Expectations `json:"master"`

	// Issue indicates the Gerrit issue of this baseline. -1 indicates the master branch.
	Issue int64
}

// Copy returns a deep copy of the given instance of Baseline.
// Note: It assumes all members except for Baseline to be immutable, thus only
// Baseline is "deep" copied.
func (c *Baseline) Copy() *Baseline {
	ret := &Baseline{}
	*ret = *c
	ret.Expectations = c.Expectations.DeepCopy()
	return ret
}

// EmptyBaseline returns an instance of Baseline with the provided commits and nil
// values in all other fields. The Baseline field contains an empty instance of types.Expectations.
func EmptyBaseline() *Baseline {
	return &Baseline{
		Expectations: types.Expectations{},
		MD5:          md5SumEmptyExp,
	}
}

// TODO(kjlubick): delete this once fs_baseliner lands
type BaselineWriter interface {
	// CanWriteBaseline returns true if this instance was configured to write baseline files.
	CanWriteBaseline() bool

	// PushMasterBaselines writes the baselines for the master branch to GCS.
	// If tileInfo is nil the tile of the last call to PushMasterBaselines is used. If the
	// function was never called before and tileInfo is nil, an error is returned.
	// If targetHash != "" we also return the baseline for corresponding commit as the first return
	// value. Otherwise the first return value is nil.
	// It is assumed that the target commit is one of the commits that are written as part of
	// this call.
	PushMasterBaselines(tileInfo TileInfo, targetHash string) (*Baseline, error)

	// PushIssueBaseline writes the baseline for a Gerrit issue to GCS.
	PushIssueBaseline(issueID int64, tileInfo TileInfo, dCounter digest_counter.DigestCounter) error
}

type BaselineFetcher interface {
	// FetchBaseline fetches the complete baseline for the given Gerrit issue by
	// loading the master baseline and the issue baseline from GCS and combining
	// them. If either of them doesn't exist an empty baseline is assumed.
	// If issueOnly is true and issueID > 0 then only the expectations attached to the issue are
	// returned (omitting the baselines of the master branch).
	// issueOnly is primarily used for debugging.
	// TODO(kjlubick): remove commitHash as it has no meaning anymore, now that per-commit
	// baselines have been removed.
	FetchBaseline(commitHash string, issueID int64, issueOnly bool) (*Baseline, error)
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
	GetTile(is types.IgnoreState) *tiling.Tile
}
