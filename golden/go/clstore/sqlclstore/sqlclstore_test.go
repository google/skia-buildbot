package sqlclstore

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

func TestPutChangelist_CLDoesNotExist_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	store := New(db, "gerrit")

	const unqualifiedID = "987654"

	// Should not exist initially
	_, err := store.GetChangelist(ctx, unqualifiedID)
	require.Error(t, err)
	require.Equal(t, clstore.ErrNotFound, err)

	cl := code_review.Changelist{
		SystemID: unqualifiedID,
		Owner:    "test@example.com",
		Status:   code_review.Abandoned,
		Subject:  "some code",
		Updated:  time.Date(2019, time.August, 13, 12, 11, 10, 0, time.UTC),
	}

	err = store.PutChangelist(ctx, cl)
	require.NoError(t, err)

	actual, err := store.GetChangelist(ctx, unqualifiedID)
	require.NoError(t, err)
	assert.Equal(t, cl, actual)

	// Check the SQL directly so we can trust GetChangelist in other tests.
	row := db.QueryRow(ctx, `SELECT * FROM Changelists LIMIT 1`)
	var r schema.ChangelistRow
	require.NoError(t, row.Scan(&r.ChangelistID, &r.System, &r.Status, &r.OwnerEmail, &r.Subject, &r.LastIngestedData))
	r.LastIngestedData = r.LastIngestedData.UTC()
	assert.Equal(t, schema.ChangelistRow{
		ChangelistID:     "gerrit_987654",
		System:           "gerrit",
		Status:           schema.StatusAbandoned,
		OwnerEmail:       "test@example.com",
		Subject:          "some code",
		LastIngestedData: time.Date(2019, time.August, 13, 12, 11, 10, 0, time.UTC),
	}, r)
}

func TestPutChangelist_CLExists_CLUpdated(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	store := New(db, "gerrit")

	const unqualifiedID = "987654"
	err := store.PutChangelist(ctx, code_review.Changelist{
		SystemID: unqualifiedID,
		Owner:    "test@example.com",
		Status:   code_review.Open,
		Subject:  "some code",
		Updated:  time.Date(2019, time.August, 13, 12, 11, 10, 0, time.UTC),
	})
	require.NoError(t, err)
	err = store.PutChangelist(ctx, code_review.Changelist{
		SystemID: unqualifiedID,
		Owner:    "test@example.com",
		Status:   code_review.Landed,
		Subject:  "some code",
		Updated:  time.Date(2021, time.January, 1, 2, 3, 4, 0, time.UTC),
	})
	require.NoError(t, err)

	actual, err := store.GetChangelist(ctx, unqualifiedID)
	require.NoError(t, err)
	assert.Equal(t, code_review.Changelist{
		SystemID: unqualifiedID,
		Owner:    "test@example.com",
		Status:   code_review.Landed,
		Subject:  "some code",
		Updated:  time.Date(2021, time.January, 1, 2, 3, 4, 0, time.UTC),
	}, actual)
}

func TestGetChangelist_SameIDDifferentSystems_NoConflict(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	gerrit := New(db, "gerrit")
	github := New(db, "github")

	const conflictingID = "987654"
	gerritCL := code_review.Changelist{
		SystemID: conflictingID,
		Owner:    "test@example.com",
		Status:   code_review.Abandoned,
		Subject:  "some code on gerrit",
		Updated:  time.Date(2019, time.August, 13, 12, 11, 10, 0, time.UTC),
	}

	githubCL := code_review.Changelist{
		SystemID: conflictingID,
		Owner:    "test2@example.com",
		Status:   code_review.Open,
		Subject:  "some code on github",
		Updated:  time.Date(2019, time.August, 15, 12, 11, 10, 0, time.UTC),
	}

	// Both systems have a CL with the same ID
	err := gerrit.PutChangelist(ctx, gerritCL)
	require.NoError(t, err)
	err = github.PutChangelist(ctx, githubCL)
	require.NoError(t, err)

	actualGerrit, err := gerrit.GetChangelist(ctx, conflictingID)
	require.NoError(t, err)
	actualGithub, err := github.GetChangelist(ctx, conflictingID)
	require.NoError(t, err)

	assert.NotEqual(t, actualGerrit, actualGithub)
	assert.Equal(t, gerritCL, actualGerrit)
	assert.Equal(t, githubCL, actualGithub)
}

