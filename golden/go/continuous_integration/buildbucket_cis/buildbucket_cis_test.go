package buildbucket_cis

import (
	"context"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/buildbucket/mocks"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/continuous_integration"
)

func TestGetTryJobSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mbi := &mocks.BuildBucketInterface{}
	defer mbi.AssertExpectations(t)

	c := New(mbi, "bucket")

	id := "1234"
	ts := time.Date(2019, time.August, 21, 16, 44, 26, 0, time.UTC)

	cl, err := c.GetTryJob(context.Background(), id)
	assert.NoError(t, err)
	assert.Equal(t, continuous_integration.TryJob{
		SystemID: id,
		Name:     "rosanante",
		Status:   continuous_integration.Running,
		Updated:  ts,
	}, cl)
}
