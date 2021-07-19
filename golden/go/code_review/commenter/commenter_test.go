package commenter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/code_review"
	mock_codereview "go.skia.org/infra/golden/go/code_review/mocks"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
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

	gerritClient.On("GetChangelist", testutils.AnyContext, dks.ChangelistIDThatAttemptsToFixIOS).Return(
		code_review.Changelist{Status: code_review.Open}, nil)
	gerritClient.On("CommentOn", testutils.AnyContext, dks.ChangelistIDThatAttemptsToFixIOS,
		"Gold has detected about 2 new digest(s) on patchset 3.\nPlease triage them at gold.skia.org/cl/gerrit/CL_fix_ios.").Return(nil)

	gerritInternalClient.On("GetChangelist", testutils.AnyContext, dks.ChangelistIDThatAddsNewTests).Return(
		code_review.Changelist{Status: code_review.Open}, nil)
	gerritInternalClient.On("CommentOn", testutils.AnyContext, dks.ChangelistIDThatAddsNewTests,
		"Gold has detected about 4 new digest(s) on patchset 4.\nPlease triage them at gold.skia.org/cl/gerrit-internal/CL_new_tests.").Return(nil)

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
	}, {
		PatchsetID:    "gerrit_PShaslanded",
		System:        dks.GerritCRS,
		ChangelistID:  "gerrit_CLhaslanded",
		Order:         1,
		GitHash:       "aaaaa11111111111111111111111111111111111",
		CommentedOnCL: false,
	}, {
		PatchsetID:    "gerrit_PSisabandoned",
		System:        "gerrit",
		ChangelistID:  "gerrit_CLisabandoned",
		Order:         1,
		GitHash:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb55555",
		CommentedOnCL: false,
	}}, actualPatchsets)

	actualChangelists := sqltest.GetAllRows(ctx, t, db, "Changelists", &schema.ChangelistRow{}).([]schema.ChangelistRow)
	assert.Equal(t, []schema.ChangelistRow{{
		ChangelistID:     "gerrit-internal_CL_new_tests",
		System:           dks.GerritInternalCRS,
		Status:           schema.StatusOpen, // This shouldn't be modified
		OwnerEmail:       dks.UserTwo,
		Subject:          "Increase test coverage",
		LastIngestedData: time.Date(2020, time.December, 12, 9, 20, 33, 0, time.UTC),
	}, {
		ChangelistID:     "gerrit_CL_fix_ios",
		System:           dks.GerritCRS,
		Status:           schema.StatusOpen, // This shouldn't be modified
		OwnerEmail:       dks.UserOne,
		Subject:          "Fix iOS",
		LastIngestedData: time.Date(2020, time.December, 10, 4, 5, 6, 0, time.UTC),
	}, {
		ChangelistID:     "gerrit_CLhaslanded",
		System:           dks.GerritCRS,
		Status:           schema.StatusLanded,
		OwnerEmail:       dks.UserTwo,
		Subject:          "was landed",
		LastIngestedData: time.Date(2020, time.May, 5, 5, 5, 0, 0, time.UTC),
	}, {
		ChangelistID:     "gerrit_CLisabandoned",
		System:           dks.GerritCRS,
		Status:           schema.StatusAbandoned,
		OwnerEmail:       dks.UserOne,
		Subject:          "was abandoned",
		LastIngestedData: time.Date(2020, time.June, 6, 6, 6, 0, 0, time.UTC),
	}}, actualChangelists)

	assert.Equal(t, c.lastCheck, afterCLs)
}

