package commit_msg

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

var (
	// namedCommitMsgTemplates contains pre-defined commit message templates
	// which may be referenced by name in config files.
	namedCommitMsgTemplates = map[config.CommitMsgConfig_BuiltIn]*template.Template{
		config.CommitMsgConfig_ANDROID:       tmplAndroid,
		config.CommitMsgConfig_ANDROID_NO_CR: tmplAndroidNoCR,
		config.CommitMsgConfig_DEFAULT:       tmplCommitMsg,
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

// Builder is a helper used to build commit messages.
type Builder struct {
	cfg            *config.CommitMsgConfig
	childBugLink   string
	childName      string
	parentBugLink  string
	parentName     string
	reg            *config_vars.Registry
	serverURL      string
	transitiveDeps []*config.TransitiveDepConfig
}

// NewBuilder returns a Builder instance.
func NewBuilder(c *config.CommitMsgConfig, reg *config_vars.Registry, childName, parentName, serverURL, childBugLink, parentBugLink string, transitiveDeps []*config.TransitiveDepConfig) (*Builder, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	if childName == "" {
		return nil, skerr.Fmt("childName is required")
	}
	if serverURL == "" {
		return nil, skerr.Fmt("serverURL is required")
	}
	for _, td := range transitiveDeps {
		if err := td.Validate(); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	return &Builder{
		cfg:            c,
		childName:      childName,
		childBugLink:   childBugLink,
		parentBugLink:  parentBugLink,
		parentName:     parentName,
		reg:            reg,
		serverURL:      serverURL,
		transitiveDeps: transitiveDeps,
	}, nil
}

// Build a commit message for the given roll.
func (b *Builder) Build(from, to *revision.Revision, rolling []*revision.Revision, reviewers []string) (string, error) {
	return buildCommitMsg(b.cfg, b.reg.Vars(), b.childName, b.parentName, b.serverURL, b.childBugLink, b.parentBugLink, b.transitiveDeps, from, to, rolling, reviewers)
}

// buildCommitMsg builds a commit message for the given roll.
func buildCommitMsg(c *config.CommitMsgConfig, cv *config_vars.Vars, childName, parentName, serverURL, childBugLink, parentBugLink string, transitiveDeps []*config.TransitiveDepConfig, from, to *revision.Revision, rolling []*revision.Revision, reviewers []string) (string, error) {
	vars, err := makeVars(c, cv, childName, parentName, serverURL, childBugLink, parentBugLink, transitiveDeps, from, to, rolling, reviewers)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	// Create the commit message.
	commitMsgTmpl := tmplCommitMsg
	if c.GetCustom() != "" {
		commitMsgTmpl, err = parseCommitMsgTemplate(tmplCommitMsg, "customCommitMsg", c.GetCustom())
		if err != nil {
			return "", skerr.Wrap(err)
		}
	} else {
		var ok bool
		commitMsgTmpl, ok = namedCommitMsgTemplates[c.GetBuiltIn()]
		if !ok {
			return "", skerr.Fmt("Unknown built-in config %q", c.GetBuiltIn())
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
func makeVars(c *config.CommitMsgConfig, cv *config_vars.Vars, childName, parentName, serverURL, childBugLink, parentBugLink string, transitiveDeps []*config.TransitiveDepConfig, from, to *revision.Revision, revisions []*revision.Revision, reviewers []string) (*commitMsgVars, error) {
	// Create the commitMsgVars object to be used as input to the template.
	revsCopy := make([]*revision.Revision, 0, len(revisions))
	for _, rev := range revisions {
		revsCopy = append(revsCopy, fixupRevision(rev))
	}
	vars := &commitMsgVars{
		CommitMsgConfig: c,
		Vars:            cv,
		ChildName:       childName,
		ChildBugLink:    childBugLink,
		ParentName:      parentName,
		ParentBugLink:   parentBugLink,
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

	// CqExtraTrybots.
	if c.CqExtraTrybots != nil {
		vars.CqExtraTrybots = make([]string, 0, len(c.CqExtraTrybots))
		for _, trybot := range c.CqExtraTrybots {
			// Note: it'd be slightly more efficient to keep these
			// templates around, but that would require passing them
			// in to this function, which is a bit ugly.
			tmpl, err := config_vars.NewTemplate(trybot)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			if err := tmpl.Update(cv); err != nil {
				return nil, skerr.Wrap(err)
			}
			vars.CqExtraTrybots = append(vars.CqExtraTrybots, tmpl.String())
		}
	}

	// Log URL.
	vars.ChildLogURL = ""
	if c.ChildLogUrlTmpl != "" {
		childLogURLTmpl, err := parseCommitMsgTemplate(nil, "childLogURL", c.ChildLogUrlTmpl)
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
		oldRev, ok := from.Dependencies[td.Child.Id]
		if !ok {
			return nil, skerr.Fmt("Transitive dependency %q is missing from revision %s", td.Child.Id, from.Id)
		}
		newRev, ok := to.Dependencies[td.Child.Id]
		if !ok {
			return nil, skerr.Fmt("Transitive dependency %q is missing from revision %s", td.Child.Id, to.Id)
		}
		if oldRev != newRev {
			transitiveUpdates = append(transitiveUpdates, &transitiveDepUpdate{
				Dep:         td.Parent.Id,
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
	*config.CommitMsgConfig
	*config_vars.Vars
	Bugs           []string
	ChildBugLink   string
	ChildLogURL    string
	ChildName      string
	CqExtraTrybots []string
	ParentBugLink  string
	ParentName     string
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

var fakeTransitiveDeps = []*config.TransitiveDepConfig{
	{
		Child: &config.VersionFileConfig{
			Id:   fakeChildDep1,
			Path: "DEPS",
		},
		Parent: &config.VersionFileConfig{
			Id:   "parent/dep1",
			Path: "DEPS",
		},
	},
	{
		Child: &config.VersionFileConfig{
			Id:   fakeChildDep2,
			Path: "DEPS",
		},
		Parent: &config.VersionFileConfig{
			Id:   "parent/dep2",
			Path: "DEPS",
		},
	},
	{
		Child: &config.VersionFileConfig{
			Id:   fakeChildDep3,
			Path: "DEPS",
		},
		Parent: &config.VersionFileConfig{
			Id:   "parent/dep3",
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
		URL:       "https://fake.com/aaaaaaaaaaaa",
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
		URL:       "https://fake.com/bbbbbbbbbbbb",
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
		URL:       "https://fake.com/cccccccccccc",
	}
	return a, c, []*revision.Revision{c, b}, []string{"reviewer@google.com"}
}
