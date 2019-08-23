package fs_clstore

import (
	"context"
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

	acutal, err := f.GetChangeList(ctx, expectedID)
	assert.NoError(t, err)
	assert.Equal(t, cl, acutal)
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

	acutal, err := f.GetPatchSet(ctx, expectedCLID, expectedPSID)
	assert.NoError(t, err)
	assert.Equal(t, ps, acutal)
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
