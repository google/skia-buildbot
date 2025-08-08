package git_common

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common/mocks"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/testutils"
)

func TestGetNotSubmittedReason(t *testing.T) {
	// Most recent first.
	commits := []string{
		"dddddddddddddddddddddddddddddddddddddddd",
		"cccccccccccccccccccccccccccccccccccccccc",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	tr := &mocks.RepoInterface{}
	tr.On("ResolveRef", testutils.AnyContext, git.MainBranch).Return(commits[0], nil)
	tr.On("ResolveRef", testutils.AnyContext, git.MasterBranch).Return("", errors.New("this branch doesn't exist in this repo"))
	tr.On("IsAncestor", testutils.AnyContext, commits[1], git.MainBranch).Return(true, nil)
	tr.On("IsAncestor", testutils.AnyContext, commits[2], git.MainBranch).Return(true, nil)
	tr.On("IsAncestor", testutils.AnyContext, commits[3], git.MainBranch).Return(true, nil)

	for _, c := range commits {
		result, err := GetNotSubmittedReason(t.Context(), tr, c, git.MainBranch)
		require.NoError(t, err)
		require.Equal(t, "", result)
	}
	// We expect to call ResolveRef once for every commit.
	tr.AssertNumberOfCalls(t, "ResolveRef", 4)
	// We expect to call IsAncestor for every commit that isn't at the branch
	// head.
	tr.AssertNumberOfCalls(t, "IsAncestor", 3)

	// Now try a commit that isn't submitted.
	bogus := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	tr.On("IsAncestor", testutils.AnyContext, bogus, git.MainBranch).Return(false, nil)
	result, err := GetNotSubmittedReason(t.Context(), tr, bogus, git.MainBranch)
	require.NoError(t, err)
	require.NotEmpty(t, result)
}
