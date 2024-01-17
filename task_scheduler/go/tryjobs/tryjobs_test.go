package tryjobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	buildbucket_api "go.chromium.org/luci/common/api/buildbucket/buildbucket/v1"
	"go.skia.org/infra/go/buildbucket/mocks"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/mockhttpclient"
	pubsub_mocks "go.skia.org/infra/go/pubsub/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/job_creation/buildbucket_taskbackend"
	"go.skia.org/infra/task_scheduler/go/types"
	"google.golang.org/protobuf/proto"
)

var distantFutureTime = time.Date(3000, time.January, 1, 0, 0, 0, 0, time.UTC)

func assertActiveTryJob(t *testing.T, trybots *TryJobIntegrator, j *types.Job) {
	active, err := trybots.getActiveTryJobs(context.Background())
	require.NoError(t, err)
	expect := []*types.Job{}
	if j != nil {
		expect = append(expect, j)
	}
	assertdeep.Equal(t, expect, active)
}

func assertNoActiveTryJobs(t *testing.T, trybots *TryJobIntegrator) {
	assertActiveTryJob(t, trybots, nil)
}

// Verify that updateJobs sends heartbeats for unfinished try Jobs and
// success/failure for finished Jobs.
func TestUpdateJob_NoJobs_NoAction(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	assertNoActiveTryJobs(t, trybots)
	require.NoError(t, trybots.updateJobs(ctx))
	require.True(t, mock.Empty(), mock.List())
}

func TestUpdateJobsV1_OneUnfinished_SendsHeartbeat(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	j1 := tryjobV1(ctx, repoUrl)
	MockHeartbeats(t, mock, ts, []*types.Job{j1}, nil)
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	require.NoError(t, trybots.updateJobs(ctx))
	require.True(t, mock.Empty(), mock.List())
	assertActiveTryJob(t, trybots, j1)
}

func TestUpdateJobsV2_OneUnfinished_SendsPubSub(t *testing.T) {
	ctx, trybots, _, _, topic := setup(t)

	// Create the Job.
	j1 := tryjobV2(ctx, repoUrl)
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})

	// Mock the pubsub message.
	update := &buildbucketpb.BuildTaskUpdate{
		BuildId: strconv.FormatInt(j1.BuildbucketBuildId, 10),
		Task:    buildbucket_taskbackend.JobToBuildbucketTask(ctx, j1, trybots.buildbucketTarget, trybots.host),
	}
	b, err := proto.Marshal(update)
	require.NoError(t, err)
	result := &pubsub_mocks.PublishResult{}
	result.On("Get", testutils.AnyContext).Return("fake-server-id", nil)
	topic.On("Publish", testutils.AnyContext, &pubsub.Message{Data: b}).Return(result)

	// Run updateJobs, assert that we sent the message.
	require.NoError(t, trybots.updateJobs(ctx))
	assertActiveTryJob(t, trybots, j1)
	topic.AssertExpectations(t)
	result.AssertExpectations(t)
}

func TestUpdateJobsV1_FinishedJob_SendSuccess(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	j1 := tryjobV1(ctx, repoUrl)
	j1.Status = types.JOB_STATUS_SUCCESS
	j1.Finished = ts
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	require.NotEmpty(t, j1.BuildbucketLeaseKey)
	MockJobSuccess(mock, j1, ts, false)
	require.NoError(t, trybots.updateJobs(ctx))
	require.True(t, mock.Empty(), mock.List())
	assertNoActiveTryJobs(t, trybots)
	j1, err := trybots.db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	require.Empty(t, j1.BuildbucketLeaseKey)
	require.Empty(t, j1.BuildbucketToken)
}

func TestUpdateJobsV2_FinishedJob_SendSuccess(t *testing.T) {
	ctx, trybots, _, mockBB, _ := setup(t)

	j1 := tryjobV2(ctx, repoUrl)
	j1.Status = types.JOB_STATUS_SUCCESS
	j1.Finished = ts
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	require.NotEmpty(t, j1.BuildbucketToken)
	mockBB.On("UpdateBuild", testutils.AnyContext, &buildbucketpb.Build{
		Id: j1.BuildbucketBuildId,
		Output: &buildbucketpb.Build_Output{
			Status: buildbucketpb.Status_SUCCESS,
		},
	}, j1.BuildbucketToken).Return(nil)
	require.NoError(t, trybots.updateJobs(ctx))
	mockBB.AssertExpectations(t)
	assertNoActiveTryJobs(t, trybots)
	j1, err := trybots.db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	require.Empty(t, j1.BuildbucketLeaseKey)
	require.Empty(t, j1.BuildbucketToken)
}

func TestUpdateJobsV1_FailedJob_SendFailure(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	j1 := tryjobV1(ctx, repoUrl)
	j1.Status = types.JOB_STATUS_FAILURE
	j1.Finished = ts
	j1.BuildbucketLeaseKey = 12345
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	require.NotEmpty(t, j1.BuildbucketLeaseKey)
	MockJobFailure(mock, j1, ts)
	require.NoError(t, trybots.updateJobs(ctx))
	require.True(t, mock.Empty(), mock.List())
	assertNoActiveTryJobs(t, trybots)
	j1, err := trybots.db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	require.Empty(t, j1.BuildbucketLeaseKey)
	require.Empty(t, j1.BuildbucketToken)
}

