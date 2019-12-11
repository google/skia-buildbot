package fs_clstore

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
)

// allCLs is a self-documenting way to match all CLs in a call to clstore
var allCLs *clstore.SearchOptions = nil

func TestPutGetChangeList(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	f := New(c, "gerrit")
	ctx := context.Background()

	expectedID := "987654"

	// Should not exist initially
	_, err := f.GetChangeList(ctx, expectedID)
	require.Error(t, err)
	require.Equal(t, clstore.ErrNotFound, err)

	cl := code_review.ChangeList{
		SystemID: expectedID,
		Owner:    "test@example.com",
		Status:   code_review.Abandoned,
		Subject:  "some code",
		Updated:  time.Date(2019, time.August, 13, 12, 11, 10, 0, time.UTC),
	}

	err = f.PutChangeList(ctx, cl)
	require.NoError(t, err)

	actual, err := f.GetChangeList(ctx, expectedID)
	require.NoError(t, err)
	require.Equal(t, cl, actual)
}

func TestPutGetPatchSet(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	f := New(c, "gerrit")
	ctx := context.Background()

	expectedCLID := "987654"
	expectedPSID := "abcdef012345"

	// Should not exist initially
	_, err := f.GetPatchSet(ctx, expectedCLID, expectedPSID)
	require.Error(t, err)
	require.Equal(t, clstore.ErrNotFound, err)

	ps := code_review.PatchSet{
		SystemID:     expectedPSID,
		ChangeListID: expectedCLID,
		Order:        3,
		GitHash:      "fedcba98765443321",
	}

	err = f.PutPatchSet(ctx, ps)
	require.NoError(t, err)

	actual, err := f.GetPatchSet(ctx, expectedCLID, expectedPSID)
	require.NoError(t, err)
	require.Equal(t, ps, actual)
}

func TestPutGetPatchSetByOrder(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	f := New(c, "gerrit")
	ctx := context.Background()

	expectedCLID := "987654"
	expectedPSOrder := 3
	otherPSOrder := 117

	// Should not exist initially
	_, err := f.GetPatchSetByOrder(ctx, expectedCLID, expectedPSOrder)
	require.Error(t, err)
	require.Equal(t, clstore.ErrNotFound, err)

	ps := code_review.PatchSet{
		SystemID:     "abcdef012345",
		ChangeListID: expectedCLID,
		Order:        expectedPSOrder,
		GitHash:      "fedcba98765443321",
	}

	err = f.PutPatchSet(ctx, ps)
	require.NoError(t, err)

	ps2 := code_review.PatchSet{
		SystemID:     "zyx9876",
		ChangeListID: expectedCLID,
		Order:        otherPSOrder,
		GitHash:      "notthisone",
	}

	err = f.PutPatchSet(ctx, ps2)
	require.NoError(t, err)

	actual, err := f.GetPatchSetByOrder(ctx, expectedCLID, expectedPSOrder)
	require.NoError(t, err)
	require.Equal(t, ps, actual)

	actual, err = f.GetPatchSetByOrder(ctx, expectedCLID, otherPSOrder)
	require.NoError(t, err)
	require.Equal(t, ps2, actual)
}

// TestDifferentSystems makes sure that two systems in the same
// firestore namespace don't overlap.
func TestDifferentSystems(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	gerrit := New(c, "gerrit")
	github := New(c, "github")
	ctx := context.Background()

	expectedCLID := "987654"

	gerritCL := code_review.ChangeList{
		SystemID: expectedCLID,
		Owner:    "test@example.com",
		Status:   code_review.Abandoned,
		Subject:  "some code on gerrit",
		Updated:  time.Date(2019, time.August, 13, 12, 11, 10, 0, time.UTC),
	}

	githubCL := code_review.ChangeList{
		SystemID: expectedCLID,
		Owner:    "test2@example.com",
		Status:   code_review.Open,
		Subject:  "some code on github",
		Updated:  time.Date(2019, time.August, 15, 12, 11, 10, 0, time.UTC),
	}

	// Both systems have a CL with the same ID
	err := gerrit.PutChangeList(ctx, gerritCL)
	require.NoError(t, err)
	err = github.PutChangeList(ctx, githubCL)
	require.NoError(t, err)

	actualGerrit, err := gerrit.GetChangeList(ctx, expectedCLID)
	require.NoError(t, err)
	actualGithub, err := github.GetChangeList(ctx, expectedCLID)
	require.NoError(t, err)

	require.NotEqual(t, actualGerrit, actualGithub)
	require.Equal(t, gerritCL, actualGerrit)
	require.Equal(t, githubCL, actualGithub)
}

