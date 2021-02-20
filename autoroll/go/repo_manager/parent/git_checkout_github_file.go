package parent

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// NewGitCheckoutGithubFile returns a Parent which uses a local checkout and a
// version file (eg. DEPS) to manage dependencies.
func NewGitCheckoutGithubFile(ctx context.Context, c *config.GitCheckoutGitHubFileParentConfig, reg *config_vars.Registry, client *http.Client, serverURL, workdir, rollerName string, cr codereview.CodeReview) (*GitCheckoutParent, error) {
	// Pre-upload steps are run after setting the new dependency version and
	// syncing, but before committing and uploading.
	preUploadSteps, err := GetPreUploadSteps(c.PreUploadSteps)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	createRollHelper := gitCheckoutFileCreateRollFunc(c.GitCheckout.GitCheckout.Dep)
	createRoll := func(ctx context.Context, co *git.Checkout, from *revision.Revision, to *revision.Revision, rolling []*revision.Revision, commitMsg string) (string, error) {
		// Run the helper to add commits pointing to each of the Revision in the
		// roll.
		// TODO(borenet): This should be optional and configured in
		// GitCheckoutGithubFileConfig.
		prev := from
		for i := len(rolling) - 1; i >= 0; i-- {
			rev := rolling[i]
			msg := fmt.Sprintf("%s %s", rev.Id[:9], rev.Description)
			if _, err := createRollHelper(ctx, co, prev, rev, []*revision.Revision{rev}, msg); err != nil {
				return "", skerr.Wrap(err)
			}
		}

		// Run the pre-upload steps.
		sklog.Infof("Running %d pre-upload steps.", len(preUploadSteps))
		for _, s := range preUploadSteps {
			if err := s(ctx, nil, client, co.Dir(), from, to); err != nil {
				return "", skerr.Wrapf(err, "failed pre-upload step")
			}
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
	return NewGitCheckoutGithub(ctx, c.GitCheckout, reg, serverURL, workdir, rollerName, cr, createRoll)
}
