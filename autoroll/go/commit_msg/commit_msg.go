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
		TmplNameAfdo:    TmplAfdo,
		TmplNameAndroid: TmplAndroid,
		TmplNameDefault: TmplDefault,
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
	BugProject      string `json:"bugProject"`
	ChildLogURLTmpl string `json:"childLogURLTmpl"`
	ChildName       string `json:"childName"`
	// CommitMsgTmpl is either a full commit message template string or the name
	// of an entry in NamedCommitMsgTemplates.
	CommitMsgTmpl        string   `json:"commitMsgTmpl"`
	CqExtraTrybots       []string `json:"cqExtraTrybots"`
	IncludeBugs          bool     `json:"includeBugs"`
	IncludeLog           bool     `json:"includeLog"`
	IncludeRevisionCount bool     `json:"includeRevisionCount"`
	IncludeTbrLine       bool     `json:"includeTbrLine"`
	IncludeTests         bool     `json:"includeTests"`
}

// See documentation for util.Validator interface.
func (c *CommitMsgConfig) Validate() error {
	// We are not concerned with the presence or absence of any given field,
	// since some rollers may not need all of the fields. If we are able to
	// execute the template given a typical set of inputs, we consider the
	// CommitMsgConfig to be valid.
	from, to, revs, reviewers := FakeCommitMsgInputs()
	_, err := buildCommitMsg(c, fakeServerURL, fakeTransitiveDeps, from, to, revs, reviewers)
	return skerr.Wrap(err)
}

// Builder is a helper used to build commit messages.
type Builder struct {
	cfg            *CommitMsgConfig
	serverURL      string
	transitiveDeps version_file_common.TransitiveDepConfigs
}

// NewBuilder returns a Builder instance.
func NewBuilder(c *CommitMsgConfig, serverURL string, transitiveDeps version_file_common.TransitiveDepConfigs) (*Builder, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	if serverURL == "" {
		return nil, skerr.Fmt("serverURL is required")
	}
	if err := transitiveDeps.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	return &Builder{
		cfg:            c,
		serverURL:      serverURL,
		transitiveDeps: transitiveDeps,
	}, nil
}

// Build a commit message for the given roll.
func (b *Builder) Build(from, to *revision.Revision, rolling []*revision.Revision, reviewers []string) (string, error) {
	return buildCommitMsg(b.cfg, b.serverURL, b.transitiveDeps, from, to, rolling, reviewers)
}

// buildCommitMsg builds a commit message for the given roll.
func buildCommitMsg(c *CommitMsgConfig, serverURL string, transitiveDeps []*version_file_common.TransitiveDepConfig, from, to *revision.Revision, rolling []*revision.Revision, reviewers []string) (string, error) {
	vars, err := makeVars(c, serverURL, transitiveDeps, from, to, rolling, reviewers)
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
func makeVars(c *CommitMsgConfig, serverURL string, transitiveDeps []*version_file_common.TransitiveDepConfig, from, to *revision.Revision, revisions []*revision.Revision, reviewers []string) (*commitMsgVars, error) {
	// Create the commitMsgVars object to be used as input to the template.
	vars := &commitMsgVars{
		CommitMsgConfig: c,
		Reviewers:       reviewers,
		Revisions:       revisions,
		RollingFrom:     from,
		RollingTo:       to,
		ServerURL:       serverURL,
	}

	// Bugs.
	vars.Bugs = nil
	if c.IncludeBugs {
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
				bugStr := fmt.Sprintf("%s:%s", c.BugProject, bug)
				if c.BugProject == util.BUG_PROJECT_BUGANIZER {
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
	var transitiveUpdates []*TransitiveDepUpdate
	for _, td := range transitiveDeps {
		// Find the versions of the transitive dep in the old and new revisions.
		oldRev, ok := from.Dependencies[td.Child.ID]
		if !ok {
			return nil, skerr.Fmt("Transitive dependency %q is missing from revision %s", td.Child.ID, from.Id)
		}
		newRev, ok := to.Dependencies[td.Child.ID]
		if !ok {
			return nil, skerr.Fmt("Transitive dependency %q is missing from revision %s", td.Child.ID, to.Id)
		}
		transitiveUpdates = append(transitiveUpdates, &TransitiveDepUpdate{
			Dep:         td.Parent.ID,
			RollingFrom: oldRev,
			RollingTo:   newRev,
		})
	}
	vars.TransitiveDeps = transitiveUpdates
	return vars, nil
}

// commitMsgVars contains variables used to fill in a commit message template.
type commitMsgVars struct {
	*CommitMsgConfig
	Bugs           []string
	ChildLogURL    string
	Reviewers      []string
	Revisions      []*revision.Revision
	RollingFrom    *revision.Revision
	RollingTo      *revision.Revision
	ServerURL      string
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

// Fake values for various configuration entries used for testing.
const fakeBugProject = "fakebugproject"
const fakeServerURL = "https://fake.server.com/r/fake-autoroll"
const fakeChildDep = "child/dep"

var fakeTransitiveDeps = []*version_file_common.TransitiveDepConfig{
	{
		Child: &version_file_common.VersionFileConfig{
			ID:   fakeChildDep,
			Path: "DEPS",
		},
		Parent: &version_file_common.VersionFileConfig{
			ID:   "parent/dep",
			Path: "DEPS",
		},
	},
}

// FakeCommitMsgInputs returns Revisions which may be used to validate commit
// message templates.
func FakeCommitMsgInputs() (*revision.Revision, *revision.Revision, []*revision.Revision, []string) {
	a := &revision.Revision{
		Id:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Display: "aaaaaaaaaaaa",
		Author:  "a@google.com",
		Dependencies: map[string]string{
			fakeChildDep: "dddddddddddddddddddddddddddddddddddddddd",
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
			fakeChildDep: "dddddddddddddddddddddddddddddddddddddddd",
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
			fakeChildDep: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		},
		Description: "Commit C",
		Details: `blah blah
	
	ccccccc
	
	blah`,
		Timestamp: time.Unix(1587081600, 0),
	}
	return a, c, []*revision.Revision{c, b}, []string{"reviewer@google.com"}
}
