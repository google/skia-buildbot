/*
   Package mem_get provides convenience functionality for writing test data into a GitStore.
*/

package mem_git

import (
	"context"
	"crypto/sha1"
	"fmt"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

var (
	// baseTime is an arbitrary timestamp used as the time of the first
	// commit. Subsequent commits add a fixed duration to the timestamp
	// of their parent(s). This keeps the commit hashes predictable.
	baseTime = time.Unix(1571926390, 0).UTC()
)

// FakeCommit creates a LongCommit with the given message, belonging to the
// given branch, and with the given parent commits. Its Timestamp and Index
// increase monotonically with respect to the parents.
func FakeCommit(t sktest.TestingT, msg, branch string, parents ...*vcsinfo.LongCommit) *vcsinfo.LongCommit {
	index := 0
	ts := baseTime
	var parentHashes []string
	if len(parents) > 0 {
		parentHashes = make([]string, 0, len(parents))
		for _, p := range parents {
			if p.Index >= index {
				index = p.Index + 1
			}
			if !ts.After(p.Timestamp) {
				ts = p.Timestamp.Add(time.Minute)
			}
			parentHashes = append(parentHashes, p.Hash)
		}
	}
	lc := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Author:  "me@google.com",
			Subject: msg,
		},
		Parents:   parentHashes,
		Timestamp: ts,
		Index:     index,
		Branches: map[string]bool{
			branch: true,
		},
	}
	j := testutils.MarshalJSON(t, lc)
	lc.Hash = fmt.Sprintf("%040x", sha1.Sum([]byte(j)))
	return lc
}

// New returns a MemGit instance which writes to the given GitStore.
func New(t sktest.TestingT, gs gitstore.GitStore) *MemGit {
	return &MemGit{
		branch: "master",
		gs:     gs,
		t:      t,
	}
}

// MemGit is a struct used for writing fake commits into a GitStore.
type MemGit struct {
	branch string
	gs     gitstore.GitStore
	t      sktest.TestingT
}

// head returns the current head of the given branch.
func (g *MemGit) head(ctx context.Context, branch string) string {
	branches, err := g.gs.GetBranches(ctx)
	require.NoError(g.t, err)
	ptr, ok := branches[branch]
	if !ok {
		return ""
	}
	return ptr.Head
}

// makeCommit creates a commit with the given message and parents on the
// currently-active branch.
func (g *MemGit) makeCommit(ctx context.Context, msg string, parentHashes []string) *vcsinfo.LongCommit {
	parents := []*vcsinfo.LongCommit{}
	if len(parentHashes) > 0 {
		var err error
		parents, err = g.gs.Get(ctx, parentHashes)
		require.NoError(g.t, err)
	}
	lc := FakeCommit(g.t, msg, g.branch, parents...)
	require.NoError(g.t, g.gs.Put(ctx, []*vcsinfo.LongCommit{lc}))
	require.NoError(g.t, g.gs.PutBranches(ctx, map[string]string{
		g.branch: lc.Hash,
	}))
	return lc
}

// Commit adds a commit to the GitStore and returns its hash.
func (g *MemGit) Commit(ctx context.Context, msg string) string {
	head := g.head(ctx, g.branch)
	var parents []string
	if head != "" {
		parents = []string{head}
	}
	return g.makeCommit(ctx, msg, parents).Hash
}

// CommitN adds N commits to the GitStore and returns their hashes in reverse
// chronological order.
func (g *MemGit) CommitN(ctx context.Context, n int) []string {
	hashes := make([]string, 0, n)
	for i := 0; i < n; i++ {
		hashes = append(hashes, g.Commit(ctx, fmt.Sprintf("Dummy #%d/%d", i, n)))
	}
	return util.Reverse(hashes)
}

// CheckoutBranch switches to the given branch.
func (g *MemGit) CheckoutBranch(ctx context.Context, branch string) {
	branches, err := g.gs.GetBranches(ctx)
	require.NoError(g.t, err)
	ptr, ok := branches[branch]
	require.True(g.t, ok)
	require.NotNil(g.t, ptr)
	require.NotEqual(g.t, "", ptr.Head)
	g.branch = branch
}

// NewBranch creates the given branch at the given commit hash.
func (g *MemGit) NewBranch(ctx context.Context, branch, head string) {
	branches, err := g.gs.GetBranches(ctx)
	require.NoError(g.t, err)
	_, ok := branches[branch]
	require.False(g.t, ok, "Already have %s", branch)
	require.NoError(g.t, g.gs.PutBranches(ctx, map[string]string{
		branch: head,
	}))
}

// Merge creates a new commit which merges the given branch into the currently
// active branch.
func (g *MemGit) Merge(ctx context.Context, branch string) *vcsinfo.LongCommit {
	return g.makeCommit(ctx, fmt.Sprintf("Merge %s", branch), []string{g.head(ctx, g.branch), g.head(ctx, branch)})
}