func TestCommentOnCLs_NoPatchsetsNeedComments_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := dks.Build()
	// Pretend we commented on everything already
	for i := range existingData.Patchsets {
		existingData.Patchsets[i].CommentedOnCL = true
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	c, err := New(db, []ReviewSystem{
		{ID: dks.GerritCRS, Client: nil}, // This test doesn't talk to the clients
		{ID: dks.GerritInternalCRS, Client: nil},
	}, basicTemplate, instanceURL, 100)
	require.NoError(t, err)

	c.lastCheck = beforeCLs // Fake this time so both CLs appear in the time window.
	ctx = context.WithValue(ctx, now.ContextKey, afterCLs)

	err = c.CommentOnChangelistsWithUntriagedDigests(ctx)
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
	}, {
		PatchsetID:    "gerrit_PShaslanded",
		System:        dks.GerritCRS,
		ChangelistID:  "gerrit_CLhaslanded",
		Order:         1,
		GitHash:       "aaaaa11111111111111111111111111111111111",
		CommentedOnCL: true,
	}, {
		PatchsetID:    "gerrit_PSisabandoned",
		System:        "gerrit",
		ChangelistID:  "gerrit_CLisabandoned",
		Order:         1,
		GitHash:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb55555",
		CommentedOnCL: true,
	}}, actualPatchsets)
}

func TestCommentOnCLs_OnePatchsetNeedsComment_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := dks.Build()
	// Pretend we commented on an earlier patchset for this CL
	for i, ps := range existingData.Patchsets {
		if ps.PatchsetID != "gerrit-internal_PS_adds_new_corpus_and_test" {
			existingData.Patchsets[i].CommentedOnCL = true
		}
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	gerritInternalClient := &mock_codereview.Client{}

	gerritInternalClient.On("GetChangelist", testutils.AnyContext, dks.ChangelistIDThatAddsNewTests).Return(
		code_review.Changelist{Status: code_review.Open}, nil)
	gerritInternalClient.On("CommentOn", testutils.AnyContext, dks.ChangelistIDThatAddsNewTests,
		"Gold has detected about 4 new digest(s) on patchset 4.\nPlease triage them at gold.skia.org/cl/gerrit-internal/CL_new_tests.").Return(nil)

	c, err := New(db, []ReviewSystem{
		{ID: dks.GerritCRS, Client: nil},
		{ID: dks.GerritInternalCRS, Client: gerritInternalClient},
	}, basicTemplate, instanceURL, 100)
	require.NoError(t, err)

	c.lastCheck = beforeCLs // Fake this time so both CLs appear in the time window.
	ctx = context.WithValue(ctx, now.ContextKey, afterCLs)

	err = c.CommentOnChangelistsWithUntriagedDigests(ctx)
	require.NoError(t, err)

	gerritInternalClient.AssertExpectations(t)

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
	}, {
		PatchsetID:    "gerrit_PShaslanded",
		System:        dks.GerritCRS,
		ChangelistID:  "gerrit_CLhaslanded",
		Order:         1,
		GitHash:       "aaaaa11111111111111111111111111111111111",
		CommentedOnCL: true,
	}, {
		PatchsetID:    "gerrit_PSisabandoned",
		System:        "gerrit",
		ChangelistID:  "gerrit_CLisabandoned",
		Order:         1,
		GitHash:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb55555",
		CommentedOnCL: true,
	}}, actualPatchsets)
}

func TestCommentOnCLs_NoCLsInWindow_NothingUpdated(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	c, err := New(db, []ReviewSystem{
		{ID: dks.GerritCRS, Client: nil}, // This test doesn't talk to the clients
		{ID: dks.GerritInternalCRS, Client: nil},
	}, basicTemplate, instanceURL, 100)
	require.NoError(t, err)

	// Don't fake the time, comments should all be in the distant past

	err = c.CommentOnChangelistsWithUntriagedDigests(ctx)
	require.NoError(t, err)

	// Make sure no patchset was modified.
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
		CommentedOnCL: false,
	}, {
		PatchsetID:    "gerrit_PS_fixes_ipad_but_not_iphone",
		System:        dks.GerritCRS,
		ChangelistID:  "gerrit_CL_fix_ios",
		Order:         3,
		GitHash:       "ffff111111111111111111111111111111111111",
		CommentedOnCL: false,
	}, {
		PatchsetID:    "gerrit_PShaslanded",
		System:        dks.GerritCRS,
		ChangelistID:  "gerrit_CLhaslanded",
		Order:         1,
		GitHash:       "aaaaa11111111111111111111111111111111111",
		CommentedOnCL: false,
	}, {
		PatchsetID:    "gerrit_PSisabandoned",
		System:        "gerrit",
		ChangelistID:  "gerrit_CLisabandoned",
		Order:         1,
		GitHash:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb55555",
		CommentedOnCL: false,
	}}, actualPatchsets)
}

