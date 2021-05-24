package commenter

import (
	"context"
	"testing"
	"time"

	"go.skia.org/infra/go/testutils"

	"go.skia.org/infra/go/now"

	"go.skia.org/infra/golden/go/sql/schema"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	mock_codereview "go.skia.org/infra/golden/go/code_review/mocks"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

var (
	// beforeCLs is a time that is before any CL in datakitchensink
	beforeCLs = time.Date(2020, time.December, 9, 0, 0, 0, 0, time.UTC)
	// afterCLs is a time that is after all CLs in datakitchensink
	afterCLs = time.Date(2020, time.December, 13, 0, 0, 0, 0, time.UTC)
)

func TestCommentOnCLs_MultiplePatchsetsNeedComments_CommentsMade(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	gerritClient := &mock_codereview.Client{}
	gerritInternalClient := &mock_codereview.Client{}

	gerritClient.On("CommentOn", testutils.AnyContext, dks.ChangelistIDThatAttemptsToFixIOS,
		"Gold has detected about 2 untriaged digest(s) on patchset 3.\nPlease triage them at gold.skia.org/cl/gerrit/CL_fix_ios.").Return(nil)
	gerritInternalClient.On("CommentOn", testutils.AnyContext, dks.ChangelistIDThatAddsNewTests,
		"Gold has detected about 4 untriaged digest(s) on patchset 4.\nPlease triage them at gold.skia.org/cl/gerrit-internal/CL_new_tests.").Return(nil)

	c, err := New(db, []ReviewSystem{
		{ID: dks.GerritCRS, Client: gerritClient},
		{ID: dks.GerritInternalCRS, Client: gerritInternalClient},
	}, basicTemplate, instanceURL, 100)
	require.NoError(t, err)
	c.lastCheck = beforeCLs // Fake this time so both CLs appear in the time window.

	ctx = context.WithValue(ctx, now.ContextKey, afterCLs)
	err = c.CommentOnChangelistsWithUntriagedDigests(ctx)
	require.NoError(t, err)

	gerritClient.AssertExpectations(t)
	gerritInternalClient.AssertExpectations(t)

	actualPatchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}).([]schema.PatchsetRow)
	assert.Equal(t, []schema.PatchsetRow{{
		PatchsetID:    "gerrit-internal_PS_adds_new_corpus",
		System:        dks.GerritInternalCRS,
		ChangelistID:  "gerrit-internal_CL_new_tests",
		Order:         1,
		GitHash:       "eeee222222222222222222222222222222222222",
		CommentedOnCL: false,
	}, {
		PatchsetID:    "gerrit-internal_PS_adds_new_corpus_and_test",
		System:        dks.GerritInternalCRS,
		ChangelistID:  "gerrit-internal_CL_new_tests",
		Order:         4,
		GitHash:       "eeee333333333333333333333333333333333333",
		CommentedOnCL: true,
	}, {
		PatchsetID:    "gerrit_PS_fixes_ipad_but_not_iphone",
		System:        dks.GerritCRS,
		ChangelistID:  "gerrit_CL_fix_ios",
		Order:         3,
		GitHash:       "ffff111111111111111111111111111111111111",
		CommentedOnCL: true,
	}}, actualPatchsets)
	assert.Equal(t, c.lastCheck, afterCLs)
}

func TestCommentOnCLs_NoPatchsetsNeedComments_Success(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := dks.Build()
	// Pretend we commented on everything already
	for i := range existingData.Patchsets {
		existingData.Patchsets[i].CommentedOnCL = true
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	gerritClient := &mock_codereview.Client{}
	gerritInternalClient := &mock_codereview.Client{}

	c, err := New(db, []ReviewSystem{
		{ID: dks.GerritCRS, Client: gerritClient},
		{ID: dks.GerritInternalCRS, Client: gerritInternalClient},
	}, basicTemplate, instanceURL, 100)
	require.NoError(t, err)

	err = c.CommentOnChangelistsWithUntriagedDigests(context.Background())
	require.NoError(t, err)

	actualPatchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}).([]schema.PatchsetRow)
	assert.Equal(t, []schema.PatchsetRow{{
		PatchsetID:    "gerrit-internal_PS_adds_new_corpus",
		System:        dks.GerritInternalCRS,
		ChangelistID:  "gerrit-internal_CL_new_tests",
		Order:         1,
		GitHash:       "eeee222222222222222222222222222222222222",
		CommentedOnCL: true,
	}, {
		PatchsetID:    "gerrit-internal_PS_adds_new_corpus_and_test",
		System:        dks.GerritInternalCRS,
		ChangelistID:  "gerrit-internal_CL_new_tests",
		Order:         4,
		GitHash:       "eeee333333333333333333333333333333333333",
		CommentedOnCL: true,
	}, {
		PatchsetID:    "gerrit_PS_fixes_ipad_but_not_iphone",
		System:        dks.GerritCRS,
		ChangelistID:  "gerrit_CL_fix_ios",
		Order:         3,
		GitHash:       "ffff111111111111111111111111111111111111",
		CommentedOnCL: true,
	}}, actualPatchsets)
}

