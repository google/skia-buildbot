package buildbucket_cis

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/go/buildbucket/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/continuous_integration"
)

func TestGetTryJobSunnyDay(t *testing.T) {

	mbi := &mocks.BuildBucketInterface{}
	c := New(mbi)

	const id = int64(8904420728436446512)
	ts := time.Date(2019, time.August, 22, 13, 21, 39, 0, time.UTC)

	cb := getCompletedBuild()
	mbi.On("GetBuild", testutils.AnyContext, id).Return(&cb, nil)

	tj, err := c.GetTryJob(context.Background(), strconv.FormatInt(id, 10))
	require.NoError(t, err)
	assert.Equal(t, continuous_integration.TryJob{
		SystemID:    strconv.FormatInt(id, 10),
		System:      buildbucketSystem,
		DisplayName: "Infra-PerCommit-Medium",
		Updated:     ts,
	}, tj)
}

func TestGetTryJobRunning(t *testing.T) {

	mbi := &mocks.BuildBucketInterface{}
	c := New(mbi)

	const id = int64(8904420728436446512)
	ts := time.Date(2019, time.August, 22, 14, 31, 21, 0, time.UTC)

	rb := getRunningBuild()
	mbi.On("GetBuild", testutils.AnyContext, id).Return(&rb, nil)

	tj, err := c.GetTryJob(context.Background(), strconv.FormatInt(id, 10))
	require.NoError(t, err)
	assert.Equal(t, continuous_integration.TryJob{
		SystemID:    strconv.FormatInt(id, 10),
		System:      buildbucketSystem,
		DisplayName: "linux-rel",
		Updated:     ts,
	}, tj)
}

func TestGetTryJobDoesNotExist(t *testing.T) {

	mbi := &mocks.BuildBucketInterface{}
	c := New(mbi)

	const id = int64(8904420728436446512)

	mbi.On("GetBuild", testutils.AnyContext, id).Return(nil, errors.New("rpc error: code = NotFound desc = not found"))

	_, err := c.GetTryJob(context.Background(), strconv.FormatInt(id, 10))
	require.Error(t, err)
	assert.Equal(t, continuous_integration.ErrNotFound, err)
}

func TestGetTryJobOtherError(t *testing.T) {

	mbi := &mocks.BuildBucketInterface{}
	c := New(mbi)

	const id = int64(8904420728436446512)

	mbi.On("GetBuild", testutils.AnyContext, id).Return(nil, errors.New("oops, sentient AI"))

	_, err := c.GetTryJob(context.Background(), strconv.FormatInt(id, 10))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching Tryjob")
	assert.Contains(t, err.Error(), "oops")
}

func ts(t time.Time) *timestamp.Timestamp {
	rv, err := ptypes.TimestampProto(t)
	if err != nil {
		panic(err)
	}
	return rv
}

// This code can be used to fetch real data from buildbucket
// func TestReal(t *testing.T) {
// 	bb := buildbucket.NewClient(httputils.DefaultClientConfig().Client())
// 	b, err := bb.GetBuild(context.Background(), 8904415893681430384)
// 	spew.Dump(b)
// 	fmt.Printf("err: %v\n", err)
// }

// Based on a real-world query for a Tryjob that completed
func getCompletedBuild() buildbucketpb.Build {
	return buildbucketpb.Build{
		Builder: &buildbucketpb.BuilderID{
			Project: "skia",
			Bucket:  "my.bucket",
			Builder: "Infra-PerCommit-Medium",
		},
		EndTime:    ts(time.Date(2019, time.August, 22, 13, 21, 39, 0, time.UTC)),
		CreatedBy:  "test@example.com",
		CreateTime: ts(time.Date(2019, time.August, 22, 13, 14, 31, 0, time.UTC)),
		Id:         8904420728436446512,
		Status:     buildbucketpb.Status_SUCCESS,
	}
}

// Based on a real-world query for a Tryjob that was still running
func getRunningBuild() buildbucketpb.Build {
	return buildbucketpb.Build{
		Builder: &buildbucketpb.BuilderID{
			Project: "other",
			Bucket:  "other.bucket",
			Builder: "linux-rel",
		},
		EndTime:    nil,
		CreatedBy:  "test@example.com",
		CreateTime: ts(time.Date(2019, time.August, 22, 14, 31, 21, 0, time.UTC)),
		Id:         8904415893681430384,
		Status:     buildbucketpb.Status_STARTED,
	}
}

const buildbucketSystem = "buildbucket"
