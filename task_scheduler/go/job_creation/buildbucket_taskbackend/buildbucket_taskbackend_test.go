package buildbucket_taskbackend

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	buildbucket_mocks "go.skia.org/infra/go/buildbucket/mocks"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/mocks"
	"go.skia.org/infra/task_scheduler/go/types"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	fakeBuildbucketPubsubTopic = "fake-bb-pubsub"
	fakeBuildbucketTarget      = "skia://fake-scheduler"
	fakeBuildbucketToken       = "fake-token"
	fakeBuildIdStr             = "12345"
	fakeJobName                = "fake-job-name"
	fakeTaskSchedulerHost      = "https://fake-scheduler"
	fakeProject                = "fake-project"
	fakeRepo                   = "https://fake.git"
	fakeGerritHost             = "fake-project-review.googlesource.com"
	fakeGerritChange           = 6789
	fakeGerritPatchset         = 3
)

var (
	// This is a var instead of a constant so that we can take its address.
	fakeBuildIdInt int64 = 12345
	fakeCreateTime       = firestore.FixTimestamp(time.Unix(1702395110, 0)) // Arbitrary timestamp.
)

func setup(t *testing.T) (context.Context, *TaskBackend, *mocks.JobDB, *buildbucket_mocks.BuildBucketInterface) {
	ctx := now.TimeTravelingContext(fakeCreateTime.Add(time.Minute))
	bb := &buildbucket_mocks.BuildBucketInterface{}
	projectRepoMapping := map[string]string{
		fakeProject: fakeRepo,
	}
	db := &mocks.JobDB{}
	tb := NewTaskBackend(fakeBuildbucketTarget, fakeTaskSchedulerHost, projectRepoMapping, db, bb)
	t.Cleanup(func() {
		db.AssertExpectations(t)
		bb.AssertExpectations(t)
	})
	return ctx, tb, db, bb
}

func fakeBuild() *buildbucketpb.Build {
	return &buildbucketpb.Build{
		Id: fakeBuildIdInt,
		Builder: &buildbucketpb.BuilderID{
			Project: fakeProject,
			Builder: fakeJobName,
		},
		CreateTime: timestamppb.New(fakeCreateTime),
		Input: &buildbucketpb.Build_Input{
			GerritChanges: []*buildbucketpb.GerritChange{
				{
					Project:  fakeProject,
					Host:     fakeGerritHost,
					Change:   fakeGerritChange,
					Patchset: fakeGerritPatchset,
				},
			},
		},
	}
}

func fakeRunTaskRequest() *buildbucketpb.RunTaskRequest {
	return &buildbucketpb.RunTaskRequest{
		BuildId:     fakeBuildIdStr,
		PubsubTopic: fakeBuildbucketPubsubTopic,
		Secrets: &buildbucketpb.BuildSecrets{
			StartBuildToken: fakeBuildbucketToken,
		},
		Target: fakeBuildbucketTarget,
	}
}

func makeJob(ctx context.Context) *types.Job {
	return &types.Job{
		Name:                   fakeJobName,
		BuildbucketBuildId:     fakeBuildIdInt,
		BuildbucketPubSubTopic: fakeBuildbucketPubsubTopic,
		BuildbucketToken:       fakeBuildbucketToken,
		Requested:              fakeCreateTime,
		Created:                firestore.FixTimestamp(now.Now(ctx)),
		RepoState: types.RepoState{
			Patch: types.Patch{
				Server:    "https://" + fakeGerritHost,
				Issue:     strconv.FormatInt(fakeGerritChange, 10),
				PatchRepo: fakeRepo,
				Patchset:  strconv.FormatInt(fakeGerritPatchset, 10),
			},
			Repo:     fakeRepo,
			Revision: "",
		},
		Status: types.JOB_STATUS_REQUESTED,
	}
}

func TestTaskBackend_RunTask_Success(t *testing.T) {
	ctx, tb, d, bb := setup(t)
	d.On("SearchJobs", testutils.AnyContext, &db.JobSearchParams{
		BuildbucketBuildID: &fakeBuildIdInt,
	}).Return(nil, nil)
	bb.On("GetBuild", testutils.AnyContext, fakeBuildIdInt).Return(fakeBuild(), nil)
	expectedJob := makeJob(ctx)
	d.On("PutJob", testutils.AnyContext, expectedJob).Return(nil)
	resp, err := tb.RunTask(ctx, fakeRunTaskRequest())
	require.NoError(t, err)
	require.Equal(t, &buildbucketpb.RunTaskResponse{
		Task: JobToBuildbucketTask(ctx, expectedJob, fakeBuildbucketTarget, fakeTaskSchedulerHost),
	}, resp)
}