func TestPutPatchset_CLExists_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	store := New(db, "gerrit")

	const unqualifiedCLID = "987654"
	const unqualifiedPSID = "abcdef"

	err := store.PutChangelist(ctx, code_review.Changelist{
		SystemID: unqualifiedCLID,
		Owner:    "test@example.com",
		Status:   code_review.Open,
		Subject:  "some code",
		Updated:  time.Date(2021, time.January, 1, 2, 3, 4, 0, time.UTC),
	})
	require.NoError(t, err)

	ps := code_review.Patchset{
		SystemID:                      unqualifiedPSID,
		ChangelistID:                  unqualifiedCLID,
		Order:                         3,
		GitHash:                       "fedcba98765443321",
		CommentedOnCL:                 true,
		LastCheckedIfCommentNecessary: time.Date(2021, time.January, 1, 2, 40, 0, 0, time.UTC),
	}

	err = store.PutPatchset(ctx, ps)
	require.NoError(t, err)

	// Check the SQL directly so we can trust GetPatchset* in other tests.
	row := db.QueryRow(ctx, `SELECT * FROM Patchsets LIMIT 1`)
	var r schema.PatchsetRow
	require.NoError(t, row.Scan(&r.PatchsetID, &r.System, &r.ChangelistID, &r.Order, &r.GitHash,
		&r.CommentedOnCL, &r.LastCheckedIfCommentNecessary))
	r.LastCheckedIfCommentNecessary = r.LastCheckedIfCommentNecessary.UTC()
	assert.Equal(t, schema.PatchsetRow{
		PatchsetID:                    "gerrit_abcdef",
		System:                        "gerrit",
		ChangelistID:                  "gerrit_987654",
		Order:                         3,
		GitHash:                       "fedcba98765443321",
		CommentedOnCL:                 true,
		LastCheckedIfCommentNecessary: time.Date(2021, time.January, 1, 2, 40, 0, 0, time.UTC),
	}, r)

	actual, err := store.GetPatchset(ctx, unqualifiedCLID, unqualifiedPSID)
	require.NoError(t, err)
	assert.Equal(t, ps, actual)
	actual, err = store.GetPatchsetByOrder(ctx, unqualifiedCLID, 3)
	require.NoError(t, err)
	assert.Equal(t, ps, actual)
	actualList, err := store.GetPatchsets(ctx, unqualifiedCLID)
	require.NoError(t, err)
	assert.Equal(t, []code_review.Patchset{ps}, actualList)
}

func TestPutPatchset_CLDoesNotExists_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	store := New(db, "gerrit")

	const unqualifiedCLID = "987654"
	const unqualifiedPSID = "abcdef"

	ps := code_review.Patchset{
		SystemID:     unqualifiedPSID,
		ChangelistID: unqualifiedCLID,
		Order:        3,
		GitHash:      "fedcba98765443321",
	}

	err := store.PutPatchset(ctx, ps)
	require.Error(t, err)

	_, err = store.GetPatchset(ctx, unqualifiedCLID, unqualifiedPSID)
	require.Error(t, err)
	assert.Equal(t, err, clstore.ErrNotFound)
}

