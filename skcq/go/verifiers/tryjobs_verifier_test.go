package verifiers

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	bb_mocks "go.skia.org/infra/go/buildbucket/mocks"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	cr_mocks "go.skia.org/infra/skcq/go/codereview/mocks"
	"go.skia.org/infra/skcq/go/footers"
	"go.skia.org/infra/skcq/go/types"
	"go.skia.org/infra/task_scheduler/go/specs"
)

var (
	currentTime = time.Unix(1598467386, 0).UTC()
)

func TestVerify_NoTryJobs_TasksCfg(t *testing.T) {
	unittest.SmallTest(t)

	tv := &TryJobsVerifier{
		tasksCfg: nil,
	}
	vs, _, err := tv.Verify(context.Background(), nil, 0)
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)

	tasksCfg := &specs.TasksCfg{}
	tv = &TryJobsVerifier{
		tasksCfg: tasksCfg,
	}
	vs, _, err = tv.Verify(context.Background(), nil, 0)
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)

	tasksCfg = &specs.TasksCfg{
		CommitQueue: map[string]*specs.CommitQueueJobConfig{},
	}
	tv = &TryJobsVerifier{
		tasksCfg: tasksCfg,
	}
	vs, _, err = tv.Verify(context.Background(), nil, 0)
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)
}

func TestVerify_NoTryJobs_Footer(t *testing.T) {
	unittest.SmallTest(t)

	tasksCfg := &specs.TasksCfg{
		CommitQueue: map[string]*specs.CommitQueueJobConfig{
			"try_job1": {},
		},
	}
	ci := &gerrit.ChangeInfo{
		Issue: 123,
	}

	tv := &TryJobsVerifier{
		tasksCfg: tasksCfg,
		footersMap: map[string]string{
			string(footers.NoTryFooter): "true",
		},
	}
	vs, _, err := tv.Verify(context.Background(), ci, 0)
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)
}

func setupTest() (*gerrit.ChangeInfo, string, *specs.TasksCfg) {
	ci := &gerrit.ChangeInfo{
		Issue:   123,
		Project: "skia",
	}
	gerritURL := "skia-review.googlesource.com"
	tasksCfg := &specs.TasksCfg{
		CommitQueue: map[string]*specs.CommitQueueJobConfig{
			"try_job1": {},
			"try_job2": {},
		},
	}
	timeNowFunc = func() time.Time {
		return currentTime
	}
	return ci, gerritURL, tasksCfg
}

func TestVerify_AllSuccessfulTryJobs_InSamePatchSet(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID := int64(5)

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID).Once()
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID).Return([]int64{latestPatchsetID}).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuilds := []*buildbucketpb.Build{
		{
			Builder:    &buildbucketpb.BuilderID{Builder: "try_job1"},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: "try_job2"},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID, "https://"+gerritURL, map[string]string(nil)).Return(bbBuilds, nil).Once()

	// Test verify.
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	vs, _, err := tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)
}

func TestVerify_AllSuccessfulTryJobs_InEquivalentPatchSets(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID5 := int64(5)
	equivalentPatchsetID4 := int64(4)
	equivalentPatchsetID3 := int64(3)

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID5).Once()
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID5).Return([]int64{latestPatchsetID5, equivalentPatchsetID4, equivalentPatchsetID3}).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuildPS4 := &buildbucketpb.Build{
		Builder:    &buildbucketpb.BuilderID{Builder: "try_job1"},
		CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
		Status:     buildbucketpb.Status_SUCCESS,
	}
	bbBuildPS3 := &buildbucketpb.Build{
		Builder:    &buildbucketpb.BuilderID{Builder: "try_job2"},
		CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
		Status:     buildbucketpb.Status_SUCCESS,
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID5, "https://"+gerritURL, map[string]string(nil)).Return([]*buildbucketpb.Build{}, nil).Once()
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, equivalentPatchsetID4, "https://"+gerritURL, map[string]string(nil)).Return([]*buildbucketpb.Build{bbBuildPS4}, nil).Once()
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, equivalentPatchsetID3, "https://"+gerritURL, map[string]string(nil)).Return([]*buildbucketpb.Build{bbBuildPS3}, nil).Once()

	// Test verify.
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	vs, _, err := tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)
}

