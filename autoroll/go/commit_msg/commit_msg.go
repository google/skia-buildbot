package commit_msg

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

var (
	// namedCommitMsgTemplates contains pre-defined commit message templates
	// which may be referenced by name in config files.
	namedCommitMsgTemplates = map[string]*template.Template{
		TmplNameAndroid: tmplAndroid,
		TmplNameDefault: tmplCommitMsg,
	}

	limitEmptyLinesRegex = regexp.MustCompile(`\n\n\n+`)
	newlineAtEndRegex    = regexp.MustCompile(`\n*$`)
)

// transitiveDepUpdate represents an update to one transitive dependency.
type transitiveDepUpdate struct {
	Dep         string
	RollingFrom string
	RollingTo   string
}

// CommitMsgConfig provides configuration for commit messages.
type CommitMsgConfig struct {
	BugProject           string   `json:"bugProject,omitempty"`
	ChildLogURLTmpl      string   `json:"childLogURLTmpl,omitempty"`
	CqExtraTrybots       []string `json:"cqExtraTrybots,omitempty"`
	IncludeLog           bool     `json:"includeLog,omitempty"`
	IncludeRevisionCount bool     `json:"includeRevisionCount,omitempty"`
	IncludeTbrLine       bool     `json:"includeTbrLine,omitempty"`
	IncludeTests         bool     `json:"includeTests,omitempty"`
	// Template is either a full commit message template string or the name of
	// an entry in NamedCommitMsgTemplates. If not specified, the default
	// template is used.
	Template string `json:"template,omitempty"`
}

// See documentation for util.Validator interface.
func (c *CommitMsgConfig) Validate() error {
	// We are not concerned with the presence or absence of any given field,
	// since some rollers may not need all of the fields. If we are able to
	// execute the template given a typical set of inputs, we consider the
	// CommitMsgConfig to be valid.
	from, to, revs, reviewers := FakeCommitMsgInputs()
	_, err := buildCommitMsg(c, fakeChildName, fakeServerURL, fakeTransitiveDeps, from, to, revs, reviewers)
	return skerr.Wrap(err)
}

// Builder is a helper used to build commit messages.
type Builder struct {
	cfg            *CommitMsgConfig
	childName      string
	serverURL      string
	transitiveDeps version_file_common.TransitiveDepConfigs
}

// NewBuilder returns a Builder instance.
func NewBuilder(c *CommitMsgConfig, childName, serverURL string, transitiveDeps version_file_common.TransitiveDepConfigs) (*Builder, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	if childName == "" {
		return nil, skerr.Fmt("childName is required")
	}
	if serverURL == "" {
		return nil, skerr.Fmt("serverURL is required")
	}
	if err := transitiveDeps.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	return &Builder{
		cfg:            c,
		childName:      childName,
		serverURL:      serverURL,
		transitiveDeps: transitiveDeps,
	}, nil
}

// Build a commit message for the given roll.
func (b *Builder) Build(from, to *revision.Revision, rolling []*revision.Revision, reviewers []string) (string, error) {
	return buildCommitMsg(b.cfg, b.childName, b.serverURL, b.transitiveDeps, from, to, rolling, reviewers)
}

// buildCommitMsg builds a commit message for the given roll.
func buildCommitMsg(c *CommitMsgConfig, childName, serverURL string, transitiveDeps []*version_file_common.TransitiveDepConfig, from, to *revision.Revision, rolling []*revision.Revision, reviewers []string) (string, error) {
	vars, err := makeVars(c, childName, serverURL, transitiveDeps, from, to, rolling, reviewers)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	// Create the commit message.
	commitMsgTmpl := tmplCommitMsg
	if vars.Template != "" {
		if builtin, ok := namedCommitMsgTemplates[vars.Template]; ok {
			commitMsgTmpl = builtin
		} else {
			commitMsgTmpl, err = parseCommitMsgTemplate(tmplCommitMsg, "customCommitMsg", vars.Template)
			if err != nil {
				return "", skerr.Wrap(err)
			}
		}
	}
	var buf bytes.Buffer
	if err := commitMsgTmpl.ExecuteTemplate(&buf, tmplNameCommitMsg, vars); err != nil {
		return "", skerr.Wrap(err)
	}

	// Templates make whitespace tricky when they involve optional sections. To
	// ensure that the message looks reasonable, limit to two newlines in a row
	// (ie. at most one empty line), and ensure that the message ends in exactly
	// one newline.
	msg := limitEmptyLinesRegex.ReplaceAllString(buf.String(), "\n\n")
	msg = newlineAtEndRegex.ReplaceAllString(msg, "\n")
	return msg, nil
}

