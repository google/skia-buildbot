package revision

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
)

func TestCopyRevision(t *testing.T) {

	v := &Revision{
		Id:               "abc123",
		ExternalChangeId: "xyz123",
		Author:           "me@google.com",
		Bugs: map[string][]string{
			"project": {"123"},
		},
		Display:     "abc",
		Description: "This is a great commit.",
		Dependencies: map[string]string{
			"dep": "version1",
		},
		Details:       "blah blah blah",
		InvalidReason: "flu",
		Tests:         []string{"test1"},
		Timestamp:     time.Now(),
		URL:           "www.best-commit.com",
	}
	assertdeep.Copy(t, v, v.Copy())
}

func TestParseTests(t *testing.T) {

	bodyWithThreeTestLines := `testing
Test: tested with 0
testing
BUG=skia:123
Bug: skia:456
Test: tested with 1
BUG=b/123
Bug: b/234

Test: tested with 2
`
	testLines := parseTests(bodyWithThreeTestLines)
	require.Equal(t, []string{"Test: tested with 0", "Test: tested with 1", "Test: tested with 2"}, testLines)

	bodyWithNoTestLines := `testing
no test
lines
included
here
`
	testLines = parseTests(bodyWithNoTestLines)
	require.Equal(t, 0, len(testLines))
}

func TestBugsFromCommitMsg(t *testing.T) {
	cases := []struct {
		in  string
		out map[string][]string
	}{
		{
			in: "BUG=skia:1234",
			out: map[string][]string{
				"skia": {"1234"},
			},
		},
		{
			in: "BUG=skia:1234,skia:4567",
			out: map[string][]string{
				"skia": {"1234", "4567"},
			},
		},
		{
			in: "BUG=skia:1234,skia:4567,skia:8901",
			out: map[string][]string{
				"skia": {"1234", "4567", "8901"},
			},
		},
		{
			in: "BUG=1234",
			out: map[string][]string{
				"chromium": {"1234"},
			},
		},
		{
			in: "BUG=skia:1234, 456",
			out: map[string][]string{
				"chromium": {"456"},
				"skia":     {"1234"},
			},
		},
		{
			in: "BUG=skia:1234,456",
			out: map[string][]string{
				"chromium": {"456"},
				"skia":     {"1234"},
			},
		},
		{
			in: `Lorem ipsum dolor sit amet, consectetur adipiscing elit.

Quisque feugiat, mi et tristique dignissim, sapien risus tristique mi, non dignissim nibh erat ut ex.

BUG=1234, skia:5678
`,
			out: map[string][]string{
				"chromium": {"1234"},
				"skia":     {"5678"},
			},
		},
		{
			in: "Bug: skia:1234",
			out: map[string][]string{
				"skia": {"1234"},
			},
		},
		{
			in: "Bug: skia:1234,skia:4567",
			out: map[string][]string{
				"skia": {"1234", "4567"},
			},
		},
		{
			in: "Bug: skia:1234,skia:4567,skia:8901",
			out: map[string][]string{
				"skia": {"1234", "4567", "8901"},
			},
		},
		{
			in: "Bug: 1234",
			out: map[string][]string{
				"chromium": {"1234"},
			},
		},
		{
			in: "Bug: skia:1234, 456",
			out: map[string][]string{
				"chromium": {"456"},
				"skia":     {"1234"},
			},
		},
		{
			in: "Bug: skia:1234,456",
			out: map[string][]string{
				"chromium": {"456"},
				"skia":     {"1234"},
			},
		},
		{
			in: "Bug: 1234,456",
			out: map[string][]string{
				"chromium": {"1234", "456"},
			},
		},
		{
			in: "Bug: skia:1234,chromium:456",
			out: map[string][]string{
				"chromium": {"456"},
				"skia":     {"1234"},
			},
		},
		{
			in: `asdf
Bug: skia:1234,456
BUG=skia:888
`,
			out: map[string][]string{
				"chromium": {"456"},
				"skia":     {"1234", "888"},
			},
		},
		{
			in: "Bug: skia:123 chromium:456",
			out: map[string][]string{
				"chromium": {"456"},
				"skia":     {"123"},
			},
		},
		{
			in: "Bug: skia:123, chromium:456",
			out: map[string][]string{
				"chromium": {"456"},
				"skia":     {"123"},
			},
		},
		{
			in: "Bug: skia:123,chromium:",
			out: map[string][]string{
				"skia": {"123"},
			},
		},
		{
			in: "Bug: b/123",
			out: map[string][]string{
				BugProjectBuganizer: {"123"},
			},
		},
		{
			in: "Bug: skia:123,b/456",
			out: map[string][]string{
				"skia":              {"123"},
				BugProjectBuganizer: {"456"},
			},
		},
		{
			in: `testing
Test: tested
BUG=skia:123
Bug: skia:456
BUG=b/123
Bug: b/234`,
			out: map[string][]string{
				"skia":              {"123", "456"},
				BugProjectBuganizer: {"123", "234"},
			},
		},
		{
			in: `testing
Test: tested
BUG=skia:123
Bug: skia:456
BUG=ba/123
Bug: bb/234`,
			out: map[string][]string{
				"skia": {"123", "456"},
			},
		},
	}
	for _, tc := range cases {
		result := bugsFromCommitMsg(tc.in, "")
		require.Equal(t, tc.out, result, "Input was: %q", tc.in)
	}

	// Test a passed-in default bug project.
	result := bugsFromCommitMsg("Bug: 1234", "fake-project")
	require.Equal(t, map[string][]string{
		"fake-project": {"1234"},
	}, result)
}