func TestVerify_AllSuccessfulTryJobs_RetriggerFooter(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID := int64(5)
	tryJobName1 := "try_job1"
	tryJobName2 := "try_job2"

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID).Once()
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID).Return([]int64{latestPatchsetID}).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuilds := []*buildbucketpb.Build{
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName1},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName2},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID, "https://"+gerritURL, map[string]string(nil)).Return(bbBuilds, nil).Twice()
	defaultTags := map[string]string{
		"triggered_by":    "skcq",
		"cq_experimental": "false",
	}
	botsToTags := map[string]map[string]string{
		tryJobName2: defaultTags,
		tryJobName1: defaultTags,
	}
	bb.On("ScheduleBuilds", testutils.AnyContext, mock.Anything, botsToTags, ci.Issue, latestPatchsetID, gerritURL, ci.Project, BuildBucketDefaultSkiaProject, BuildBucketDefaultSkiaBucket).Return(bbBuilds, nil).Once()

	// Test verify.
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
		footersMap: map[string]string{
			string(footers.RerunTryjobsFooter): "true",
		},
	}
	vs, _, err := tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierWaitingState, vs)
}

func TestVerify_IncludeTryjobsFooter(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID := int64(5)
	tryJobName1 := "try_job1"
	tryJobName2 := "try_job2"
	tryJobName3 := "try_job3"

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID).Times(3)
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID).Return([]int64{latestPatchsetID}).Times(3)

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuilds := []*buildbucketpb.Build{
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName1},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName2},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName3},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID, "https://"+gerritURL, map[string]string(nil)).Return(bbBuilds, nil).Times(3)

	// Test verify.

	// Specify tryJobName3 in the footers.IncludeTryjobsFooter map.
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
		footersMap: map[string]string{
			string(footers.IncludeTryjobsFooter): fmt.Sprintf("%s/%s:%s", BuildBucketDefaultSkiaProject, BuildBucketDefaultSkiaBucket, tryJobName3),
		},
	}
	vs, _, err := tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)

	// Specify tryJobName3 twice in the footers.IncludeTryjobsFooter map but
	// with a different format.
	tv = &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
		footersMap: map[string]string{
			string(footers.IncludeTryjobsFooter): fmt.Sprintf("%s/%s:%s;luci.%s.%s:%s", BuildBucketDefaultSkiaProject, BuildBucketDefaultSkiaBucket, tryJobName3, BuildBucketDefaultSkiaProject, BuildBucketDefaultSkiaBucket, tryJobName3),
		},
	}
	vs, _, err = tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)

	// Use an unsupported footers.IncludeTryjobsFooter format.
	tv.footersMap = map[string]string{
		string(footers.IncludeTryjobsFooter): fmt.Sprintf("Unsupported-format:%s", tryJobName3),
	}
	vs, _, err = tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierFailureState, vs)
}

func TestVerify_FailureTryJob_CurrentCQAttempt_WithinQuota(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID := int64(5)
	tryJobName1 := "try_job1"
	tryJobName2 := "try_job2"

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID).Once()
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID).Return([]int64{latestPatchsetID}).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuilds := []*buildbucketpb.Build{
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName1},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName2},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			// Set end time to after this CQ attempt start time.
			EndTime: &timestamppb.Timestamp{Seconds: currentTime.Unix() + 1000},
			Status:  buildbucketpb.Status_FAILURE,
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID, "https://"+gerritURL, map[string]string(nil)).Return(bbBuilds, nil).Twice()
	tagsWithRetry := map[string]string{
		"triggered_by":    "skcq",
		"cq_experimental": "false",
		RetryTagName:      strconv.FormatInt(currentTime.Unix(), 10),
	}
	botsToTags := map[string]map[string]string{
		tryJobName2: tagsWithRetry,
	}
	bb.On("ScheduleBuilds", testutils.AnyContext, mock.Anything, botsToTags, ci.Issue, latestPatchsetID, gerritURL, ci.Project, BuildBucketDefaultSkiaProject, BuildBucketDefaultSkiaBucket).Return(bbBuilds, nil).Once()

	// Test verify.
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	vs, _, err := tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierWaitingState, vs)
}

func TestVerify_FailureTwoTryJobs_CurrentCQAttempt_WithinQuota(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID := int64(5)
	tryJobName1 := "try_job1"
	tryJobName2 := "try_job2"

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID).Once()
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID).Return([]int64{latestPatchsetID}).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuilds := []*buildbucketpb.Build{
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName1},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			// Set end time to after this CQ attempt start time.
			EndTime: &timestamppb.Timestamp{Seconds: currentTime.Unix() + 1000},
			Status:  buildbucketpb.Status_FAILURE,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName2},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			// Set end time to after this CQ attempt start time.
			EndTime: &timestamppb.Timestamp{Seconds: currentTime.Unix() + 1000},
			Status:  buildbucketpb.Status_FAILURE,
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID, "https://"+gerritURL, map[string]string(nil)).Return(bbBuilds, nil).Twice()
	tagsWithRetry := map[string]string{
		"triggered_by":    "skcq",
		"cq_experimental": "false",
		RetryTagName:      strconv.FormatInt(currentTime.Unix(), 10),
	}
	botsToTags := map[string]map[string]string{
		tryJobName1: tagsWithRetry,
		tryJobName2: tagsWithRetry,
	}
	bb.On("ScheduleBuilds", testutils.AnyContext, mock.Anything, botsToTags, ci.Issue, latestPatchsetID, gerritURL, ci.Project, BuildBucketDefaultSkiaProject, BuildBucketDefaultSkiaBucket).Return(bbBuilds, nil).Once()

	// Test verify.
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	vs, _, err := tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierWaitingState, vs)
}

