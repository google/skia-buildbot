package regression

import (
	"context"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

// Store persists Regressions.
type Store interface {
	// Range returns a map from types.CommitNumber to *Regressions that exist in the
	// given range of commits. Note that if begin==end that results
	// will be returned for begin.
	Range(ctx context.Context, begin, end types.CommitNumber) (map[types.CommitNumber]*AllRegressionsForCommit, error)

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

	// GetNotificationId returns the notificationId for the regression at the given commit number for the alert.
	GetNotificationId(ctx context.Context, commitNumber types.CommitNumber, alertID string) (string, error)

	// GetOldestCommit returns the commit with the lowest commit number
	GetOldestCommit(ctx context.Context) (*types.CommitNumber, error)

	// DeleteByCommit deletes a regression from the Regression table via the CommitNumber.
	// Use with caution.
	DeleteByCommit(ctx context.Context, commitNumber types.CommitNumber, tx pgx.Tx) error
}

// FullSummary describes a single regression.
type FullSummary struct {
	Summary clustering2.ClusterSummary `json:"summary"`
	Triage  TriageStatus               `json:"triage"`
	Frame   frame.FrameResponse        `json:"frame"`
}
