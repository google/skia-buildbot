package mem_git

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/gitstore/mem_gitstore"
)

func TestMemGit(t *testing.T) {

	ctx := context.Background()

	// Verify that we generate the correct commits.
	gs := mem_gitstore.New()
	g := New(t, gs)
	hashes := g.CommitN(2)
	require.Equal(t, 2, len(hashes))
	// Reverse chronological order.
	require.Equal(t, Commit1, hashes[0])
	require.Equal(t, Commit0, hashes[1])
	commits, err := gs.Get(ctx, hashes)
	require.NoError(t, err)
	require.Equal(t, Commit1, commits[0].Hash)
	require.Equal(t, BaseTime.Add(time.Minute), commits[0].Timestamp)
	assertdeep.Equal(t, []string{Commit0}, commits[0].Parents)
	assertdeep.Equal(t, map[string]bool{git.MainBranch: true}, commits[0].Branches)
	require.Equal(t, 1, commits[0].Index)
	require.Equal(t, Commit0, commits[1].Hash)
	require.Equal(t, BaseTime, commits[1].Timestamp)
	require.Nil(t, commits[1].Parents)
	assertdeep.Equal(t, map[string]bool{git.MainBranch: true}, commits[1].Branches)
	require.Equal(t, 0, commits[1].Index)
	branches, err := gs.GetBranches(ctx)
	require.NoError(t, err)
	assertdeep.Equal(t, map[string]*gitstore.BranchPointer{
		git.MainBranch: {
			Head:  Commit1,
			Index: 1,
		},
	}, branches)

	// Create a branch.
	g.NewBranch("branch2", Commit0)
	branches, err = gs.GetBranches(ctx)
	require.NoError(t, err)
	assertdeep.Equal(t, map[string]*gitstore.BranchPointer{
		git.MainBranch: {
			Head:  Commit1,
			Index: 1,
		},
		"branch2": {
			Head:  Commit0,
			Index: 0,
		},
	}, branches)
	bcHash := g.Commit("branch commit")
	require.NotEqual(t, Commit1, bcHash)
	bcs, err := gs.Get(ctx, []string{bcHash})
	require.NoError(t, err)
	bc := bcs[0]
	require.Equal(t, bcHash, bc.Hash)
	require.Equal(t, BaseTime.Add(time.Minute), bc.Timestamp)
	assertdeep.Equal(t, []string{Commit0}, bc.Parents)
	assertdeep.Equal(t, map[string]bool{"branch2": true}, bc.Branches)
	require.Equal(t, 1, bc.Index)
	branches, err = gs.GetBranches(ctx)
	require.NoError(t, err)
	assertdeep.Equal(t, map[string]*gitstore.BranchPointer{
		git.MainBranch: {
			Head:  Commit1,
			Index: 1,
		},
		"branch2": {
			Head:  bcHash,
			Index: 1,
		},
	}, branches)

	// Merge into the main branch.
	g.CheckoutBranch(git.MainBranch)
	merge := g.Merge("branch2")
	mcs, err := gs.Get(ctx, []string{merge})
	require.NoError(t, err)
	mc := mcs[0]
	assertdeep.Equal(t, merge, mc.Hash)
	assertdeep.Equal(t, []string{Commit1, bcHash}, mc.Parents)
	assertdeep.Equal(t, map[string]bool{git.MainBranch: true}, mc.Branches)
	require.Equal(t, 2, mc.Index)
}

func TestCommit_Parents(t *testing.T) {

	ctx := context.Background()
	gs := mem_gitstore.New()
	g := New(t, gs)

	c0 := g.Commit("c0")

	t.Run("InitialCommitHasNoParents", func(t *testing.T) {
		commits, err := gs.Get(ctx, []string{c0})
		require.NoError(t, err)
		require.Len(t, commits, 1)
		require.NotNil(t, commits[0])
		require.Len(t, commits[0].Parents, 0)
	})

	c1 := g.Commit("c1")

	t.Run("SecondCommitHasOneParent", func(t *testing.T) {
		commits, err := gs.Get(ctx, []string{c1})
		require.NoError(t, err)
		require.Len(t, commits, 1)
		require.NotNil(t, commits[0])
		require.Len(t, commits[0].Parents, 1)
		require.Equal(t, commits[0].Parents[0], c0)
	})

	t.Run("ExplicitlySetSingleParent", func(t *testing.T) {
		c2 := g.Commit("c2", c0)
		commits, err := gs.Get(ctx, []string{c2})
		require.NoError(t, err)
		require.Len(t, commits, 1)
		require.NotNil(t, commits[0])
		require.Len(t, commits[0].Parents, 1)
		require.Equal(t, commits[0].Parents[0], c0)
	})

	t.Run("SetTwoParents", func(t *testing.T) {
		c3 := g.Commit("c3", c0, c1)
		commits, err := gs.Get(ctx, []string{c3})
		require.NoError(t, err)
		require.Len(t, commits, 1)
		require.NotNil(t, commits[0])
		require.Len(t, commits[0].Parents, 2)
		require.Equal(t, commits[0].Parents[0], c0)
		require.Equal(t, commits[0].Parents[1], c1)
	})
}

func TestCommitAt(t *testing.T) {

	ctx := context.Background()
	gs := mem_gitstore.New()
	g := New(t, gs)

	t0 := time.Date(2021, time.October, 12, 4, 14, 43, 0, time.UTC)

	t.Run("SetSpecificTime", func(t *testing.T) {
		c0 := g.CommitAt("c0", t0)
		commits, err := gs.Get(ctx, []string{c0})
		require.NoError(t, err)
		require.Len(t, commits, 1)
		require.NotNil(t, commits[0])
		require.Equal(t, t0, commits[0].Timestamp)
	})

	t.Run("TimestampIncrementsWhenNotSet", func(t *testing.T) {
		c1 := g.CommitAt("c1", time.Time{})
		commits, err := gs.Get(ctx, []string{c1})
		require.NoError(t, err)
		require.Len(t, commits, 1)
		require.NotNil(t, commits[0])
		require.Equal(t, t0.Add(time.Minute).UnixNano(), commits[0].Timestamp.UnixNano())
	})

	t.Run("SetSpecificTime2", func(t *testing.T) {
		c2 := g.CommitAt("c2", t0)
		commits, err := gs.Get(ctx, []string{c2})
		require.NoError(t, err)
		require.Len(t, commits, 1)
		require.NotNil(t, commits[0])
		require.Equal(t, t0, commits[0].Timestamp)
	})
}