func TestVerify_FailureThreeTryJobs_CurrentCQAttempt_OutsideQuota(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID := int64(5)
	tryJobName1 := "try_job1"
	tryJobName2 := "try_job2"
	tryJobName3 := "try_job3"
	tasksCfg.CommitQueue[tryJobName3] = &specs.CommitQueueJobConfig{}

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID).Once()
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID).Return([]int64{latestPatchsetID}).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuilds := []*buildbucketpb.Build{
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName1},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			// Set end time to after this CQ attempt start time.
			EndTime: &timestamppb.Timestamp{Seconds: currentTime.Unix() + 1000},
			Status:  buildbucketpb.Status_FAILURE,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName2},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			// Set end time to after this CQ attempt start time.
			EndTime: &timestamppb.Timestamp{Seconds: currentTime.Unix() + 1000},
			Status:  buildbucketpb.Status_FAILURE,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName3},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			// Set end time to after this CQ attempt start time.
			EndTime: &timestamppb.Timestamp{Seconds: currentTime.Unix() + 1000},
			Status:  buildbucketpb.Status_FAILURE,
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID, "https://"+gerritURL, map[string]string(nil)).Return(bbBuilds, nil).Twice()

	// Test verify.
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	vs, _, err := tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierFailureState, vs)
}

func TestVerify_FailureTryJob_CurrentCQAttempt_OutsideQuotaWithTwoRetries(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID := int64(5)
	tryJobName1 := "try_job1"
	tryJobName2 := "try_job2"
	tryJobName3 := "try_job3"
	tasksCfg.CommitQueue[tryJobName3] = &specs.CommitQueueJobConfig{}

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID).Once()
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID).Return([]int64{latestPatchsetID}).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	retryTags := []*buildbucketpb.StringPair{
		{
			Key:   RetryTagName,
			Value: strconv.FormatInt(timeNowFunc().Unix(), 10),
		},
	}
	bbBuilds := []*buildbucketpb.Build{
		{
			Id:         int64(111),
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName1},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SCHEDULED,
			Tags:       retryTags,
		},
		{
			Id:         int64(222),
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName2},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
			Tags:       retryTags,
		},
		{
			Id:         int64(333),
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName3},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			// Set end time to after this CQ attempt start time.
			EndTime: &timestamppb.Timestamp{Seconds: currentTime.Unix() + 1000},
			Status:  buildbucketpb.Status_FAILURE,
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID, "https://"+gerritURL, map[string]string(nil)).Return(bbBuilds, nil).Twice()

	// Test verify.
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	vs, _, err := tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierFailureState, vs)
}

func TestVerify_FailureTryJob_CurrentCQAttempt_AlreadyRetried(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID := int64(5)

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID).Once()
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID).Return([]int64{latestPatchsetID}).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuilds := []*buildbucketpb.Build{
		{
			Builder:    &buildbucketpb.BuilderID{Builder: "try_job1"},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: "try_job2"},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			// Set end time to after this CQ attempt start time.
			EndTime: &timestamppb.Timestamp{Seconds: currentTime.Unix() + 1000},
			Status:  buildbucketpb.Status_FAILURE,
			Tags: []*buildbucketpb.StringPair{
				{
					Key:   RetryTagName,
					Value: strconv.FormatInt(currentTime.Unix(), 10),
				},
			},
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID, "https://"+gerritURL, map[string]string(nil)).Return(bbBuilds, nil).Once()

	// Test verify.
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	vs, _, err := tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierFailureState, vs)
}

