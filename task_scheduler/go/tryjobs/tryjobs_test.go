package tryjobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
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

func TestUpdateJobsV2_OneUnfinished_SendsPubSub(t *testing.T) {
	ctx, trybots, _, _, topic := setup(t)

	// Create the Job.
	j1 := tryjobV2(ctx)
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

func TestUpdateJobsV2_FinishedJob_SendSuccess(t *testing.T) {
	ctx, trybots, _, mockBB, topic := setup(t)

	j1 := tryjobV2(ctx)
	j1.Status = types.JOB_STATUS_SUCCESS
	j1.Finished = ts
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	require.NotEmpty(t, j1.BuildbucketToken)

	// Mock the UpdateBuild call.
	mockBB.On("UpdateBuild", testutils.AnyContext, &buildbucketpb.Build{
		Id: j1.BuildbucketBuildId,
		Output: &buildbucketpb.Build_Output{
			Status: buildbucketpb.Status_SUCCESS,
		},
		Infra: &buildbucketpb.BuildInfra{
			Backend: &buildbucketpb.BuildInfra_Backend{
				Task: buildbucket_taskbackend.JobToBuildbucketTask(ctx, j1, trybots.buildbucketTarget, trybots.host),
			},
		},
	}, j1.BuildbucketToken).Return(nil)

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

	// Check the result.
	require.NoError(t, trybots.updateJobs(ctx))
	mockBB.AssertExpectations(t)
	assertNoActiveTryJobs(t, trybots)
	j1, err = trybots.db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	require.Empty(t, j1.BuildbucketLeaseKey)
	require.Empty(t, j1.BuildbucketToken)
}

func TestUpdateJobsV2_FailedJob_SendFailure(t *testing.T) {
	ctx, trybots, _, mockBB, topic := setup(t)

	j1 := tryjobV2(ctx)
	j1.Status = types.JOB_STATUS_FAILURE
	j1.Finished = ts
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	require.NotEmpty(t, j1.BuildbucketToken)

	// Mock the UpdateBuild call.
	mockBB.On("UpdateBuild", testutils.AnyContext, &buildbucketpb.Build{
		Id: j1.BuildbucketBuildId,
		Output: &buildbucketpb.Build_Output{
			Status: buildbucketpb.Status_FAILURE,
		},
		Infra: &buildbucketpb.BuildInfra{
			Backend: &buildbucketpb.BuildInfra_Backend{
				Task: buildbucket_taskbackend.JobToBuildbucketTask(ctx, j1, trybots.buildbucketTarget, trybots.host),
			},
		},
	}, j1.BuildbucketToken).Return(nil)

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

	// Check the result.
	require.NoError(t, trybots.updateJobs(ctx))
	mockBB.AssertExpectations(t)
	assertNoActiveTryJobs(t, trybots)
	j1, err = trybots.db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	require.Empty(t, j1.BuildbucketLeaseKey)
	require.Empty(t, j1.BuildbucketToken)
}

func TestUpdateJobsV2_CancelJob_CallCancelBuild(t *testing.T) {
	ctx, trybots, _, mockBB, topic := setup(t)

	j1 := tryjobV2(ctx)
	j1.Status = types.JOB_STATUS_CANCELED
	j1.StatusDetails = "job is canceled"
	j1.Finished = ts
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	require.NotEmpty(t, j1.BuildbucketToken)
	mockBB.On("CancelBuild", testutils.AnyContext, j1.BuildbucketBuildId, j1.StatusDetails).Return(nil, nil)

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

	// Check the result.
	require.NoError(t, trybots.updateJobs(ctx))
	mockBB.AssertExpectations(t)
	assertNoActiveTryJobs(t, trybots)
	j1, err = trybots.db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	require.Empty(t, j1.BuildbucketLeaseKey)
	require.Empty(t, j1.BuildbucketToken)
}

func TestUpdateJobsV2_ManyInProgress_MultiplePubSubMessages(t *testing.T) {
	ctx, trybots, _, _, topic := setup(t)

	// Create the Jobs.
	var jobs []*types.Job
	for i := 0; i < 27; i++ { // Arbitrary number of jobs.
		job := tryjobV2(ctx)
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

func TestJobStartedV2_Success(t *testing.T) {
	ctx, trybots, _, mockBB, _ := setup(t)

	j := tryjobV2(ctx)

	mockBB.On("StartBuild", testutils.AnyContext, j.BuildbucketBuildId, j.Id, j.BuildbucketToken).Return(bbFakeUpdateToken, nil)
	bbToken, err := trybots.jobStarted(ctx, j)
	require.NoError(t, err)
	require.Equal(t, bbFakeUpdateToken, bbToken)
	mockBB.AssertExpectations(t)
}

func TestJobStartedV2_Failure(t *testing.T) {
	ctx, trybots, _, mockBB, _ := setup(t)

	j := tryjobV2(ctx)

	mockBB.On("StartBuild", testutils.AnyContext, j.BuildbucketBuildId, j.Id, j.BuildbucketToken).Return("", errors.New("failed"))
	bbToken, err := trybots.jobStarted(ctx, j)
	require.ErrorContains(t, err, "failed")
	require.Empty(t, bbToken)
	mockBB.AssertExpectations(t)
}

func TestJobFinishedV2_JobSucceeded_UpdateSucceeds(t *testing.T) {
	ctx, trybots, _, mockBB, topic := setup(t)

	j := tryjobV2(ctx)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_SUCCESS
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})

	// Mock the UpdateBuild call.
	mockBB.On("UpdateBuild", testutils.AnyContext, &buildbucketpb.Build{
		Id: j.BuildbucketBuildId,
		Output: &buildbucketpb.Build_Output{
			Status: buildbucketpb.Status_SUCCESS,
		},
		Infra: &buildbucketpb.BuildInfra{
			Backend: &buildbucketpb.BuildInfra_Backend{
				Task: buildbucket_taskbackend.JobToBuildbucketTask(ctx, j, trybots.buildbucketTarget, trybots.host),
			},
		},
	}, j.BuildbucketToken).Return(nil)

	// Mock the pubsub message.
	update := &buildbucketpb.BuildTaskUpdate{
		BuildId: strconv.FormatInt(j.BuildbucketBuildId, 10),
		Task:    buildbucket_taskbackend.JobToBuildbucketTask(ctx, j, trybots.buildbucketTarget, trybots.host),
	}
	b, err := proto.Marshal(update)
	require.NoError(t, err)
	result := &pubsub_mocks.PublishResult{}
	result.On("Get", testutils.AnyContext).Return("fake-server-id", nil)
	topic.On("Publish", testutils.AnyContext, &pubsub.Message{Data: b}).Return(result)

	// Check the result.
	require.NoError(t, trybots.jobFinished(ctx, j))
	mockBB.AssertExpectations(t)
}

func TestJobFinishedV2_JobSucceeded_UpdateFails(t *testing.T) {
	ctx, trybots, _, mockBB, _ := setup(t)

	j := tryjobV2(ctx)
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
		Infra: &buildbucketpb.BuildInfra{
			Backend: &buildbucketpb.BuildInfra_Backend{
				Task: buildbucket_taskbackend.JobToBuildbucketTask(ctx, j, trybots.buildbucketTarget, trybots.host),
			},
		},
	}, j.BuildbucketToken).Return(errors.New("failed"))
	require.ErrorContains(t, trybots.jobFinished(ctx, j), "failed")
	mockBB.AssertExpectations(t)
}

