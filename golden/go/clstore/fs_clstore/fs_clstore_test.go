package fs_clstore

import (
	"context"
	"strconv"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
)

func TestSetGetChangeList(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	f := New(c, "gerrit")
	ctx := context.Background()

	expectedID := "987654"

	// Should not exist initially
	_, err := f.GetChangeList(ctx, expectedID)
	assert.Error(t, err)
	assert.Equal(t, clstore.ErrNotFound, err)

	cl := code_review.ChangeList{
		SystemID: expectedID,
		Owner:    "test@example.com",
		Status:   code_review.Abandoned,
		Subject:  "some code",
		Updated:  time.Date(2019, time.August, 13, 12, 11, 10, 0, time.UTC),
	}

	err = f.PutChangeList(ctx, cl)
	assert.NoError(t, err)

	actual, err := f.GetChangeList(ctx, expectedID)
	assert.NoError(t, err)
	assert.Equal(t, cl, actual)
}

func TestSetGetPatchSet(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	f := New(c, "gerrit")
	ctx := context.Background()

	expectedCLID := "987654"
	expectedPSID := "abcdef012345"

	// Should not exist initially
	_, err := f.GetPatchSet(ctx, expectedCLID, expectedPSID)
	assert.Error(t, err)
	assert.Equal(t, clstore.ErrNotFound, err)

	ps := code_review.PatchSet{
		SystemID:     expectedPSID,
		ChangeListID: expectedCLID,
		Order:        3,
		GitHash:      "fedcba98765443321",
	}

	err = f.PutPatchSet(ctx, ps)
	assert.NoError(t, err)

	actual, err := f.GetPatchSet(ctx, expectedCLID, expectedPSID)
	assert.NoError(t, err)
	assert.Equal(t, ps, actual)
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
	assert.NoError(t, err)
	err = github.PutChangeList(ctx, githubCL)
	assert.NoError(t, err)

	actualGerrit, err := gerrit.GetChangeList(ctx, expectedCLID)
	assert.NoError(t, err)
	actualGithub, err := github.GetChangeList(ctx, expectedCLID)
	assert.NoError(t, err)

	assert.NotEqual(t, actualGerrit, actualGithub)
	assert.Equal(t, gerritCL, actualGerrit)
	assert.Equal(t, githubCL, actualGithub)
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
	assert.NoError(t, err)
	assert.Empty(t, xps)

	// Create the ChangeList, but don't add any PatchSets yet.
	err = f.PutChangeList(ctx, code_review.ChangeList{SystemID: expectedID})
	assert.NoError(t, err)

	// Still no PatchSets
	xps, err = f.GetPatchSets(ctx, expectedID)
	assert.NoError(t, err)
	assert.Empty(t, xps)

	for i := 0; i < 3; i++ {
		ps := code_review.PatchSet{
			SystemID:     "other_id" + strconv.Itoa(i),
			ChangeListID: "not this CL",
			GitHash:      "nope",
			Order:        i + 1,
		}
		assert.NoError(t, f.PutPatchSet(ctx, ps))
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
		assert.NoError(t, f.PutPatchSet(ctx, ps))
	}

	for i := 0; i < 9; i += 3 {
		ps := code_review.PatchSet{
			SystemID:     "other_other_id" + strconv.Itoa(20-i),
			ChangeListID: sparseID,
			GitHash:      "sparse",
			Order:        i + 1,
		}
		assert.NoError(t, f.PutPatchSet(ctx, ps))
	}

	// Check that sequential orders work
	xps, err = f.GetPatchSets(ctx, expectedID)
	assert.NoError(t, err)
	assert.Len(t, xps, 4)
	// Make sure they are in order
	for i, ps := range xps {
		assert.Equal(t, i+1, ps.Order)
		assert.Equal(t, expectedID, ps.ChangeListID)
		assert.Equal(t, "whatever", ps.GitHash)
	}

	// Check that sparse patchsets work.
	xps, err = f.GetPatchSets(ctx, sparseID)
	assert.NoError(t, err)
	assert.Len(t, xps, 3)
	// Make sure they are in order
	for i, ps := range xps {
		assert.Equal(t, i*3+1, ps.Order)
		assert.Equal(t, sparseID, ps.ChangeListID)
		assert.Equal(t, "sparse", ps.GitHash)
	}
}