func TestVerify_FailureTryJob_CurrentCQAttempt_RetryFromDifferentAttempt(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID := int64(5)
	tryJobName1 := "try_job1"
	tryJobName2 := "try_job2"

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID).Once()
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID).Return([]int64{latestPatchsetID}).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuilds := []*buildbucketpb.Build{
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName1},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName2},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			// Set end time to after this CQ attempt start time.
			EndTime: &timestamppb.Timestamp{Seconds: currentTime.Unix() + 1000},
			Status:  buildbucketpb.Status_FAILURE,
			Tags: []*buildbucketpb.StringPair{
				{
					Key:   RetryTagName,
					Value: strconv.FormatInt(currentTime.Unix()-100, 10), // Different attempt.
				},
			},
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID, "https://"+gerritURL, map[string]string(nil)).Return(bbBuilds, nil).Twice()
	retryTags := map[string]string{
		"triggered_by":    "skcq",
		"cq_experimental": "false",
		RetryTagName:      strconv.FormatInt(currentTime.Unix(), 10),
	}
	botsToTags := map[string]map[string]string{
		tryJobName2: retryTags,
	}
	bb.On("ScheduleBuilds", testutils.AnyContext, mock.Anything, botsToTags, ci.Issue, latestPatchsetID, gerritURL, ci.Project, BuildBucketDefaultSkiaProject, BuildBucketDefaultSkiaBucket).Return(bbBuilds, nil).Once()

	// Test verify.
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	vs, _, err := tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierWaitingState, vs)
}

func TestVerify_FailureTryJob_NotCurrentCQAttempt(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID := int64(5)
	tryJobName1 := "try_job1"
	tryJobName2 := "try_job2"

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID).Once()
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID).Return([]int64{latestPatchsetID}).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuilds := []*buildbucketpb.Build{
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName1},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName2},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			// Set end time to before this CQ attempt start time so that it is
			// retriggered.
			EndTime: &timestamppb.Timestamp{Seconds: currentTime.Unix() - 1000},
			Status:  buildbucketpb.Status_FAILURE,
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID, "https://"+gerritURL, map[string]string(nil)).Return(bbBuilds, nil).Twice()
	defaultTags := map[string]string{
		"triggered_by":    "skcq",
		"cq_experimental": "false",
	}
	botsToTags := map[string]map[string]string{
		tryJobName2: defaultTags,
	}
	bb.On("ScheduleBuilds", testutils.AnyContext, mock.Anything, botsToTags, ci.Issue, latestPatchsetID, gerritURL, ci.Project, BuildBucketDefaultSkiaProject, BuildBucketDefaultSkiaBucket).Return(bbBuilds, nil).Once()

	// Test verify.
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	vs, _, err := tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierWaitingState, vs)
}

func TestVerify_ExperimentalTryJob(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID := int64(5)
	tryJobName1 := "try_job1"
	tryJobName2 := "try_job2"
	// Set tryJobName2 as experimental and fail it. Return value of all
	// verifiers still be a success.
	tasksCfg.CommitQueue[tryJobName2] = &specs.CommitQueueJobConfig{
		Experimental: true,
	}

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID).Once()
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID).Return([]int64{latestPatchsetID}).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuilds := []*buildbucketpb.Build{
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName1},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName2},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			// Set end time to after this CQ attempt start time.
			EndTime: &timestamppb.Timestamp{Seconds: currentTime.Unix() + 1000},
			Status:  buildbucketpb.Status_FAILURE,
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID, "https://"+gerritURL, map[string]string(nil)).Return(bbBuilds, nil).Once()

	// Test verify.
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	vs, _, err := tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)
}

func TestVerify_LocationRegex(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID := int64(5)
	tryJobName1 := "try_job1"
	tryJobName2 := "try_job2"

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID).Twice()
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID).Return([]int64{latestPatchsetID}).Twice()
	cr.On("GetFileNames", testutils.AnyContext, ci).Return([]string{"dir1/dir2/file"}, nil).Twice()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuilds := []*buildbucketpb.Build{
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName1},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName2},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			// Set end time to after this CQ attempt start time.
			EndTime: &timestamppb.Timestamp{Seconds: currentTime.Unix() + 1000},
			Status:  buildbucketpb.Status_FAILURE,
			// Already retried.
			Tags: []*buildbucketpb.StringPair{
				{
					Key:   RetryTagName,
					Value: strconv.FormatInt(currentTime.Unix(), 10),
				},
			},
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID, "https://"+gerritURL, map[string]string(nil)).Return(bbBuilds, nil).Twice()

	// Test verify.

	// Set tryJobName2 with a location regex that matches cr.GetFileNames.
	tasksCfg.CommitQueue[tryJobName2] = &specs.CommitQueueJobConfig{
		LocationRegexes: []string{"dir1/.*"},
	}
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	vs, _, err := tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierFailureState, vs)

	// Set tryJobName2 with a location regex that does not matches
	// cr.GetFileNames.
	tasksCfg.CommitQueue[tryJobName2] = &specs.CommitQueueJobConfig{
		LocationRegexes: []string{"dir1/dir2/dir3/.*"},
	}
	tv.tasksCfg = tasksCfg
	vs, _, err = tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)
}

