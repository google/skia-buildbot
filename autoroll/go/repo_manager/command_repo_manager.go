package repo_manager

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// CommandTmplVars is used as input to the text template provided to the
// SetPinnedRev command for updating the revision of the Child.
type CommandTmplVars struct {
	// RollingFrom is the revision we're updating from.
	RollingFrom string

	// RollingTo is the revision we're updating to.
	RollingTo string
}

// CommandConfig provides configuration for a command used by
// CommandRepoManager.
type CommandConfig struct {
	// Command is the command to run. Required. If this is the Command used to
	// update the revision of the Child, this should be a text template which
	// uses SetPinnedRevVars to get the from-and-to-revisions.
	Command []string `json:"command"`

	// Dir is the directory in which to run the command, relative to the
	// checkout path. Optional.
	Dir string `json:"dir,omitempty"`

	// Env are the environment variables to supply to the command. Optional.
	Env map[string]string `json:"env,omitempty"`
}

// See documentation for util.Validate interface.
func (c CommandConfig) Validate() error {
	if len(c.Command) == 0 {
		return skerr.Fmt("Command is required")
	}
	return nil
}

// CommandRepoManagerConfig provides configuration for CommandRepoManager.
type CommandRepoManagerConfig struct {
	git_common.GitCheckoutConfig
	// ShortRevRegex is a regular expression used to shorten revision IDs for
	// display.
	ShortRevRegex *config_vars.Template `json:"shortRevRegex"`
	// GetTipRev is the command used to obtain the latest revision of the Child.
	GetTipRev *CommandConfig `json:"getTipRev"`
	// GetPinnedRev is the command used to obtain the currently-pinned revision
	// of the Child.
	GetPinnedRev *CommandConfig `json:"getPinnedRev"`
	// SetPinnedRev is the command used to update the currently-pinned revision
	// of the Child.
	SetPinnedRev *CommandConfig `json:"setPinnedRev"`
}

// See documentation for util.Validate interface.
func (c CommandRepoManagerConfig) Validate() error {
	if err := c.GitCheckoutConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.ShortRevRegex != nil {
		if err := c.ShortRevRegex.Validate(); err != nil {
			return err
		}
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

// See documentation for RepoManagerConfig interface.
func (c CommandRepoManagerConfig) ValidStrategies() []string {
	return []string{strategy.ROLL_STRATEGY_BATCH}
}

// See documentation for RepoManagerConfig interface.
func (c CommandRepoManagerConfig) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

// See documentation for RepoMangerConfig interface.
func (c CommandRepoManagerConfig) NoCheckout() bool {
	return false
}

// CommandRepoManager implements RepoManager by shelling out to various
// configured commands to perform all of the work.
type CommandRepoManager struct {
	co            *git_common.Checkout
	shortRevRegex *config_vars.Template
	getTipRev     *CommandConfig
	getPinnedRev  *CommandConfig
	setPinnedRev  *CommandConfig
	createRoll    git_common.CreateRollFunc
	uploadRoll    git_common.UploadRollFunc
}

// NewCommandRepoManager returns a RepoManager implementation which rolls
// trace_processor_shell into Chrome.
func NewCommandRepoManager(ctx context.Context, c CommandRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, serverURL string, cr codereview.CodeReview) (*CommandRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	if c.ShortRevRegex != nil {
		if err := reg.Register(c.ShortRevRegex); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	var rm *CommandRepoManager
	createRoll := func(ctx context.Context, co *git.Checkout, from, to *revision.Revision, _ []*revision.Revision, commitMsg string) (string, error) {
		vars := &CommandTmplVars{
			RollingFrom: from.Id,
			RollingTo:   to.Id,
		}
		if _, err := rm.run(ctx, rm.setPinnedRev, vars); err != nil {
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
	co, err := git_common.NewCheckout(ctx, c.GitCheckoutConfig, reg, workdir, cr.UserName(), cr.UserEmail(), nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := gerrit_common.SetupGerrit(ctx, co.Checkout, g); err != nil {
		return nil, skerr.Wrap(err)
	}
	rm = &CommandRepoManager{
		co:            co,
		shortRevRegex: c.ShortRevRegex,
		getTipRev:     c.GetTipRev,
		getPinnedRev:  c.GetPinnedRev,
		setPinnedRev:  c.SetPinnedRev,
		createRoll:    createRoll,
		uploadRoll:    uploadRoll,
	}
	return rm, nil
}

func makeCommand(cfg *CommandConfig, baseDir string, vars *CommandTmplVars) (*exec.Command, error) {
	args := make([]string, 0, len(cfg.Command))
	for _, arg := range cfg.Command {
		tmpl, err := template.New(arg).Parse(arg)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, vars); err != nil {
			return nil, skerr.Wrap(err)
		}
		args = append(args, buf.String())
	}
	c := &exec.Command{
		Name: args[0],
		Args: args[1:],
		Dir:  baseDir,
	}
	if cfg.Dir != "" {
		c.Dir = filepath.Join(c.Dir, cfg.Dir)
	}
	for k, v := range cfg.Env {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", k, v))
	}
	c.InheritEnv = true
	return c, nil
}

// Run the given command and return the output.
func (rm *CommandRepoManager) run(ctx context.Context, cmd *CommandConfig, vars *CommandTmplVars) (string, error) {
	c, err := makeCommand(cmd, rm.co.Dir(), vars)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	sklog.Infof("Running: %s %s", c.Name, strings.Join(c.Args, " "))
	out, err := exec.RunCommand(ctx, c)
	if err != nil {
		return out, err
	}
	return strings.TrimSpace(out), nil
}

// See documentation for RepoManager interface.
func (rm *CommandRepoManager) Update(ctx context.Context) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	if _, _, err := rm.co.Update(ctx); err != nil {
		return nil, nil, nil, skerr.Wrap(err)
	}
	tipRevStr, err := rm.run(ctx, rm.getTipRev, nil)
	if err != nil {
		return nil, nil, nil, skerr.Wrap(err)
	}
	tipRev, err := rm.GetRevision(ctx, tipRevStr)
	if err != nil {
		return nil, nil, nil, skerr.Wrap(err)
	}
	lastRollRevStr, err := rm.run(ctx, rm.getPinnedRev, nil)
	if err != nil {
		return nil, nil, nil, skerr.Wrap(err)
	}
	lastRollRev, err := rm.GetRevision(ctx, lastRollRevStr)
	if err != nil {
		return nil, nil, nil, skerr.Wrap(err)
	}
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
	rev := &revision.Revision{Id: id}
	if rm.shortRevRegex != nil {
		shortRev, err := child.ShortRev(rm.shortRevRegex.String(), rev.Id)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rev.Display = shortRev
	}
	return rev, nil
}

// See documentation for RepoManager interface.
func (rm *CommandRepoManager) CreateNewRoll(ctx context.Context, rollingFrom *revision.Revision, rollingTo *revision.Revision, revisions []*revision.Revision, reviewers []string, dryRun bool, commitMsg string) (int64, error) {
	return rm.co.CreateNewRoll(ctx, rollingFrom, rollingTo, revisions, reviewers, dryRun, commitMsg, rm.createRoll, rm.uploadRoll)
}

// commandRepoManager implements RepoManager.
var _ RepoManager = &CommandRepoManager{}
