package git

import (
	"fmt"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
)

func TestGitDetails(t *testing.T) {
	start := time.Now().Add(-2 * time.Second) // Need a small buffer due to 1-second granularity.

	gb, commits := setup(t)
	defer gb.Cleanup()

	g := GitDir(gb.Dir())
	for i, c := range commits {
		d, err := g.Details(c)
		assert.NoError(t, err)

		assert.Equal(t, c, d.Hash)
		assert.True(t, strings.Contains(d.Author, "@"))
		num := len(commits) - 1 - i
		assert.Equal(t, fmt.Sprintf("Commit Title #%d", num), d.Subject)
		if num != 0 {
			assert.Equal(t, 1, len(d.Parents))
		} else {
			assert.Equal(t, 0, len(d.Parents))
		}
		assert.Equal(t, fmt.Sprintf("Commit Body #%d", num), d.Body)
		assert.True(t, d.Timestamp.After(start))
	}
}

func TestGitBranch(t *testing.T) {
	gb, commits := setup(t)
	defer gb.Cleanup()

	g := GitDir(gb.Dir())
	branches, err := g.Branches()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(branches))
	master := branches[0]
	assert.Equal(t, commits[0], master.Head)
	assert.Equal(t, "master", master.Name)

	// Add a branch.
	gb.CreateBranchTrackBranch("newbranch", "master")
	c10 := gb.CommitGen("branchfile")
	branches, err = g.Branches()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(branches))
	m, o := branches[0], branches[1]
	if o.Name == "master" {
		m, o = branches[1], branches[0]
	}
	assert.Equal(t, commits[0], m.Head)
	assert.Equal(t, "master", m.Name)
	assert.Equal(t, c10, o.Head)
	assert.Equal(t, "newbranch", o.Name)
}
