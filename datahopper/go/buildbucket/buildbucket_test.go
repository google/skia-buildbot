package buildbucket

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	bb_mocks "go.skia.org/infra/go/buildbucket/mocks"
	metrics_util "go.skia.org/infra/go/metrics2/testutils"
	"go.skia.org/infra/go/testutils"
	ts_mocks "go.skia.org/infra/task_scheduler/go/mocks"
	"go.skia.org/infra/task_scheduler/go/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	fakeBuildbucketProject = "fake-project"
	fakeBuildbucketBucket  = "fake.bucket"
)

func TestDoMetrics(t *testing.T) {
	ctx := context.Background()
	tsDb := &ts_mocks.JobDB{}
	defer tsDb.AssertExpectations(t)
	bb2 := &bb_mocks.BuildBucketInterface{}
	defer bb2.AssertExpectations(t)

	// Set up mocks for buildbucket and task scheduler DB.
	currentTime := time.Unix(1708706564, 0) // Arbitrary starting point.
	const build1Age = 10 * time.Minute
	const build2Age = 1 * time.Minute
	const build1FinishLagTime = 5 * time.Minute
	build1Created := currentTime.Add(-build1Age)
	build2Created := currentTime.Add(-build2Age)
	const job1Id = "job1"
	job1Finished := currentTime.Add(-build1FinishLagTime)

	build1 := &buildbucketpb.Build{
		Id:         1,
		CreateTime: timestamppb.New(build1Created),
		Status:     buildbucketpb.Status_STARTED,
		Infra: &buildbucketpb.BuildInfra{
			Backend: &buildbucketpb.BuildInfra_Backend{
				Task: &buildbucketpb.Task{
					Id: &buildbucketpb.TaskID{
						Id: job1Id,
					},
				},
			},
		},
	}
	build2 := &buildbucketpb.Build{
		Id:         2,
		CreateTime: timestamppb.New(build2Created),
		Status:     buildbucketpb.Status_SCHEDULED,
	}
	job1 := &types.Job{
		Id:       job1Id,
		Status:   types.JOB_STATUS_SUCCESS,
		Finished: job1Finished,
	}

	bb2.On("Search", testutils.AnyContext, &buildbucketpb.BuildPredicate{
		Builder: &buildbucketpb.BuilderID{
			Project: fakeBuildbucketProject,
			Bucket:  fakeBuildbucketBucket,
		},
		Status: buildbucketpb.Status_SCHEDULED,
	}).Return([]*buildbucketpb.Build{build2}, nil)
	bb2.On("Search", testutils.AnyContext, &buildbucketpb.BuildPredicate{
		Builder: &buildbucketpb.BuilderID{
			Project: fakeBuildbucketProject,
			Bucket:  fakeBuildbucketBucket,
		},
		Status: buildbucketpb.Status_STARTED,
	}).Return([]*buildbucketpb.Build{build1}, nil)
	tsDb.On("GetJobById", testutils.AnyContext, job1Id).Return(job1, nil)

	// Run doMetrics.
	metrics, err := doMetrics(ctx, tsDb, bb2, fakeBuildbucketProject, fakeBuildbucketBucket, currentTime)
	require.NoError(t, err)
	require.Len(t, metrics, 3)

	// Retrieve the expected metrics and assert that they have the expected
	// values.
	tags1 := map[string]string{
		"build_id":    strconv.FormatInt(build1.Id, 10),
		"build_state": build1.Status.String(),
		"job_id":      job1Id,
	}
	actualBuild1Age, err := strconv.ParseFloat(metrics_util.GetRecordedMetric(t, buildAgeMetric, tags1), 64)
	require.NoError(t, err)
	require.Equal(t, build1Age.Seconds(), actualBuild1Age)

	tags2 := map[string]string{
		"build_id":    strconv.FormatInt(build2.Id, 10),
		"build_state": build2.Status.String(),
		"job_id":      "",
	}
	actualBuild2Age, err := strconv.ParseFloat(metrics_util.GetRecordedMetric(t, buildAgeMetric, tags2), 64)
	require.NoError(t, err)
	require.Equal(t, int64(build2Age.Seconds()), int64(actualBuild2Age))

	tags3 := map[string]string{
		"build_id":    strconv.FormatInt(build1.Id, 10),
		"build_state": build1.Status.String(),
		"job_id":      job1Id,
		"job_state":   string(job1.Status),
	}
	actualBuild1FinishLag, err := strconv.ParseFloat(metrics_util.GetRecordedMetric(t, finishLagMetric, tags3), 64)
	require.NoError(t, err)
	require.Equal(t, int64(build1FinishLagTime.Seconds()), int64(actualBuild1FinishLag))
}
