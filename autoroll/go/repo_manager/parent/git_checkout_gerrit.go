package parent

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

// GitCheckoutUploadGerritRollFunc returns a GitCheckoutUploadRollFunc which
// uploads a CL to Gerrit.
func GitCheckoutUploadGerritRollFunc(g gerrit.GerritInterface) git_common.UploadRollFunc {
	return func(ctx context.Context, co git.Checkout, upstreamBranch, hash string, emails []string, dryRun, canary bool, commitMsg string) (int64, error) {
		// Find the change ID in the commit message.
		out, err := co.Git(ctx, "log", "-n1", hash)
		if err != nil {
			return 0, skerr.Wrap(err)
		}
		changeId, err := gerrit.ParseChangeId(out)
		if err != nil {
			return 0, skerr.Wrapf(err, "Commit message:\n%s", out)
		}

		// Upload CL.
		if _, err := co.Git(ctx, "push", git.DefaultRemote, fmt.Sprintf("%s:refs/for/%s", hash, upstreamBranch)); err != nil {
			return 0, skerr.Wrap(err)
		}
		ci, err := g.GetChange(ctx, changeId)
		if err != nil {
			return 0, skerr.Wrap(err)
		}

		// TODO(borenet): We shouldn't assume that the commit has the correct
		// message; instead, we should edit the CL to use the passed-in
		// commitMsg.

		if err := gerrit_common.SetChangeLabels(ctx, g, ci, emails, dryRun, canary); err != nil {
			return 0, skerr.Wrap(err)
		}

		return ci.Issue, nil
	}
}

// NewGitCheckoutGerrit returns an implementation of Parent which uses a local
// git checkout and uploads pull requests to Github.
func NewGitCheckoutGerrit(ctx context.Context, c *config.GitCheckoutGerritParentConfig, client *http.Client, serverURL, workdir, rollerName string, cr codereview.CodeReview) (*GitCheckoutParent, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}

	gerritClient, ok := cr.Client().(gerrit.GerritInterface)
	if !ok {
		return nil, skerr.Fmt("GitCheckoutGerrit must use Gerrit for code review.")
	}

	// See documentation for GitCheckoutUploadRollFunc.
	uploadRoll := GitCheckoutUploadGerritRollFunc(gerritClient)

	createRollHelper := gitCheckoutFileCreateRollFunc(c.GitCheckout.Dep)
	createRoll := func(ctx context.Context, co git.Checkout, from *revision.Revision, to *revision.Revision, rolling []*revision.Revision, commitMsg string) (string, error) {
		// Run the helper to set the new dependency version(s).
		if _, err := createRollHelper(ctx, co, from, to, rolling, commitMsg); err != nil {
			return "", skerr.Wrap(err)
		}

		// Run the pre-upload steps.
		if err := RunPreUploadStep(ctx, c.PreUploadCommands, nil, client, co.Dir(), from, to); err != nil {
			return "", skerr.Wrapf(err, "failed pre-upload step: %s", err)
		}

		// Commit.
		if _, err := co.Git(ctx, "commit", "-a", "--amend", "--no-edit"); err != nil {
			return "", skerr.Wrap(err)
		}
		out, err := co.RevParse(ctx, "HEAD")
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return strings.TrimSpace(out), nil
	}

	// Create the GitCheckout Parent.
	p, err := NewGitCheckout(ctx, c.GitCheckout, workdir, cr, nil, createRoll, uploadRoll)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := gerrit_common.SetupGerrit(ctx, p.Checkout.Checkout, gerritClient); err != nil {
		return nil, skerr.Wrap(err)
	}
	return p, nil
}
