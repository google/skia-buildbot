package parent

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	autoroll_git_common "go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	goModFile = "go.mod"
)

type goModParent struct {
	*git_common.Checkout
	modulePath string
	regex      *regexp.Regexp
	createRoll git_common.CreateRollFunc
	uploadRoll git_common.UploadRollFunc
}

// NewGoModGerritParent returns a Parent which updates go.mod and uploads CLs to
// Gerrit.
func NewGoModGerritParent(ctx context.Context, c *config.GoModGerritParentConfig, reg *config_vars.Registry, client *http.Client, workdir string, cr codereview.CodeReview) (*goModParent, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	gerritClient, ok := cr.Client().(*gerrit.Gerrit)
	if !ok {
		return nil, skerr.Fmt("GitCheckoutGithub must use GitHub for code review.")
	}
	uploadRoll := GitCheckoutUploadGerritRollFunc(gerritClient)
	parentRM, err := NewGoModParent(ctx, c.GoMod, reg, client, workdir, cr, uploadRoll)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := gerrit_common.SetupGerrit(ctx, parentRM.Checkout.Checkout, gerritClient); err != nil {
		return nil, skerr.Wrap(err)
	}
	return parentRM, nil
}

// NewGoModParent returns a Parent which updates go.mod.
func NewGoModParent(ctx context.Context, c *config.GoModParentConfig, reg *config_vars.Registry, client *http.Client, workdir string, cr codereview.CodeReview, uploadRoll autoroll_git_common.UploadRollFunc) (*goModParent, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	co, err := git_common.NewCheckout(ctx, c.GitCheckout, reg, workdir, cr.UserName(), cr.UserEmail(), nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	regex, err := regexp.Compile(fmt.Sprintf(`%s (\S+)`, c.ModulePath))
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Pre-upload steps are run after setting the new dependency version and
	// syncing, but before committing and uploading.
	preUploadSteps, err := GetPreUploadSteps(c.PreUploadSteps, c.PreUploadCommands)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	createRoll := func(ctx context.Context, co *git.Checkout, from *revision.Revision, to *revision.Revision, rolling []*revision.Revision, commitMsg string) (string, error) {
		// Update the Go module.
		if _, err := exec.RunCwd(ctx, co.Dir(), "go", "get", fmt.Sprintf("%s@%s", c.ModulePath, to.Id)); err != nil {
			return "", skerr.Wrap(err)
		}
		if _, err := exec.RunCwd(ctx, co.Dir(), "go", "mod", "tidy"); err != nil {
			return "", skerr.Wrap(err)
		}

		// Run the pre-upload steps.
		sklog.Infof("Running %d pre-upload steps.", len(preUploadSteps))
		for _, s := range preUploadSteps {
			if err := s(ctx, nil, client, co.Dir(), from, to); err != nil {
				return "", skerr.Wrapf(err, "failed pre-upload step")
			}
		}

		// Commit.
		if _, err := co.Git(ctx, "commit", "-a", "-m", commitMsg); err != nil {
			return "", skerr.Wrap(err)
		}
		out, err := co.RevParse(ctx, "HEAD")
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return strings.TrimSpace(out), nil
	}

	return &goModParent{
		Checkout:   co,
		modulePath: c.ModulePath,
		regex:      regex,
		createRoll: createRoll,
		uploadRoll: uploadRoll,
	}, nil
}

// getPinnedRevision reads the go.mod file and extracts the pinned revision of
// the configured dependency.
func (p *goModParent) getPinnedRevision() (string, error) {
	// Note: this flow only handles module dependencies which use Git revisions,
	// ie. the dependency looks like:
	// `go.skia.org/infra v0.0.0-20221018142618-5ea492a442f6`
	// We extract the version after the module path and split it on "-", taking
	// the last element, which is an abbreviated commit hash. If we wanted to
	// generalize this, we'd need a special type of Child which retrieves
	// semantic version Git tags instead of individual commit hashes, and we'd
	// have to distinguish between the two flows here.
	b, err := ioutil.ReadFile(filepath.Join(p.Checkout.Dir(), goModFile))
	if err != nil {
		return "", skerr.Wrapf(err, "failed to read %s", goModFile)
	}
	matches := p.regex.FindSubmatch(b)
	if len(matches) != 2 {
		return "", skerr.Fmt("failed to find %s in:\n%s", p.modulePath, string(b))
	}
	split := strings.Split(string(matches[1]), "-")
	return split[len(split)-1], nil
}

// See documentation for Parent interface.
func (p *goModParent) Update(ctx context.Context) (string, error) {
	_, _, err := p.Checkout.Update(ctx)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to update repo")
	}
	return p.getPinnedRevision()
}

// See documentation for Parent interface.
func (p *goModParent) CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, dryRun bool, commitMsg string) (int64, error) {
	return p.Checkout.CreateNewRoll(ctx, from, to, rolling, emails, dryRun, commitMsg, p.createRoll, p.uploadRoll)
}

var _ Parent = &goModParent{}
