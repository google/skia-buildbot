package fs_tjstore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/tjstore"
)

func TestSetGetTryJob(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	f := New(c, "buildbucket")
	ctx := context.Background()

	expectedID := "987654"
	psID := tjstore.CombinedPSID{
		CL:  "1234",
		CRS: "github",
		PS:  "abcd",
	}

	// Should not exist initially
	_, err := f.GetTryJob(ctx, expectedID)
	assert.Error(t, err)
	assert.Equal(t, tjstore.ErrNotFound, err)

	tj := ci.TryJob{
		SystemID:    expectedID,
		DisplayName: "My-Test",
		Updated:     time.Date(2019, time.August, 13, 12, 11, 10, 0, time.UTC),
	}

	err = f.PutTryJob(ctx, psID, tj)
	assert.NoError(t, err)

	actual, err := f.GetTryJob(ctx, expectedID)
	assert.NoError(t, err)
	assert.Equal(t, tj, actual)
}
