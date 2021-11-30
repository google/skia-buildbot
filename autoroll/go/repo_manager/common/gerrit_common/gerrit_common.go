package gerrit_common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

// SetChangeLabels sets the necessary labels on the given change, marking it
// ready for review and starting the commit queue (or submitting the change
// outright, if there is no configured commit queue).
func SetChangeLabels(ctx context.Context, g gerrit.GerritInterface, ci *gerrit.ChangeInfo, emails []string, dryRun bool) error {
	// Mark the change as ready for review, if necessary.
	if err := UnsetWIP(ctx, g, ci, 0); err != nil {
		return skerr.Wrapf(err, "failed to unset WIP")
	}

	// Set the CQ bit as appropriate.
	labels := g.Config().SetCqLabels
	if dryRun {
		labels = g.Config().SetDryRunLabels
	}
	labels = gerrit.MergeLabels(labels, g.Config().SelfApproveLabels)
	if err := g.SetReview(ctx, ci, "", labels, emails, "", nil, "", 0, nil); err != nil {
		// TODO(borenet): Should we try to abandon the CL?
		return skerr.Wrapf(err, "failed to set review")
	}

	// Manually submit if necessary.
	if !g.Config().HasCq {
		if err := g.Submit(ctx, ci); err != nil {
			// TODO(borenet): Should we try to abandon the CL?
			return skerr.Wrapf(err, "failed to submit")
		}
	}

	return nil
}

// UnsetWIP is a helper function for unsetting the WIP bit on a Gerrit CL if
// necessary. Either the change or issueNum parameter is required; if change is
// not  provided, it will be loaded from Gerrit. unsetWIP checks for a nil
// GerritInterface, so this is safe to call from RepoManagers which don't
// use Gerrit. If we fail to unset the WIP bit, unsetWIP abandons the change.
func UnsetWIP(ctx context.Context, g gerrit.GerritInterface, change *gerrit.ChangeInfo, issueNum int64) error {
	if g != nil {
		if change == nil {
			var err error
			change, err = g.GetIssueProperties(ctx, issueNum)
			if err != nil {
				return err
			}
		}
		if change.WorkInProgress {
			if err := g.SetReadyForReview(ctx, change); err != nil {
				if err2 := g.Abandon(ctx, change, "Failed to set ready for review."); err2 != nil {
					return fmt.Errorf("Failed to set ready for review with: %s\nand failed to abandon with: %s", err, err2)
				}
				return fmt.Errorf("Failed to set ready for review: %s", err)
			}
		}
	}
	return nil
}

// DownloadCommitMsgHook downloads the Gerrit Change-Id hook and installs it in
// the given git checkout.
func DownloadCommitMsgHook(ctx context.Context, g gerrit.GerritInterface, co *git.Checkout) error {
	out, err := co.Git(ctx, "rev-parse", "--git-dir")
	if err != nil {
		return skerr.Wrap(err)
	}
	hookFile := filepath.Join(co.Dir(), strings.TrimSpace(out), "hooks", "commit-msg")
	if _, err := os.Stat(hookFile); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(hookFile), os.ModePerm); err != nil {
			return skerr.Wrap(err)
		}
		if err := g.DownloadCommitMsgHook(ctx, hookFile); err != nil {
			return skerr.Wrap(err)
		}
	} else if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// SetupGerrit performs additional setup for a Checkout which uses Gerrit. This
// is required for all users of GitCheckoutUploadGerritRollFunc.
// TODO(borenet): This is needed for RepoManagers which use NewDEPSLocal, since
// they need to pass in a GitCheckoutUploadRollFunc but can't do other
// initialization. Find a way to make this unnecessary.
func SetupGerrit(ctx context.Context, co *git.Checkout, g gerrit.GerritInterface) error {
	// Install the Gerrit Change-Id hook.
	if err := DownloadCommitMsgHook(ctx, g, co); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}