func TestTaskBackend_RunTask_WrongTarget(t *testing.T) {
	ctx, tb, _, _ := setup(t)
	req := fakeRunTaskRequest()
	req.Target = "bogus target"
	resp, err := tb.RunTask(ctx, req)
	require.ErrorContains(t, err, "incorrect target for this scheduler")
	require.Nil(t, resp)
}

func TestTaskBackend_RunTask_InvalidID(t *testing.T) {
	ctx, tb, _, _ := setup(t)
	req := fakeRunTaskRequest()
	req.BuildId = "not parseable as int64"
	resp, err := tb.RunTask(ctx, req)
	require.ErrorContains(t, err, "invalid build ID")
	require.Nil(t, resp)
}

func TestTaskBackend_RunTask_Duplicate(t *testing.T) {
	ctx, tb, d, _ := setup(t)
	expectedJob := makeJob(ctx)
	d.On("SearchJobs", testutils.AnyContext, &db.JobSearchParams{
		BuildbucketBuildID: &fakeBuildIdInt,
	}).Return([]*types.Job{expectedJob}, nil)
	resp, err := tb.RunTask(ctx, fakeRunTaskRequest())
	require.NoError(t, err)
	require.Equal(t, &buildbucketpb.RunTaskResponse{
		Task: JobToBuildbucketTask(ctx, expectedJob, fakeBuildbucketTarget, fakeTaskSchedulerHost),
	}, resp)
}

func TestTaskBackend_RunTask_MultipleDuplicates_UseFirst(t *testing.T) {
	ctx, tb, d, _ := setup(t)
	expectedJob := makeJob(ctx)
	otherJob := makeJob(ctx)
	otherJob.Id = "not this one"
	d.On("SearchJobs", testutils.AnyContext, &db.JobSearchParams{
		BuildbucketBuildID: &fakeBuildIdInt,
	}).Return([]*types.Job{expectedJob, otherJob}, nil)
	resp, err := tb.RunTask(ctx, fakeRunTaskRequest())
	require.NoError(t, err)
	require.Equal(t, &buildbucketpb.RunTaskResponse{
		Task: JobToBuildbucketTask(ctx, expectedJob, fakeBuildbucketTarget, fakeTaskSchedulerHost),
	}, resp)
}

func TestTaskBackend_RunTask_FailedSearchingJobs(t *testing.T) {
	ctx, tb, d, _ := setup(t)
	req := fakeRunTaskRequest()
	d.On("SearchJobs", testutils.AnyContext, &db.JobSearchParams{
		BuildbucketBuildID: &fakeBuildIdInt,
	}).Return(nil, errors.New("can't find the jobs!"))
	resp, err := tb.RunTask(ctx, req)
	require.ErrorContains(t, err, "failed looking for duplicate jobs")
	require.Nil(t, resp)
}

func TestTaskBackend_RunTask_GetBuildFailed(t *testing.T) {
	ctx, tb, d, bb := setup(t)
	d.On("SearchJobs", testutils.AnyContext, &db.JobSearchParams{
		BuildbucketBuildID: &fakeBuildIdInt,
	}).Return(nil, nil)
	bb.On("GetBuild", testutils.AnyContext, fakeBuildIdInt).Return(nil, errors.New("can't find the build!"))
	req := fakeRunTaskRequest()
	resp, err := tb.RunTask(ctx, req)
	require.ErrorContains(t, err, "failed to retrieve build")
	require.Nil(t, resp)
}

func TestTaskBackend_RunTask_NoBuilder(t *testing.T) {
	ctx, tb, d, bb := setup(t)
	d.On("SearchJobs", testutils.AnyContext, &db.JobSearchParams{
		BuildbucketBuildID: &fakeBuildIdInt,
	}).Return(nil, nil)
	build := fakeBuild()
	build.Builder = nil
	bb.On("GetBuild", testutils.AnyContext, fakeBuildIdInt).Return(build, nil)
	req := fakeRunTaskRequest()
	resp, err := tb.RunTask(ctx, req)
	require.ErrorContains(t, err, "builder isn't set")
	require.Nil(t, resp)
}