func TestGetPatchsets_PatchsetsSavedOutOfOrder_ReturnsPatchsetsInAscendingOrder(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	store := New(db, "gerrit")

	const unqualifiedCLID = "987654"
	const unqualifiedPSID4 = "fourfourfour"
	const unqualifiedPSID1 = "oneoneone"

	err := store.PutChangelist(ctx, code_review.Changelist{
		SystemID: unqualifiedCLID,
		Owner:    "test@example.com",
		Status:   code_review.Open,
		Subject:  "some code",
		Updated:  time.Date(2021, time.January, 1, 2, 3, 4, 0, time.UTC),
	})
	require.NoError(t, err)

	ps4 := code_review.Patchset{
		SystemID:     unqualifiedPSID4,
		ChangelistID: unqualifiedCLID,
		Order:        4,
		GitHash:      "444444444444444444",
	}
	err = store.PutPatchset(ctx, ps4)
	require.NoError(t, err)
	ps1 := code_review.Patchset{
		SystemID:     unqualifiedPSID1,
		ChangelistID: unqualifiedCLID,
		Order:        1,
		GitHash:      "1111111111111111111",
	}
	err = store.PutPatchset(ctx, ps1)
	require.NoError(t, err)

	actual, err := store.GetPatchsets(ctx, unqualifiedCLID)
	require.NoError(t, err)
	assert.Equal(t, []code_review.Patchset{ps1, ps4}, actual)
}

func TestGetPatchsetByOrder_PSDoesNotExist_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	store := New(db, "gerrit")

	const unqualifiedCLID = "987654"
	const unqualifiedPSID4 = "fourfourfour"

	err := store.PutChangelist(ctx, code_review.Changelist{
		SystemID: unqualifiedCLID,
		Owner:    "test@example.com",
		Status:   code_review.Open,
		Subject:  "some code",
		Updated:  time.Date(2021, time.January, 1, 2, 3, 4, 0, time.UTC),
	})
	require.NoError(t, err)

	ps4 := code_review.Patchset{
		SystemID:     unqualifiedPSID4,
		ChangelistID: unqualifiedCLID,
		Order:        4,
		GitHash:      "444444444444444444",
	}
	err = store.PutPatchset(ctx, ps4)
	require.NoError(t, err)

	_, err = store.GetPatchsetByOrder(ctx, unqualifiedCLID, 3)
	require.Error(t, err)
	assert.Equal(t, err, clstore.ErrNotFound)
}

func TestGetChangelists_StartAndLimitProvided_RespectsStartAndLimit(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, makeTestCLs()))

	store := New(db, "gerrit")
	waitForSystemTime() // GetChangelists has "AS OF SYSTEM TIME"

	// Get all of them
	cls, total, err := store.GetChangelists(ctx, clstore.SearchOptions{
		StartIdx: 0,
		Limit:    50,
	})
	require.NoError(t, err)
	assert.Len(t, cls, 30)
	assert.Equal(t, 30, total)

	// Get the first ones
	cls, total, err = store.GetChangelists(ctx, clstore.SearchOptions{
		StartIdx: 0,
		Limit:    3,
	})
	require.NoError(t, err)
	require.Len(t, cls, 3)
	assert.Equal(t, 30, total)
	// spot check the dates to make sure the CLs are in the right order. We expect to see
	// the most recent one first.
	assert.Equal(t, time.Date(2021, time.January, 15, 0, 29, 0, 0, time.UTC), cls[0].Updated)
	assert.Equal(t, time.Date(2021, time.January, 15, 0, 28, 0, 0, time.UTC), cls[1].Updated)
	assert.Equal(t, time.Date(2021, time.January, 15, 0, 27, 0, 0, time.UTC), cls[2].Updated)

	// Get some in the middle. Check that the full data is filled out.
	cls, total, err = store.GetChangelists(ctx, clstore.SearchOptions{
		StartIdx: 5,
		Limit:    2,
	})
	require.NoError(t, err)
	require.Len(t, cls, 2)
	require.Equal(t, 30, total)
	assert.Equal(t, code_review.Changelist{
		SystemID: "cl24",
		Owner:    "test@example.com",
		Status:   code_review.Open,
		Subject:  "whatever",
		Updated:  time.Date(2021, time.January, 15, 0, 24, 0, 0, time.UTC),
	}, cls[0])
	assert.Equal(t, code_review.Changelist{
		SystemID: "cl23",
		Owner:    "test@example.com",
		Status:   code_review.Abandoned,
		Subject:  "whatever",
		Updated:  time.Date(2021, time.January, 15, 0, 23, 0, 0, time.UTC),
	}, cls[1])

	// Get some at the end.
	cls, total, err = store.GetChangelists(ctx, clstore.SearchOptions{
		StartIdx: 28,
		Limit:    10,
	})
	require.NoError(t, err)
	require.Len(t, cls, 2)
	assert.Equal(t, 30, total)
	assert.Equal(t, time.Date(2021, time.January, 15, 0, 1, 0, 0, time.UTC), cls[0].Updated)
	assert.Equal(t, time.Date(2021, time.January, 15, 0, 0, 0, 0, time.UTC), cls[1].Updated)

	// Way off the end
	cls, total, err = store.GetChangelists(ctx, clstore.SearchOptions{
		StartIdx: 999,
		Limit:    3,
	})
	require.NoError(t, err)
	assert.Empty(t, cls)
	assert.Equal(t, 30, total)
}

