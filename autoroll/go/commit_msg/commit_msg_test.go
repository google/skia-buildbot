package commit_msg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/chrome_branch/mocks"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
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
		CqExtraTrybots:       []string{"some-trybot-on-m{{.Branches.Chromium.Beta.Milestone}}"},
		CqDoNotCancelTrybots: true,
		ExtraFooters:         []string{"My-Footer: BlahBlah", "My-Other-Footer: Blah"},
		IncludeLog:           true,
		IncludeRevisionCount: true,
		IncludeTbrLine:       true,
		IncludeTests:         true,
		Template: &config.CommitMsgConfig_BuiltIn_{
			BuiltIn: config.CommitMsgConfig_DEFAULT,
		},
	}
	// Sanity check.
	require.NoError(t, c.Validate())
	return c
}

// fakeRegistry returns a config_vars.Registry instance.
func fakeRegistry(t *testing.T) *config_vars.Registry {
	cbc := &mocks.Client{}
	cbc.On("Get", testutils.AnyContext).Return(&chrome_branch.Branches{
		Main: &chrome_branch.Branch{
			Milestone: 93,
			Number:    4577,
			Ref:       "refs/branch-heads/4577",
		},
		Beta: &chrome_branch.Branch{
			Milestone: 92,
			Number:    4515,
			Ref:       "refs/branch-heads/4515",
		},
		Stable: &chrome_branch.Branch{
			Milestone: 91,
			Number:    4472,
			Ref:       "refs/branch-heads/4472",
		},
	}, nil)
	reg, err := config_vars.NewRegistry(context.Background(), cbc)
	require.NoError(t, err)
	return reg
}

// fakeBuilder returns a Builder instance.
func fakeBuilder(t *testing.T) *Builder {
	reg := fakeRegistry(t)
	b, err := NewBuilder(fakeCommitMsgConfig(t), reg, fakeChildName, fakeParentName, fakeServerURL, "", "", fakeTransitiveDeps)
	require.NoError(t, err)
	return b
}

func TestMakeVars(t *testing.T) {
	unittest.SmallTest(t)

	reg := fakeRegistry(t)

	check := func(fn func(*Builder)) {
		c := fakeCommitMsgConfig(t)
		b, err := NewBuilder(c, reg, fakeChildName, fakeParentName, fakeServerURL, fakeChildBugLink, fakeParentBugLink, fakeTransitiveDeps)
		require.NoError(t, err)
		fn(b)
		from, to, revs, reviewers := FakeCommitMsgInputs()
		vars, err := makeVars(c, reg.Vars(), b.childName, b.parentName, b.serverURL, fakeChildBugLink, fakeParentBugLink, b.transitiveDeps, from, to, revs, reviewers)
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
		require.Equal(t, "some-trybot-on-m92", vars.CqExtraTrybots[0])

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
	unittest.SmallTest(t)

	cfg := fakeCommitMsgConfig(t)
	for tmpl := range namedCommitMsgTemplates {
		cfg.Template = &config.CommitMsgConfig_BuiltIn_{
			BuiltIn: tmpl,
		}
		require.NoError(t, cfg.Validate())
	}
}
