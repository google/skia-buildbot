package regression

import (
	"context"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
)

// DetailLookup is used by RegressionStore to look up commit details.
type DetailLookup func(context.Context, *cid.CommitID) (*cid.CommitDetail, error)

// Store persists Regressions.
//
// TODO(jcgregorio) Move away cid.ID()'s to types.CommitNumber.
type Store interface {
	// Range returns a map from cid.ID()'s to *Regressions that exist in the
	// given time range.
	//
	// TODO(jcgregorio) Convert begin and end from Unix timestamps to
	// types.CommitNumbers.
	Range(ctx context.Context, begin, end types.CommitNumber) (map[types.CommitNumber]*AllRegressionsForCommit, error)

	// SetHigh sets the ClusterSummary for a high regression at the given commit and alertID.
	SetHigh(ctx context.Context, cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, high *clustering2.ClusterSummary) (bool, error)

	// SetLow sets the ClusterSummary for a low regression at the given commit and alertID.
	SetLow(ctx context.Context, cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, low *clustering2.ClusterSummary) (bool, error)

	// TriageLow sets the triage status for the low cluster at the given commit and alertID.
	TriageLow(ctx context.Context, cid *cid.CommitDetail, alertID string, tr TriageStatus) error

	// TriageHigh sets the triage status for the high cluster at the given commit and alertID.
	TriageHigh(ctx context.Context, cid *cid.CommitDetail, alertID string, tr TriageStatus) error

	// Write the Regressions to the store. The provided 'regressions' maps from
	// cid.ID()'s to all the regressions for that commit.
	Write(ctx context.Context, regressions map[types.CommitNumber]*AllRegressionsForCommit) error
}

// UnixTimestampRangeToCommitNumberRange converts a range of commits given in
// Unit timestamps into a range of types.CommitNumbers.
//
// Note this could return two equal commitNumbers.
func UnixTimestampRangeToCommitNumberRange(vcs vcsinfo.VCS, begin, end int64) (types.CommitNumber, types.CommitNumber, error) {
	commits := vcs.Range(time.Unix(begin, 0), time.Unix(end, 0))
	if len(commits) == 0 {
		return types.BadCommitNumber, types.BadCommitNumber, skerr.Fmt("Didn't find any commits in range: %d %d", begin, end)
	}
	return types.CommitNumber(commits[0].Index), types.CommitNumber(commits[len(commits)-1].Index), nil
}
