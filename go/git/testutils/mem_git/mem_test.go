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
	"go.skia.org/infra/go/testutils/unittest"
)

func TestMemGit(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.Background()

	// Verify that we generate the correct commits.
	gs := mem_gitstore.New()
	g := New(t, gs)
	hashes := g.CommitN(ctx, 2)
	require.Equal(t, 2, len(hashes))
	// Reverse chronological order.
	require.Equal(t, Commit1, hashes[0])
	require.Equal(t, Commit0, hashes[1])
	commits, err := gs.Get(ctx, hashes)
	require.NoError(t, err)
	require.Equal(t, Commit1, commits[0].Hash)
	require.Equal(t, BaseTime.Add(time.Minute), commits[0].Timestamp)
	assertdeep.Equal(t, []string{Commit0}, commits[0].Parents)
	assertdeep.Equal(t, map[string]bool{git.DefaultBranch: true}, commits[0].Branches)
	require.Equal(t, 1, commits[0].Index)
	require.Equal(t, Commit0, commits[1].Hash)
	require.Equal(t, BaseTime, commits[1].Timestamp)
	require.Nil(t, commits[1].Parents)
	assertdeep.Equal(t, map[string]bool{git.DefaultBranch: true}, commits[1].Branches)
	require.Equal(t, 0, commits[1].Index)
	branches, err := gs.GetBranches(ctx)
	require.NoError(t, err)
	assertdeep.Equal(t, map[string]*gitstore.BranchPointer{
		git.DefaultBranch: {
			Head:  Commit1,
			Index: 1,
		},
	}, branches)

	// Create a branch.
	g.NewBranch(ctx, "branch2", Commit0)
	branches, err = gs.GetBranches(ctx)
	require.NoError(t, err)
	assertdeep.Equal(t, map[string]*gitstore.BranchPointer{
		git.DefaultBranch: {
			Head:  Commit1,
			Index: 1,
		},
		"branch2": {
			Head:  Commit0,
			Index: 0,
		},
	}, branches)
	bcHash := g.Commit(ctx, "branch commit")
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
		git.DefaultBranch: {
			Head:  Commit1,
			Index: 1,
		},
		"branch2": {
			Head:  bcHash,
			Index: 1,
		},
	}, branches)

	// Merge into the main branch.
	g.CheckoutBranch(ctx, git.DefaultBranch)
	merge := g.Merge(ctx, "branch2")
	mcs, err := gs.Get(ctx, []string{merge.Hash})
	require.NoError(t, err)
	mc := mcs[0]
	assertdeep.Equal(t, merge, mc)
	assertdeep.Equal(t, []string{Commit1, bcHash}, mc.Parents)
	assertdeep.Equal(t, map[string]bool{git.DefaultBranch: true}, mc.Branches)
	require.Equal(t, 2, mc.Index)
}
