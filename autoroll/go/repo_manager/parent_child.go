package repo_manager

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
)

type ParentChildRepoManagerConfig struct {
	// Required fields.

	// If false, roll CLs do not link to bugs referenced by the Revisions
	// in the roll.
	IncludeBugs bool `json:"includeBugs"`
	// If true, include the "git log" (or other revision details) in the
	// commit message. This should be false for internal -> external rollers
	// to avoid leaking internal commit messages.
	IncludeLog bool `json:"includeLog"`

	// Optional fields.

	// CommitMsgTmpl is a template used to build commit messages. See the
	// parent.CommitMsgVars type for more information.
	CommitMsgTmpl string `json:"commitMsgTmpl"`

	// Monorail project name associated with the Parent.
	// TODO(borenet): Add a BugFramework interface to support other
	// frameworks (eg. Buganizer, GitHub).
	MonorailProject string `json:"monorailProject,omitempty"`
}

// TODO: Validation!

// parentChildRepoManager combines a Parent and a Child to implement the
// RepoManager interface.
type parentChildRepoManager struct {
	c               child.Child
	p               parent.Parent
	commitMsgTmpl   *template.Template
	includeBugs     bool
	includeLog      bool
	monorailProject bool
}

// newParentChildRepoManager returns a RepoManager which pairs a Parent with a
// Child.
func newParentChildRepoManager(ctx context.Context, cfg *ParentChildRepoManagerConfig, p parent.Parent, c child.Child) (RepoManager, error) {
	commitMsgTmplStr := parent.TMPL_COMMIT_MSG_DEFAULT
	if cfg.CommitMsgTmpl != "" {
		commitMsgTmplStr = cfg.CommitMsgTmpl
	}
	commitMsgTmpl, err := parent.ParseCommitMsgTemplate(commitMsgTmplStr)
	if err != nil {
		return nil, err
	}
	return &parentChildRepoManager{
		c:               c,
		p:               p,
		commitMsgTmpl:   commitMsgTmpl,
		includeBugs:     cfg.IncludeBugs,
		includeLog:      cfg.IncludeLog,
		monorailProject: cfg.MonorailProject,
	}, nil
}

// buildCommitMsg is a helper function used to create commit messages.
// TODO(borenet): This duplicates code in RepoManager.
func (rm *parentChildRepoManager) buildCommitMsg(vars *parent.CommitMsgVars) (string, error) {
	// Override Bugs and IncludeLog.
	if p.includeBugs {
		if vars.Bugs == nil {
			vars.Bugs = parseMonorailBugs(vars.Revisions, p.monorailProject)
		}
	} else {
		vars.Bugs = nil
	}
	vars.IncludeLog = p.includeLog
	var buf bytes.Buffer
	if err := p.commitMsgTmpl.Execute(&buf, vars); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// See documentation for RepoManager interface.
func (rm *parentChildRepoManager) Update(ctx context.Context) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	lastRollRevId, err := rm.p.Update(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Failed to update Parent: %s", err)
	}
	lastRollRev, err := rm.c.GetRevision(ctx, lastRollRevId)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Failed to obtain last-rolled revision %q: %s", lastRollRevId, err)
	}
	tipRev, notRolledRevs, err := rm.c.Update(ctx, lastRollRev)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Failed to get next revision to roll from Child: %s", err)
	}
	return lastRollRev, tipRev, notRolledRevs, nil
}

// See documentation for RepoManager interface.
func (rm *parentChildRepoManager) CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	commitMsg, err := rm.buildCommitMsg(&parent.CommitMsgVars{
		// TODO
	})
	return rm.p.CreateNewRoll(ctx, from, to, rolling, emails, cqExtraTrybots, dryRun, commitMsg)
}
