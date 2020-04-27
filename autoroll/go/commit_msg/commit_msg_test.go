package commit_msg

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/go/testutils/unittest"
)

// fakeCommitMsgConfig returns a valid CommitMsgConfig instance.
func fakeCommitMsgConfig(t *testing.T) *CommitMsgConfig {
	c := &CommitMsgConfig{
		BugProject:      fakeBugProject,
		CommitMsgTmpl:   "gclient",
		ChildName:       "fake/child/src",
		ChildLogURLTmpl: "https://fake-child-log/{{.RollingFrom}}..{{.RollingTo}}",
		CqExtraTrybots:  "some-trybot",
		IncludeBugs:     true,
		IncludeLog:      true,
		IncludeTbrLine:  true,
		IncludeTests:    true,
		Reviewers:       []string{"reviewer@google.com"},
		ServerURL:       "https://fake.server.com/r/fake-autoroll",
		TransitiveDeps: []*version_file_common.TransitiveDepConfig{
			{
				Child: &version_file_common.VersionFileConfig{
					ID:   "child/dep",
					Path: "DEPS",
				},
				Parent: &version_file_common.VersionFileConfig{
					ID:   "parent/dep",
					Path: "DEPS",
				},
			},
		},
	}
	// Sanity check.
	require.NoError(t, c.Validate())
	return c
}

func TestConfigMakeVars(t *testing.T) {
	unittest.SmallTest(t)

	check := func(fn func(*CommitMsgConfig)) {
		c := fakeCommitMsgConfig(t)
		fn(c)
		from, to, revs := fakeCommitMsgInputs()
		vars, err := c.makeVars(from, to, revs)
		require.NoError(t, err)

		// Bugs.
		var expectBugs int
		if !c.IncludeBugs {
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
		require.Equal(t, from, vars.RollingFrom)
		require.Equal(t, to, vars.RollingTo)

		// Tests.
		if c.IncludeTests {
			require.Len(t, vars.Tests, 1)
		} else {
			require.Len(t, vars.Tests, 0)
		}

		// TransitiveDeps.
		if len(c.TransitiveDeps) == 0 {
			require.Len(t, vars.TransitiveDeps, 0)
		} else {
			require.Len(t, vars.TransitiveDeps, 1)
		}
	}

	// Default config includes everything.
	check(func(c *CommitMsgConfig) {})
	// No bugs.
	check(func(c *CommitMsgConfig) {
		c.IncludeBugs = false
	})
	check(func(c *CommitMsgConfig) {
		c.BugProject = ""
	})
	check(func(c *CommitMsgConfig) {
		c.BugProject = "bogus project; doesn't match anything"
	})
	// No log URL template.
	check(func(c *CommitMsgConfig) {
		c.ChildLogURLTmpl = ""
	})
	// No revisions.
	check(func(c *CommitMsgConfig) {
		c.IncludeLog = false
	})
	// No tests.
	check(func(c *CommitMsgConfig) {
		c.IncludeTests = false
	})
	// No transitive deps.
	check(func(c *CommitMsgConfig) {
		c.TransitiveDeps = nil
	})
}

func TestNamedTemplatesValid(t *testing.T) {
	unittest.SmallTest(t)

	cfg := fakeCommitMsgConfig(t)
	for _, tmpl := range NamedCommitMsgTemplates {
		cfg.CommitMsgTmpl = tmpl
		require.NoError(t, cfg.Validate())
	}
}
