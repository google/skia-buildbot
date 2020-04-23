package commit_msg

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	commitMsgInfoText = `
If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
{{.ServerURL}}
Please CC {{stringsJoin .Reviewers ","}} on the revert to ensure that a human
is aware of the problem.
	
To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md
`

	tmplCommitMsgGClient = `Roll {{.ChildName}} {{.RollingFrom.String}}..{{.RollingTo.String}} ({{len .Revisions}} commits)

{{.ChildLogLink}}

{{if .IncludeLog}}git log {{.RollingFrom}}..{{.RollingTo}} --date=short --first-parent --format='%ad %ae %s'
{{range .Revisions}}{{.Timestamp.Format "2006-01-02"}} {{.Author}} {{.Description}}
{{end}}{{end}}{{if len .TransitiveDeps}}
Also rolling transitive DEPS:
{{range .TransitiveDeps}}  {{.Dep}} {{substr .RollingFrom 0 12}}..{{substr .RollingTo 0 12}}
{{end}}{{end}}
Created with:
  gclient setdep -r {{.ChildName}}@{{.RollingTo}}
` + commitMsgInfoText + `
{{if .CqExtraTrybots}}Cq-Include-Trybots: {{.CqExtraTrybots}}
{{end}}Bug: {{if .Bugs}}{{stringsJoin .Bugs ","}}{{else}}None{{end}}
Tbr: {{stringsJoin .Reviewers ","}}`
)

// CommitMsgConfig provides configuration for commit messages.
type CommitMsgConfig struct {
	BugProject      string                                   `json:"bugProject"`
	CommitMsgTmpl   string                                   `json:"commitMsgTmpl"`
	ChildName       string                                   `json:"childName"`
	ChildLogURLTmpl string                                   `json:"childLogURLTmpl"`
	CqExtraTrybots  string                                   `json:"cqExtraTrybots"`
	IncludeBugs     bool                                     `json:"includeBugs"`
	IncludeLog      bool                                     `json:"includeLog"`
	Reviewers       []string                                 `json:"reviewers"`
	ServerURL       string                                   `json:"serverURL"`
	TransitiveDeps  version_file_common.TransitiveDepConfigs `json:"transitiveDeps"`
}

// See documentation for util.Validator interface.
func (c *CommitMsgConfig) Validate() error {
	// TODO
	return nil
}

// BuildCommitMsg builds a commit message for the given roll.
func (c *CommitMsgConfig) BuildCommitMsg(from, to *revision.Revision, rolling []*revision.Revision) (string, error) {
	vars, err := c.makeVars(from, to, rolling)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	// Create the commit message.
	commitMsgTmpl, err := ParseCommitMsgTemplate(vars.CommitMsgTmpl)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	var buf bytes.Buffer
	if err := commitMsgTmpl.Execute(&buf, vars); err != nil {
		return "", skerr.Wrap(err)
	}
	commitMsg := buf.String()

	// Temporary hack to substitute P4 for "Pixel4". See skbug.com/9595.
	commitMsg = strings.Replace(commitMsg, "Pixel4", "P4", -1)
	return commitMsg, nil
}

