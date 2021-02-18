package gerrit

import (
	"context"
	"strings"

	"go.skia.org/infra/go/skerr"
)

// EditChange is a helper for creating a new patch set on an existing
// Change. Pass in a function which creates and modifies a ChangeEdit, and the
// result will be automatically published as a new patch set, or in the case of
// failure, reverted.
func EditChange(ctx context.Context, g GerritInterface, ci *ChangeInfo, fn func(context.Context, GerritInterface, *ChangeInfo) error) (rvErr error) {
	defer func() {
		if rvErr == nil {
			rvErr = g.PublishChangeEdit(ctx, ci)
		}
		if rvErr != nil {
			if err := g.DeleteChangeEdit(ctx, ci); err != nil {
				rvErr = skerr.Wrapf(rvErr, "failed to edit change and failed to delete edit with: %s", err)
			}
		}
	}()
	return fn(ctx, g, ci)
}

// CreateAndEditChange is a helper which creates a new Change in the given
// project based on the given branch with the given commit message. Pass in a
// function which modifies a ChangeEdit, and the result will be automatically
// published as a new patch set, or in the case of failure, reverted. If an
// error is encountered after the Change is created, the ChangeInfo is returned
// so that the caller can decide whether to abandon the change or try again.
func CreateAndEditChange(ctx context.Context, g GerritInterface, project, branch, commitMsg, baseCommit string, fn func(context.Context, GerritInterface, *ChangeInfo) error) (*ChangeInfo, error) {
	ci, err := g.CreateChange(ctx, project, branch, strings.Split(commitMsg, "\n")[0], baseCommit)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create change")
	}
	commitMsg = strings.TrimSpace(commitMsg) + "\n" + "Change-Id: " + ci.ChangeId
	if err := EditChange(ctx, g, ci, func(ctx context.Context, g GerritInterface, ci *ChangeInfo) error {
		if err := g.SetCommitMessage(ctx, ci, commitMsg); err != nil {
			return skerr.Wrapf(err, "failed to set commit message to:\n\n%s\n\n", commitMsg)
		}
		return fn(ctx, g, ci)
	}); err != nil {
		return ci, skerr.Wrapf(err, "failed to edit change")
	}
	// Update the view of the Change to include the new patchset.
	ci2, err := g.GetIssueProperties(ctx, ci.Issue)
	if err != nil {
		return ci, skerr.Wrapf(err, "failed to retrieve issue properties")
	}
	return ci2, nil
}
