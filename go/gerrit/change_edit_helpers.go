package gerrit

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
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
	splitCommitMsg := strings.Split(commitMsg, "\n")
	ci, err := g.CreateChange(ctx, project, branch, splitCommitMsg[0], baseCommit)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create change")
	}
	if err := EditChange(ctx, g, ci, func(ctx context.Context, g GerritInterface, ci *ChangeInfo) error {
		if len(splitCommitMsg) > 1 {
			commitMsg, err = git.AddTrailer(commitMsg, "Change-Id: "+ci.ChangeId)
			if err != nil {
				return skerr.Wrap(err)
			}
			if err := g.SetCommitMessage(ctx, ci, commitMsg); err != nil {
				return skerr.Wrapf(err, "failed to set commit message to:\n\n%s\n\n", commitMsg)
			}
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

// CreateCLWithChanges is a helper which creates a new Change in the given
// project based on the given branch with the given commit message and the given
// map of filepath to new file contents. Empty file contents indicate deletion
// of the file. If reviewers are provided, the change is sent for review.
func CreateCLWithChanges(ctx context.Context, g GerritInterface, project, branch, commitMsg, baseCommit string, changes map[string]string, reviewers []string) (*ChangeInfo, error) {
	ci, err := CreateAndEditChange(ctx, g, project, branch, commitMsg, baseCommit, func(ctx context.Context, g GerritInterface, ci *ChangeInfo) error {
		for filepath, contents := range changes {
			if contents == "" {
				if err := g.DeleteFile(ctx, ci, filepath); err != nil {
					return skerr.Wrap(err)
				}
			} else {
				if err := g.EditFile(ctx, ci, filepath, contents); err != nil {
					if !strings.Contains(err.Error(), ErrNoChanges) {
						return skerr.Wrap(err)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if len(reviewers) > 0 {
		if ci.WorkInProgress {
			if err := g.SetReadyForReview(ctx, ci); err != nil {
				return ci, skerr.Wrapf(err, "failed to set ready for review")
			}
		}
		if err := g.SetReview(ctx, ci, "", nil, reviewers, "", "", 0); err != nil {
			return ci, skerr.Wrapf(err, "failed to set review")
		}
	}
	return ci, nil
}

// CreateCLFromLocalDiffs is a helper which creates a Change based on the
// diffs from the provided branch in a local checkout. Note that the diff is
// performed against the given branch on "origin", and not any local version.
func CreateCLFromLocalDiffs(ctx context.Context, g GerritInterface, project, branch, commitMsg string, reviewers []string, co *git.Checkout) (*ChangeInfo, error) {
	baseCommit, err := co.Git(ctx, "rev-parse", fmt.Sprintf("origin/%s", branch))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	baseCommit = strings.TrimSpace(baseCommit)
	diff, err := co.Git(ctx, "diff", "--name-only", baseCommit)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	diffSplit := strings.Split(diff, "\n")
	changes := make(map[string]string, len(diffSplit))
	for _, diffLine := range diffSplit {
		if diffLine != "" {
			contents, err := ioutil.ReadFile(filepath.Join(co.Dir(), diffLine))
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			changes[diffLine] = string(contents)
		}
	}
	return CreateCLWithChanges(ctx, g, project, branch, commitMsg, baseCommit, changes, reviewers)
}