func TestUpdateJobsV2_FailedJob_SendFailure(t *testing.T) {
	ctx, trybots, _, mockBB, _ := setup(t)

	j1 := tryjobV2(ctx, repoUrl)
	j1.Status = types.JOB_STATUS_FAILURE
	j1.Finished = ts
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	require.NotEmpty(t, j1.BuildbucketToken)
	mockBB.On("UpdateBuild", testutils.AnyContext, &buildbucketpb.Build{
		Id: j1.BuildbucketBuildId,
		Output: &buildbucketpb.Build_Output{
			Status: buildbucketpb.Status_FAILURE,
		},
	}, j1.BuildbucketToken).Return(nil)
	require.NoError(t, trybots.updateJobs(ctx))
	mockBB.AssertExpectations(t)
	assertNoActiveTryJobs(t, trybots)
	j1, err := trybots.db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	require.Empty(t, j1.BuildbucketLeaseKey)
	require.Empty(t, j1.BuildbucketToken)
}

func TestUpdateJobsV2_CancelJob_CallCancelBuilds(t *testing.T) {
	ctx, trybots, _, mockBB, _ := setup(t)

	j1 := tryjobV2(ctx, repoUrl)
	j1.Status = types.JOB_STATUS_CANCELED
	j1.StatusDetails = "job is canceled"
	j1.Finished = ts
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	require.NotEmpty(t, j1.BuildbucketToken)
	mockBB.On("CancelBuilds", testutils.AnyContext, []int64{j1.BuildbucketBuildId}, j1.StatusDetails).Return(nil, nil)
	require.NoError(t, trybots.updateJobs(ctx))
	mockBB.AssertExpectations(t)
	assertNoActiveTryJobs(t, trybots)
	j1, err := trybots.db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	require.Empty(t, j1.BuildbucketLeaseKey)
	require.Empty(t, j1.BuildbucketToken)
}

func TestUpdateJobsV1_ManyInProgress_MultipleHeartbeatBatches(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	jobs := []*types.Job{}
	for i := 0; i < LEASE_BATCH_SIZE+2; i++ {
		jobs = append(jobs, tryjobV1(ctx, repoUrl))
	}
	sort.Sort(heartbeatJobSlice(jobs))
	MockHeartbeats(t, mock, ts, jobs[:LEASE_BATCH_SIZE], nil)
	MockHeartbeats(t, mock, ts, jobs[LEASE_BATCH_SIZE:], nil)
	require.NoError(t, trybots.db.PutJobs(ctx, jobs))
	trybots.jCache.AddJobs(jobs)
	require.NoError(t, trybots.updateJobs(ctx))
	require.True(t, mock.Empty(), mock.List())
}

func TestUpdateJobsV2_ManyInProgress_MultiplePubSubMessages(t *testing.T) {
	ctx, trybots, _, _, topic := setup(t)

	// Create the Jobs.
	var jobs []*types.Job
	for i := 0; i < 27; i++ { // Arbitrary number of jobs.
		job := tryjobV2(ctx, repoUrl)
		job.BuildbucketBuildId = int64(i) // Easier to debug.
		jobs = append(jobs, job)
	}
	require.NoError(t, trybots.db.PutJobs(ctx, jobs))
	trybots.jCache.AddJobs(jobs)

	// Mock the pubsub messages.
	allMocks := []*mock.Mock{&topic.Mock}
	for _, job := range jobs {
		update := &buildbucketpb.BuildTaskUpdate{
			BuildId: strconv.FormatInt(job.BuildbucketBuildId, 10),
			Task:    buildbucket_taskbackend.JobToBuildbucketTask(ctx, job, trybots.buildbucketTarget, trybots.host),
		}
		b, err := proto.Marshal(update)
		require.NoError(t, err)
		result := &pubsub_mocks.PublishResult{}
		result.On("Get", testutils.AnyContext).Return("fake-server-id", nil)
		topic.On("Publish", testutils.AnyContext, &pubsub.Message{Data: b}).Return(result)
		allMocks = append(allMocks, &result.Mock)
	}

	// Call updateJobs, assert that we sent the messages.
	require.NoError(t, trybots.updateJobs(ctx))
	for _, mock := range allMocks {
		mock.AssertExpectations(t)
	}
}

func TestUpdateJobsV1_HeartbeatBatchOneFailed_JobIsCanceled(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	jobs := []*types.Job{}
	for i := 0; i < LEASE_BATCH_SIZE+2; i++ {
		jobs = append(jobs, tryjobV1(ctx, repoUrl))
	}
	j1, j2 := jobs[0], jobs[1]
	for _, j := range jobs[2:] {
		j.Status = types.JOB_STATUS_SUCCESS
		j.Finished = time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	}
	require.NoError(t, trybots.db.PutJobs(ctx, jobs))
	trybots.jCache.AddJobs(jobs[2:])
	for _, j := range jobs[2:] {
		MockJobSuccess(mock, j, ts, false)
	}
	MockHeartbeats(t, mock, ts, []*types.Job{j1, j2}, map[string]*heartbeatResp{
		j1.Id: {
			BuildId: strconv.FormatInt(j1.BuildbucketBuildId, 10),
			Error: &buildbucket_api.LegacyApiErrorMessage{
				Reason:  "unknown reason",
				Message: "fail",
			},
		},
	})
	require.NoError(t, trybots.updateJobs(ctx))
	require.True(t, mock.Empty(), mock.List())
	active, err := trybots.getActiveTryJobs(ctx)
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Job{j2}, active)
	canceled, err := trybots.db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	require.True(t, canceled.Done())
	require.Equal(t, types.JOB_STATUS_CANCELED, canceled.Status)
}

