package parent

import (
	"context"
	"fmt"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

// GitCheckoutGerritConfig provides configuration for Parents which use a local
// git checkout and upload changes to Gerrit.
type GitCheckoutGerritConfig struct {
	GitCheckoutConfig
	Gerrit *codereview.GerritConfig `json:"gerrit"`
}

// GitCheckoutUploadGerritRollFunc returns a GitCheckoutUploadRollFunc which
// uploads a CL to Gerrit.
func GitCheckoutUploadGerritRollFunc(g gerrit.GerritInterface) GitCheckoutUploadRollFunc {
	return func(ctx context.Context, co *git.Checkout, upstreamBranch, hash string, emails []string, dryRun bool) (int64, error) {
		// Find the change ID in the commit message.
		out, err := co.Git(ctx, "log", "-n1", hash)
		if err != nil {
			return 0, skerr.Wrap(err)
		}
		changeId, err := gerrit.ParseChangeId(out)
		if err != nil {
			return 0, skerr.Wrap(err)
		}

		// Upload CL.
		if _, err := co.Git(ctx, "push", "origin", fmt.Sprintf("%s:refs/for/%s", hash, upstreamBranch)); err != nil {
			return 0, skerr.Wrap(err)
		}
		ci, err := g.GetChange(ctx, changeId)
		if err != nil {
			return 0, skerr.Wrap(err)
		}
		if err := gerrit_common.SetChangeLabels(ctx, g, ci, emails, dryRun); err != nil {
			return 0, skerr.Wrap(err)
		}

		return ci.Issue, nil
	}
}

// NewGitCheckoutGerrit returns an implementation of Parent which uses a local
// git checkout and uploads changes to Gerrit.
func NewGitCheckoutGerrit(ctx context.Context, c GitCheckoutGerritConfig, reg *config_vars.Registry, client *http.Client, serverURL, workdir string, getLastRollRev GitCheckoutGetLastRollRevFunc, createRoll GitCheckoutCreateRollFunc) (*GitCheckoutParent, error) {
	gc, err := c.Gerrit.GetConfig()
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get Gerrit config")
	}
	g, err := gerrit.NewGerritWithConfig(gc, c.Gerrit.URL, client)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create Gerrit client")
	}
	uploadRoll := GitCheckoutUploadGerritRollFunc(g)
	p, err := NewGitCheckout(ctx, c.GitCheckoutConfig, reg, client, serverURL, workdir, getLastRollRev, createRoll, uploadRoll)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Install the Gerrit Change-Id hook.
	if err := gerrit_common.DownloadCommitMsgHook(ctx, g, p.co); err != nil {
		return nil, skerr.Wrap(err)
	}
	return p, nil
}