// TestGetPatchSets stores several patchsets and then makes sure we can fetch the ones
// for a specific CL and they arrive sorted by Order, even if the PatchSets are sparse.
func TestGetPatchSets(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	f := New(c, "gerrit")
	ctx := context.Background()

	expectedID := "987654"
	sparseID := "sparse"
	// None should exist initially
	xps, err := f.GetPatchSets(ctx, expectedID)
	require.NoError(t, err)
	require.Empty(t, xps)

	// Create the ChangeList, but don't add any PatchSets yet.
	err = f.PutChangeList(ctx, code_review.ChangeList{SystemID: expectedID})
	require.NoError(t, err)

	// Still no PatchSets
	xps, err = f.GetPatchSets(ctx, expectedID)
	require.NoError(t, err)
	require.Empty(t, xps)

	for i := 0; i < 3; i++ {
		ps := code_review.PatchSet{
			SystemID:     "other_id" + strconv.Itoa(i),
			ChangeListID: "not this CL",
			GitHash:      "nope",
			Order:        i + 1,
		}
		require.NoError(t, f.PutPatchSet(ctx, ps))
	}
	// use random ids to make sure the we are truly sorting on ids
	randIDs := []string{"zkdf", "bkand", "d-sd9f9s3n", "csdfksdfn1"}
	// put them in backwards to make sure they get resorted by order
	for i := 4; i > 0; i-- {
		ps := code_review.PatchSet{
			// use an ID
			SystemID:     randIDs[i-1],
			ChangeListID: expectedID,
			GitHash:      "whatever",
			Order:        i,
		}
		require.NoError(t, f.PutPatchSet(ctx, ps))
	}

	for i := 0; i < 9; i += 3 {
		ps := code_review.PatchSet{
			SystemID:     "other_other_id" + strconv.Itoa(20-i),
			ChangeListID: sparseID,
			GitHash:      "sparse",
			Order:        i + 1,
		}
		require.NoError(t, f.PutPatchSet(ctx, ps))
	}

	// Check that sequential orders work
	xps, err = f.GetPatchSets(ctx, expectedID)
	require.NoError(t, err)
	require.Len(t, xps, 4)
	// Make sure they are in order
	for i, ps := range xps {
		require.Equal(t, i+1, ps.Order)
		require.Equal(t, expectedID, ps.ChangeListID)
		require.Equal(t, "whatever", ps.GitHash)
	}

	// Check that sparse patchsets work.
	xps, err = f.GetPatchSets(ctx, sparseID)
	require.NoError(t, err)
	require.Len(t, xps, 3)
	// Make sure they are in order
	for i, ps := range xps {
		require.Equal(t, i*3+1, ps.Order)
		require.Equal(t, sparseID, ps.ChangeListID)
		require.Equal(t, "sparse", ps.GitHash)
	}
}

func TestGetChangeLists(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	f := New(c, "gerrit")
	ctx := context.Background()

	// None to start
	cls, total, err := f.GetChangeLists(ctx, clstore.SearchOptions{
		StartIdx: 0,
		Limit:    50,
	})
	require.NoError(t, err)
	require.Len(t, cls, 0)
	require.Equal(t, 0, total)

	for i := 0; i < 40; i += 2 {
		cl := code_review.ChangeList{
			SystemID: "cl" + strconv.Itoa(i),
			Owner:    "test@example.com",
			Status:   code_review.Open,
			Subject:  "blarg",
			Updated:  time.Date(2019, time.August, 31, 14, i, i, 0, time.UTC),
		}
		require.NoError(t, f.PutChangeList(ctx, cl))
	}

	// Put in a few other ones:
	for i := 1; i < 10; i += 2 {
		cl := code_review.ChangeList{
			SystemID: "cl" + strconv.Itoa(i),
			Owner:    "test@example.com",
			Status:   code_review.Abandoned,
			Subject:  "blarg",
			Updated:  time.Date(2019, time.September, 1, 4, i, i, 0, time.UTC),
		}
		require.NoError(t, f.PutChangeList(ctx, cl))
	}

	for i := 31; i < 40; i += 2 {
		cl := code_review.ChangeList{
			SystemID: "cl" + strconv.Itoa(i),
			Owner:    "test@example.com",
			Status:   code_review.Landed,
			Subject:  "blarg",
			Updated:  time.Date(2019, time.September, 1, 2, i, i, 0, time.UTC),
		}
		require.NoError(t, f.PutChangeList(ctx, cl))
	}

	// Get all of them
	cls, total, err = f.GetChangeLists(ctx, clstore.SearchOptions{
		StartIdx: 0,
		Limit:    50,
	})
	require.NoError(t, err)
	require.Len(t, cls, 30)
	require.Equal(t, 30, total)

	// Get the first ones
	cls, total, err = f.GetChangeLists(ctx, clstore.SearchOptions{
		StartIdx: 0,
		Limit:    3,
	})
	require.NoError(t, err)
	require.Len(t, cls, 3)
	require.Equal(t, clstore.CountMany, total)
	// spot check the dates to make sure the CLs are in the right order.
	require.Equal(t, time.Date(2019, time.September, 1, 4, 9, 9, 0, time.UTC), cls[0].Updated)
	require.Equal(t, time.Date(2019, time.September, 1, 4, 7, 7, 0, time.UTC), cls[1].Updated)
	require.Equal(t, time.Date(2019, time.September, 1, 4, 5, 5, 0, time.UTC), cls[2].Updated)

	// Get some in the middle
	cls, total, err = f.GetChangeLists(ctx, clstore.SearchOptions{
		StartIdx: 5,
		Limit:    2,
	})
	require.NoError(t, err)
	require.Len(t, cls, 2)
	require.Equal(t, clstore.CountMany, total)
	require.Equal(t, time.Date(2019, time.September, 1, 2, 39, 39, 0, time.UTC), cls[0].Updated)
	require.Equal(t, time.Date(2019, time.September, 1, 2, 37, 37, 0, time.UTC), cls[1].Updated)

	// Get some at the end.
	cls, total, err = f.GetChangeLists(ctx, clstore.SearchOptions{
		StartIdx: 28,
		Limit:    10,
	})
	require.NoError(t, err)
	require.Len(t, cls, 2)
	require.Equal(t, 30, total)
	require.Equal(t, time.Date(2019, time.August, 31, 14, 2, 2, 0, time.UTC), cls[0].Updated)
	require.Equal(t, time.Date(2019, time.August, 31, 14, 0, 0, 0, time.UTC), cls[1].Updated)

	// If we query off the end, we don't know how many there are, so 0 is a fine response.
	cls, total, err = f.GetChangeLists(ctx, clstore.SearchOptions{
		StartIdx: 999,
		Limit:    3,
	})
	require.NoError(t, err)
	require.Len(t, cls, 0)
	require.Equal(t, 999, total)
}

