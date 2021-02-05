package repo_manager

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
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

// CommandRepoManager implements RepoManager by shelling out to various
// configured commands to perform all of the work.
type CommandRepoManager struct {
	co            *git_common.Checkout
	shortRevRegex *config_vars.Template
	getTipRev     *config.CommandRepoManagerConfig_CommandConfig
	getPinnedRev  *config.CommandRepoManagerConfig_CommandConfig
	setPinnedRev  *config.CommandRepoManagerConfig_CommandConfig
	createRoll    git_common.CreateRollFunc
	uploadRoll    git_common.UploadRollFunc
}

// NewCommandRepoManager returns a RepoManager implementation which rolls
// trace_processor_shell into Chrome.
func NewCommandRepoManager(ctx context.Context, c *config.CommandRepoManagerConfig, reg *config_vars.Registry, workdir, serverURL string, cr codereview.CodeReview) (*CommandRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	var shortRevRegex *config_vars.Template
	if c.ShortRevRegex != "" {
		var err error
		shortRevRegex, err = config_vars.NewTemplate(c.ShortRevRegex)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		if err := reg.Register(shortRevRegex); err != nil {
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
	g, ok := cr.Client().(gerrit.GerritInterface)
	if !ok {
		return nil, skerr.Fmt("CommandRepoManager must use Gerrit for code review.")
	}
	uploadRoll := parent.GitCheckoutUploadGerritRollFunc(g)
	co, err := git_common.NewCheckout(ctx, c.GitCheckout, reg, workdir, cr.UserName(), cr.UserEmail(), nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := gerrit_common.SetupGerrit(ctx, co.Checkout, g); err != nil {
		return nil, skerr.Wrap(err)
	}
	rm = &CommandRepoManager{
		co:            co,
		shortRevRegex: shortRevRegex,
		getTipRev:     c.GetTipRev,
		getPinnedRev:  c.GetPinnedRev,
		setPinnedRev:  c.SetPinnedRev,
		createRoll:    createRoll,
		uploadRoll:    uploadRoll,
	}
	return rm, nil
}

func makeCommand(cfg *config.CommandRepoManagerConfig_CommandConfig, baseDir string, vars *CommandTmplVars) (*exec.Command, error) {
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
func (rm *CommandRepoManager) run(ctx context.Context, cmd *config.CommandRepoManagerConfig_CommandConfig, vars *CommandTmplVars) (string, error) {
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

// Update implements RepoManager.
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

// GetRevision implements RepoManager.
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

// CreateNewRoll implements RepoManager.
func (rm *CommandRepoManager) CreateNewRoll(ctx context.Context, rollingFrom *revision.Revision, rollingTo *revision.Revision, revisions []*revision.Revision, reviewers []string, dryRun bool, commitMsg string) (int64, error) {
	return rm.co.CreateNewRoll(ctx, rollingFrom, rollingTo, revisions, reviewers, dryRun, commitMsg, rm.createRoll, rm.uploadRoll)
}

// commandRepoManager implements RepoManager.
var _ RepoManager = &CommandRepoManager{}
