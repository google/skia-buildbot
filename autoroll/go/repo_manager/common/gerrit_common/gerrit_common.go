package gerrit_common

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

// SetChangeLabels sets the necessary labels on the given change, marking it
// ready for review and starting the commit queue (or submitting the change
// outright, if there is no configured commit queue).
func SetChangeLabels(ctx context.Context, g gerrit.GerritInterface, ci *gerrit.ChangeInfo, emails []string, dryRun, canary bool) error {
	// Mark the change as ready for review, if necessary.
	if err := UnsetWIP(ctx, g, ci, 0); err != nil {
		return skerr.Wrapf(err, "failed to unset WIP")
	}

	// Set the CQ bit as appropriate.
	labels := gerrit.MergeLabels(g.Config().SelfApproveLabels, g.Config().SetCqLabels)
	if canary {
		labels = gerrit.MergeLabels(g.Config().SetDryRunLabels, g.Config().DisapproveLabels)
	} else if dryRun {
		labels = g.Config().SetDryRunLabels
	}
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
func DownloadCommitMsgHook(ctx context.Context, g gerrit.GerritInterface, co git.Checkout) error {
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
func SetupGerrit(ctx context.Context, co git.Checkout, g gerrit.GerritInterface) error {
	// Install the Gerrit Change-Id hook.
	if err := DownloadCommitMsgHook(ctx, g, co); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// GetNotSubmittedReason returns the reason we think the given revision ID is
// not submitted, or the empty string if we think it is.
func GetNotSubmittedReason(ctx context.Context, rev *revision.Revision, c *http.Client) (string, error) {
	// Find the Gerrit CL associated with the requested revision. Check whether
	// it has been merged.
	changeID, err := gerrit.ParseChangeId(rev.Details)
	if err != nil {
		return "Revision is not a Gerrit change; cannot verify that it has been reviewed and submitted", nil
	}
	gerritURL, err := commitURLToGerritURL(rev.URL)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to build Gerrit URL for revision %s", rev.Id)
	}
	g, err := gerrit.NewGerrit(gerritURL, c)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to create Gerrit client for revision %q", rev.Id)
	}
	cis, err := g.Search(ctx, 0, false, gerrit.SearchCommit(rev.Id))
	if err != nil {
		return "", skerr.Wrapf(err, "failed to search Gerrit changes for revision %q", rev.Id)
	}
	var foundCL *gerrit.ChangeInfo
	for _, ci := range cis {
		if ci.ChangeId == changeID {
			foundCL = ci
			break
		}
	}
	if foundCL == nil {
		// Fall back to retrieving the change by ID.
		ci, err := g.GetChange(ctx, changeID)
		if err != nil {
			if err == gerrit.ErrNotFound || strings.Contains(err.Error(), "status code 404") {
				// Returning an error here will result in an infinite retry loop
				// unless this is some transient problem on the Gerrit server.
				return fmt.Sprintf("failed to retrieve Gerrit CL for change ID %q", changeID), nil
			}
			return "", skerr.Wrapf(err, "failed to retrieve Gerrit CL for change ID %q", changeID)
		}
		foundCL = ci
	}
	if !foundCL.IsMerged() {
		return fmt.Sprintf("CL %s is not merged", g.Url(foundCL.Issue)), nil
	}
	return "", nil
}

// commitURLToGerritURL converts a Gitiles URL for a commit to a Gerrit host URL.
func commitURLToGerritURL(commitURL string) (string, error) {
	u, err := url.Parse(commitURL)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to parse url %q", commitURL)
	}
	u.Host = strings.Replace(u.Host, ".googlesource.com", "-review.googlesource.com", 1)
	u.Path = ""
	return u.String(), nil
}
