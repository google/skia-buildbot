package git

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	exec_testutils "go.skia.org/infra/go/exec/testutils"
	"go.skia.org/infra/go/testutils"
)

func TestGitDetails(t *testing.T) {
	start := time.Now().Add(-2 * time.Second) // Need a small buffer due to 1-second granularity.
	ctx, gb, commits := setup(t)
	defer gb.Cleanup()

	g := GitDir(gb.Dir())
	for i, c := range commits {
		d, err := g.Details(ctx, c)
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
	ctx, gb, commits := setup(t)
	defer gb.Cleanup()

	tmpDir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmpDir)

	g, err := newGitDir(ctx, gb.Dir(), tmpDir, false)
	assert.NoError(t, err)
	branches, err := g.Branches(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(branches))
	master := branches[0]
	assert.Equal(t, commits[0], master.Head)
	assert.Equal(t, "master", master.Name)

	// Add a branch.
	gb.CreateBranchTrackBranch(ctx, "newbranch", "master")
	c10 := gb.CommitGen(ctx, "branchfile")
	_, err = g.Git(ctx, "fetch", "origin")
	assert.NoError(t, err)
	_, err = g.Git(ctx, "checkout", "-b", "newbranch", "-t", "origin/newbranch")
	assert.NoError(t, err)
	branches, err = g.Branches(ctx)
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

	// Add an ambiguous ref to ensure that Branches() doesn't have a
	// problem with it.
	exec_testutils.Run(t, ctx, gb.Dir(), "git", "update-ref", "refs/heads/meta/config", commits[6])
	exec_testutils.Run(t, ctx, gb.Dir(), "git", "push", "origin", "refs/heads/meta/config")
	exec_testutils.Run(t, ctx, gb.Dir(), "git", "update-ref", "refs/tags/meta/config", commits[3])
	exec_testutils.Run(t, ctx, gb.Dir(), "git", "push", "origin", "refs/tags/meta/config")
	_, err = g.Git(ctx, "fetch")
	assert.NoError(t, err)
	_, err = g.Git(ctx, "checkout", "-b", "meta/config", "-t", "origin/meta/config")
	assert.NoError(t, err)

	// Verify that it's actually ambiguous. We're also testing that RevParse
	// returns an error for ambiguous refs, since Git doesn't exit with non-
	// zero code in that case.
	_, err = g.RevParse(ctx, "meta/config")
	assert.Error(t, err)

	// Make sure Branches() succeeds and uses the correct ref.
	branches, err = g.Branches(ctx)
	assert.NoError(t, err)
	checked := false
	for _, b := range branches {
		if b.Name == "meta/config" {
			assert.Equal(t, b.Head, commits[6])
			checked = true
			break
		}
	}
	assert.True(t, checked)
}

func TestIsAncestor(t *testing.T) {
	ctx, gb, commits := setup(t)
	defer gb.Cleanup()

	tmpDir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmpDir)

	g, err := newGitDir(ctx, gb.Dir(), tmpDir, false)
	assert.NoError(t, err)

	// Commits are in decreasing chronological order; commits[0] is the most
	// recent and therefore is not an ancestor of commits[9].
	b, err := g.IsAncestor(ctx, commits[0], commits[len(commits)-1])
	assert.NoError(t, err)
	assert.False(t, b)

	for i := 0; i < len(commits)-1; i++ {
		b, err := g.IsAncestor(ctx, commits[i], commits[i+1])
		assert.NoError(t, err)
		assert.False(t, b)

		b, err = g.IsAncestor(ctx, commits[i+1], commits[i])
		assert.NoError(t, err)
		assert.True(t, b)
	}
}