func TestJobFinishedV2_JobFailed_UpdateSucceeds(t *testing.T) {
	ctx, trybots, _, mockBB, topic := setup(t)

	j := tryjobV2(ctx)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_FAILURE
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})

	// Mock the UpdateBuild call.
	mockBB.On("UpdateBuild", testutils.AnyContext, &buildbucketpb.Build{
		Id: j.BuildbucketBuildId,
		Output: &buildbucketpb.Build_Output{
			Status: buildbucketpb.Status_FAILURE,
		},
		Infra: &buildbucketpb.BuildInfra{
			Backend: &buildbucketpb.BuildInfra_Backend{
				Task: buildbucket_taskbackend.JobToBuildbucketTask(ctx, j, trybots.buildbucketTarget, trybots.host),
			},
		},
	}, j.BuildbucketToken).Return(nil)

	// Mock the pubsub message.
	update := &buildbucketpb.BuildTaskUpdate{
		BuildId: strconv.FormatInt(j.BuildbucketBuildId, 10),
		Task:    buildbucket_taskbackend.JobToBuildbucketTask(ctx, j, trybots.buildbucketTarget, trybots.host),
	}
	b, err := proto.Marshal(update)
	require.NoError(t, err)
	result := &pubsub_mocks.PublishResult{}
	result.On("Get", testutils.AnyContext).Return("fake-server-id", nil)
	topic.On("Publish", testutils.AnyContext, &pubsub.Message{Data: b}).Return(result)

	// Check the result.
	require.NoError(t, trybots.jobFinished(ctx, j))
	mockBB.AssertExpectations(t)
}