func TestUpdateJobsV1_HeartbeatBatchLeaseExpired_LeaseRenewed(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	j1 := tryjobV1(ctx, repoUrl)
	jobs := []*types.Job{j1}
	require.NoError(t, trybots.db.PutJobs(ctx, jobs))
	trybots.jCache.AddJobs(jobs)
	MockHeartbeats(t, mock, ts, []*types.Job{j1}, map[string]*heartbeatResp{
		j1.Id: {
			BuildId: strconv.FormatInt(j1.BuildbucketBuildId, 10),
			Error: &buildbucket_api.LegacyApiErrorMessage{
				Reason:  BUILDBUCKET_API_ERROR_REASON_LEASE_EXPIRED,
				Message: "fail",
			},
		},
	})
	require.NoError(t, trybots.updateJobs(ctx))
	require.True(t, mock.Empty(), mock.List())
	active, err := trybots.getActiveTryJobs(ctx)
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Job{j1}, active)
	j1, err = trybots.db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	require.False(t, j1.Done())
}

func TestGetRevision(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	// Get the (only) commit from the repo.
	r, err := trybots.getRepo(repoUrl)
	require.NoError(t, err)
	c := r.Get(git.MainBranch).Hash

	// Fake response from Gerrit.
	ci := &gerrit.ChangeInfo{
		Branch: git.MainBranch,
	}
	serialized := []byte(testutils.MarshalJSON(t, ci))
	// Gerrit API prepends garbage to prevent XSS.
	serialized = append([]byte("abcd\n"), serialized...)
	url := fmt.Sprintf("%s/a/changes/%d/detail?o=ALL_REVISIONS&o=SUBMITTABLE", fakeGerritUrl, gerritIssue)
	mock.Mock(url, mockhttpclient.MockGetDialogue(serialized))

	got, err := trybots.getRevision(ctx, r, strconv.Itoa(gerritIssue))
	require.NoError(t, err)
	require.Equal(t, c, got)
}

func TestRemoteCancelV1Build_Success(t *testing.T) {
	_, trybots, mock, _, _ := setup(t)

	const id = int64(12345)
	MockCancelBuild(mock, id, "Canceling!")
	require.NoError(t, trybots.remoteCancelV1Build(id, "Canceling!"))
	require.True(t, mock.Empty(), mock.List())
}

func TestRemoteCancelV1Build_Success_LongMessageTruncated(t *testing.T) {
	_, trybots, mock, _, _ := setup(t)

	const id = int64(12345)
	MockCancelBuild(mock, id, strings.Repeat("X", maxCancelReasonLen-3)+"...")
	require.NoError(t, trybots.remoteCancelV1Build(id, strings.Repeat("X", maxCancelReasonLen+50)))
	require.True(t, mock.Empty(), mock.List())
}

func TestRemoteCancelV1Build_Failed(t *testing.T) {
	_, trybots, mock, _, _ := setup(t)

	const id = int64(12345)
	expectErr := "Build does not exist!"
	MockCancelBuildFailed(mock, id, "Canceling!", expectErr)
	require.EqualError(t, trybots.remoteCancelV1Build(id, "Canceling!"), expectErr)
	require.True(t, mock.Empty(), mock.List())
}

func TestTryLeaseV1Build_Success(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	const id = int64(12345)
	MockTryLeaseBuild(mock, id)
	k, bbError, err := trybots.tryLeaseV1Build(ctx, id)
	require.NoError(t, err)
	require.Nil(t, bbError)
	require.NotEqual(t, k, 0)
	require.True(t, mock.Empty(), mock.List())
}

func TestTryLeaseV1Build_Failure(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	const id = int64(12345)
	expect := "Can't lease this!"
	MockTryLeaseBuildFailed(mock, id, expect, "CANNOT_LEASE_BUILD")
	_, bbError, err := trybots.tryLeaseV1Build(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, bbError)
	require.Contains(t, bbError.Message, expect)
	require.True(t, mock.Empty(), mock.List())
}

func TestJobStartedV1_Success(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	j := tryjobV1(ctx, repoUrl)

	MockJobStarted(mock, j.BuildbucketBuildId)
	bbToken, bbError, err := trybots.jobStarted(ctx, j)
	require.NoError(t, err)
	require.Nil(t, bbError)
	require.Empty(t, bbToken) // No update token for V1 builds.
	require.True(t, mock.Empty(), mock.List())
}

func TestJobStartedV2_Success(t *testing.T) {
	ctx, trybots, _, mockBB, _ := setup(t)

	j := tryjobV2(ctx, repoUrl)

	mockBB.On("StartBuild", testutils.AnyContext, j.BuildbucketBuildId, j.Id, j.BuildbucketToken).Return(bbFakeUpdateToken, nil)
	bbToken, bbError, err := trybots.jobStarted(ctx, j)
	require.NoError(t, err)
	require.Nil(t, bbError)
	require.Equal(t, bbFakeUpdateToken, bbToken)
	mockBB.AssertExpectations(t)
}

func TestJobStartedV1_Failure(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	j := tryjobV1(ctx, repoUrl)

	expectErr := "fail"
	MockJobStartedFailed(mock, j.BuildbucketBuildId, expectErr, "INVALID_INPUT")
	bbToken, bbError, err := trybots.jobStarted(ctx, j)
	require.NoError(t, err)
	require.NotNil(t, bbError)
	require.Empty(t, bbToken)
	require.Contains(t, bbError.Message, expectErr)
	require.Contains(t, bbError.Reason, "INVALID_INPUT")
	require.True(t, mock.Empty(), mock.List())
}

func TestJobStartedV2_Failure(t *testing.T) {
	ctx, trybots, _, mockBB, _ := setup(t)

	j := tryjobV2(ctx, repoUrl)

	mockBB.On("StartBuild", testutils.AnyContext, j.BuildbucketBuildId, j.Id, j.BuildbucketToken).Return("", errors.New("failed"))
	bbToken, bbError, err := trybots.jobStarted(ctx, j)
	require.ErrorContains(t, err, "failed")
	require.Nil(t, bbError)
	require.Empty(t, bbToken)
	mockBB.AssertExpectations(t)
}