func TestCommentOnCLs_CLWasAbandoned_DBNotUpdated(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	gerritInternalClient := &mock_codereview.Client{}

	gerritInternalClient.On("GetChangelist", testutils.AnyContext, dks.ChangelistIDThatAddsNewTests).Return(
		code_review.Changelist{Status: code_review.Abandoned}, nil)

	c, err := New(db, []ReviewSystem{
		{ID: dks.GerritInternalCRS, Client: gerritInternalClient},
	}, basicTemplate, instanceURL, 100)
	require.NoError(t, err)

	// Only one CL should appear in the window
	ctx = context.WithValue(ctx, now.ContextKey, afterCLs)

	err = c.CommentOnChangelistsWithUntriagedDigests(ctx)
	require.NoError(t, err)

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
		CommentedOnCL: false, // should not be updated
	}, {
		PatchsetID:    "gerrit_PS_fixes_ipad_but_not_iphone",
		System:        dks.GerritCRS,
		ChangelistID:  "gerrit_CL_fix_ios",
		Order:         3,
		GitHash:       "ffff111111111111111111111111111111111111",
		CommentedOnCL: false,
	}, {
		PatchsetID:    "gerrit_PShaslanded",
		System:        dks.GerritCRS,
		ChangelistID:  "gerrit_CLhaslanded",
		Order:         1,
		GitHash:       "aaaaa11111111111111111111111111111111111",
		CommentedOnCL: false,
	}, {
		PatchsetID:    "gerrit_PSisabandoned",
		System:        "gerrit",
		ChangelistID:  "gerrit_CLisabandoned",
		Order:         1,
		GitHash:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb55555",
		CommentedOnCL: false,
	}}, actualPatchsets)

	// The changelists records should not be altered here - that will be taken care of by
	// a different background task, the same one that merges in expectations from a CL for landed
	// CLs.
	actualChangelists := sqltest.GetAllRows(ctx, t, db, "Changelists", &schema.ChangelistRow{}).([]schema.ChangelistRow)
	assert.Equal(t, []schema.ChangelistRow{{
		ChangelistID:     "gerrit-internal_CL_new_tests",
		System:           dks.GerritInternalCRS,
		Status:           schema.StatusOpen, // This shouldn't be modified
		OwnerEmail:       dks.UserTwo,
		Subject:          "Increase test coverage",
		LastIngestedData: time.Date(2020, time.December, 12, 9, 20, 33, 0, time.UTC),
	}, {
		ChangelistID:     "gerrit_CL_fix_ios",
		System:           dks.GerritCRS,
		Status:           schema.StatusOpen, // This shouldn't be modified
		OwnerEmail:       dks.UserOne,
		Subject:          "Fix iOS",
		LastIngestedData: time.Date(2020, time.December, 10, 4, 5, 6, 0, time.UTC),
	}, {
		ChangelistID:     "gerrit_CLhaslanded",
		System:           dks.GerritCRS,
		Status:           schema.StatusLanded,
		OwnerEmail:       dks.UserTwo,
		Subject:          "was landed",
		LastIngestedData: time.Date(2020, time.May, 5, 5, 5, 0, 0, time.UTC),
	}, {
		ChangelistID:     "gerrit_CLisabandoned",
		System:           dks.GerritCRS,
		Status:           schema.StatusAbandoned,
		OwnerEmail:       dks.UserOne,
		Subject:          "was abandoned",
		LastIngestedData: time.Date(2020, time.June, 6, 6, 6, 0, 0, time.UTC),
	}}, actualChangelists)
}

