package gerrit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gerrit/mocks"
)

const (
	testRepo         = "skia-test"
	testBranch       = "chrome/m100"
	cherrypickChange = 100
	changeID1        = int64(123)
	changeID2        = int64(456)
)

var (
	changeInfo1 = &gerrit.ChangeInfo{Issue: changeID1}
	changeInfo2 = &gerrit.ChangeInfo{Issue: changeID2}
)

func TestFindAllOpenCherrypicks_ValidQuery_ReturnsFoundCherryPicks(t *testing.T) {
	ctx := context.Background()

	// Mock gerrit.
	g := &mocks.GerritInterface{}
	defer g.AssertExpectations(t)
	// Mock search call.
	g.On(
		"Search",
		ctx,
		maxGerritSearchResults,
		true,
		gerrit.SearchProject(testRepo),
		gerrit.SearchBranch(testBranch),
		gerrit.SearchStatus(gerrit.ChangeStatusOpen),
	).Return([]*gerrit.ChangeInfo{changeInfo1, changeInfo2}, nil).Once()

	cherrypicks, err := FindAllOpenCherrypicks(ctx, g, testRepo, testBranch)
	require.Nil(t, err)
	require.Len(t, cherrypicks, 2)
	require.Equal(t, changeID1, cherrypicks[0].Issue)
	require.Equal(t, changeID2, cherrypicks[1].Issue)
}

func TestIsCherrypickIn_TargetBranchTwoCherrypicks_ReturnsTrue(t *testing.T) {
	ctx := context.Background()

	// Mock gerrit.
	g := &mocks.GerritInterface{}
	defer g.AssertExpectations(t)

	// Mock search call that returns 2 changes with the cherrypick.
	g.On(
		"Search",
		ctx,
		maxGerritSearchResults,
		true,
		gerrit.SearchProject(testRepo),
		gerrit.SearchBranch(testBranch),
		gerrit.SearchCherrypickOf(cherrypickChange),
	).Return([]*gerrit.ChangeInfo{changeInfo1, changeInfo2}, nil).Once()
	// isCherrypickIn should return true because 2 changes had the cherrypick.
	isCherrypickIn, err := IsCherrypickIn(ctx, g, testRepo, testBranch, cherrypickChange)
	require.Nil(t, err)
	require.True(t, isCherrypickIn)
}

func TestIsCherrypickIn_TargetBranchNoCherrypicks_ReturnsFalse(t *testing.T) {
	ctx := context.Background()

	// Mock gerrit.
	g := &mocks.GerritInterface{}
	defer g.AssertExpectations(t)

	// Mock search call that does not return any changes.
	g.On(
		"Search",
		ctx,
		maxGerritSearchResults,
		true,
		gerrit.SearchProject(testRepo),
		gerrit.SearchBranch(testBranch),
		gerrit.SearchCherrypickOf(cherrypickChange),
	).Return([]*gerrit.ChangeInfo{}, nil).Once()
	// isCherrypickIn should return false because no changes had the cherrypick.
	isCherrypickIn, err := IsCherrypickIn(ctx, g, testRepo, testBranch, cherrypickChange)
	require.Nil(t, err)
	require.False(t, isCherrypickIn)
}

func TestAddReminderComment_ValidQuery_ReturnsNoErrors(t *testing.T) {
	ctx := context.Background()
	testComment := "test comment"

	// Mock gerrit.
	g := &mocks.GerritInterface{}
	defer g.AssertExpectations(t)
	// Mock call to get the fully populated ChangeInfo obj.
	g.On("GetIssueProperties", ctx, changeID1).Return(changeInfo1, nil).Once()
	// Mock call to publish comment.
	g.On("SetReview", ctx, changeInfo1, testComment, map[string]int{}, mock.Anything, gerrit.NotifyOwner, mock.Anything, "", 0, mock.Anything).Return(nil).Once()

	err := AddReminderComment(ctx, g, changeInfo1, testComment)
	require.Nil(t, err)
}