func TestJobFinishedV2_JobFailed_UpdateFails(t *testing.T) {
	ctx, trybots, _, mockBB, _ := setup(t)

	j := tryjobV2(ctx)
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
		Infra: &buildbucketpb.BuildInfra{
			Backend: &buildbucketpb.BuildInfra_Backend{
				Task: buildbucket_taskbackend.JobToBuildbucketTask(ctx, j, trybots.buildbucketTarget, trybots.host),
			},
		},
	}, j.BuildbucketToken).Return(errors.New("failed"))
	require.ErrorContains(t, trybots.jobFinished(ctx, j), "failed")
	mockBB.AssertExpectations(t)
}

func TestJobFinishedV2_JobMishap_UpdateSucceeds(t *testing.T) {
	ctx, trybots, _, mockBB, topic := setup(t)

	j := tryjobV2(ctx)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_MISHAP
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})

	// Mock the UpdateBuild call.
	mockBB.On("UpdateBuild", testutils.AnyContext, &buildbucketpb.Build{
		Id: j.BuildbucketBuildId,
		Output: &buildbucketpb.Build_Output{
			Status: buildbucketpb.Status_INFRA_FAILURE,
		},
		Infra: &buildbucketpb.BuildInfra{
			Backend: &buildbucketpb.BuildInfra_Backend{
				Task: buildbucket_taskbackend.JobToBuildbucketTask(ctx, j, trybots.buildbucketTarget, trybots.host),
			},
		},
	}, j.BuildbucketToken).Return(nil)

	// Mock the pubsub message.
	update := &buildbucketpb.BuildTaskUpdate{
		BuildId: strconv.FormatInt(j.BuildbucketBuildId, 10),
		Task:    buildbucket_taskbackend.JobToBuildbucketTask(ctx, j, trybots.buildbucketTarget, trybots.host),
	}
	b, err := proto.Marshal(update)
	require.NoError(t, err)
	result := &pubsub_mocks.PublishResult{}
	result.On("Get", testutils.AnyContext).Return("fake-server-id", nil)
	topic.On("Publish", testutils.AnyContext, &pubsub.Message{Data: b}).Return(result)

	// Check the result.
	require.NoError(t, trybots.jobFinished(ctx, j))
	mockBB.AssertExpectations(t)
}

func TestJobFinishedV2_JobMishap_UpdateFails(t *testing.T) {
	ctx, trybots, _, mockBB, _ := setup(t)

	j := tryjobV2(ctx)
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
		Infra: &buildbucketpb.BuildInfra{
			Backend: &buildbucketpb.BuildInfra_Backend{
				Task: buildbucket_taskbackend.JobToBuildbucketTask(ctx, j, trybots.buildbucketTarget, trybots.host),
			},
		},
	}, j.BuildbucketToken).Return(errors.New("failed"))
	require.ErrorContains(t, trybots.jobFinished(ctx, j), "failed")
	mockBB.AssertExpectations(t)
}

