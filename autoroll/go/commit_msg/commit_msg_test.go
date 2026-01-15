package commit_msg

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/go/deepequal/assertdeep"
)

const (
	fakeChildBugLink  = "https://file-a-child-bug.com"
	fakeParentBugLink = "https://file-a-parent-bug.com"
	fakeParentName    = "fake/parent"
)

// fakeCommitMsgConfig returns a valid CommitMsgConfig instance.
func fakeCommitMsgConfig(t *testing.T) *config.CommitMsgConfig {
	c := &config.CommitMsgConfig{
		BugProject:           fakeBugProject,
		ChildLogUrlTmpl:      "https://fake-child-log/{{.RollingFrom}}..{{.RollingTo}}",
		CqExtraTrybots:       []string{"luci.fakeproject.try:some-trybot"},
		CqDoNotCancelTrybots: true,
		ExtraFooters:         []string{"My-Footer: BlahBlah", "My-Other-Footer: Blah"},
		IncludeLog:           true,
		IncludeRevisionCount: true,
		IncludeTbrLine:       true,
		IncludeTests:         true,
		BuiltIn:              config.CommitMsgConfig_DEFAULT,
	}
	// Sanity check.
	require.NoError(t, c.Validate())
	return c
}

// fakeBuilder returns a Builder instance.
func fakeBuilder(t *testing.T) *Builder {
	b, err := NewBuilder(fakeCommitMsgConfig(t), fakeChildName, fakeParentName, fakeServerURL, "", "", fakeTransitiveDeps)
	require.NoError(t, err)
	return b
}

func TestMakeVars(t *testing.T) {
	check := func(fn func(*Builder)) {
		c := fakeCommitMsgConfig(t)
		b, err := NewBuilder(c, fakeChildName, fakeParentName, fakeServerURL, fakeChildBugLink, fakeParentBugLink, fakeTransitiveDeps)
		require.NoError(t, err)
		fn(b)
		from, to, revs, reviewers, contacts, _, manualRollRequester := FakeCommitMsgInputs()
		vars, err := makeVars(c, b.childName, b.parentName, b.serverURL, fakeChildBugLink, fakeParentBugLink, b.transitiveDeps, from, to, revs, reviewers, contacts, manualRollRequester)
		require.NoError(t, err)

		// Bugs.
		var expectBugs int
		if c.BugProject == "" {
			expectBugs = 0
		} else if c.BugProject != fakeBugProject {
			expectBugs = 0
		} else {
			expectBugs = 2 // From fakeCommitMsgInputs.
		}
		require.Len(t, vars.Bugs, expectBugs)

		// CqExtratrybots.
		require.Len(t, vars.CqExtraTrybots, 1)
		require.Equal(t, "luci.fakeproject.try:some-trybot", vars.CqExtraTrybots[0])

		// Log URL.
		if c.ChildLogUrlTmpl == "" {
			require.Equal(t, vars.ChildLogURL, "")
		} else {
			require.Equal(t, vars.ChildLogURL, "https://fake-child-log/aaaaaaaaaaaa..cccccccccccc")
		}

		// RollingFrom and RollingTo.
		require.Equal(t, fixupRevision(from), vars.RollingFrom)
		require.Equal(t, fixupRevision(to), vars.RollingTo)

		// Tests.
		if c.IncludeTests {
			require.Len(t, vars.Tests, 1)
		} else {
			require.Len(t, vars.Tests, 0)
		}

		// TransitiveDeps.
		if len(b.transitiveDeps) == 0 {
			require.Len(t, vars.TransitiveDeps, 0)
		} else {
			// Only two of the transitive deps differ.
			require.Len(t, vars.TransitiveDeps, 2)
			assertdeep.Equal(t, &transitiveDepUpdate{
				Dep:         "parent/dep1",
				RollingFrom: "dddddddddddddddddddddddddddddddddddddddd",
				RollingTo:   "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
				LogURL:      "https://fake-dep1/+log/dddddddddddddddddddddddddddddddddddddddd..eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
			}, vars.TransitiveDeps[0])
			assertdeep.Equal(t, &transitiveDepUpdate{
				Dep:         "parent/dep3",
				RollingFrom: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				RollingTo:   "cccccccccccccccccccccccccccccccccccccccc",
			}, vars.TransitiveDeps[1])
		}
	}

	// Default config includes everything.
	check(func(b *Builder) {})
	// No bugs.
	check(func(b *Builder) {
		b.cfg.BugProject = ""
	})
	check(func(b *Builder) {
		b.cfg.BugProject = "bogus project; doesn't match anything"
	})
	// No log URL template.
	check(func(b *Builder) {
		b.cfg.ChildLogUrlTmpl = ""
	})
	// No revisions.
	check(func(b *Builder) {
		b.cfg.IncludeLog = false
	})
	// No tests.
	check(func(b *Builder) {
		b.cfg.IncludeTests = false
	})
	// No transitive deps.
	check(func(b *Builder) {
		b.transitiveDeps = nil
	})
}

func TestNamedTemplatesValid(t *testing.T) {

	cfg := fakeCommitMsgConfig(t)
	for tmpl := range namedCommitMsgTemplates {
		cfg.BuiltIn = tmpl
		require.NoError(t, cfg.Validate())
	}
}

func TestQuotedLines(t *testing.T) {

	c := fakeCommitMsgConfig(t)
	c.Custom = `{{- define "revisions" -}}
{{ range .Revisions }}{{ .Timestamp.Format "2006-01-02" }} {{ .Author }} {{ .Description }}
{{ quotedLines .Details }}

{{ end }}
{{ end -}}
`
	b, err := NewBuilder(c, fakeChildName, fakeParentName, fakeServerURL, "", "", fakeTransitiveDeps)
	require.NoError(t, err)

	from, to, revs, reviewers, contacts, _, manualRollRequester := FakeCommitMsgInputs()
	for _, rev := range revs {
		rev.Details += `

Change-Id: If3fd7d9b2ec5aaf7f048df1029b732b28378999d
`
	}

	msg, err := b.Build(from, to, revs, reviewers, contacts, false, manualRollRequester)
	require.NoError(t, err)
	require.Equal(t, `Roll fake/child/src from aaaaaaaaaaaa to cccccccccccc (2 revisions)

2020-04-17 c@google.com Commit C
> blah blah
> 
> ccccccc
> 
> blah
> 
> Change-Id: If3fd7d9b2ec5aaf7f048df1029b732b28378999d
> 

2020-04-16 b@google.com Commit B
> blah blah
> 
> bbbbbbb
> 
> blah
> 
> Change-Id: If3fd7d9b2ec5aaf7f048df1029b732b28378999d
> 

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
https://fake.server.com/r/fake-autoroll
Please CC contact@google.com,reviewer@google.com on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://issues.skia.org/issues/new?component=1389291&template=1850622

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/main/autoroll/README.md

Cq-Include-Trybots: luci.fakeproject.try:some-trybot
Cq-Do-Not-Cancel-Tryjobs: true
Bug: fakebugproject:1234,fakebugproject:5678
Tbr: reviewer@google.com
Test: some-test
My-Footer: BlahBlah
My-Other-Footer: Blah
`, msg)
}
