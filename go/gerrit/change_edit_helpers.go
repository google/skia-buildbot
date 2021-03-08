package gerrit

import (
	"context"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
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
	commitMsg, err = git.AddTrailer(commitMsg, "Change-Id: "+ci.ChangeId)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := EditChange(ctx, g, ci, func(ctx context.Context, g GerritInterface, ci *ChangeInfo) error {
		if err := g.SetCommitMessage(ctx, ci, commitMsg); err != nil {
			return skerr.Wrapf(err, "failed to set commit message to:\n\n%s\n\n", commitMsg)
		}
		return fn(ctx, g, ci)
	}); err != nil {
		return ci, skerr.Wrapf(err, "failed to edit change")
	}
	// Update the view of the Change to include the new patchset. Sometimes
	// Gerrit lags and doesn't include the second patchset in the response, so
	// we use retries with exponential backoff until it shows up or the allotted
	// time runs out.
	exp := &backoff.ExponentialBackOff{
		InitialInterval:     time.Second,
		RandomizationFactor: 0.5,
		Multiplier:          2,
		MaxInterval:         16 * time.Second,
		MaxElapsedTime:      time.Minute,
		Clock:               backoff.SystemClock,
	}
	var ci2 *ChangeInfo
	loadChange := func() error {
		ci2, err = g.GetIssueProperties(ctx, ci.Issue)
		if err != nil {
			return skerr.Wrapf(err, "failed to retrieve issue properties")
		}
		if len(ci2.Revisions) < 2 {
			sklog.Errorf("Change is missing second patchset; reloading.")
			return skerr.Fmt("change is missing second patchset")
		}
		sklog.Info("Retrieved issue properties successfully.")
		return nil
	}

	return ci2, skerr.Wrap(backoff.Retry(loadChange, exp))
}
