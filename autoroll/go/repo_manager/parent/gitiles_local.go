package parent

/*
  Parent implementations which use Gitiles and a local checkout to create changes.
*/

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

const (
	// rollBranch is the git branch which is used to create rolls.
	rollBranch = "roll_branch"
)

// GitilesConfig provides configuration for a Parent which uses Gitiles and a
// local checkout to create changes.
type GitilesLocalConfig struct {
	GitilesConfig
}

// GitilesLocalParent is a base for implementations of Parent which use Gitiles
// and a local checkout to create changes.
type GitilesLocalParent struct {
	*gitilesParent
	co         *git.Checkout
	createRoll GitilesLocalCreateRollFunc
}

// GitilesLocalCreateRollFunc generates commit(s) in the local Git checkout to
// be used in the next roll. GitilesLocalParent handles creation of the roll
// branch and uploading the CL.
type GitilesLocalCreateRollFunc func(context.Context, *git.Checkout, *revision.Revision, *revision.Revision, []*revision.Revision, string) error

// NewGitilesLocal returns a base for implementations of Parent which use
// Gitiles and a local checkout to create changes.
func NewGitilesLocal(ctx context.Context, c GitilesLocalConfig, reg *config_vars.Registry, client *http.Client, serverURL, workdir string, update gitilesGetLastRollRevFunc, createRoll GitilesLocalCreateRollFunc) (*GitilesLocalParent, error) {
	// Create a gitilesParent to be used for Update().
	p, err := newGitiles(ctx, c.GitilesConfig, reg, client, serverURL, update, nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Create the local checkout.
	co, err := git.NewCheckout(ctx, c.RepoURL, workdir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Install the Gerrit Change-Id hook.
	out, err := co.Git(ctx, "rev-parse", "--git-dir")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	hookFile := filepath.Join(co.Dir(), strings.TrimSpace(out), "hooks", "commit-msg")
	if _, err := os.Stat(hookFile); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(hookFile), os.ModePerm); err != nil {
			return nil, skerr.Wrap(err)
		}
		if err := p.gerrit.DownloadCommitMsgHook(ctx, hookFile); err != nil {
			return nil, skerr.Wrap(err)
		}
	} else if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &GitilesLocalParent{
		gitilesParent: p,
		co:            co,
		createRoll:    createRoll,
	}, nil
}

// See documentation for Parent interface.
func (p *GitilesLocalParent) CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	p.baseCommitMtx.Lock()
	defer p.baseCommitMtx.Unlock()

	// Create the roll branch.
	upstreamBranch := p.Branch()
	if err := p.co.CleanupBranch(ctx, upstreamBranch); err != nil {
		return 0, skerr.Wrap(err)
	}
	_, _ = p.co.Git(ctx, "branch", "-D", rollBranch) // Fails if the branch does not exist.
	if _, err := p.co.Git(ctx, "checkout", "-b", rollBranch, "-t", fmt.Sprintf("origin/%s", upstreamBranch)); err != nil {
		return 0, skerr.Wrap(err)
	}
	if _, err := p.co.Git(ctx, "reset", "--hard", p.baseCommit); err != nil {
		return 0, skerr.Wrap(err)
	}

	// Generate the commit message.
	// TODO(borenet): This should probably move into parentChildRepoManager.
	commitMsg, err := p.buildCommitMsg(from, to, rolling, emails, cqExtraTrybots, nil)
	if err != nil {
		return 0, skerr.Wrap(err)
	}

	// Run the provided function to create the changes for the roll.
	if err := p.createRoll(ctx, p.co, from, to, rolling, commitMsg); err != nil {
		return 0, skerr.Wrap(err)
	}

	// Ensure that createRoll generated at least one commit downstream of
	// p.baseCommit, and that it did not leave uncommitted changes.
	commits, err := p.co.RevList(ctx, "--ancestry-path", "--first-parent", fmt.Sprintf("%s..HEAD", p.baseCommit))
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	if len(commits) == 0 {
		return 0, skerr.Fmt("createRoll generated no commits!")
	}
	if _, err := p.co.Git(ctx, "diff", "--quiet"); err != nil {
		return 0, skerr.Wrapf(err, "createRoll left uncommitted changes")
	}
	out, err := p.co.Git(ctx, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	if len(strings.Fields(out)) > 0 {
		return 0, skerr.Fmt("createRoll left untracked files:\n%s", out)
	}

	// Find the change ID in the commit message.
	// TODO(borenet): Should we use the most recent or the first (new) commit?
	out, err = p.co.Git(ctx, "log", "-n1", commits[0])
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	changeId, err := gerrit.ParseChangeId(out)
	if err != nil {
		return 0, skerr.Wrap(err)
	}

	// Upload CL.
	if _, err := p.co.Git(ctx, "push", "origin", fmt.Sprintf("HEAD:refs/for/%s", upstreamBranch)); err != nil {
		return 0, skerr.Wrap(err)
	}
	ci, err := p.gerrit.GetChange(ctx, changeId)
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	if err := gerrit_common.SetChangeLabels(ctx, p.gerrit, ci, emails, dryRun); err != nil {
		return 0, skerr.Wrap(err)
	}

	return ci.Issue, nil
}

var _ Parent = &GitilesLocalParent{}
