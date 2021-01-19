package repo_manager

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"sync"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	ROLL_BRANCH = "roll_branch"
)

// RepoManager is the interface used by different Autoroller implementations
// to manage checkouts.
type RepoManager interface {
	// Create a new roll attempt.
	CreateNewRoll(ctx context.Context, rollingFrom *revision.Revision, rollingTo *revision.Revision, revisions []*revision.Revision, reviewers []string, dryRun bool, commitMsg string) (int64, error)

	// Update the RepoManager's view of the world. Depending on the
	// implementation, this may sync repos and may take some time. Returns
	// the currently-rolled Revision, the tip-of-tree Revision, and a list
	// of all revisions which have not yet been rolled (ie. those between
	// the current and tip-of-tree, including the latter), in reverse
	// chronological order.
	Update(context.Context) (*revision.Revision, *revision.Revision, []*revision.Revision, error)

	// GetRevision returns a revision.Revision instance from the given
	// revision ID.
	GetRevision(context.Context, string) (*revision.Revision, error)
}

// CommonRepoManagerConfig provides configuration for commonRepoManager.
type CommonRepoManagerConfig struct {
	// Required fields.

	// Branch of the child repo we want to roll.
	ChildBranch *config_vars.Template `json:"childBranch,omitempty"`
	// Path of the child repo within the parent repo.
	ChildPath string `json:"childPath,omitempty"`
	// Branch of the parent repo we want to roll into.
	ParentBranch *config_vars.Template `json:"parentBranch"`
	// URL of the parent repo.
	ParentRepo string `json:"parentRepo"`

	// Optional fields.

	// ChildRevLinkTmpl is a template used to create links to revisions of
	// the child repo. If not supplied, no links will be created.
	ChildRevLinkTmpl string `json:"childRevLinkTmpl,omitempty"`
	// ChildSubdir indicates the subdirectory of the workdir in which
	// the childPath should be rooted. In most cases, this should be empty,
	// but if ChildPath is relative to the parent repo dir (eg. when DEPS
	// specifies use_relative_paths), then this is required.
	ChildSubdir string `json:"childSubdir,omitempty"`
	// Named steps to run before uploading roll CLs.
	PreUploadSteps []string `json:"preUploadSteps,omitempty"`
}

// Validate the config.
func (c *CommonRepoManagerConfig) Validate() error {
	if c.ChildBranch == nil {
		return errors.New("ChildBranch is required.")
	}
	if err := c.ChildBranch.Validate(); err != nil {
		return err
	}
	if c.ChildPath == "" {
		return errors.New("ChildPath is required.")
	}
	if c.ParentBranch == nil {
		return errors.New("ParentBranch is required.")
	}
	if err := c.ParentBranch.Validate(); err != nil {
		return err
	}
	if c.ParentRepo == "" {
		return errors.New("ParentRepo is required.")
	}
	for _, s := range c.PreUploadSteps {
		if _, err := parent.GetPreUploadStep(s); err != nil {
			return err
		}
	}
	return nil
}

// See documentation for RepoManagerConfig interface.
func (r *CommonRepoManagerConfig) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

// See documentation for RepoManagerConfig interface.
func (r *CommonRepoManagerConfig) NoCheckout() bool {
	return false
}

// See documentation for RepoManagerConfig interface.
func (r *CommonRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
		strategy.ROLL_STRATEGY_SINGLE,
	}
}

// commonRepoManager is a struct used by the AutoRoller implementations for
// managing checkouts.
type commonRepoManager struct {
	childBranch      *config_vars.Template
	childDir         string
	childPath        string
	childRepo        *git.Checkout
	childRevLinkTmpl string
	g                gerrit.GerritInterface
	httpClient       *http.Client
	parentBranch     *config_vars.Template
	preUploadSteps   []parent.PreUploadStep
	repoMtx          sync.RWMutex
	workdir          string
}