func TestTaskBackend_RunTask_NoGerritChanges(t *testing.T) {
	ctx, tb, d, bb := setup(t)
	d.On("SearchJobs", testutils.AnyContext, &db.JobSearchParams{
		BuildbucketBuildID: &fakeBuildIdInt,
	}).Return(nil, nil)
	build := fakeBuild()
	build.Input.GerritChanges = nil
	bb.On("GetBuild", testutils.AnyContext, fakeBuildIdInt).Return(build, nil)
	req := fakeRunTaskRequest()
	resp, err := tb.RunTask(ctx, req)
	require.ErrorContains(t, err, "input should have exactly one GerritChanges")
	require.Nil(t, resp)
}

func TestTaskBackend_RunTask_UnknownPatchProject(t *testing.T) {
	ctx, tb, d, bb := setup(t)
	d.On("SearchJobs", testutils.AnyContext, &db.JobSearchParams{
		BuildbucketBuildID: &fakeBuildIdInt,
	}).Return(nil, nil)
	build := fakeBuild()
	build.Input.GerritChanges[0].Project = "bogus project"
	bb.On("GetBuild", testutils.AnyContext, fakeBuildIdInt).Return(build, nil)
	req := fakeRunTaskRequest()
	resp, err := tb.RunTask(ctx, req)
	require.ErrorContains(t, err, "unknown patch project")
	require.Nil(t, resp)
}

func TestTaskBackend_RunTask_FailedDBInsert(t *testing.T) {
	ctx, tb, d, bb := setup(t)
	d.On("SearchJobs", testutils.AnyContext, &db.JobSearchParams{
		BuildbucketBuildID: &fakeBuildIdInt,
	}).Return(nil, nil)
	bb.On("GetBuild", testutils.AnyContext, fakeBuildIdInt).Return(fakeBuild(), nil)
	expectedJob := makeJob(ctx)
	d.On("PutJob", testutils.AnyContext, expectedJob).Return(errors.New("db failed"))
	resp, err := tb.RunTask(ctx, fakeRunTaskRequest())
	require.ErrorContains(t, err, "failed to insert Job into the DB")
	require.Nil(t, resp)
}

func TestTaskBackend_RunTask_MissingSecrets(t *testing.T) {
	ctx, tb, _, _ := setup(t)
	req := fakeRunTaskRequest()
	req.Secrets = nil
	resp, err := tb.RunTask(ctx, req)
	require.ErrorContains(t, err, "secrets not set on request")
	require.Nil(t, resp)
}

func TestTaskBackend_RunTask_MissingStartBuildToken(t *testing.T) {
	ctx, tb, _, _ := setup(t)
	req := fakeRunTaskRequest()
	req.Secrets.StartBuildToken = ""
	resp, err := tb.RunTask(ctx, req)
	require.ErrorContains(t, err, "missing StartBuildToken")
	require.Nil(t, resp)
}