func TestJobFinishedV1_NotActuallyFinished(t *testing.T) {
	ctx, trybots, _, _, _ := setup(t)

	j := tryjobV1(ctx, repoUrl)

	require.ErrorContains(t, trybots.jobFinished(ctx, j), "JobFinished called for unfinished Job!")
}

func TestJobFinishedV1_JobSucceeded_UpdateSucceeds(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	j := tryjobV1(ctx, repoUrl)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_SUCCESS
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	MockJobSuccess(mock, j, now, false)
	require.NoError(t, trybots.jobFinished(ctx, j))
	require.True(t, mock.Empty(), mock.List())
}

func TestJobFinishedV2_JobSucceeded_UpdateSucceeds(t *testing.T) {
	ctx, trybots, _, mockBB, _ := setup(t)

	j := tryjobV2(ctx, repoUrl)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_SUCCESS
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	mockBB.On("UpdateBuild", testutils.AnyContext, &buildbucketpb.Build{
		Id: j.BuildbucketBuildId,
		Output: &buildbucketpb.Build_Output{
			Status: buildbucketpb.Status_SUCCESS,
		},
	}, j.BuildbucketToken).Return(nil)
	require.NoError(t, trybots.jobFinished(ctx, j))
	mockBB.AssertExpectations(t)
}

func TestJobFinishedV1_JobSucceeded_UpdateFails(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	j := tryjobV1(ctx, repoUrl)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_SUCCESS
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	expectErr := "fail"
	MockJobSuccess_Failed(mock, j, now, false, expectErr)
	require.EqualError(t, trybots.jobFinished(ctx, j), expectErr)
	require.True(t, mock.Empty(), mock.List())
}

func TestJobFinishedV2_JobSucceeded_UpdateFails(t *testing.T) {
	ctx, trybots, _, mockBB, _ := setup(t)

	j := tryjobV2(ctx, repoUrl)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_SUCCESS
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	mockBB.On("UpdateBuild", testutils.AnyContext, &buildbucketpb.Build{
		Id: j.BuildbucketBuildId,
		Output: &buildbucketpb.Build_Output{
			Status: buildbucketpb.Status_SUCCESS,
		},
	}, j.BuildbucketToken).Return(errors.New("failed"))
	require.ErrorContains(t, trybots.jobFinished(ctx, j), "failed")
	mockBB.AssertExpectations(t)
}

func TestJobFinishedV1_JobFailed_UpdateSucceeds(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	j := tryjobV1(ctx, repoUrl)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_FAILURE
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	MockJobFailure(mock, j, now)
	require.NoError(t, trybots.jobFinished(ctx, j))
	require.True(t, mock.Empty(), mock.List())
}

func TestJobFinishedV2_JobFailed_UpdateSucceeds(t *testing.T) {
	ctx, trybots, _, mockBB, _ := setup(t)

	j := tryjobV2(ctx, repoUrl)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_FAILURE
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	mockBB.On("UpdateBuild", testutils.AnyContext, &buildbucketpb.Build{
		Id: j.BuildbucketBuildId,
		Output: &buildbucketpb.Build_Output{
			Status: buildbucketpb.Status_FAILURE,
		},
	}, j.BuildbucketToken).Return(nil)
	require.NoError(t, trybots.jobFinished(ctx, j))
	mockBB.AssertExpectations(t)
}

func TestJobFinishedV1_JobFailed_UpdateFails(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	j := tryjobV1(ctx, repoUrl)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_FAILURE
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	expectErr := "fail"
	MockJobFailure_Failed(mock, j, now, expectErr)
	require.EqualError(t, trybots.jobFinished(ctx, j), expectErr)
	require.True(t, mock.Empty(), mock.List())
}

func TestJobFinishedV2_JobFailed_UpdateFails(t *testing.T) {
	ctx, trybots, _, mockBB, _ := setup(t)

	j := tryjobV2(ctx, repoUrl)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_FAILURE
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	mockBB.On("UpdateBuild", testutils.AnyContext, &buildbucketpb.Build{
		Id: j.BuildbucketBuildId,
		Output: &buildbucketpb.Build_Output{
			Status: buildbucketpb.Status_FAILURE,
		},
	}, j.BuildbucketToken).Return(errors.New("failed"))
	require.ErrorContains(t, trybots.jobFinished(ctx, j), "failed")
	mockBB.AssertExpectations(t)
}

func TestJobFinishedV1_JobMishap_UpdateSucceeds(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	j := tryjobV1(ctx, repoUrl)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_MISHAP
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	MockJobMishap(mock, j, now)
	require.NoError(t, trybots.jobFinished(ctx, j))
	require.True(t, mock.Empty(), mock.List())
}

func TestJobFinishedV2_JobMishap_UpdateSucceeds(t *testing.T) {
	ctx, trybots, _, mockBB, _ := setup(t)

	j := tryjobV2(ctx, repoUrl)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_MISHAP
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	mockBB.On("UpdateBuild", testutils.AnyContext, &buildbucketpb.Build{
		Id: j.BuildbucketBuildId,
		Output: &buildbucketpb.Build_Output{
			Status: buildbucketpb.Status_INFRA_FAILURE,
		},
	}, j.BuildbucketToken).Return(nil)
	require.NoError(t, trybots.jobFinished(ctx, j))
	mockBB.AssertExpectations(t)
}

func TestJobFinishedV1_JobMishap_UpdateFails(t *testing.T) {
	ctx, trybots, mock, _, _ := setup(t)

	j := tryjobV1(ctx, repoUrl)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_MISHAP
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	expectErr := "fail"
	MockJobMishap_Failed(mock, j, now, expectErr)
	require.EqualError(t, trybots.jobFinished(ctx, j), expectErr)
	require.True(t, mock.Empty(), mock.List())
}