func TestCommentOnCLs_OnePatchsetNeedsComment_Success(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := dks.Build()
	// Pretend we commented on an earlier patchset for this CL
	for i, ps := range existingData.Patchsets {
		if ps.PatchsetID != "gerrit-internal_PS_adds_new_corpus_and_test" {
			existingData.Patchsets[i].CommentedOnCL = true
		}
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	gerritClient := &mock_codereview.Client{}
	gerritInternalClient := &mock_codereview.Client{}

	c, err := New(db, []ReviewSystem{
		{ID: dks.GerritCRS, Client: gerritClient},
		{ID: dks.GerritInternalCRS, Client: gerritInternalClient},
	}, basicTemplate, instanceURL, 100)
	require.NoError(t, err)

	err = c.CommentOnChangelistsWithUntriagedDigests(context.Background())
	require.NoError(t, err)

	actualPatchsets := sqltest.GetAllRows(ctx, t, db, "Patchsets", &schema.PatchsetRow{}).([]schema.PatchsetRow)
	assert.Equal(t, []schema.PatchsetRow{{
		PatchsetID:    "gerrit-internal_PS_adds_new_corpus",
		System:        dks.GerritInternalCRS,
		ChangelistID:  "gerrit-internal_CL_new_tests",
		Order:         1,
		GitHash:       "eeee222222222222222222222222222222222222",
		CommentedOnCL: true,
	}, {
		PatchsetID:    "gerrit-internal_PS_adds_new_corpus_and_test",
		System:        dks.GerritInternalCRS,
		ChangelistID:  "gerrit-internal_CL_new_tests",
		Order:         4,
		GitHash:       "eeee333333333333333333333333333333333333",
		CommentedOnCL: true,
	}, {
		PatchsetID:    "gerrit_PS_fixes_ipad_but_not_iphone",
		System:        dks.GerritCRS,
		ChangelistID:  "gerrit_CL_fix_ios",
		Order:         3,
		GitHash:       "ffff111111111111111111111111111111111111",
		CommentedOnCL: true,
	}}, actualPatchsets)
}

//// TestCommentOnCLsCommentError tests the case where leaving a comment fails. The whole function
//// should fail then and the DB should not be updated so we can try again later.
//func TestCommentOnCLs_CommentingResultsInError_ReturnsError(t *testing.T) {
//	unittest.SmallTest(t)
//
//	mcr := &mock_codereview.Client{}
//	defer mcr.AssertExpectations(t)
//
//	mcr.On("CommentOn", testutils.AnyContext, mock.Anything, mock.Anything).Return(errors.New("internet down"))
//
//	err := c.CommentOnChangelistsWithUntriagedDigests(context.Background())
//	assertErrorWasCanceledOrContains(t, err, "internet down")
//}
//
//// TestCommentOnCLs_CLNotFound_NoError does not return an error when a CL is not found, as this
//// can happen if a CL is made private and we don't want to erroring continuously.
//func TestCommentOnCLs_CLNotFound_NoError(t *testing.T) {
//	unittest.SmallTest(t)
//
//	mcr := &mock_codereview.Client{}
//	defer mcr.AssertExpectations(t)
//
//	mcr.On("CommentOn", testutils.AnyContext, mock.Anything, mock.Anything).Return(code_review.ErrNotFound)
//
//	err := c.CommentOnChangelistsWithUntriagedDigests(context.Background())
//	require.NoError(t, err)
//}
//
//// assertErrorWasCanceledOrContains helps with the cases where the error that is returned is
//// non-deterministic, for example, when using an errgroup. It checks that the error message matches
//// a context being canceled or contains the given submessages.
//func assertErrorWasCanceledOrContains(t *testing.T, err error, submessages ...string) {
//	require.Error(t, err)
//	e := err.Error()
//	if strings.Contains(e, "canceled") {
//		return
//	}
//	for _, m := range submessages {
//		assert.Contains(t, err.Error(), m)
//	}
//}

const (
	instanceURL   = "gold.skia.org"
	basicTemplate = `Gold has detected about {{.NumNewDigests}} new digest(s) on patchset {{.PatchsetOrder}}.
Please triage them at {{.InstanceURL}}/cl/{{.CRS}}/{{.ChangelistID}}.`
)
