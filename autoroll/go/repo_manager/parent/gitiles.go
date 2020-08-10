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
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
)

// GitilesConfig provides configuration for a Parent which uses Gitiles.
type GitilesConfig struct {
	gitiles_common.GitilesConfig
	version_file_common.DependencyConfig
	// Gerrit provides configuration for the Gerrit instance used for
	// uploading rolls.
	Gerrit *codereview.GerritConfig `json:"gerrit,omitempty"`
}

// See documentation for util.Validator interface.
func (c GitilesConfig) Validate() error {
	if c.Gerrit == nil {
		return skerr.Fmt("Gerrit is required")
	}
	if err := c.Gerrit.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.GitilesConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.DependencyConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if len(c.GitilesConfig.Dependencies) != 0 {
		return skerr.Fmt("Dependencies are inherited from the DependencyConfig and should not be set on the GitilesConfig.")
	}
	return nil
}

// gitilesGetChangesForRollFunc computes the changes to be made in the next
// roll. These are returned in a map[string]string whose keys are file paths
// within the repo and values are the new whole contents of the files. Also
// returns any dependencies which are being transitively rolled.
type gitilesGetChangesForRollFunc func(context.Context, *gitiles_common.GitilesRepo, string, *revision.Revision, *revision.Revision, []*revision.Revision) (map[string]string, error)

// gitilesParent is a base for implementations of Parent which use Gitiles.
type gitilesParent struct {
	*gitiles_common.GitilesRepo
	childID      string
	gerrit       gerrit.GerritInterface
	gerritConfig *codereview.GerritConfig
	serverURL    string

	getChangesForRoll gitilesGetChangesForRollFunc

	// TODO(borenet): We could make this "stateless" by having Parent.Update
	// also return the tip revision of the Parent and passing it through to
	// Parent.CreateNewRoll.
	baseCommit    string
	baseCommitMtx sync.Mutex // protects baseCommit.
}

// newGitiles returns a base for implementations of Parent which use Gitiles.
func newGitiles(ctx context.Context, c GitilesConfig, reg *config_vars.Registry, client *http.Client, serverURL string, getChangesForRoll gitilesGetChangesForRollFunc) (*gitilesParent, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	deps := make([]*version_file_common.VersionFileConfig, 0, len(c.DependencyConfig.TransitiveDeps)+1)
	deps = append(deps, &c.DependencyConfig.VersionFileConfig)
	for _, td := range c.DependencyConfig.TransitiveDeps {
		deps = append(deps, td.Parent)
	}
	c.GitilesConfig.Dependencies = deps
	gr, err := gitiles_common.NewGitilesRepo(ctx, c.GitilesConfig, reg, client)
	if err != nil {
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
	return &gitilesParent{
		childID:           c.DependencyConfig.ID,
		GitilesRepo:       gr,
		gerrit:            g,
		gerritConfig:      c.Gerrit,
		getChangesForRoll: getChangesForRoll,
	}, nil
}

// See documentation for Parent interface.
func (p *gitilesParent) Update(ctx context.Context) (string, error) {
	// Find the head of the branch we're tracking.
	baseCommit, err := p.GetTipRevision(ctx)
	if err != nil {
		return "", err
	}
	lastRollRev, ok := baseCommit.Dependencies[p.childID]
	if !ok {
		return "", skerr.Fmt("Unable to find dependency %q in %#v", p.childID, lastRollRev)
	}

	// Save the data.
	p.baseCommitMtx.Lock()
	defer p.baseCommitMtx.Unlock()
	p.baseCommit = baseCommit.Id
	return lastRollRev, nil
}

// See documentation for Parent interface.
func (p *gitilesParent) CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, dryRun bool, commitMsg string) (int64, error) {
	p.baseCommitMtx.Lock()
	defer p.baseCommitMtx.Unlock()

	nextRollChanges, err := p.getChangesForRoll(ctx, p.GitilesRepo, p.baseCommit, from, to, rolling)
	if err != nil {
		return 0, skerr.Wrapf(err, "getChangesForRoll func failed")
	}
	return CreateNewGerritRoll(ctx, p.gerrit, p.gerritConfig.Project, p.Branch(), commitMsg, p.baseCommit, nextRollChanges, emails, dryRun)
}

// CreateNewGerritRoll uploads a Gerrit CL with the given changes and returns
// the issue number or any error which occurred.
func CreateNewGerritRoll(ctx context.Context, g gerrit.GerritInterface, project, branch, commitMsg, baseCommit string, changes map[string]string, emails []string, dryRun bool) (int64, error) {
	// Create the change.
	ci, err := gerrit.CreateAndEditChange(ctx, g, project, branch, commitMsg, baseCommit, func(ctx context.Context, g gerrit.GerritInterface, ci *gerrit.ChangeInfo) error {
		for file, contents := range changes {
			if contents == "" {
				if err := g.DeleteFile(ctx, ci, file); err != nil {
					return skerr.Wrapf(err, "failed to delete file %s", file)
				}
			} else {
				if err := g.EditFile(ctx, ci, file, contents); err != nil {
					return skerr.Wrapf(err, "failed to edit file %s", file)
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
	if err := gerrit_common.SetChangeLabels(ctx, g, ci, emails, dryRun); err != nil {
		return 0, skerr.Wrap(err)
	}
	return ci.Issue, nil
}

var _ Parent = &gitilesParent{}
