package commit_msg

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils/unittest"
)

// fakeCommitMsgConfig returns a valid CommitMsgConfig instance.
func fakeCommitMsgConfig(t *testing.T) *CommitMsgConfig {
	c := &CommitMsgConfig{
		BugProject:           fakeBugProject,
		Template:             TmplNameDefault,
		ChildLogURLTmpl:      "https://fake-child-log/{{.RollingFrom}}..{{.RollingTo}}",
		CqExtraTrybots:       []string{"some-trybot"},
		IncludeLog:           true,
		IncludeRevisionCount: true,
		IncludeTbrLine:       true,
		IncludeTests:         true,
	}
	// Sanity check.
	require.NoError(t, c.Validate())
	return c
}

// fakeBuilder returns a Builder instance.
func fakeBuilder(t *testing.T) *Builder {
	b, err := NewBuilder(fakeCommitMsgConfig(t), fakeChildName, fakeServerURL, fakeTransitiveDeps)
	require.NoError(t, err)
	return b
}

func TestMakeVars(t *testing.T) {
	unittest.SmallTest(t)

	check := func(fn func(*Builder)) {
		c := fakeCommitMsgConfig(t)
		b, err := NewBuilder(c, fakeChildName, fakeServerURL, fakeTransitiveDeps)
		require.NoError(t, err)
		fn(b)
		from, to, revs, reviewers := FakeCommitMsgInputs()
		vars, err := makeVars(c, b.childName, b.serverURL, b.transitiveDeps, from, to, revs, reviewers)
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

		// Log URL.
		if c.ChildLogURLTmpl == "" {
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
		b.cfg.ChildLogURLTmpl = ""
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
	for name := range namedCommitMsgTemplates {
		cfg.Template = name
		require.NoError(t, cfg.Validate())
	}
}