// Returns a commonRepoManager instance.
func newCommonRepoManager(ctx context.Context, c CommonRepoManagerConfig, reg *config_vars.Registry, workdir, serverURL string, g gerrit.GerritInterface, client *http.Client, cr codereview.CodeReview, local bool) (*commonRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(workdir, os.ModePerm); err != nil {
		return nil, err
	}
	childDir := path.Join(workdir, c.ChildPath)
	if c.ChildSubdir != "" {
		childDir = path.Join(workdir, c.ChildSubdir, c.ChildPath)
	}
	childRepo := &git.Checkout{GitDir: git.GitDir(childDir)}

	if _, err := os.Stat(workdir); err == nil {
		if err := git.DeleteLockFiles(ctx, workdir); err != nil {
			return nil, err
		}
	}
	preUploadSteps, err := parent.GetPreUploadSteps(c.PreUploadSteps)
	if err != nil {
		return nil, err
	}
	if err := reg.Register(c.ChildBranch); err != nil {
		return nil, err
	}
	if err := reg.Register(c.ParentBranch); err != nil {
		return nil, err
	}
	return &commonRepoManager{
		childBranch:      c.ChildBranch,
		childDir:         childDir,
		childPath:        c.ChildPath,
		childRepo:        childRepo,
		childRevLinkTmpl: c.ChildRevLinkTmpl,
		g:                g,
		httpClient:       client,
		parentBranch:     c.ParentBranch,
		preUploadSteps:   preUploadSteps,
		workdir:          workdir,
	}, nil
}

func (r *commonRepoManager) getTipRev(ctx context.Context) (*revision.Revision, error) {
	c, err := r.childRepo.Details(ctx, fmt.Sprintf("origin/%s", r.childBranch))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return revision.FromLongCommit(r.childRevLinkTmpl, c), nil
}

func (r *commonRepoManager) getCommitsNotRolled(ctx context.Context, lastRollRev, tipRev *revision.Revision) ([]*revision.Revision, error) {
	if tipRev.Id == lastRollRev.Id {
		return []*revision.Revision{}, nil
	}
	commits, err := r.childRepo.RevList(ctx, "--first-parent", git.LogFromTo(lastRollRev.Id, tipRev.Id))
	if err != nil {
		return nil, err
	}
	notRolled := make([]*vcsinfo.LongCommit, 0, len(commits))
	for _, c := range commits {
		detail, err := r.childRepo.Details(ctx, c)
		if err != nil {
			return nil, err
		}
		notRolled = append(notRolled, detail)
	}
	return revision.FromLongCommits(r.childRevLinkTmpl, notRolled), nil
}

// See documentation for RepoManager interface.
func (r *commonRepoManager) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	details, err := r.childRepo.Details(ctx, id)
	if err != nil {
		return nil, err
	}
	return revision.FromLongCommit(r.childRevLinkTmpl, details), nil
}

// DepotToolsRepoManagerConfig provides configuration for depotToolsRepoManager.
type DepotToolsRepoManagerConfig struct {
	CommonRepoManagerConfig

	// Optional fields.

	// Override the default gclient spec with this string.
	GClientSpec string `json:"gclientSpec,omitempty"`

	// Run "gclient runhooks" if true.
	RunHooks bool `json:"runHooks,omitempty"`
}

// depotToolsRepoManager is a struct used by AutoRoller implementations that use
// depot_tools to manage checkouts.
type depotToolsRepoManager struct {
	*commonRepoManager
	depotTools    string
	depotToolsEnv []string
	gclient       string
	gclientSpec   string
	parentDir     string
	parentRepo    string
	runhooks      bool
}

// NoCheckoutRepoManagerConfig provides configuration for RepoManagers which
// don't use a local checkout.
type NoCheckoutRepoManagerConfig struct {
	CommonRepoManagerConfig
}

// See documentation for RepoManagerConfig interface.
func (c *NoCheckoutRepoManagerConfig) NoCheckout() bool {
	return true
}

// See documentation for util.Validator interface.
func (c *NoCheckoutRepoManagerConfig) Validate() error {
	if err := c.CommonRepoManagerConfig.Validate(); err != nil {
		return err
	}
	if len(c.PreUploadSteps) > 0 {
		return errors.New("Checkout-less rollers don't support pre-upload steps")
	}
	return nil
}
