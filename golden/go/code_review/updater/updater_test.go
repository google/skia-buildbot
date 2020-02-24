package updater

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/clstore"
	mock_clstore "go.skia.org/infra/golden/go/clstore/mocks"
	"go.skia.org/infra/golden/go/code_review"
	mock_codereview "go.skia.org/infra/golden/go/code_review/mocks"
	"go.skia.org/infra/golden/go/expectations"
	mock_expectations "go.skia.org/infra/golden/go/expectations/mocks"
	"go.skia.org/infra/golden/go/types"
)

// TestUpdateSunnyDay checks a case in which three commits are seen, one of which we already know
// is landed and two more that are open with some CLExpectations.
func TestUpdateSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mc := &mock_codereview.Client{}
	mes := &mock_expectations.Store{}
	mcs := &mock_clstore.Store{}
	alphaExp := &mock_expectations.Store{}
	betaExp := &mock_expectations.Store{}
	defer mc.AssertExpectations(t)
	defer mes.AssertExpectations(t)
	defer mcs.AssertExpectations(t)
	defer alphaExp.AssertExpectations(t)
	defer betaExp.AssertExpectations(t)

	commits := makeCommits()

	var alphaChanges expectations.Expectations
	alphaChanges.Set(someTest, digestOne, expectations.Negative)
	alphaDelta := expectations.AsDelta(&alphaChanges)

	var betaChanges expectations.Expectations
	betaChanges.Set(someTest, digestTwo, expectations.Positive)
	betaDelta := expectations.AsDelta(&betaChanges)

	// This data is all arbitrary.
	mc.On("GetChangeListIDForCommit", testutils.AnyContext, commits[0]).Return(landedCL, nil)
	mc.On("GetChangeListIDForCommit", testutils.AnyContext, commits[1]).Return(openCLAlpha, nil)
	mc.On("GetChangeList", testutils.AnyContext, openCLAlpha).Return(code_review.ChangeList{
		SystemID: openCLAlpha,
		Status:   code_review.Landed, // the CRS says they are landed, but the store thinks not.
		Owner:    alphaAuthor,
		Updated:  time.Date(2019, time.May, 15, 14, 14, 12, 0, time.UTC),
	}, nil)
	mc.On("GetChangeListIDForCommit", testutils.AnyContext, commits[2]).Return(openCLBeta, nil)
	mc.On("GetChangeList", testutils.AnyContext, openCLBeta).Return(code_review.ChangeList{
		SystemID: openCLBeta,
		Status:   code_review.Landed, // the CRS says they are landed, but the store thinks not.
		Owner:    betaAuthor,
		Updated:  time.Date(2019, time.May, 15, 14, 18, 12, 0, time.UTC),
	}, nil)

	mc.On("System").Return(crs)

	mes.On("ForChangeList", openCLAlpha, crs).Return(alphaExp)
	mes.On("ForChangeList", openCLBeta, crs).Return(betaExp)
	mes.On("AddChange", testutils.AnyContext, alphaDelta, alphaAuthor).Return(nil)
	mes.On("AddChange", testutils.AnyContext, betaDelta, betaAuthor).Return(nil)

	alphaExp.On("Get", testutils.AnyContext).Return(&alphaChanges, nil)
	betaExp.On("Get", testutils.AnyContext).Return(&betaChanges, nil)

	mcs.On("GetChangeList", testutils.AnyContext, landedCL).Return(code_review.ChangeList{
		SystemID: landedCL,
		Status:   code_review.Landed, // Already in the store as landed - should be skipped.
		Owner:    alphaAuthor,
	}, nil)
	mcs.On("GetChangeList", testutils.AnyContext, openCLAlpha).Return(code_review.ChangeList{
		SystemID: openCLAlpha,
		Status:   code_review.Open, // the CRS says they are landed, but the store thinks not.
		Owner:    alphaAuthor,
	}, nil)
	mcs.On("GetChangeList", testutils.AnyContext, openCLBeta).Return(code_review.ChangeList{
		SystemID: openCLBeta,
		Status:   code_review.Open, // the CRS says they are landed, but the store thinks not.
		Owner:    betaAuthor,
	}, nil)
	clChecker := func(cl code_review.ChangeList) bool {
		if cl.SystemID == openCLAlpha || cl.SystemID == openCLBeta {
			require.Equal(t, code_review.Landed, cl.Status)
			require.NotZero(t, cl.Updated)
			require.Equal(t, time.May, cl.Updated.Month())
			return true
		}
		return false
	}
	mcs.On("PutChangeList", testutils.AnyContext, mock.MatchedBy(clChecker)).Return(nil).Twice()

	u := New(mc, mes, mcs)
	err := u.UpdateChangeListsAsLanded(context.Background(), commits)
	require.NoError(t, err)
}

