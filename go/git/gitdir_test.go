package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	exec_testutils "go.skia.org/infra/go/exec/testutils"
	"go.skia.org/infra/go/testutils"
)

func TestGitDetails(t *testing.T) {
	start := time.Now().Add(-2 * time.Second) // Need a small buffer due to 1-second granularity.
	ctx, gb, commits := setup(t)
	defer gb.Cleanup()

	g := CheckoutDir(gb.Dir())
	for i, c := range commits {
		d, err := g.Details(ctx, c)
		require.NoError(t, err)

		require.Equal(t, c, d.Hash)
		require.True(t, strings.Contains(d.Author, "@"))
		num := len(commits) - 1 - i
		require.Equal(t, fmt.Sprintf("Commit Title #%d", num), d.Subject)
		if num != 0 {
			require.Equal(t, 1, len(d.Parents))
		} else {
			require.Equal(t, 0, len(d.Parents))
		}
		require.Equal(t, fmt.Sprintf("Commit Body #%d", num), d.Body)
		require.True(t, d.Timestamp.After(start))
	}
}

func TestGitBranch(t *testing.T) {
	ctx, gb, commits := setup(t)
	defer gb.Cleanup()

	tmpDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmpDir)

	g, err := NewCheckout(ctx, gb.Dir(), tmpDir)
	require.NoError(t, err)
	branches, err := g.Branches(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(branches))
	main := branches[0]
	require.Equal(t, commits[0], main.Head)
	require.Equal(t, MainBranch, main.Name)

	// Add a branch.
	gb.CreateBranchTrackBranch(ctx, "newbranch", MainBranch)
	c10 := gb.CommitGen(ctx, "branchfile")
	_, err = g.Git(ctx, "fetch", DefaultRemote)
	require.NoError(t, err)
	_, err = g.Git(ctx, "checkout", "-b", "newbranch", "-t", "origin/newbranch")
	require.NoError(t, err)
	branches, err = g.Branches(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, len(branches))
	m, o := branches[0], branches[1]
	if o.Name == MainBranch {
		m, o = branches[1], branches[0]
	}
	require.Equal(t, commits[0], m.Head)
	require.Equal(t, MainBranch, m.Name)
	require.Equal(t, c10, o.Head)
	require.Equal(t, "newbranch", o.Name)

	// Add an ambiguous ref to ensure that Branches() doesn't have a
	// problem with it.
	git, err := Executable(ctx)
	require.NoError(t, err)
	exec_testutils.Run(t, ctx, gb.Dir(), git, "update-ref", "refs/heads/meta/config", commits[6])
	exec_testutils.Run(t, ctx, gb.Dir(), git, "push", DefaultRemote, "refs/heads/meta/config")
	exec_testutils.Run(t, ctx, gb.Dir(), git, "update-ref", "refs/tags/meta/config", commits[3])
	exec_testutils.Run(t, ctx, gb.Dir(), git, "push", DefaultRemote, "refs/tags/meta/config")
	_, err = g.Git(ctx, "fetch")
	require.NoError(t, err)
	_, err = g.Git(ctx, "checkout", "-b", "meta/config", "-t", "origin/meta/config")
	require.NoError(t, err)

	// Verify that it's actually ambiguous. We're also testing that RevParse
	// returns an error for ambiguous refs, since Git doesn't exit with non-
	// zero code in that case.
	_, err = g.RevParse(ctx, "meta/config")
	require.Error(t, err)

	// Make sure Branches() succeeds and uses the correct ref.
	branches, err = g.Branches(ctx)
	require.NoError(t, err)
	checked := false
	for _, b := range branches {
		if b.Name == "meta/config" {
			require.Equal(t, b.Head, commits[6])
			checked = true
			break
		}
	}
	require.True(t, checked)
}

func TestIsAncestor(t *testing.T) {
	ctx, gb, commits := setup(t)
	defer gb.Cleanup()

	tmpDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmpDir)

	g, err := NewCheckout(ctx, gb.Dir(), tmpDir)
	require.NoError(t, err)

	// Commits are in decreasing chronological order; commits[0] is the most
	// recent and therefore is not an ancestor of commits[9].
	b, err := g.IsAncestor(ctx, commits[0], commits[len(commits)-1])
	require.NoError(t, err)
	require.False(t, b)

	for i := 0; i < len(commits)-1; i++ {
		b, err := g.IsAncestor(ctx, commits[i], commits[i+1])
		require.NoError(t, err)
		require.False(t, b)

		b, err = g.IsAncestor(ctx, commits[i+1], commits[i])
		require.NoError(t, err)
		require.True(t, b)
	}
}

func TestGetFile(t *testing.T) {
	ctx, gb, commits := setup(t)
	defer gb.Cleanup()

	tmpDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmpDir)

	g, err := NewCheckout(ctx, gb.Dir(), tmpDir)
	require.NoError(t, err)

	contents, err := g.GetFile(ctx, checkedInFile, "HEAD")
	require.NoError(t, err)

	contentsAtHead, err := os.ReadFile(filepath.Join(g.Dir(), checkedInFile))
	require.NoError(t, err)
	require.Equal(t, string(contentsAtHead), contents)

	contents, err = g.GetFile(ctx, checkedInFile, commits[1])
	require.NoError(t, err)
	// We use rng to change file content between commits so it should be
	// different.
	require.NotEqual(t, string(contentsAtHead), contents)
}

func TestReadUpdateSubmodule(t *testing.T) {
	ctx, gb, _ := setup(t)
	defer gb.Cleanup()

	tmpDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmpDir)

	g, err := NewCheckout(ctx, gb.Dir(), tmpDir)
	require.NoError(t, err)

	// Create submodule
	require.NoError(t, err)
	_, err = g.Git(ctx, "-c", "protocol.file.allow=always", "submodule", "add", g.Dir(), "my-submodule")
	require.NoError(t, err)
	hash, err := g.FullHash(ctx, "HEAD")
	require.NoError(t, err)
	_, err = g.Git(ctx, "commit", "-m", "update submodule")
	require.NoError(t, err)

	// Test Read submodule
	contents, err := g.ReadSubmodule(ctx, "my-submodule", "HEAD")
	require.NoError(t, err)
	require.Equal(t, hash, contents)

	// Test Read directory
	contents, err = g.ReadSubmodule(ctx, ".", "HEAD")
	require.Error(t, err)
	require.Equal(t, "", contents)

	// Test Write & Read
	newHash := "2222222222222222222222222222222222222222"
	err = g.UpdateSubmodule(ctx, "my-submodule", newHash)
	require.NoError(t, err)
	_, err = g.Git(ctx, "commit", "-m", "update submodule #2")
	require.NoError(t, err)

	contents, err = g.ReadSubmodule(ctx, "my-submodule", "HEAD")
	require.NoError(t, err)
	require.Equal(t, newHash, contents)
}
