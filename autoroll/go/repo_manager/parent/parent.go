package parent

/*
   Package parent contains implementations of the Parent interface.
*/

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"text/template"

	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// Parent represents a git repo (or other destination) which depends on a Child
// and is capable of producing rolls.
type Parent interface {
	// Update returns the pinned version of the dependency at the most
	// recent revision of the Parent. For implementations which use local
	// checkouts, this implies a sync.
	Update(context.Context) (string, error)

	// CreateNewRoll uploads a CL which updates the pinned version of the
	// dependency to the given Revision.
	CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, cqExtraTrybots string, dryRun bool) (int64, error)
}

// BaseConfig provides common configuration for all implementations of Parent.
// It is intended to be embedded in the various Parent Config types.
type BaseConfig struct {
	// Required fields.

	// If false, roll CLs do not link to bugs referenced by the Revisions
	// in the roll.
	IncludeBugs bool `json:"includeBugs"`
	// If true, include the "git log" (or other revision details) in the
	// commit message. This should be false for internal -> external rollers
	// to avoid leaking internal commit messages.
	IncludeLog bool `json:"includeLog"`

	// TODO(borenet): These fields are not applicable to some rollers, but
	// they're needed for CommitMsgVars. We should probably revisit how we
	// build commit messages.
	ChildPath string `json:"childPath"`
	ChildRepo string `json:"childRepo"`

	// Optional fields.

	// CommitMsgTmpl is a template used to build commit messages. See the
	// parent.CommitMsgVars type for more information.
	CommitMsgTmpl string `json:"commitMsgTmpl"`

	// Monorail project name associated with the Parent.
	// TODO(borenet): Add a BugFramework interface to support other
	// frameworks (eg. Buganizer, GitHub).
	MonorailProject string `json:"monorailProject,omitempty"`
}

// See documentation for util.Validator interface.
func (c BaseConfig) Validate() error {
	if c.ChildPath == "" {
		return skerr.Fmt("ChildPath is required")
	}
	if c.ChildRepo == "" {
		return skerr.Fmt("ChildRepo is required")
	}
	// All other fields are booleans or optional.
	return nil
}

// baseParent provides common functionality for all implementations of Parent.
type baseParent struct {
	childPath       string
	childRepoUrl    string
	commitMsgTmpl   *template.Template
	includeBugs     bool
	includeLog      bool
	monorailProject string
	serverUrl       string
}

func newBaseParent(ctx context.Context, c BaseConfig, serverUrl string) (*baseParent, error) {
	commitMsgTmplStr := TMPL_COMMIT_MSG_DEFAULT
	if c.CommitMsgTmpl != "" {
		commitMsgTmplStr = c.CommitMsgTmpl
	}
	commitMsgTmpl, err := ParseCommitMsgTemplate(commitMsgTmplStr)
	if err != nil {
		return nil, err
	}
	return &baseParent{
		childPath:       c.ChildPath,
		childRepoUrl:    c.ChildRepo,
		commitMsgTmpl:   commitMsgTmpl,
		includeBugs:     c.IncludeBugs,
		includeLog:      c.IncludeLog,
		monorailProject: c.MonorailProject,
		serverUrl:       serverUrl,
	}, nil
}

// buildCommitMsg is a helper function used to create commit messages.
func (p *baseParent) buildCommitMsg(from, to *revision.Revision, rolling []*revision.Revision, reviewers []string, cqExtraTrybots string, transitiveDeps []*TransitiveDep) (string, error) {
	// Basic variables.
	vars := &CommitMsgVars{
		ChildPath:      p.childPath,
		ChildRepo:      p.childRepoUrl,
		CqExtraTrybots: cqExtraTrybots,
		Reviewers:      reviewers,
		Revisions:      rolling,
		RollingFrom:    from,
		RollingTo:      to,
		ServerURL:      p.serverUrl,
		TransitiveDeps: transitiveDeps,
	}

	// Bugs.
	vars.Bugs = nil
	if p.includeBugs {
		// TODO(borenet): Move this to a util.MakeBugLines utility?
		bugMap := map[string]bool{}
		for _, rev := range vars.Revisions {
			for _, bug := range rev.Bugs[p.monorailProject] {
				bugMap[bug] = true
			}
		}
		if len(bugMap) > 0 {
			vars.Bugs = make([]string, 0, len(bugMap))
			for bug := range bugMap {
				bugStr := fmt.Sprintf("%s:%s", p.monorailProject, bug)
				if p.monorailProject == util.BUG_PROJECT_BUGANIZER {
					bugStr = fmt.Sprintf("b/%s", bug)
				}
				vars.Bugs = append(vars.Bugs, bugStr)
			}
			sort.Strings(vars.Bugs)
		}
	}

	// IncludeLog.
	vars.IncludeLog = p.includeLog

	// Tests.
	vars.Tests = nil
	testsMap := map[string]bool{}
	for _, rev := range vars.Revisions {
		for _, test := range rev.Tests {
			testsMap[test] = true
		}
	}
	if len(testsMap) > 0 {
		vars.Tests = make([]string, 0, len(testsMap))
		for test := range testsMap {
			vars.Tests = append(vars.Tests, test)
		}
		sort.Strings(vars.Tests)
	}

	// Create the commit message.
	var buf bytes.Buffer
	if err := p.commitMsgTmpl.Execute(&buf, vars); err != nil {
		return "", err
	}
	return buf.String(), nil
}