// TestGetChangeListsOptions checks that various search options function.
func TestGetChangeListsOptions(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	f := New(c, "gerrit")
	ctx := context.Background()
	clA := code_review.ChangeList{
		SystemID: "abc",
		Owner:    "first@example.com",
		Status:   code_review.Open,
		Subject:  "Open sesame",
		Updated:  time.Date(2019, time.October, 5, 4, 3, 2, 0, time.UTC),
	}
	clB := code_review.ChangeList{
		SystemID: "def",
		Owner:    "second@example.com",
		Status:   code_review.Landed,
		Subject:  "Landed Panda",
		Updated:  time.Date(2019, time.October, 10, 4, 3, 2, 0, time.UTC),
	}
	clC := code_review.ChangeList{
		SystemID: "ghi",
		Owner:    "third@example.com",
		Status:   code_review.Open,
		Subject:  "Open again",
		Updated:  time.Date(2019, time.October, 15, 4, 3, 2, 0, time.UTC),
	}

	require.NoError(t, f.PutChangeList(ctx, clA))
	require.NoError(t, f.PutChangeList(ctx, clB))
	require.NoError(t, f.PutChangeList(ctx, clC))

	xcl, _, err := f.GetChangeLists(ctx, clstore.SearchOptions{
		Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, []code_review.ChangeList{clC, clB, clA}, xcl)

	xcl, _, err = f.GetChangeLists(ctx, clstore.SearchOptions{
		OpenCLsOnly: true,
		Limit:       10,
	})
	require.NoError(t, err)
	assert.Equal(t, []code_review.ChangeList{clC, clA}, xcl)

	xcl, _, err = f.GetChangeLists(ctx, clstore.SearchOptions{
		After: time.Date(2019, time.September, 28, 0, 0, 0, 0, time.UTC),
		Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, []code_review.ChangeList{clC, clB, clA}, xcl)

	xcl, _, err = f.GetChangeLists(ctx, clstore.SearchOptions{
		After: time.Date(2019, time.November, 28, 0, 0, 0, 0, time.UTC),
		Limit: 10,
	})
	require.NoError(t, err)
	assert.Empty(t, xcl)

	xcl, _, err = f.GetChangeLists(ctx, clstore.SearchOptions{
		After: time.Date(2019, time.October, 7, 0, 0, 0, 0, time.UTC),
		Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, []code_review.ChangeList{clC, clB}, xcl)
	xcl, _, err = f.GetChangeLists(ctx, clstore.SearchOptions{
		OpenCLsOnly: true,
		After:       time.Date(2019, time.October, 7, 0, 0, 0, 0, time.UTC),
		Limit:       10,
	})
	require.NoError(t, err)
	assert.Equal(t, []code_review.ChangeList{clC}, xcl)
}

func TestGetChangeListsNoLimit(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	f := New(c, "gerrit")
	ctx := context.Background()

	_, _, err := f.GetChangeLists(ctx, clstore.SearchOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "limit")
}
