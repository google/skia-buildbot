package revision

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestCopyRevision(t *testing.T) {
	unittest.SmallTest(t)

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
	unittest.SmallTest(t)

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