func TestJobFinishedV2_JobMishap_UpdateFails(t *testing.T) {
	ctx, trybots, _, mockBB, _ := setup(t)

	j := tryjobV2(ctx, repoUrl)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_MISHAP
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	mockBB.On("UpdateBuild", testutils.AnyContext, &buildbucketpb.Build{
		Id: j.BuildbucketBuildId,
		Output: &buildbucketpb.Build_Output{
			Status: buildbucketpb.Status_INFRA_FAILURE,
		},
	}, j.BuildbucketToken).Return(errors.New("failed"))
	require.ErrorContains(t, trybots.jobFinished(ctx, j), "failed")
	mockBB.AssertExpectations(t)
}

type addedJobs map[string]*types.Job

func (aj addedJobs) getAddedJob(ctx context.Context, t *testing.T, d db.JobReader) *types.Job {
	allJobs, err := d.GetJobsFromDateRange(ctx, time.Time{}, distantFutureTime, "")
	require.NoError(t, err)
	for _, job := range allJobs {
		if _, ok := aj[job.Id]; !ok {
			aj[job.Id] = job
			return job
		}
	}
	return nil
}

func TestInsertNewJobV1_LeaseSucceeds_StatusIsRequested(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	aj := addedJobs(map[string]*types.Job{})

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)

	// Normal job, Gerrit patch.
	b1 := Build(t, now)
	mockBB.On("GetBuild", ctx, b1.Id).Return(b1, nil)
	MockTryLeaseBuild(mock, b1.Id)
	err := trybots.insertNewJobV1(ctx, b1.Id)
	require.NoError(t, err)
	require.True(t, mock.Empty(), mock.List())
	result := aj.getAddedJob(ctx, t, trybots.db)
	require.Equal(t, result.BuildbucketBuildId, b1.Id)
	require.NotEqual(t, "", result.BuildbucketLeaseKey)
	require.Equal(t, types.JOB_STATUS_REQUESTED, result.Status)
	require.True(t, result.RepoState.Patch.Full())
	require.True(t, result.RepoState.Patch.Valid())
	// Revision is expected to be unset; set it and check that everything else
	// is valid.
	require.Empty(t, result.RepoState.Revision)
	result.RepoState.Revision = "fake"
	require.True(t, result.RepoState.Valid())
}

func TestInsertNewJobV1_NoGerritChanges_BuildIsCanceled(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	aj := addedJobs(map[string]*types.Job{})

	b2 := Build(t, now)
	b2.Input.GerritChanges = nil
	MockCancelBuild(mock, b2.Id, fmt.Sprintf("Invalid Build %d: input should have exactly one GerritChanges: ", b2.Id))
	mockBB.On("GetBuild", ctx, b2.Id).Return(b2, nil)
	err := trybots.insertNewJobV1(ctx, b2.Id)
	require.NoError(t, err) // We don't report errors for bad data from buildbucket.
	result := aj.getAddedJob(ctx, t, trybots.db)
	require.Nil(t, result)
	require.True(t, mock.Empty(), mock.List())
}

func TestInsertNewJobV1_InvalidRepo_BuildIsCanceled(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	aj := addedJobs(map[string]*types.Job{})

	b3 := Build(t, now)
	b3.Input.GerritChanges[0].Project = "bogus-repo"
	MockCancelBuild(mock, b3.Id, `Unknown patch project \\\"bogus-repo\\\"`)
	mockBB.On("GetBuild", ctx, b3.Id).Return(b3, nil)
	err := trybots.insertNewJobV1(ctx, b3.Id)
	require.NoError(t, err) // We don't report errors for bad data from buildbucket.
	result := aj.getAddedJob(ctx, t, trybots.db)
	require.Nil(t, result)
	require.True(t, mock.Empty(), mock.List())
}

func TestInsertNewJobV1_LeaseFailed_BuildIsCanceled(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	aj := addedJobs(map[string]*types.Job{})

	b4 := Build(t, now)
	mockBB.On("GetBuild", ctx, b4.Id).Return(b4, nil)
	expectErr := "Can't lease this!"
	MockTryLeaseBuildFailed(mock, b4.Id, expectErr, "CANNOT_LEASE_BUILD")
	MockCancelBuild(mock, b4.Id, `Buildbucket refused lease with \\\"Can't lease this!\\\" (CANNOT_LEASE_BUILD)`)
	err := trybots.insertNewJobV1(ctx, b4.Id)
	require.NoError(t, err) // We don't report errors for bad data from buildbucket.
	result := aj.getAddedJob(ctx, t, trybots.db)
	require.Nil(t, result)
	require.True(t, mock.Empty(), mock.List())
}

func TestInsertNewJobV1_LeaseFailed_InvalidInput_BuildIsNotCanceled(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	aj := addedJobs(map[string]*types.Job{})

	b4 := Build(t, now)
	mockBB.On("GetBuild", ctx, b4.Id).Return(b4, nil)
	expectErr := "Can't lease this!"
	MockTryLeaseBuildFailed(mock, b4.Id, expectErr, BUILDBUCKET_API_ERROR_REASON_INVALID_INPUT)
	err := trybots.insertNewJobV1(ctx, b4.Id)
	require.NoError(t, err) // We don't report errors for bad data from buildbucket.
	result := aj.getAddedJob(ctx, t, trybots.db)
	require.Nil(t, result)
	require.True(t, mock.Empty(), mock.List())
}