func TestJobFinishedV2_BuildAlreadyDone_NoError(t *testing.T) {
	ctx, trybots, _, mockBB, _ := setup(t)

	j := tryjobV2(ctx)
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
		Infra: &buildbucketpb.BuildInfra{
			Backend: &buildbucketpb.BuildInfra_Backend{
				Task: buildbucket_taskbackend.JobToBuildbucketTask(ctx, j, trybots.buildbucketTarget, trybots.host),
			},
		},
	}, j.BuildbucketToken).Return(errors.New(buildAlreadyFinishedErr))
	require.NoError(t, trybots.jobFinished(ctx, j))
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

func TestStartJobV2_NormalJob_Succeeds(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	j1 := tryjobV2(ctx)
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

func TestStartJobV2_RevisionAlreadySet_Succeeds(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	j1 := tryjobV2(ctx)
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

func TestStartJobV2_NormalJob_Failed(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	j1 := tryjobV2(ctx)
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

func TestStartJobV2_InvalidJobSpec_Failed(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	j1 := tryjobV2(ctx)
	j1.Name = "bogus-job"
	j1.Revision = "" // No revision is set initially; it's derived in startJob.
	j1.Status = types.JOB_STATUS_REQUESTED
	require.NoError(t, trybots.db.PutJob(ctx, j1))
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	mockBB.On("StartBuild", testutils.AnyContext, j1.BuildbucketBuildId, j1.Id, j1.BuildbucketToken).Return(bbFakeUpdateToken, nil)
	err := trybots.startJob(ctx, j1)
	require.ErrorContains(t, err, "no such job: bogus-job")
	j1, err = trybots.db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	require.Equal(t, commit2.Hash, j1.Revision)
	require.Equal(t, types.JOB_STATUS_MISHAP, j1.Status)
	require.Equal(t, bbFakeUpdateToken, j1.BuildbucketToken)
	require.True(t, j1.Valid())
	require.Contains(t, j1.StatusDetails, "Failed to start Job: no such job: bogus-job")
	require.NotEmpty(t, j1.Finished)
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

func TestRetryV2(t *testing.T) {
	ctx, trybots, mock, mockBB, _ := setup(t)

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)

	// Insert one try job.
	j1 := tryjobV2(ctx)
	j1.Revision = "" // No revision is set initially; it's derived in startJob.
	j1.Status = types.JOB_STATUS_REQUESTED
	require.NoError(t, trybots.db.PutJob(ctx, j1))
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	mockBB.On("StartBuild", testutils.AnyContext, j1.BuildbucketBuildId, j1.Id, j1.BuildbucketToken).Return(bbFakeUpdateToken, nil)
	require.NoError(t, trybots.startJob(ctx, j1))
	mockBB.AssertExpectations(t)

	// Obtain a second try job, ensure that it gets IsForce = true.
	j2 := tryjobV2(ctx)
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

func TestJobQueues_DifferentRepoStatesInParallel(t *testing.T) {
	// These Jobs all have different RepoStates. Ensure that we call workFn in
	// parallel.
	j1 := &types.Job{
		Id: "1",
		RepoState: types.RepoState{
			Revision: "1",
		},
	}
	j2 := &types.Job{
		Id: "2",
		RepoState: types.RepoState{
			Revision: "2",
		},
	}
	j3 := &types.Job{
		Id: "3",
		RepoState: types.RepoState{
			Revision: "3",
		},
	}

	// We'll signal that each instance of workFn may finish using these
	// channels.
	startCh := make(chan *types.Job)
	doneCh := map[*types.Job]chan struct{}{
		j1: make(chan struct{}),
		j2: make(chan struct{}),
		j3: make(chan struct{}),
	}

	q := &jobQueues{
		queues: map[types.RepoState]*jobQueue{},
		workFn: func(job *types.Job) {
			startCh <- job
			<-doneCh[job]
		},
	}
	q.Enqueue(j1)
	q.Enqueue(j2)
	q.Enqueue(j3)

	// Wait until all three instances of workFn have started before signalling
	// that they may all finish.
	<-startCh
	<-startCh
	<-startCh
	for _, done := range doneCh {
		done <- struct{}{}
	}
}

func TestJobQueues_SameRepoStatesSerially(t *testing.T) {
	// These Jobs all have different RepoStates. Ensure that we call workFn in
	// parallel.
	j1 := &types.Job{
		Id: "1",
		RepoState: types.RepoState{
			Revision: "1",
		},
	}
	j2 := &types.Job{
		Id: "2",
		RepoState: types.RepoState{
			Revision: "1",
		},
	}
	j3 := &types.Job{
		Id: "3",
		RepoState: types.RepoState{
			Revision: "1",
		},
	}

	// We'll signal that each instance of workFn may finish using these
	// channels.
	startCh := make(chan *types.Job)
	doneCh := map[*types.Job]chan struct{}{
		j1: make(chan struct{}),
		j2: make(chan struct{}),
		j3: make(chan struct{}),
	}
	var mtx sync.Mutex
	count := 0
	q := &jobQueues{
		queues: map[types.RepoState]*jobQueue{},
		workFn: func(job *types.Job) {
			mtx.Lock()
			require.Equal(t, 0, count)
			count++
			mtx.Unlock()

			startCh <- job
			<-doneCh[job]

			mtx.Lock()
			require.Equal(t, 1, count)
			count--
			mtx.Unlock()
		},
	}
	q.Enqueue(j1)
	q.Enqueue(j2)
	q.Enqueue(j3)

	// Start and finish each workFn sequentially.
	<-startCh
	doneCh[j1] <- struct{}{}
	<-startCh
	doneCh[j2] <- struct{}{}
	<-startCh
	doneCh[j3] <- struct{}{}
}

func TestJobQueues_Deduplicate(t *testing.T) {
	// Verify that we deduplicate jobs in the queue.
	j1 := &types.Job{
		Id: "1",
		RepoState: types.RepoState{
			Revision: "1",
		},
	}

	// We'll signal that each instance of workFn may finish using these
	// channels.
	startCh := make(chan *types.Job)
	doneCh := make(chan struct{})
	var mtx sync.Mutex
	count := 0
	q := &jobQueues{
		queues: map[types.RepoState]*jobQueue{},
		workFn: func(job *types.Job) {
			mtx.Lock()
			count++
			mtx.Unlock()
			startCh <- job
			<-doneCh
		},
	}
	q.Enqueue(j1)
	q.Enqueue(j1)
	q.Enqueue(j1)

	// Consume one job from startCh and then signal that the first workFn can
	// finish.
	<-startCh
	doneCh <- struct{}{}
	require.Equal(t, 1, count)
}

func TestJobQueues_Cleanup(t *testing.T) {
	// Verify that we remove the (correct) queue once it's empty.
	j1 := &types.Job{
		Id: "1",
		RepoState: types.RepoState{
			Patch: types.Patch{
				Issue: "12345",
			},
		},
	}

	// We'll signal that each instance of workFn may finish using these
	// channels.
	startCh := make(chan *types.Job)
	doneCh := make(chan struct{})

	q := &jobQueues{
		queues: map[types.RepoState]*jobQueue{},
		workFn: func(job *types.Job) {
			startCh <- job
			// Set the job's revision. This is analogous to what happens in the
			// real startJob function, where we sync the repo to the most recent
			// commit on the branch in question and use that as the revision.
			if job.Id == j1.Id {
				job.Revision = "1"
			}
			<-doneCh
		},
	}
	q.Enqueue(j1)

	// Wait until workFn has started.
	<-startCh

	// Verify that the expected queue exists.
	repoState := types.RepoState{
		Patch: types.Patch{
			Issue: "12345",
		},
	}
	q.mtx.Lock()
	_, ok := q.queues[repoState]
	q.mtx.Unlock()
	require.True(t, ok)

	// Allow j2's workFn to finish.
	doneCh <- struct{}{}

	// This is inherently racy. Wait up to two seconds for the queue to be
	// deleted.
	require.Eventually(t, func() bool {
		q.mtx.Lock()
		_, ok := q.queues[repoState]
		q.mtx.Unlock()
		return !ok
	}, 2*time.Second, 10*time.Millisecond)
}