// waitForSystemTime waits for a time greater than the duration mentioned in "AS OF SYSTEM TIME"
// clauses in queries. This way, the queries will be accurate.
func waitForSystemTime() {
	time.Sleep(150 * time.Millisecond)
}

func TestGetChangelists_InvalidStartsAndLimits_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.Background()
	store := New(nil, "gerrit")

	// Limit missing
	_, _, err := store.GetChangelists(ctx, clstore.SearchOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "limit")

	// Limit negative
	_, _, err = store.GetChangelists(ctx, clstore.SearchOptions{Limit: -10})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "limit")

	// Start index negative
	_, _, err = store.GetChangelists(ctx, clstore.SearchOptions{StartIdx: -10, Limit: 10})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start")
}

func TestGetChangelists_OptionsRespected_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, makeTestCLs()))

	store := New(db, "gerrit")
	waitForSystemTime() // GetChangelists has "AS OF SYSTEM TIME"

	// Get the ones after the 27th minute
	cls, total, err := store.GetChangelists(ctx, clstore.SearchOptions{
		Limit: 5,
		After: time.Date(2021, time.January, 15, 0, 27, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Len(t, cls, 2)
	assert.Equal(t, 30, total)
	// There are only 2 cls after the searched time.
	assert.Equal(t, time.Date(2021, time.January, 15, 0, 29, 0, 0, time.UTC), cls[0].Updated)
	assert.Equal(t, time.Date(2021, time.January, 15, 0, 28, 0, 0, time.UTC), cls[1].Updated)

	// Get the open ones
	cls, total, err = store.GetChangelists(ctx, clstore.SearchOptions{
		Limit:       2,
		OpenCLsOnly: true,
	})
	require.NoError(t, err)
	require.Len(t, cls, 2)
	assert.Equal(t, 30, total)
	// These are the first 2 open CLs.
	assert.Equal(t, code_review.Changelist{
		SystemID: "cl27",
		Owner:    "test@example.com",
		Status:   code_review.Open,
		Subject:  "whatever",
		Updated:  time.Date(2021, time.January, 15, 0, 27, 0, 0, time.UTC),
	}, cls[0])
	assert.Equal(t, code_review.Changelist{
		SystemID: "cl24",
		Owner:    "test@example.com",
		Status:   code_review.Open,
		Subject:  "whatever",
		Updated:  time.Date(2021, time.January, 15, 0, 24, 0, 0, time.UTC),
	}, cls[1])
}

// makeTestCLs returns a tables data with 30 commits with procedurally generated ids (the first
// has id "cl0", the last has id "cl29"
func makeTestCLs() schema.Tables {
	var rv schema.Tables

	for i := 0; i < 30; i++ {
		row := schema.ChangelistRow{
			ChangelistID: "cl" + strconv.Itoa(i),
			System:       "gerrit",
			OwnerEmail:   "test@example.com",
			Subject:      "whatever",
			// Note that the minute can tell us when this CL happened.
			LastIngestedData: time.Date(2021, time.January, 15, 0, i, 0, 0, time.UTC),
		}
		// Rotate through the given states
		switch i % 3 {
		case 0:
			row.Status = schema.StatusOpen
		case 1:
			row.Status = schema.StatusLanded
		case 2:
			row.Status = schema.StatusAbandoned
		}
		rv.Changelists = append(rv.Changelists, row)
	}
	return rv
}
