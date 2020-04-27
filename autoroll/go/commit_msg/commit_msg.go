package commit_msg

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"

	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	commitMsgInfoText = `If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
{{.ServerURL}}
Please CC {{stringsJoin .Reviewers ","}} on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md`
)

var (
	// NamedCommitMsgTemplates contains pre-defined commit mesage templates
	// which may be referenced by name in config files.
	NamedCommitMsgTemplates = map[string]string{
		TmplNameAndroid: TmplAndroid,
		TmplNameGClient: TmplGClient,
	}
)

// TransitiveDepUpdate represents an update to one transitive dependency.
type TransitiveDepUpdate struct {
	Dep         string
	RollingFrom string
	RollingTo   string
}

// CommitMsgConfig provides configuration for commit messages.
type CommitMsgConfig struct {
	BugProject string `json:"bugProject"`
	// CommitMsgTmpl is either a full commit message template string or the name
	// of an entry in NamedCommitMsgTemplates.
	CommitMsgTmpl   string                                   `json:"commitMsgTmpl"`
	ChildName       string                                   `json:"childName"`
	ChildLogURLTmpl string                                   `json:"childLogURLTmpl"`
	CqExtraTrybots  string                                   `json:"cqExtraTrybots"`
	IncludeBugs     bool                                     `json:"includeBugs"`
	IncludeLog      bool                                     `json:"includeLog"`
	IncludeTbrLine  bool                                     `json:"includeTbrLine"`
	IncludeTests    bool                                     `json:"includeTests"`
	Reviewers       []string                                 `json:"reviewers"`
	ServerURL       string                                   `json:"serverURL"`
	TransitiveDeps  version_file_common.TransitiveDepConfigs `json:"transitiveDeps"`
}

// See documentation for util.Validator interface.
func (c *CommitMsgConfig) Validate() error {
	// We are not concerned with the presence or absence of any given field,
	// since some rollers may not need all of the fields. If we are able to
	// execute the template given a typical set of inputs, we consider the
	// CommitMsgConfig to be valid.
	_, err := c.BuildCommitMsg(fakeCommitMsgInputs())
	return skerr.Wrap(err)
}

// BuildCommitMsg builds a commit message for the given roll.
func (c *CommitMsgConfig) BuildCommitMsg(from, to *revision.Revision, rolling []*revision.Revision) (string, error) {
	vars, err := c.makeVars(from, to, rolling)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	// Create the commit message.
	commitMsgTmplStr, ok := NamedCommitMsgTemplates[vars.CommitMsgTmpl]
	if !ok {
		commitMsgTmplStr = vars.CommitMsgTmpl
	}
	commitMsgTmpl, err := parseCommitMsgTemplate(commitMsgTmplStr)
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
func (c *CommitMsgConfig) makeVars(from, to *revision.Revision, revisions []*revision.Revision) (*commitMsgVars, error) {
	// Create the commitMsgVars object to be used as input to the template.
	vars := &commitMsgVars{
		CommitMsgConfig: c,
		Revisions:       revisions,
		RollingFrom:     from,
		RollingTo:       to,
	}

	// Bugs.
	vars.Bugs = nil
	if vars.IncludeBugs {
		// TODO(borenet): Move this to a util.MakeBugLines utility?
		bugMap := map[string]bool{}
		for _, rev := range revisions {
			for _, bug := range rev.Bugs[c.BugProject] {
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

	// Log URL.
	vars.ChildLogURL = ""
	if c.ChildLogURLTmpl != "" {
		childLogURLTmpl, err := parseCommitMsgTemplate(c.ChildLogURLTmpl)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		var buf bytes.Buffer
		if err := childLogURLTmpl.Execute(&buf, vars); err != nil {
			return nil, skerr.Wrap(err)
		}
		vars.ChildLogURL = buf.String()
	}

	// Tests.
	if c.IncludeTests {
		testsMap := map[string]bool{}
		for _, rev := range revisions {
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
	}

	// Transitive deps. Note that we can't verify that the repo manager
	// implementation actually included these changes in the roll; we assume
	// that it would do so if working correctly and would error out otherwise.
	var transitiveDeps []*TransitiveDepUpdate
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
		transitiveDeps = append(transitiveDeps, &TransitiveDepUpdate{
			Dep:         td.Parent.ID,
			RollingFrom: oldRev,
			RollingTo:   newRev,
		})
	}
	vars.TransitiveDeps = transitiveDeps
	return vars, nil
}

// commitMsgVars contains variables used to fill in a commit message template.
type commitMsgVars struct {
	*CommitMsgConfig
	Bugs           []string
	ChildLogURL    string
	Revisions      []*revision.Revision
	RollingFrom    *revision.Revision
	RollingTo      *revision.Revision
	Tests          []string
	TransitiveDeps []*TransitiveDepUpdate
}

// parseCommitMsgTemplate parses the given commit message template string and
// returns a Template instance.
func parseCommitMsgTemplate(tmpl string) (*template.Template, error) {
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

// fakeBugProject is a project
const fakeBugProject = "fakebugproject"

// fakeCommitMsgInputs returns Revisions which may be used to validate commit
// message templates.
func fakeCommitMsgInputs() (*revision.Revision, *revision.Revision, []*revision.Revision) {
	a := &revision.Revision{
		Id:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Display: "aaaaaaaaaaaa",
		Author:  "a@google.com",
		Dependencies: map[string]string{
			"child/dep": "dddddddddddddddddddddddddddddddddddddddd",
		},
		Description: "Commit A",
		Details: `blah blah
	
	aaaaaaa
	
	blah`,
		Timestamp: time.Unix(1586908800, 0),
	}
	b := &revision.Revision{
		Id:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Display: "bbbbbbbbbbbb",
		Author:  "b@google.com",
		Bugs: map[string][]string{
			fakeBugProject: {"1234"},
		},
		Dependencies: map[string]string{
			"child/dep": "dddddddddddddddddddddddddddddddddddddddd",
		},
		Description: "Commit B",
		Details: `blah blah
	
	bbbbbbb
	
	blah`,
		Timestamp: time.Unix(1586995200, 0),
	}
	c := &revision.Revision{
		Id:      "cccccccccccccccccccccccccccccccccccccccc",
		Display: "cccccccccccc",
		Author:  "c@google.com",
		Bugs: map[string][]string{
			fakeBugProject: {"5678"},
		},
		Tests: []string{"some-test"},
		Dependencies: map[string]string{
			"child/dep": "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		},
		Description: "Commit C",
		Details: `blah blah
	
	ccccccc
	
	blah`,
		Timestamp: time.Unix(1587081600, 0),
	}
	return a, c, []*revision.Revision{c, b}
}