// TestUpdateEmpty checks the common case of there being no CLExpectations
func TestUpdateEmpty(t *testing.T) {
	unittest.SmallTest(t)

	mc := &mock_codereview.Client{}
	mes := &mock_expectations.Store{}
	mcs := &mock_clstore.Store{}
	betaExp := &mock_expectations.Store{}
	defer mc.AssertExpectations(t)
	defer mes.AssertExpectations(t)
	defer mcs.AssertExpectations(t)
	defer betaExp.AssertExpectations(t)

	commits := makeCommits()[2:]

	betaChanges := expectations.Expectations{}

	mc.On("GetChangeListIDForCommit", testutils.AnyContext, commits[0]).Return(openCLBeta, nil)
	mc.On("GetChangeList", testutils.AnyContext, openCLBeta).Return(code_review.ChangeList{
		SystemID: openCLBeta,
		Status:   code_review.Landed,
		Owner:    betaAuthor,
		Updated:  time.Date(2019, time.May, 15, 14, 18, 12, 0, time.UTC),
	}, nil)
	mc.On("System").Return(crs)

	mes.On("ForChangeList", openCLBeta, crs).Return(betaExp)

	betaExp.On("Get", testutils.AnyContext).Return(&betaChanges, nil)

	mcs.On("GetChangeList", testutils.AnyContext, openCLBeta).Return(code_review.ChangeList{
		SystemID: openCLBeta,
		Status:   code_review.Open, // the CRS says they are landed, but the store thinks not.
		Owner:    betaAuthor,
	}, nil)
	clChecker := func(cl code_review.ChangeList) bool {
		if cl.SystemID == openCLBeta {
			require.Equal(t, code_review.Landed, cl.Status)
			require.NotZero(t, cl.Updated)
			require.Equal(t, time.May, cl.Updated.Month())
			return true
		}
		return false
	}
	mcs.On("PutChangeList", testutils.AnyContext, mock.MatchedBy(clChecker)).Return(nil).Once()

	u := New(mc, mes, mcs)
	err := u.UpdateChangeListsAsLanded(context.Background(), commits)
	require.NoError(t, err)
}

// TestUpdateNoTryJobsSeen checks the common case of there being no TryJobs that uploaded data
// associated with this CL (thus, it won't be in clstore)
func TestUpdateNoTryJobsSeen(t *testing.T) {
	unittest.SmallTest(t)

	mc := &mock_codereview.Client{}
	mes := &mock_expectations.Store{}
	mcs := &mock_clstore.Store{}
	defer mc.AssertExpectations(t)
	defer mes.AssertExpectations(t)
	defer mcs.AssertExpectations(t)

	commits := makeCommits()[2:]

	mc.On("GetChangeListIDForCommit", testutils.AnyContext, commits[0]).Return(openCLBeta, nil)
	mc.On("System").Return(crs)

	mcs.On("GetChangeList", testutils.AnyContext, openCLBeta).Return(code_review.ChangeList{}, clstore.ErrNotFound)

	u := New(mc, mes, mcs)
	err := u.UpdateChangeListsAsLanded(context.Background(), commits)
	require.NoError(t, err)
}

// TestUpdateNoChangeList checks the exceptional case where a commit lands without being tied to
// a ChangeList in the CRS (we should skip it and not crash).
func TestUpdateNoChangeList(t *testing.T) {
	unittest.SmallTest(t)

	mc := &mock_codereview.Client{}
	defer mc.AssertExpectations(t)

	commits := makeCommits()[2:]

	mc.On("GetChangeListIDForCommit", testutils.AnyContext, commits[0]).Return("", code_review.ErrNotFound)
	mc.On("System").Return(crs)

	u := New(mc, nil, nil)
	err := u.UpdateChangeListsAsLanded(context.Background(), commits)
	require.NoError(t, err)
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
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: landedCL,
			},
			// All other fields are ignored
			Body: "Reviewed-on: https://skia-review.googlesource.com/c/skia/+/1landed",
		},
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: openCLAlpha,
			},
			// All other fields are ignored
			Body: "Reviewed-on: https://skia-review.googlesource.com/c/skia/+/2open",
		},
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: openCLBeta,
			},
			// All other fields are ignored
			Body: "Reviewed-on: https://skia-review.googlesource.com/c/skia/+/3open",
		},
	}
}