func TestStartJobV1_NormalJob_Succeeds(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	aj := addedJobs(map[string]*types.Job{})
	b1 := Build(t, now)
	mockBB.On("GetBuild", ctx, b1.Id).Return(b1, nil)
	MockTryLeaseBuild(mock, b1.Id)
	require.NoError(t, trybots.insertNewJobV1(ctx, b1.Id))
	j1 := aj.getAddedJob(ctx, t, trybots.db)
	require.Empty(t, j1.Revision) // Revision isn't set until startJob runs.

	MockJobStarted(mock, b1.Id)
	require.NoError(t, trybots.startJob(ctx, j1))
	require.True(t, mock.Empty(), mock.List())
	require.Equal(t, j1.BuildbucketBuildId, b1.Id)
	require.NotEqual(t, "", j1.BuildbucketLeaseKey)
	require.Equal(t, commit2.Hash, j1.Revision)
}

func TestStartJobV2_NormalJob_Succeeds(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	j1 := tryjobV2(ctx, repoUrl)
	j1.Revision = "" // No revision is set initially; it's derived in startJob.
	j1.Status = types.JOB_STATUS_REQUESTED
	require.NoError(t, trybots.db.PutJob(ctx, j1))
	oldToken := j1.BuildbucketToken
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	mockBB.On("StartBuild", testutils.AnyContext, j1.BuildbucketBuildId, j1.Id, j1.BuildbucketToken).Return(bbFakeUpdateToken, nil)
	require.NoError(t, trybots.startJob(ctx, j1))
	j1, err := trybots.db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	require.Equal(t, commit2.Hash, j1.Revision)
	require.Equal(t, types.JOB_STATUS_IN_PROGRESS, j1.Status)
	// Start token is exchanged for an update token.
	require.NotEmpty(t, j1.BuildbucketToken)
	require.NotEqual(t, oldToken, j1.BuildbucketToken)
	mockBB.AssertExpectations(t)
}

func TestStartJobV1_RevisionAlreadySet_Succeeds(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	aj := addedJobs(map[string]*types.Job{})
	b1 := Build(t, now)
	mockBB.On("GetBuild", ctx, b1.Id).Return(b1, nil)
	MockTryLeaseBuild(mock, b1.Id)
	require.NoError(t, trybots.insertNewJobV1(ctx, b1.Id))
	j1 := aj.getAddedJob(ctx, t, trybots.db)
	j1.Revision = oldBranchName // We'll resolve this to the actual hash.

	MockJobStarted(mock, b1.Id)
	require.NoError(t, trybots.startJob(ctx, j1))
	require.True(t, mock.Empty(), mock.List())
	require.Equal(t, j1.BuildbucketBuildId, b1.Id)
	require.NotEqual(t, "", j1.BuildbucketLeaseKey)
	require.Equal(t, commit1.Hash, j1.Revision) // Ensure we resolved the branch
}

func TestStartJobV2_RevisionAlreadySet_Succeeds(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	j1 := tryjobV2(ctx, repoUrl)
	// Set revision to a different value; ensure we don't override.
	j1.Revision = commit1.Hash
	j1.Status = types.JOB_STATUS_REQUESTED
	require.NoError(t, trybots.db.PutJob(ctx, j1))
	oldToken := j1.BuildbucketToken
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	mockBB.On("StartBuild", testutils.AnyContext, j1.BuildbucketBuildId, j1.Id, j1.BuildbucketToken).Return(bbFakeUpdateToken, nil)
	require.NoError(t, trybots.startJob(ctx, j1))
	j1, err := trybots.db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	require.Equal(t, commit1.Hash, j1.Revision)
	require.Equal(t, types.JOB_STATUS_IN_PROGRESS, j1.Status)
	// Start token is exchanged for an update token.
	require.NotEmpty(t, j1.BuildbucketToken)
	require.NotEqual(t, oldToken, j1.BuildbucketToken)
	require.True(t, j1.Valid())
	mockBB.AssertExpectations(t)
}

func TestStartJobV1_NormalJob_Failed(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	aj := addedJobs(map[string]*types.Job{})
	b1 := Build(t, now)
	mockBB.On("GetBuild", ctx, b1.Id).Return(b1, nil)
	MockTryLeaseBuild(mock, b1.Id)
	require.NoError(t, trybots.insertNewJobV1(ctx, b1.Id))
	j1 := aj.getAddedJob(ctx, t, trybots.db)

	expectErr := "Can't start this build!"
	MockJobStartedFailed(mock, b1.Id, expectErr, "INVALID_INPUT")
	err := trybots.startJob(ctx, j1)
	require.ErrorContains(t, err, expectErr)
	require.True(t, mock.Empty(), mock.List())
	j1, err = trybots.jCache.GetJob(j1.Id)
	require.NoError(t, err)
	require.Equal(t, types.JOB_STATUS_CANCELED, j1.Status)
	require.Contains(t, j1.StatusDetails, "INVALID_INPUT")
}

func TestStartJobV2_NormalJob_Failed(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	j1 := tryjobV2(ctx, repoUrl)
	j1.Revision = "" // No revision is set initially; it's derived in startJob.
	j1.Status = types.JOB_STATUS_REQUESTED
	require.NoError(t, trybots.db.PutJob(ctx, j1))
	oldToken := j1.BuildbucketToken
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	mockBB.On("StartBuild", testutils.AnyContext, j1.BuildbucketBuildId, j1.Id, j1.BuildbucketToken).Return("", errors.New("can't start this build"))
	err := trybots.startJob(ctx, j1)
	require.ErrorContains(t, err, "can't start this build")
	require.ErrorContains(t, err, "failed to send job-started notification")
	j1, err = trybots.db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	require.Empty(t, j1.Revision)
	require.Equal(t, types.JOB_STATUS_REQUESTED, j1.Status)
	require.NotEmpty(t, j1.BuildbucketToken)
	require.Equal(t, oldToken, j1.BuildbucketToken)
	mockBB.AssertExpectations(t)
}