func fixupRevision(rev *revision.Revision) *revision.Revision {
	cpy := rev.Copy()
	cpy.Timestamp = cpy.Timestamp.UTC()
	return cpy
}

// makeVars derives commitMsgVars from the CommitMsgConfig for the given roll.
func makeVars(c *CommitMsgConfig, childName, serverURL string, transitiveDeps []*version_file_common.TransitiveDepConfig, from, to *revision.Revision, revisions []*revision.Revision, reviewers []string) (*commitMsgVars, error) {
	// Create the commitMsgVars object to be used as input to the template.
	revsCopy := make([]*revision.Revision, 0, len(revisions))
	for _, rev := range revisions {
		revsCopy = append(revsCopy, fixupRevision(rev))
	}
	vars := &commitMsgVars{
		CommitMsgConfig: c,
		ChildName:       childName,
		Reviewers:       reviewers,
		Revisions:       revsCopy,
		RollingFrom:     fixupRevision(from),
		RollingTo:       fixupRevision(to),
		ServerURL:       serverURL,
	}

	// Bugs.
	vars.Bugs = nil
	if c.BugProject != "" {
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
		childLogURLTmpl, err := parseCommitMsgTemplate(nil, "childLogURL", c.ChildLogURLTmpl)
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
	var transitiveUpdates []*transitiveDepUpdate
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
		if oldRev != newRev {
			transitiveUpdates = append(transitiveUpdates, &transitiveDepUpdate{
				Dep:         td.Parent.ID,
				RollingFrom: oldRev,
				RollingTo:   newRev,
			})
		}
	}
	vars.TransitiveDeps = transitiveUpdates
	return vars, nil
}

// commitMsgVars contains variables used to fill in a commit message template.
type commitMsgVars struct {
	*CommitMsgConfig
	Bugs           []string
	ChildLogURL    string
	ChildName      string
	Reviewers      []string
	Revisions      []*revision.Revision
	RollingFrom    *revision.Revision
	RollingTo      *revision.Revision
	ServerURL      string
	Tests          []string
	TransitiveDeps []*transitiveDepUpdate
}

// parseCommitMsgTemplate parses the given commit message template string and
// returns a Template instance.
func parseCommitMsgTemplate(parent *template.Template, name, tmpl string) (*template.Template, error) {
	var t *template.Template
	if parent != nil {
		clone, err := parent.Clone()
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		t = clone.New(name)
	} else {
		t = template.New(name)
	}
	return t.Option("missingkey=error").Funcs(template.FuncMap{
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
const fakeChildName = "fake/child/src"
const fakeServerURL = "https://fake.server.com/r/fake-autoroll"
const fakeChildDep1 = "child/dep1"
const fakeChildDep2 = "child/dep2"
const fakeChildDep3 = "child/dep3"

var fakeTransitiveDeps = []*version_file_common.TransitiveDepConfig{
	{
		Child: &version_file_common.VersionFileConfig{
			ID:   fakeChildDep1,
			Path: "DEPS",
		},
		Parent: &version_file_common.VersionFileConfig{
			ID:   "parent/dep1",
			Path: "DEPS",
		},
	},
	{
		Child: &version_file_common.VersionFileConfig{
			ID:   fakeChildDep2,
			Path: "DEPS",
		},
		Parent: &version_file_common.VersionFileConfig{
			ID:   "parent/dep2",
			Path: "DEPS",
		},
	},
	{
		Child: &version_file_common.VersionFileConfig{
			ID:   fakeChildDep3,
			Path: "DEPS",
		},
		Parent: &version_file_common.VersionFileConfig{
			ID:   "parent/dep3",
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
			fakeChildDep1: "dddddddddddddddddddddddddddddddddddddddd",
			fakeChildDep2: "1111111111111111111111111111111111111111",
			fakeChildDep3: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
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
			fakeChildDep1: "dddddddddddddddddddddddddddddddddddddddd",
			fakeChildDep2: "1111111111111111111111111111111111111111",
			fakeChildDep3: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
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
			fakeChildDep1: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
			fakeChildDep2: "1111111111111111111111111111111111111111",
			fakeChildDep3: "cccccccccccccccccccccccccccccccccccccccc",
		},
		Description: "Commit C",
		Details: `blah blah

	ccccccc

	blah`,
		Timestamp: time.Unix(1587081600, 0),
	}
	return a, c, []*revision.Revision{c, b}, []string{"reviewer@google.com"}
}
