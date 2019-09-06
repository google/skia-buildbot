package fs_tjstore

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/tjstore"
)

func TestPutGetTryJob(t *testing.T) {
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

func TestGetTryJobs(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	f := New(c, "buildbucket")
	ctx := context.Background()

	psID := tjstore.CombinedPSID{
		CL:  "1234",
		CRS: "github",
		PS:  "abcd",
	}

	// Should not exist initially
	xtj, err := f.GetTryJobs(ctx, psID)
	assert.NoError(t, err)
	assert.Empty(t, xtj)

	// Put them in backwards to check the order
	for i := 4; i > 0; i-- {
		tj := ci.TryJob{
			SystemID:    "987654" + strconv.Itoa(9-i),
			DisplayName: "My-Test-" + strconv.Itoa(i),
			Updated:     time.Date(2019, time.August, 13, 12, 11, 50-i, 0, time.UTC),
		}

		err := f.PutTryJob(ctx, psID, tj)
		assert.NoError(t, err)
	}

	tj := ci.TryJob{
		SystemID:    "ignoreme",
		DisplayName: "Perf-Ignore",
		Updated:     time.Date(2019, time.August, 13, 12, 12, 7, 0, time.UTC),
	}
	otherPSID := tjstore.CombinedPSID{
		CL:  "1234",
		CRS: "github",
		PS:  "next",
	}
	err = f.PutTryJob(ctx, otherPSID, tj)
	assert.NoError(t, err)

	xtj, err = f.GetTryJobs(ctx, psID)
	assert.NoError(t, err)
	assert.Len(t, xtj, 4)

	for i, tj := range xtj {
		assert.Equal(t, "My-Test-"+strconv.Itoa(i+1), tj.DisplayName)
	}

	xtj, err = f.GetTryJobs(ctx, otherPSID)
	assert.NoError(t, err)
	assert.Len(t, xtj, 1)
	assert.Equal(t, tj, xtj[0])
}