func TestStartJobV1_InvalidJobSpec_Failed(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	aj := addedJobs(map[string]*types.Job{})

	b2 := Build(t, now)
	b2.Builder.Builder = "bogus-job"
	mockBB.On("GetBuild", ctx, b2.Id).Return(b2, nil)
	MockTryLeaseBuild(mock, b2.Id)
	require.NoError(t, trybots.insertNewJobV1(ctx, b2.Id))
	j2 := aj.getAddedJob(ctx, t, trybots.db)
	mockBB.On("GetBuild", ctx, b2.Id).Return(b2, nil)
	err := trybots.startJob(ctx, j2)
	require.NoError(t, err) // We don't report errors for bad data from buildbucket.
	require.True(t, mock.Empty(), mock.List())
	j2, err = trybots.jCache.GetJob(j2.Id)
	require.NoError(t, err)
	require.Equal(t, types.JOB_STATUS_MISHAP, j2.Status)
	require.Contains(t, j2.StatusDetails, "Failed to start Job: no such job: bogus-job")
}

func TestStartJobV2_InvalidJobSpec_Failed(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	j1 := tryjobV2(ctx, repoUrl)
	j1.Name = "bogus-job"
	j1.Revision = "" // No revision is set initially; it's derived in startJob.
	j1.Status = types.JOB_STATUS_REQUESTED
	require.NoError(t, trybots.db.PutJob(ctx, j1))
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	require.NoError(t, trybots.startJob(ctx, j1))
	j1, err := trybots.db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	require.Equal(t, commit2.Hash, j1.Revision)
	require.Equal(t, types.JOB_STATUS_MISHAP, j1.Status)
	require.Equal(t, bbFakeStartToken, j1.BuildbucketToken)
	require.True(t, j1.Valid())
	require.Contains(t, j1.StatusDetails, "Failed to start Job: no such job: bogus-job")
	mockBB.AssertExpectations(t)
}

func mockGetChangeInfo(t *testing.T, mock *mockhttpclient.URLMock, id int, project, branch string) {
	ci := &gerrit.ChangeInfo{
		Id:      strconv.FormatInt(gerritIssue, 10),
		Project: project,
		Branch:  branch,
	}
	issueBytes, err := json.Marshal(ci)
	require.NoError(t, err)
	issueBytes = append([]byte("XSS\n"), issueBytes...)
	mock.Mock(fmt.Sprintf("%s/a%s", fakeGerritUrl, fmt.Sprintf(gerrit.URLTmplChange, ci.Id)), mockhttpclient.MockGetDialogue(issueBytes))
}

func TestRetryV1(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)

	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)

	// Insert one try job.
	aj := addedJobs(map[string]*types.Job{})
	b1 := Build(t, now)
	MockTryLeaseBuild(mock, b1.Id)
	MockJobStarted(mock, b1.Id)
	mockBB.On("GetBuild", ctx, b1.Id).Return(b1, nil)
	require.NoError(t, trybots.insertNewJobV1(ctx, b1.Id))
	j1 := aj.getAddedJob(ctx, t, trybots.db)
	require.NoError(t, trybots.startJob(ctx, j1))
	require.True(t, mock.Empty(), mock.List())
	require.Equal(t, j1.BuildbucketBuildId, b1.Id)
	require.NotEqual(t, "", j1.BuildbucketLeaseKey)
	require.True(t, j1.Valid())
	require.False(t, j1.IsForce)
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	require.NoError(t, trybots.jCache.Update(ctx))

	// Obtain a second try job, ensure that it gets IsForce = true.
	b2 := Build(t, now)
	MockTryLeaseBuild(mock, b2.Id)
	MockJobStarted(mock, b2.Id)
	mockBB.On("GetBuild", ctx, b2.Id).Return(b2, nil)
	require.NoError(t, trybots.insertNewJobV1(ctx, b2.Id))
	j2 := aj.getAddedJob(ctx, t, trybots.db)
	require.NoError(t, trybots.startJob(ctx, j2))
	require.True(t, mock.Empty(), mock.List())
	require.Equal(t, j2.BuildbucketBuildId, b2.Id)
	require.NotEqual(t, "", j2.BuildbucketLeaseKey)
	require.True(t, j2.Valid())
}

func TestRetryV2(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)

	// Insert one try job.
	j1 := tryjobV2(ctx, repoUrl)
	j1.Revision = "" // No revision is set initially; it's derived in startJob.
	j1.Status = types.JOB_STATUS_REQUESTED
	require.NoError(t, trybots.db.PutJob(ctx, j1))
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	mockBB.On("StartBuild", testutils.AnyContext, j1.BuildbucketBuildId, j1.Id, j1.BuildbucketToken).Return(bbFakeUpdateToken, nil)
	require.NoError(t, trybots.startJob(ctx, j1))
	mockBB.AssertExpectations(t)

	// Obtain a second try job, ensure that it gets IsForce = true.
	j2 := tryjobV2(ctx, repoUrl)
	j2.Revision = "" // No revision is set initially; it's derived in startJob.
	j2.Status = types.JOB_STATUS_REQUESTED
	require.NoError(t, trybots.db.PutJob(ctx, j2))
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	mockBB.On("StartBuild", testutils.AnyContext, j2.BuildbucketBuildId, j2.Id, j2.BuildbucketToken).Return(bbFakeUpdateToken, nil)
	require.NoError(t, trybots.startJob(ctx, j2))
	j2, err := trybots.db.GetJobById(ctx, j2.Id)
	require.NoError(t, err)
	require.Equal(t, commit2.Hash, j2.Revision)
	require.Equal(t, types.JOB_STATUS_IN_PROGRESS, j2.Status)
	// Start token is exchanged for an update token.
	require.NotEmpty(t, j2.BuildbucketToken)
	require.True(t, j2.IsForce)
	require.True(t, j2.Valid())
	mockBB.AssertExpectations(t)
}

