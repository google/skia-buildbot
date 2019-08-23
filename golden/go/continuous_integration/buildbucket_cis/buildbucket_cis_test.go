package buildbucket_cis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/buildbucket/mocks"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/continuous_integration"
)

func TestGetTryJobSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mbi := &mocks.BuildBucketInterface{}
	defer mbi.AssertExpectations(t)

	c := New(mbi)

	id := "8904420728436446512"
	ts := time.Date(2019, time.August, 22, 13, 21, 39, 0, time.UTC)

	cb := getCompletedBuild()
	mbi.On("GetBuild", anyctx, id).Return(&cb, nil)

	tj, err := c.GetTryJob(context.Background(), id)
	assert.NoError(t, err)
	assert.Equal(t, continuous_integration.TryJob{
		SystemID: id,
		Status:   continuous_integration.Complete,
		Updated:  ts,
	}, tj)
}

func TestGetTryJobRunning(t *testing.T) {
	unittest.SmallTest(t)

	mbi := &mocks.BuildBucketInterface{}
	defer mbi.AssertExpectations(t)

	c := New(mbi)

	id := "8904420728436446512"
	ts := time.Date(2019, time.August, 22, 14, 31, 21, 0, time.UTC)

	rb := getRunningBuild()
	mbi.On("GetBuild", anyctx, id).Return(&rb, nil)

	tj, err := c.GetTryJob(context.Background(), id)
	assert.NoError(t, err)
	assert.Equal(t, continuous_integration.TryJob{
		SystemID: id,
		Status:   continuous_integration.Running,
		Updated:  ts,
	}, tj)
}

func TestGetTryJobDoesNotExist(t *testing.T) {
	unittest.SmallTest(t)

	mbi := &mocks.BuildBucketInterface{}
	defer mbi.AssertExpectations(t)

	c := New(mbi)

	id := "8904420728436446512"

	mbi.On("GetBuild", anyctx, id).Return(nil, errors.New("rpc error: code = NotFound desc = not found"))

	_, err := c.GetTryJob(context.Background(), id)
	assert.Error(t, err)
	assert.Equal(t, continuous_integration.ErrNotFound, err)
}

func TestGetTryJobOtherError(t *testing.T) {
	unittest.SmallTest(t)

	mbi := &mocks.BuildBucketInterface{}
	defer mbi.AssertExpectations(t)

	c := New(mbi)

	id := "8904420728436446512"

	mbi.On("GetBuild", anyctx, id).Return(nil, errors.New("oops, sentient AI"))

	_, err := c.GetTryJob(context.Background(), id)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fetching Tryjob")
	assert.Contains(t, err.Error(), "oops")
}

var (
	anyctx = mock.AnythingOfType("*context.emptyCtx")
)

// Based on a real-world query for a Tryjob that completed
func getCompletedBuild() buildbucket.Build {
	return buildbucket.Build{
		Bucket:    "my.bucket",
		Completed: time.Date(2019, time.August, 22, 13, 21, 39, 0, time.UTC),
		CreatedBy: "test@example.com",
		Created:   time.Date(2019, time.August, 22, 13, 14, 31, 0, time.UTC),
		Id:        "8904420728436446512",
		Url:       "https://cr-buildbucket.appspot.com/build/8904420728436446512",
		// Parameters omitted for brevity
		Result: "SUCCESS",
		Status: "COMPLETED",
	}
}

// Based on a real-world query for a Tryjob that was still running
func getRunningBuild() buildbucket.Build {
	return buildbucket.Build{
		Bucket:    "other.bucket",
		Completed: time.Time{},
		CreatedBy: "test@example.com",
		Created:   time.Date(2019, time.August, 22, 14, 31, 21, 0, time.UTC),
		Id:        "8904415893681430384",
		Url:       "https://cr-buildbucket.appspot.com/build/8904415893681430384",
		// Parameters omitted for brevity
		Result: "",
		Status: "STARTED",
	}
}
