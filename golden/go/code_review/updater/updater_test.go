package updater

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"

	"go.skia.org/infra/golden/go/types"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/code_review"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
	mock_clstore "go.skia.org/infra/golden/go/clstore/mocks"
	mock_codereview "go.skia.org/infra/golden/go/code_review/mocks"
	"go.skia.org/infra/golden/go/mocks"
)

// TestUpdateSunnyDay checks a case in which three commits are seen, one of which we already know
// is landed and two more that are open with some CLExpectations.
func TestUpdateSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mc := &mock_codereview.Client{}
	mes := &mocks.ExpectationsStore{}
	mcs := &mock_clstore.Store{}
	alphaExp := &mocks.ExpectationsStore{}
	betaExp := &mocks.ExpectationsStore{}
	defer mc.AssertExpectations(t)
	defer mes.AssertExpectations(t)
	defer mcs.AssertExpectations(t)
	defer alphaExp.AssertExpectations(t)
	defer betaExp.AssertExpectations(t)

	commits := makeCommits()

	alphaChanges := types.Expectations{
		someTest: {
			digestOne: types.NEGATIVE,
		},
	}
	betaChanges := types.Expectations{
		someTest: {
			digestTwo: types.POSITIVE,
		},
	}

	mc.On("GetChangeListForCommit", testutils.AnyContext, commits[0]).Return(code_review.ChangeList{
		SystemID: landedCL,
		Status:   code_review.Landed,
		Owner:    "user1@example.com",
	}, nil)
	mc.On("GetChangeListForCommit", testutils.AnyContext, commits[1]).Return(code_review.ChangeList{
		SystemID: openCLAlpha,
		Status:   code_review.Open,
		Owner:    alphaAuthor,
	}, nil)
	mc.On("GetChangeListForCommit", testutils.AnyContext, commits[2]).Return(code_review.ChangeList{
		SystemID: openCLBeta,
		Status:   code_review.Open,
		Owner:    betaAuthor,
	}, nil)
	mc.On("System").Return(crs)

	mes.On("ForChangeList", openCLAlpha, crs).Return(alphaExp)
	mes.On("ForChangeList", openCLBeta, crs).Return(betaExp)
	mes.On("AddChange", testutils.AnyContext, alphaChanges, alphaAuthor).Return(nil)
	mes.On("AddChange", testutils.AnyContext, betaChanges, betaAuthor).Return(nil)

	alphaExp.On("Get").Return(alphaChanges, nil)
	betaExp.On("Get").Return(betaChanges, nil)

	clChecker := func(cl code_review.ChangeList) bool {
		if cl.SystemID == openCLAlpha || cl.SystemID == openCLBeta {
			assert.Equal(t, code_review.Landed, cl.Status)
			return true
		}
		return false
	}
	mcs.On("PutChangeList", testutils.AnyContext, mock.MatchedBy(clChecker)).Return(nil).Twice()

	u := New(mc, mes, mcs)
	err := u.UpdateChangeListsAsLanded(context.Background(), commits)
	assert.NoError(t, err)
}

// TestUpdateEmpty checks the common case of there being no CLExpectations
func TestUpdateEmpty(t *testing.T) {
	unittest.SmallTest(t)

	mc := &mock_codereview.Client{}
	mes := &mocks.ExpectationsStore{}
	mcs := &mock_clstore.Store{}
	betaExp := &mocks.ExpectationsStore{}
	defer mc.AssertExpectations(t)
	defer mes.AssertExpectations(t)
	defer mcs.AssertExpectations(t)
	defer betaExp.AssertExpectations(t)

	commits := makeCommits()[2:]

	betaChanges := types.Expectations{}

	mc.On("GetChangeListForCommit", testutils.AnyContext, commits[0]).Return(code_review.ChangeList{
		SystemID: openCLBeta,
		Status:   code_review.Open,
		Owner:    betaAuthor,
	}, nil)
	mc.On("System").Return(crs)

	mes.On("ForChangeList", openCLBeta, crs).Return(betaExp)

	betaExp.On("Get").Return(betaChanges, nil)

	clChecker := func(cl code_review.ChangeList) bool {
		if cl.SystemID == openCLBeta {
			assert.Equal(t, code_review.Landed, cl.Status)
			return true
		}
		return false
	}
	mcs.On("PutChangeList", testutils.AnyContext, mock.MatchedBy(clChecker)).Return(nil).Once()

	u := New(mc, mes, mcs)
	err := u.UpdateChangeListsAsLanded(context.Background(), commits)
	assert.NoError(t, err)
}

const (
	crs = "github"

	landedCL    = "11196d8aff4cd689c2e49336d12928a8bd23cdec"
	openCLAlpha = "aaa5f37f5bd91f1a7b3f080bf038af8e8fa4cab2"
	openCLBeta  = "bbb734d4127ab3fa7f8d08eec985e2336d5472a7"

	alphaAuthor = "user2@example.com"
	betaAuthor  = "user3@example.com"

	someTest  = types.TestName("some_test")
	digestOne = types.Digest("abc94d08ed22d21bc50cbe02da366b16")
	digestTwo = types.Digest("1232cfd382db585f297e31dbe9a0151f")
)

func makeCommits() []*vcsinfo.LongCommit {
	return []*vcsinfo.LongCommit{
		{
			// All other fields are ignored
			Body: "Reviewed-on: https://skia-review.googlesource.com/c/skia/+/1landed",
		},
		{
			// All other fields are ignored
			Body: "Reviewed-on: https://skia-review.googlesource.com/c/skia/+/2open",
		},
		{
			// All other fields are ignored
			Body: "Reviewed-on: https://skia-review.googlesource.com/c/skia/+/3open",
		},
	}
}
