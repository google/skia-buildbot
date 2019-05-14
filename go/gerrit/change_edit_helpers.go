package gerrit

import (
	"context"
	"fmt"
	"strings"
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
				rvErr = fmt.Errorf("%s and failed to delete edit with: %s", rvErr, err)
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
		return nil, fmt.Errorf("Failed to create change: %s", err)
	}
	if err := EditChange(ctx, g, ci, func(ctx context.Context, g GerritInterface, ci *ChangeInfo) error {
		if err := g.SetCommitMessage(ctx, ci, commitMsg); err != nil {
			return fmt.Errorf("Failed to set commit message: %s", err)
		}
		return fn(ctx, g, ci)
	}); err != nil {
		return ci, fmt.Errorf("Failed to edit change: %s", err)
	}
	// Update the view of the Change to include the new patchset.
	ci2, err := g.GetIssueProperties(ctx, ci.Issue)
	if err != nil {
		return ci, fmt.Errorf("Failed to retrieve issue properties: %s", err)
	}
	return ci2, nil
}