func testPollAssertAdded(t *testing.T, now time.Time, trybots *TryJobIntegrator, builds []*buildbucketpb.Build) {
	jobs, err := trybots.jCache.RequestedJobs()
	require.NoError(t, err)
	byId := make(map[int64]*types.Job, len(jobs))
	for _, j := range jobs {
		// Check that the job creation time is reasonable.
		require.True(t, j.Created.Year() > 1969 && j.Created.Year() < 3000)
		byId[j.BuildbucketBuildId] = j
		j.Status = types.JOB_STATUS_SUCCESS
		j.Finished = now
	}
	for _, b := range builds {
		_, ok := byId[b.Id]
		require.True(t, ok)
	}
	require.NoError(t, trybots.db.PutJobs(context.Background(), jobs))
	trybots.jCache.AddJobs(jobs)
}

func testPollMakeBuilds(t *testing.T, now time.Time, n int) []*buildbucketpb.Build {
	builds := make([]*buildbucketpb.Build, 0, n)
	for i := 0; i < n; i++ {
		builds = append(builds, Build(t, now))
	}
	return builds
}

func testPollMockBuilds(t *testing.T, now time.Time, trybots *TryJobIntegrator, mock *mockhttpclient.URLMock, mockBB *mocks.BuildBucketInterface, builds []*buildbucketpb.Build) []*buildbucketpb.Build {
	MockPeek(mock, builds, now, "", "")
	for _, b := range builds {
		MockTryLeaseBuild(mock, b.Id)
		mockBB.On("GetBuild", context.Background(), b.Id).Return(b, nil)
	}
	return builds
}

func testPollCheck(t *testing.T, now time.Time, trybots *TryJobIntegrator, mock *mockhttpclient.URLMock, builds []*buildbucketpb.Build) {
	require.NoError(t, trybots.Poll(context.Background()))
	require.True(t, mock.Empty(), mock.List())
	testPollAssertAdded(t, now, trybots, builds)
}

func TestPoll_OneNewBuild_Success(t *testing.T) {
	_, trybots, mock, mockBB, _ := setup(t)
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)

	testPollCheck(t, now, trybots, mock, testPollMockBuilds(t, now, trybots, mock, mockBB, testPollMakeBuilds(t, now, 1)))
}

func TestPoll_MultipleNewBuilds_Success(t *testing.T) {
	_, trybots, mock, mockBB, _ := setup(t)
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)

	testPollCheck(t, now, trybots, mock, testPollMockBuilds(t, now, trybots, mock, mockBB, testPollMakeBuilds(t, now, 5)))
}

func TestPoll_MultiplePagesOfNewBuilds_Success(t *testing.T) {
	_, trybots, mock, mockBB, _ := setup(t)
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)

	builds := testPollMakeBuilds(t, now, PEEK_MAX_BUILDS+5)
	MockPeek(mock, builds[:PEEK_MAX_BUILDS], now, "", "cursor1")
	MockPeek(mock, builds[PEEK_MAX_BUILDS:], now, "cursor1", "")
	for _, b := range builds {
		MockTryLeaseBuild(mock, b.Id)
		mockBB.On("GetBuild", context.Background(), b.Id).Return(b, nil)
	}
	testPollCheck(t, now, trybots, mock, builds)
}

func TestPoll_MultipleNewBuilds_OneFailsInsert_OthersInsertSuccessfully(t *testing.T) {
	_, trybots, mock, mockBB, _ := setup(t)
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)

	builds := testPollMakeBuilds(t, now, 5)
	failIdx := 2
	failBuild := builds[failIdx]
	failBuild.Input.GerritChanges[0].Project = "bogus"
	MockPeek(mock, builds, now, "", "")
	builds = append(builds[:failIdx], builds[failIdx+1:]...)
	for _, b := range builds {
		MockTryLeaseBuild(mock, b.Id)
		mockBB.On("GetBuild", context.Background(), b.Id).Return(b, nil)
	}
	mockBB.On("GetBuild", context.Background(), failBuild.Id).Return(failBuild, nil)
	MockCancelBuild(mock, failBuild.Id, `Unknown patch project \\\"bogus\\\"`)
	testPollCheck(t, now, trybots, mock, builds)
}

func TestPoll_MultiplePagesOfNewBuilds_OnePeekFails_OthersInsertSuccessfully(t *testing.T) {
	_, trybots, mock, mockBB, _ := setup(t)
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)

	builds := testPollMakeBuilds(t, now, PEEK_MAX_BUILDS+5)
	mockErr := "Failed peek"
	MockPeek(mock, builds[:PEEK_MAX_BUILDS], now, "", "cursor1")
	MockPeekFailed(mock, builds[PEEK_MAX_BUILDS:], now, "cursor1", "", mockErr)
	builds = builds[:PEEK_MAX_BUILDS]
	for _, b := range builds {
		MockTryLeaseBuild(mock, b.Id)
		mockBB.On("GetBuild", context.Background(), b.Id).Return(b, nil)
	}
	require.ErrorContains(t, trybots.Poll(context.Background()), "got errors loading builds from Buildbucket: [Failed peek]")
	require.True(t, mock.Empty(), mock.List())
	testPollAssertAdded(t, now, trybots, builds)
}