// makeVars derives commitMsgVars from the CommitMsgConfig for the given roll.
func (c *CommitMsgConfig) makeVars(from, to *revision.Revision, rolling []*revision.Revision) (*commitMsgVars, error) {
	// Create the commitMsgVars object to be used as input to the template.
	vars := &commitMsgVars{
		CommitMsgConfig: c,
		RollingFrom:     from,
		RollingTo:       to,
		Revisions:       rolling,
	}

	// Bugs.
	vars.Bugs = nil
	if vars.IncludeBugs {
		// TODO(borenet): Move this to a util.MakeBugLines utility?
		bugMap := map[string]bool{}
		for _, rev := range vars.Revisions {
			for _, bug := range rev.Bugs[vars.BugProject] {
				bugMap[bug] = true
			}
		}
		if len(bugMap) > 0 {
			vars.Bugs = make([]string, 0, len(bugMap))
			for bug := range bugMap {
				bugStr := fmt.Sprintf("%s:%s", vars.BugProject, bug)
				if vars.BugProject == util.BUG_PROJECT_BUGANIZER {
					bugStr = fmt.Sprintf("b/%s", bug)
				}
				vars.Bugs = append(vars.Bugs, bugStr)
			}
			sort.Strings(vars.Bugs)
		}
	}

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

	// Transitive deps. Note that we can't verify that the repo manager
	// implementation actually included these changes in the roll; we assume
	// that it would do so if working correctly and would error out otherwise.
	var transitiveDeps []*version_file_common.TransitiveDepUpdate
	for _, td := range c.TransitiveDeps {
		// Find the versions of the transitive dep in the old and new revisions.
		oldRev, ok := from.Dependencies[td.Child.ID]
		if !ok {
			return nil, skerr.Fmt("Transitive dependency %q is missing from revision %s", td.Child.ID, from.Id)
		}
		newRev, ok := to.Dependencies[td.Child.ID]
		if !ok {
			return nil, skerr.Fmt("Transitive dependency %q is missing from revision %s", td.Child.ID, to.Id)
		}
		transitiveDeps = append(transitiveDeps, &version_file_common.TransitiveDepUpdate{
			Dep:         td.Parent.ID,
			RollingFrom: oldRev,
			RollingTo:   newRev,
		})
	}
	vars.TransitiveDeps = transitiveDeps\
	return vars, nil
}

// commitMsgVars contains variables used to fill in a commit message template.
type commitMsgVars struct {
	*CommitMsgConfig
	Bugs           []string
	Revisions      []*revision.Revision
	RollingFrom    *revision.Revision
	RollingTo      *revision.Revision
	Tests          []string
	TransitiveDeps []*version_file_common.TransitiveDepUpdate
}

// ParseCommitMsgTemplate parses the given commit message template string and
// returns a Template instance.
func ParseCommitMsgTemplate(tmpl string) (*template.Template, error) {
	return template.New("commitMsg").Funcs(template.FuncMap{
		"stringsJoin": strings.Join,
		"substr": func(s string, a, b int) string {
			if a > len(s) {
				return ""
			}
			if b > len(s) {
				b = len(s)
			}
			return s[a:b]
		},
	}).Parse(tmpl)
}

// fakeCommitMsgConfig returns a valid CommitMsgConfig instance.
func fakeCommitMsgConfig() *CommitMsgConfig {
	return &CommitMsgConfig{
		ChildName:      "path/to/child",
		CqExtraTrybots: "extra-bot",
		IncludeLog:     true,
		Reviewers:      []string{"me@google.com"},
		ServerURL:      "https://fake.server.url",
		TransitiveDeps: []*version_file_common.TransitiveDepConfig{
			{
				Child: &version_file_common.VersionFileConfig{
					ID:   "child/dep",
					Path: "DEPS",
				},
				Parent: &version_file_common.VersionFileConfig{
					ID:   "path/to/other",
					Path: "DEPS",
				},
			},
		},
	}
}

// ValidateCommitMsgTemplate returns an error if the given commit message
// template cannot be parsed and executed with a typical set of inputs.
func ValidateCommitMsgTemplate(tmpl string) error {
	a := &revision.Revision{
		Id:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Display: "aaaaaaaaaaaa",
		Author:  "a@google.com",
		Dependencies: map[string]string{
			"child/dep": "dddddddddddddddddddddddddddddddddddddddd",
		},
	}
	b := &revision.Revision{
		Id:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Display: "bbbbbbbbbbbb",
		Author:  "b@google.com",
		Bugs:    map[string][]string{"skia": {"1234"}},
		Dependencies: map[string]string{
			"child/dep": "dddddddddddddddddddddddddddddddddddddddd",
		},
	}
	c := &revision.Revision{
		Id:      "cccccccccccccccccccccccccccccccccccccccc",
		Display: "cccccccccccc",
		Author:  "c@google.com",
		Tests:   []string{"some-test"},
		Dependencies: map[string]string{
			"child/dep": "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		},
	}
	cfg := fakeCommitMsgConfig()
	_, err := cfg.BuildCommitMsg(a, c, []*revision.Revision{b, c})
	return skerr.Wrap(err)
}