func TestTaskBackend_FetchTasks_Success(t *testing.T) {
	ctx, tb, d, _ := setup(t)
	job := makeJob(ctx)
	d.On("GetJobById", testutils.AnyContext, "my-job-id").Return(job, nil)
	resp, err := tb.FetchTasks(ctx, &buildbucketpb.FetchTasksRequest{
		TaskIds: []*buildbucketpb.TaskID{
			{
				Target: fakeBuildbucketTarget,
				Id:     "my-job-id",
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, &buildbucketpb.FetchTasksResponse{
		Responses: []*buildbucketpb.FetchTasksResponse_Response{
			{
				Response: &buildbucketpb.FetchTasksResponse_Response_Task{
					Task: JobToBuildbucketTask(ctx, job, fakeBuildbucketTarget, fakeTaskSchedulerHost),
				},
			},
		},
	}, resp)
}

func TestTaskBackend_FetchTasks_WrongTarget(t *testing.T) {
	ctx, tb, d, _ := setup(t)
	job := makeJob(ctx)
	d.On("GetJobById", testutils.AnyContext, "my-job-id").Return(job, nil)
	resp, err := tb.FetchTasks(ctx, &buildbucketpb.FetchTasksRequest{
		TaskIds: []*buildbucketpb.TaskID{
			{
				Target: fakeBuildbucketTarget,
				Id:     "my-job-id",
			},
			{
				Target: "bogus target",
				Id:     "fail-job-id",
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, &buildbucketpb.FetchTasksResponse{
		Responses: []*buildbucketpb.FetchTasksResponse_Response{
			{
				Response: &buildbucketpb.FetchTasksResponse_Response_Task{
					Task: JobToBuildbucketTask(ctx, job, fakeBuildbucketTarget, fakeTaskSchedulerHost),
				},
			},
			{
				Response: &buildbucketpb.FetchTasksResponse_Response_Error{
					Error: &status.Status{
						Code:    http.StatusBadRequest,
						Message: fmt.Sprintf("incorrect target for this scheduler; expected %s", fakeBuildbucketTarget),
					},
				},
			},
		},
	}, resp)
}

func TestTaskBackend_FetchTasks_DBError(t *testing.T) {
	ctx, tb, d, _ := setup(t)
	job := makeJob(ctx)
	d.On("GetJobById", testutils.AnyContext, "my-job-id").Return(job, nil)
	d.On("GetJobById", testutils.AnyContext, "fail-job-id").Return(nil, errors.New("DB error"))
	resp, err := tb.FetchTasks(ctx, &buildbucketpb.FetchTasksRequest{
		TaskIds: []*buildbucketpb.TaskID{
			{
				Target: fakeBuildbucketTarget,
				Id:     "my-job-id",
			},
			{
				Target: fakeBuildbucketTarget,
				Id:     "fail-job-id",
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, &buildbucketpb.FetchTasksResponse{
		Responses: []*buildbucketpb.FetchTasksResponse_Response{
			{
				Response: &buildbucketpb.FetchTasksResponse_Response_Task{
					Task: JobToBuildbucketTask(ctx, job, fakeBuildbucketTarget, fakeTaskSchedulerHost),
				},
			},
			{
				Response: &buildbucketpb.FetchTasksResponse_Response_Error{
					Error: &status.Status{
						Code:    http.StatusInternalServerError,
						Message: "DB error",
					},
				},
			},
		},
	}, resp)
}

func TestTaskBackend_FetchTasks_NotFound(t *testing.T) {
	ctx, tb, d, _ := setup(t)
	job := makeJob(ctx)
	d.On("GetJobById", testutils.AnyContext, "my-job-id").Return(job, nil)
	d.On("GetJobById", testutils.AnyContext, "fail-job-id").Return(nil, nil)
	resp, err := tb.FetchTasks(ctx, &buildbucketpb.FetchTasksRequest{
		TaskIds: []*buildbucketpb.TaskID{
			{
				Target: fakeBuildbucketTarget,
				Id:     "my-job-id",
			},
			{
				Target: fakeBuildbucketTarget,
				Id:     "fail-job-id",
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, &buildbucketpb.FetchTasksResponse{
		Responses: []*buildbucketpb.FetchTasksResponse_Response{
			{
				Response: &buildbucketpb.FetchTasksResponse_Response_Task{
					Task: JobToBuildbucketTask(ctx, job, fakeBuildbucketTarget, fakeTaskSchedulerHost),
				},
			},
			{
				Response: &buildbucketpb.FetchTasksResponse_Response_Error{
					Error: &status.Status{
						Code:    http.StatusNotFound,
						Message: "unknown task",
					},
				},
			},
		},
	}, resp)
}

func TestTaskBackend_CancelTasks_Success(t *testing.T) {
	ctx, tb, d, _ := setup(t)
	beforeJob := makeJob(ctx)
	afterJob := beforeJob.Copy()
	d.On("GetJobById", testutils.AnyContext, "my-job-id").Return(beforeJob, nil)
	afterJob.Finished = fakeCreateTime.Add(time.Minute)
	afterJob.Status = types.JOB_STATUS_CANCELED
	afterJob.StatusDetails = "Canceled by Buildbucket"
	d.On("PutJobs", testutils.AnyContext, []*types.Job{afterJob}).Return(nil)
	resp, err := tb.CancelTasks(ctx, &buildbucketpb.CancelTasksRequest{
		TaskIds: []*buildbucketpb.TaskID{
			{
				Target: fakeBuildbucketTarget,
				Id:     "my-job-id",
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, &buildbucketpb.CancelTasksResponse{
		Tasks: []*buildbucketpb.Task{
			JobToBuildbucketTask(ctx, afterJob, fakeBuildbucketTarget, fakeTaskSchedulerHost),
		},
	}, resp)
}

func TestTaskBackend_CancelTasks_WrongTarget(t *testing.T) {
	ctx, tb, _, _ := setup(t)
	resp, err := tb.CancelTasks(ctx, &buildbucketpb.CancelTasksRequest{
		TaskIds: []*buildbucketpb.TaskID{
			{
				Target: "bogus-target",
				Id:     "my-job-id",
			},
		},
	})
	require.ErrorContains(t, err, "incorrect target for this scheduler")
	require.Nil(t, resp)
}

func TestTaskBackend_CancelTasks_FailedDBRetrieve(t *testing.T) {
	ctx, tb, d, _ := setup(t)
	d.On("GetJobById", testutils.AnyContext, "my-job-id").Return(nil, nil)
	resp, err := tb.CancelTasks(ctx, &buildbucketpb.CancelTasksRequest{
		TaskIds: []*buildbucketpb.TaskID{
			{
				Target: fakeBuildbucketTarget,
				Id:     "my-job-id",
			},
		},
	})
	require.ErrorContains(t, err, "unknown job")
	require.Nil(t, resp)
}

func TestTaskBackend_CancelTasks_FailedDBInsert(t *testing.T) {
	ctx, tb, d, _ := setup(t)
	beforeJob := makeJob(ctx)
	afterJob := beforeJob.Copy()
	d.On("GetJobById", testutils.AnyContext, "my-job-id").Return(beforeJob, nil)
	afterJob.Finished = fakeCreateTime.Add(time.Minute)
	afterJob.Status = types.JOB_STATUS_CANCELED
	afterJob.StatusDetails = "Canceled by Buildbucket"
	d.On("PutJobs", testutils.AnyContext, []*types.Job{afterJob}).Return(errors.New("DB error"))
	resp, err := tb.CancelTasks(ctx, &buildbucketpb.CancelTasksRequest{
		TaskIds: []*buildbucketpb.TaskID{
			{
				Target: fakeBuildbucketTarget,
				Id:     "my-job-id",
			},
		},
	})
	require.ErrorContains(t, err, "DB error")
	require.Nil(t, resp)
}

func TestTaskBackend_CancelTasks_NoUpdates(t *testing.T) {
	ctx, tb, d, _ := setup(t)
	job := makeJob(ctx)
	job.Finished = fakeCreateTime.Add(time.Minute)
	job.Status = types.JOB_STATUS_CANCELED
	job.StatusDetails = "Canceled by Buildbucket"
	d.On("GetJobById", testutils.AnyContext, "my-job-id").Return(job, nil)
	resp, err := tb.CancelTasks(ctx, &buildbucketpb.CancelTasksRequest{
		TaskIds: []*buildbucketpb.TaskID{
			{
				Target: fakeBuildbucketTarget,
				Id:     "my-job-id",
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, &buildbucketpb.CancelTasksResponse{
		Tasks: []*buildbucketpb.Task{
			JobToBuildbucketTask(ctx, job, fakeBuildbucketTarget, fakeTaskSchedulerHost),
		},
	}, resp)
}

func TestTaskBackend_ValidateConfigs_Success(t *testing.T) {
	ctx, tb, _, _ := setup(t)
	resp, err := tb.ValidateConfigs(ctx, &buildbucketpb.ValidateConfigsRequest{
		Configs: []*buildbucketpb.ValidateConfigsRequest_ConfigContext{
			{
				Target:     fakeBuildbucketTarget,
				ConfigJson: nil,
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, &buildbucketpb.ValidateConfigsResponse{
		ConfigErrors: nil,
	}, resp)
}

func TestTaskBackend_ValidateConfigs_WrongTarget(t *testing.T) {
	ctx, tb, _, _ := setup(t)
	resp, err := tb.ValidateConfigs(ctx, &buildbucketpb.ValidateConfigsRequest{
		Configs: []*buildbucketpb.ValidateConfigsRequest_ConfigContext{
			{
				Target:     "bogus-target",
				ConfigJson: nil,
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, &buildbucketpb.ValidateConfigsResponse{
		ConfigErrors: []*buildbucketpb.ValidateConfigsResponse_ErrorDetail{
			{
				Index: 0,
				Error: fmt.Sprintf("incorrect target for this scheduler; expected %s", fakeBuildbucketTarget),
			},
		},
	}, resp)
}
