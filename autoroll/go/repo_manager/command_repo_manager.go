package repo_manager

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

// CommandConfig provides configuration for a command used by
// CommandRepoManager.
type CommandConfig struct {
	// Command is the command to run. Required.
	Command string `json:"command"`

	// Dir is the directory in which to run the command, relative to the
	// checkout path. Optional.
	Dir string `json:"dir"`

	// Env are the environment variables to supply to the command. Optional.
	Env map[string]string `json:"env"`
}

// See documentation for util.Validate interface.
func (c CommandConfig) Validate() error {
	if c.Command == "" {
		return skerr.Fmt("Command is required")
	}
	return nil
}

// CommandRepoManagerConfig provides configuration for CommandRepoManager.
type CommandRepoManagerConfig struct {
	git_common.GitCheckoutConfig
	GetTipRev    *CommandConfig `json:"getTipRev"`
	GetPinnedRev *CommandConfig `json:"getPinnedRev"`
	SetPinnedRev *CommandConfig `json:"setPinnedRev"`
}

// See documentation for util.Validate interface.
func (c CommandRepoManagerConfig) Validate() error {
	if err := c.GitCheckoutConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.GetTipRev == nil {
		return skerr.Fmt("GetTipRev is required")
	}
	if err := c.GetTipRev.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.GetPinnedRev == nil {
		return skerr.Fmt("GetPinnedRev is required")
	}
	if err := c.GetPinnedRev.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.SetPinnedRev == nil {
		return skerr.Fmt("SetPinnedRev is required")
	}
	if err := c.SetPinnedRev.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// CommandRepoManager implements RepoManager by shelling out to various
// configured commands to perform all of the work.
type CommandRepoManager struct {
	co           *parent.GitCheckoutParent
	getTipRev    *CommandConfig
	getPinnedRev *CommandConfig
	setPinnedRev *CommandConfig
}

// NewCommandRepoManager returns a RepoManager implementation which rolls
// trace_processor_shell into Chrome.
func NewCommandRepoManager(ctx context.Context, c CommandRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, serverURL string, cr codereview.CodeReview) (*CommandRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	fakeConfig := parent.GitCheckoutConfig{
		GitCheckoutConfig: c.GitCheckoutConfig,
		DependencyConfig: version_file_common.DependencyConfig{
			VersionFileConfig: version_file_common.VersionFileConfig{
				ID:   "fake",
				Path: "fake",
			},
		},
	}
	rm := &CommandRepoManager{
		getTipRev:    c.GetTipRev,
		getPinnedRev: c.GetPinnedRev,
		setPinnedRev: c.SetPinnedRev,
	}
	createRoll := func(ctx context.Context, co *git.Checkout, _, to *revision.Revision, _ []*revision.Revision, commitMsg string) (string, error) {
		if _, err := rm.run(ctx, rm.setPinnedRev); err != nil {
			return "", skerr.Wrap(err)
		}
		if _, err := co.Git(ctx, "commit", "-a", "-m", commitMsg); err != nil {
			return "", skerr.Wrap(err)
		}
		out, err := co.Git(ctx, "rev-parse", "HEAD")
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return strings.TrimSpace(out), nil
	}
	uploadRoll := parent.GitCheckoutUploadGerritRollFunc(g)
	co, err := parent.NewGitCheckout(ctx, fakeConfig, reg, serverURL, workdir, cr.UserName(), cr.UserEmail(), nil, createRoll, uploadRoll)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := parent.SetupGerrit(ctx, co, g); err != nil {
		return nil, skerr.Wrap(err)
	}
	rm.co = co
	return rm, nil
}

// Run the given command and return the output.
func (rm *CommandRepoManager) run(ctx context.Context, cmd *CommandConfig) (string, error) {
	c := exec.ParseCommand(cmd.Command)
	c.Dir = rm.co.Dir()
	if cmd.Dir != "" {
		c.Dir = filepath.Join(c.Dir, cmd.Dir)
	}
	for k, v := range cmd.Env {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", k, v))
	}
	c.InheritEnv = true
	out, err := exec.RunCommand(ctx, &c)
	if err != nil {
		return out, err
	}
	return strings.TrimSpace(out), nil
}

// See documentation for RepoManager interface.
func (rm *CommandRepoManager) Update(ctx context.Context) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	if _, _, err := rm.co.Checkout.Update(ctx); err != nil {
		return nil, nil, nil, skerr.Wrap(err)
	}
	tipRevStr, err := rm.run(ctx, rm.getTipRev)
	if err != nil {
		return nil, nil, nil, skerr.Wrap(err)
	}
	tipRev := &revision.Revision{Id: tipRevStr}
	lastRollRevStr, err := rm.run(ctx, rm.getPinnedRev)
	if err != nil {
		return nil, nil, nil, skerr.Wrap(err)
	}
	lastRollRev := &revision.Revision{Id: lastRollRevStr}
	var notRolledRevs []*revision.Revision
	if lastRollRevStr != tipRevStr {
		notRolledRevs = append(notRolledRevs, tipRev)
	}
	return lastRollRev, tipRev, notRolledRevs, nil
}

// See documentation for RepoManager interface.
func (rm *CommandRepoManager) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	// Just return the most basic Revision with the given ID. Note that we could
	// add a command which retrieves revision information and passes it back to
	// us in JSON format or something, but I'm not sure how valuable that would
	// be.
	return &revision.Revision{Id: id}, nil
}

// See documentation for RepoManager interface.
func (rm *CommandRepoManager) CreateNewRoll(ctx context.Context, rollingFrom *revision.Revision, rollingTo *revision.Revision, revisions []*revision.Revision, reviewers []string, dryRun bool, commitMsg string) (int64, error) {
	return rm.co.CreateNewRoll(ctx, rollingFrom, rollingTo, revisions, reviewers, dryRun, commitMsg)
}

// commandRepoManager implements RepoManager.
var _ RepoManager = &CommandRepoManager{}
