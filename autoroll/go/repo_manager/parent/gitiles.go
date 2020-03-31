package parent

/*
   Parent implementations for Git repos using Gitiles (implies Gerrit).
*/

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
)

// GitilesConfig provides configuration for a Parent which uses Gitiles.
type GitilesConfig struct {
	BaseConfig
	Gerrit  *codereview.GerritConfig `json:"gerrit"`
	Branch  *config_vars.Template    `json:"branch"`
	RepoURL string                   `json:"repoURL"`
}

// See documentation for util.Validator interface.
func (c GitilesConfig) Validate() error {
	if err := c.BaseConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.Gerrit == nil {
		return skerr.Fmt("Gerrit is required")
	}
	if err := c.Gerrit.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.Branch == nil {
		return skerr.Fmt("Branch is required")
	}
	if err := c.Branch.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.RepoURL == "" {
		return skerr.Fmt("RepoURL is required")
	}
	return nil
}

// gitilesGetChangesForRollFunc computes the changes to be made in the next
// roll. These are returned in a map[string]string whose keys are file paths
// within the repo and values are the new whole contents of the files. Also
// returns any dependencies which are being transitively rolled.
type gitilesGetChangesForRollFunc func(context.Context, *gitiles.Repo, string, *revision.Revision, *revision.Revision, []*revision.Revision) (map[string]string, []*TransitiveDep, error)

// gitilesGetLastRollRevFunc finds the last-rolled child revision ID from the
// repo at the given base commit.
type gitilesGetLastRollRevFunc func(context.Context, *gitiles.Repo, string) (string, error)

// newGitiles returns a base for implementations of Parent which use Gitiles.
func newGitiles(ctx context.Context, c GitilesConfig, reg *config_vars.Registry, client *http.Client, serverURL string, update gitilesGetLastRollRevFunc, getChangesForRoll gitilesGetChangesForRollFunc) (*gitilesParent, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := reg.Register(c.Branch); err != nil {
		return nil, skerr.Wrap(err)
	}
	gc, err := c.Gerrit.GetConfig()
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get Gerrit config")
	}
	g, err := gerrit.NewGerritWithConfig(gc, c.Gerrit.URL, client)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create Gerrit client")
	}
	base, err := newBaseParent(ctx, c.BaseConfig, serverURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &gitilesParent{
		baseParent:        base,
		branch:            c.Branch,
		gerrit:            g,
		gerritConfig:      c.Gerrit,
		getChangesForRoll: getChangesForRoll,
		update:            update,
		repo:              gitiles.NewRepo(c.RepoURL, client),
	}, nil
}

// gitilesParent is a base for implementations of Parent which use Gitiles.
type gitilesParent struct {
	*baseParent
	branch       *config_vars.Template
	gerrit       gerrit.GerritInterface
	gerritConfig *codereview.GerritConfig
	serverURL    string

	getChangesForRoll gitilesGetChangesForRollFunc
	update            gitilesGetLastRollRevFunc

	repo *gitiles.Repo

	baseCommit    string
	baseCommitMtx sync.Mutex // protects baseCommit.
}

// See documentation for Parent interface.
func (p *gitilesParent) Update(ctx context.Context) (string, error) {
	// Find the head of the branch we're tracking.
	baseCommit, err := p.repo.Details(ctx, p.branch.String())
	if err != nil {
		return "", err
	}

	// Call the provided gitilesGetLastRollRevFunc.
	lastRollRev, err := p.update(ctx, p.repo, baseCommit.Hash)
	if err != nil {
		return "", err
	}

	// Save the data.
	p.baseCommitMtx.Lock()
	defer p.baseCommitMtx.Unlock()
	p.baseCommit = baseCommit.Hash
	return lastRollRev, nil
}

// See documentation for Parent interface.
func (p *gitilesParent) CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	p.baseCommitMtx.Lock()
	defer p.baseCommitMtx.Unlock()

	nextRollChanges, transitiveDeps, err := p.getChangesForRoll(ctx, p.repo, p.baseCommit, from, to, rolling)
	if err != nil {
		return 0, skerr.Wrapf(err, "getChangesForRoll func failed")
	}

	commitMsg, err := p.buildCommitMsg(from, to, rolling, emails, cqExtraTrybots, transitiveDeps)
	if err != nil {
		return 0, skerr.Wrap(err)
	}

	return CreateNewGerritRoll(ctx, p.gerrit, p.gerritConfig.Project, p.branch.String(), commitMsg, p.baseCommit, nextRollChanges, emails, dryRun)
}

// CreateNewGerritRoll uploads a Gerrit CL with the given changes and returns
// the issue number or any error which occurred.
func CreateNewGerritRoll(ctx context.Context, g gerrit.GerritInterface, project, branch, commitMsg, baseCommit string, changes map[string]string, emails []string, dryRun bool) (int64, error) {
	// Create the change.
	ci, err := gerrit.CreateAndEditChange(ctx, g, project, branch, commitMsg, baseCommit, func(ctx context.Context, g gerrit.GerritInterface, ci *gerrit.ChangeInfo) error {
		for file, contents := range changes {
			if contents == "" {
				if err := g.DeleteFile(ctx, ci, file); err != nil {
					return fmt.Errorf("Failed to delete %s file: %s", file, err)
				}
			} else {
				if err := g.EditFile(ctx, ci, file, contents); err != nil {
					return fmt.Errorf("Failed to edit %s file: %s", file, err)
				}
			}
		}
		return nil
	})
	if err != nil {
		if ci != nil {
			if err2 := g.Abandon(ctx, ci, "Failed to create roll CL"); err2 != nil {
				return 0, fmt.Errorf("Failed to create roll with: %s\nAnd failed to abandon the change with: %s", err, err2)
			}
		}
		return 0, err
	}
	if err := SetChangeLabels(ctx, g, ci, emails, dryRun); err != nil {
		return 0, skerr.Wrap(err)
	}
	return ci.Issue, nil
}

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
	if err := g.SetReview(ctx, ci, "", labels, emails); err != nil {
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

// Helper function for unsetting the WIP bit on a Gerrit CL if necessary.
// Either the change or issueNum parameter is required; if change is not
// provided, it will be loaded from Gerrit. unsetWIP checks for a nil
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

var _ Parent = &gitilesParent{}
