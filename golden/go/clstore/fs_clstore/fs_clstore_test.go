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