// This tests the case where leaving a comment fails. The whole function should not fail, the DB
// should not be updated so we can try again later. The error should be logged instead.
func TestCommentOnCLs_CommentingResultsInError_ErrorLoggedNotReturned(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	gerritInternalClient := &mock_codereview.Client{}

	gerritInternalClient.On("GetChangelist", testutils.AnyContext, dks.ChangelistIDThatAddsNewTests).Return(
		code_review.Changelist{Status: code_review.Open}, nil)
	gerritInternalClient.On("CommentOn", testutils.AnyContext, mock.Anything, mock.Anything).Return(errors.New("internet down"))

	c, err := New(db, []ReviewSystem{
		{ID: dks.GerritInternalCRS, Client: gerritInternalClient},
	}, basicTemplate, instanceURL, 100)
	require.NoError(t, err)

	// Only one CL should appear in the window
	ctx = context.WithValue(ctx, now.ContextKey, afterCLs)

	err = c.CommentOnChangelistsWithUntriagedDigests(ctx)
	require.NoError(t, err)

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
		CommentedOnCL: false, // should not be updated
	}, {
		PatchsetID:    "gerrit_PS_fixes_ipad_but_not_iphone",
		System:        dks.GerritCRS,
		ChangelistID:  "gerrit_CL_fix_ios",
		Order:         3,
		GitHash:       "ffff111111111111111111111111111111111111",
		CommentedOnCL: false,
	}, {
		PatchsetID:    "gerrit_PShaslanded",
		System:        dks.GerritCRS,
		ChangelistID:  "gerrit_CLhaslanded",
		Order:         1,
		GitHash:       "aaaaa11111111111111111111111111111111111",
		CommentedOnCL: false,
	}, {
		PatchsetID:    "gerrit_PSisabandoned",
		System:        "gerrit",
		ChangelistID:  "gerrit_CLisabandoned",
		Order:         1,
		GitHash:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb55555",
		CommentedOnCL: false,
	}}, actualPatchsets)
}

// TestCommentOnCLs_CLNotFound_NoError does not return an error when a CL is not found, as this
// can happen if a CL is made private and we don't want to erroring continuously.
func TestCommentOnCLs_CLNotFound_NoError(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))

	gerritInternalClient := &mock_codereview.Client{}

	gerritInternalClient.On("GetChangelist", testutils.AnyContext, dks.ChangelistIDThatAddsNewTests).Return(
		code_review.Changelist{}, code_review.ErrNotFound)

	c, err := New(db, []ReviewSystem{
		{ID: dks.GerritInternalCRS, Client: gerritInternalClient},
	}, basicTemplate, instanceURL, 100)
	require.NoError(t, err)

	// Only one CL should appear in the window
	ctx = context.WithValue(ctx, now.ContextKey, afterCLs)

	err = c.CommentOnChangelistsWithUntriagedDigests(ctx)
	require.NoError(t, err)

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
		CommentedOnCL: false, // should not be updated
	}, {
		PatchsetID:    "gerrit_PS_fixes_ipad_but_not_iphone",
		System:        dks.GerritCRS,
		ChangelistID:  "gerrit_CL_fix_ios",
		Order:         3,
		GitHash:       "ffff111111111111111111111111111111111111",
		CommentedOnCL: false,
	}, {
		PatchsetID:    "gerrit_PShaslanded",
		System:        dks.GerritCRS,
		ChangelistID:  "gerrit_CLhaslanded",
		Order:         1,
		GitHash:       "aaaaa11111111111111111111111111111111111",
		CommentedOnCL: false,
	}, {
		PatchsetID:    "gerrit_PSisabandoned",
		System:        "gerrit",
		ChangelistID:  "gerrit_CLisabandoned",
		Order:         1,
		GitHash:       "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb55555",
		CommentedOnCL: false,
	}}, actualPatchsets)
}

const (
	instanceURL   = "gold.skia.org"
	basicTemplate = `Gold has detected about {{.NumNewDigests}} new digest(s) on patchset {{.PatchsetOrder}}.
Please triage them at {{.InstanceURL}}/cl/{{.CRS}}/{{.ChangelistID}}.`
)
