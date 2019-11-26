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
	"go.skia.org/infra/go/util"
)

// Parent represents a git repo (or other destination) which depends on a Child
// and is capable of producing rolls.
type Parent interface {
	// Update updates the local view of the Parent and returns the ID of the
	// last-rolled revision of the Child.
	Update(context.Context) (string, error)

	// CreateNewRoll uploads a new roll CL.
	CreateNewRoll(context.Context, *revision.Revision, *revision.Revision, []*revision.Revision, []string, string, bool, string) (int64, error)
}

// BaseConfig provides common configuration for all implementations of Parent.
type BaseConfig struct {
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

// baseParent provides common functionality for all implementations of Parent.
type baseParent struct {
	commitMsgTmpl   *template.Template
	includeBugs     bool
	includeLog      bool
	monorailProject string
}

func newBaseParent(ctx context.Context, c BaseConfig) (*baseParent, error) {
	commitMsgTmplStr := TMPL_COMMIT_MSG_DEFAULT
	if c.CommitMsgTmpl != "" {
		commitMsgTmplStr = c.CommitMsgTmpl
	}
	commitMsgTmpl, err := ParseCommitMsgTemplate(commitMsgTmplStr)
	if err != nil {
		return nil, err
	}
	return &baseParent{
		commitMsgTmpl:   commitMsgTmpl,
		includeBugs:     c.IncludeBugs,
		includeLog:      c.IncludeLog,
		monorailProject: c.MonorailProject,
	}, nil
}

// buildCommitMsg is a helper function used to create commit messages.
func (p *baseParent) buildCommitMsg(vars *CommitMsgVars) (string, error) {
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