func TestVerify_RunningTryJob(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID := int64(5)

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID).Once()
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID).Return([]int64{latestPatchsetID}).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuilds := []*buildbucketpb.Build{
		{
			Builder:    &buildbucketpb.BuilderID{Builder: "try_job1"},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: "try_job2"},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_STARTED,
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID, "https://"+gerritURL, map[string]string(nil)).Return(bbBuilds, nil).Once()

	// Test verify.
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	vs, _, err := tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierWaitingState, vs)
}

func TestVerify_RunningTryJob_Experimental(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID := int64(5)
	tryJobName1 := "try_job1"
	tryJobName2 := "try_job2"
	// Set tryJobName2 as experimental with started state. Return value of all
	// verifiers will be successful.
	tasksCfg.CommitQueue[tryJobName2] = &specs.CommitQueueJobConfig{
		Experimental: true,
	}

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID).Once()
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID).Return([]int64{latestPatchsetID}).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuilds := []*buildbucketpb.Build{
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName1},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName2},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_STARTED,
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID, "https://"+gerritURL, map[string]string(nil)).Return(bbBuilds, nil).Once()

	// Test verify.
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	vs, _, err := tv.Verify(context.Background(), ci, timeNowFunc().Unix())
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)
}

func TestCleanup_NoCancelFooter(t *testing.T) {
	unittest.SmallTest(t)

	ci := &gerrit.ChangeInfo{
		Issue: 123,
	}
	cleanupPatchsetID := int64(5)

	tv := &TryJobsVerifier{
		// tasksCfg: tasksCfg,
		footersMap: map[string]string{
			string(footers.DoNotCancelTryjobsFooter): "true",
		},
	}
	tv.Cleanup(context.Background(), ci, cleanupPatchsetID)
}

func TestCleanup_CleanupMatchesCurrentPS(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	cleanupPatchsetID := int64(5)
	currentPatchsetID := int64(5)

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetIssueProperties", testutils.AnyContext, ci.Issue).Return(ci, nil).Once()
	cr.On("GetEarliestEquivalentPatchSetID", ci).Return(int64(currentPatchsetID)).Once()

	tv := &TryJobsVerifier{
		bb2:       &bb_mocks.BuildBucketInterface{},
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	tv.Cleanup(context.Background(), ci, cleanupPatchsetID)
}

func TestCleanup_NoBuildsToCancel(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	cleanupPatchsetID := int64(5)
	currentPatchsetID := int64(6)
	tryJobName1 := "try_job1"
	tryJobName2 := "try_job2"

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetIssueProperties", testutils.AnyContext, ci.Issue).Return(ci, nil).Once()
	cr.On("GetEarliestEquivalentPatchSetID", ci).Return(int64(currentPatchsetID)).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuilds := []*buildbucketpb.Build{
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName1},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName2},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_FAILURE,
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, cleanupPatchsetID, "https://"+gerritURL, map[string]string{"triggered_by": "skcq"}).Return(bbBuilds, nil).Once()

	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	tv.Cleanup(context.Background(), ci, cleanupPatchsetID)
}

func TestCleanup_WithBuildsToCancel(t *testing.T) {
	unittest.SmallTest(t)

	ci, gerritURL, tasksCfg := setupTest()
	cleanupPatchsetID := int64(5)
	currentPatchsetID := int64(6)
	tryJobName1 := "try_job1"
	tryJobID1 := int64(111)
	tryJobName2 := "try_job2"
	tryJobID2 := int64(222)

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetIssueProperties", testutils.AnyContext, ci.Issue).Return(ci, nil).Once()
	cr.On("GetEarliestEquivalentPatchSetID", ci).Return(int64(currentPatchsetID)).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuilds := []*buildbucketpb.Build{
		{
			Id:         tryJobID1,
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName1},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
		{
			Id:         tryJobID2,
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName2},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_STARTED,
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, cleanupPatchsetID, "https://"+gerritURL, map[string]string{"triggered_by": "skcq"}).Return(bbBuilds, nil).Once()
	bb.On("CancelBuilds", testutils.AnyContext, []int64{tryJobID2}, CancelBuildsMsg).Return(bbBuilds, nil).Once()

	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	tv.Cleanup(context.Background(), ci, cleanupPatchsetID)
}
