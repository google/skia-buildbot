package parent

import (
	"context"
	"fmt"

	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

// GitCheckoutUploadGerritRollFunc returns a GitCheckoutUploadRollFunc which
// uploads a CL to Gerrit.
func GitCheckoutUploadGerritRollFunc(g gerrit.GerritInterface) git_common.UploadRollFunc {
	return func(ctx context.Context, co *git.Checkout, upstreamBranch, hash string, emails []string, dryRun bool, commitMsg string) (int64, error) {
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

		if err := gerrit_common.SetChangeLabels(ctx, g, ci, emails, dryRun); err != nil {
			return 0, skerr.Wrap(err)
		}

		return ci.Issue, nil
	}
}
