package bugs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
)

func TestBuganizer(t *testing.T) {
	unittest.SmallTest(t)

	issueTracker, err := InitIssueTracker()
	require.NoError(t, err)

	issues, err := issueTracker.Search("rmistry@google.com", nil)
	require.NoError(t, err)
	fmt.Println(issues)
}
