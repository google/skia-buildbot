package issuesync

import (
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestIssueSync(t *testing.T) {
	issueSync := New()
	assert.NotNil(t, issueSync)
}
