package regression

import (
	"context"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/perf/go/clustering2"
	pb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

// Store persists Regressions.
type Store interface {
	// Range returns a map from types.CommitNumber to *Regressions that exist in the
	// given range of commits. Note that if begin==end that results
	// will be returned for begin.
	Range(ctx context.Context, begin, end types.CommitNumber) (map[types.CommitNumber]*AllRegressionsForCommit, error)

	// RangeFiltered gets all regressions in the given commit range and trace names.
	RangeFiltered(ctx context.Context, begin, end types.CommitNumber, traceNames []string) ([]*Regression, error)

	// SetHigh sets the ClusterSummary for a high regression at the given commit and alertID.
	SetHigh(ctx context.Context, commitNumber types.CommitNumber, alertID string, df *frame.FrameResponse, high *clustering2.ClusterSummary) (bool, string, error)

	// SetLow sets the ClusterSummary for a low regression at the given commit and alertID.
	SetLow(ctx context.Context, commitNumber types.CommitNumber, alertID string, df *frame.FrameResponse, low *clustering2.ClusterSummary) (bool, string, error)

	// TriageLow sets the triage status for the low cluster at the given commit and alertID.
	TriageLow(ctx context.Context, commitNumber types.CommitNumber, alertID string, tr TriageStatus) error

	// TriageHigh sets the triage status for the high cluster at the given commit and alertID.
	TriageHigh(ctx context.Context, commitNumber types.CommitNumber, alertID string, tr TriageStatus) error

	// Write the Regressions to the store. The provided 'regressions' maps from
	// types.CommitNumber to all the regressions for that commit.
	Write(ctx context.Context, regressions map[types.CommitNumber]*AllRegressionsForCommit) error

	// Given the subscription name GetRegressionsBySubName gets all the regressions against
	// the specified subscription. The response will be paginated according to the provided
	// limit and offset.
	GetRegressionsBySubName(ctx context.Context, sub_name string, limit int, offset int) ([]*Regression, error)

	// Given a list of regression IDs (only in the regression2store),
	// return a list of regressions.
	GetByIDs(ctx context.Context, ids []string) ([]*Regression, error)

	// Return a list of regressions satisfying: previous_commit < rev <= commit.
	GetByRevision(ctx context.Context, rev string) ([]*Regression, error)

	// GetOldestCommit returns the commit with the lowest commit number
	GetOldestCommit(ctx context.Context) (*types.CommitNumber, error)

	// GetRegression returns the regression info at the given commit for specific alert.
	GetRegression(ctx context.Context, commitNumber types.CommitNumber, alertID string) (*Regression, error)

	// DeleteByCommit deletes a regression from the Regression table via the CommitNumber.
	// Use with caution.
	DeleteByCommit(ctx context.Context, commitNumber types.CommitNumber, tx pgx.Tx) error

	// SetBugID associates a set of regressions, identified by their IDs, with a bug ID.
	SetBugID(ctx context.Context, regressionIDs []string, bugID int) error

	// IgnoreAnomalies sets the triage status to Ignored and message to IgnoredMessage for the given regressions.
	IgnoreAnomalies(ctx context.Context, regressionIDs []string) error

	// ResetAnomalies sets the triage status to Untriaged, message to ResetMessage, and bugID to 0 for the given regressions.
	ResetAnomalies(ctx context.Context, regressionIDs []string) error

	// NudgeAndResetAnomalies updates the commit number and previous commit number for the given regressions,
	// and also sets the triage status to Untriaged, message to NudgedMessage, and bugID to 0.
	NudgeAndResetAnomalies(ctx context.Context, regressionIDs []string, commitNumber, prevCommitNumber types.CommitNumber) error

	// GetBugIdsForRegressions queries all bugs from regressions2, culprits and anomalygroups for given regressions.
	GetBugIdsForRegressions(ctx context.Context, regressions []*Regression) ([]*Regression, error)

	// GetSubscriptionsForRegressions returns a subset of subscription fields for given regressions, together with regression and alert ids.
	GetSubscriptionsForRegressions(ctx context.Context, regressionIDs []string) ([]string, []int64, []*pb.Subscription, error)
}

// FullSummary describes a single regression.
type FullSummary struct {
	Summary clustering2.ClusterSummary `json:"summary"`
	Triage  TriageStatus               `json:"triage"`
	Frame   frame.FrameResponse        `json:"frame"`
}
