package internal

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/backends"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/workflow"
)

// ReportStatusActivity wraps the call to IssueTracker to report culprits.
// TODO(sunxiaodi@): Update this activity for culprit verification
func ReportStatusActivity(ctx context.Context, issueID int, culprits []*pinpoint_proto.CombinedCommit) error {
	transport, err := backends.NewIssueTrackerTransport(ctx)
	if err != nil {
		return skerr.Wrapf(err, "failed to create issue tracker client")
	}

	err = transport.ReportCulprit(int64(issueID), culprits)
	if err != nil {
		return skerr.Wrap(err)
	}

	return nil
}

// PostBugCommentActivity wraps the call to Issuetracker's PostComment.
func PostBugCommentActivity(ctx context.Context, issueID int64, comment string) (bool, error) {
	transport, err := backends.NewIssueTrackerTransport(ctx)
	if err != nil {
		return false, skerr.Wrapf(err, "failed to create issue tracker client")
	}

	err = transport.PostComment(issueID, comment)
	if err != nil {
		return false, skerr.Wrap(err)
	}

	return true, nil
}

// TODO(sunxiaodi): Add a unit test for this workflow
func PostBugCommentWorkflow(ctx workflow.Context, issueID int64, comment string) (bool, error) {
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
	var success bool
	if err := workflow.ExecuteActivity(ctx, PostBugCommentActivity, issueID, comment).Get(ctx, &success); err != nil {
		return false, err
	}
	return success, nil
}
