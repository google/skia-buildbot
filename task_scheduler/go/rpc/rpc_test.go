package rpc

import (
	context "context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/deepequal/assertdeep"
	fs_testutils "go.skia.org/infra/go/firestore/testutils"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/git/testutils/mem_git"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/gitstore/mem_gitstore"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/swarming/v2/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db/memory"
	"go.skia.org/infra/task_scheduler/go/skip_tasks"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	tcc_testutils "go.skia.org/infra/task_scheduler/go/task_cfg_cache/testutils"
	"go.skia.org/infra/task_scheduler/go/task_execution/swarmingv2"
	"go.skia.org/infra/task_scheduler/go/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// Fake user emails.
	viewer = "viewer@google.com"
	editor = "editor@google.com"

	fakeRepo = "fake.git"
)

var (
	unauthorizedStatus = alogin.Status{
		EMail: alogin.NotLoggedIn,
		Roles: roles.Roles{},
	}

	viewerStatus = alogin.Status{
		EMail: alogin.EMail(viewer),
		Roles: roles.Roles{roles.Viewer},
	}

	editorStatus = alogin.Status{
		EMail: alogin.EMail(editor),
		Roles: roles.Roles{roles.Editor},
	}
)

func setup(t *testing.T) (context.Context, *taskSchedulerServiceImpl, *types.Task, *types.Job, *skip_tasks.Rule, *mocks.SwarmingV2Client, func()) {
	ctx := context.Background()

	// Git repo.
	d := memory.NewInMemoryDB()
	gs := mem_gitstore.New()
	gb := mem_git.New(t, gs)
	hashes := gb.CommitN(2)
	ri, err := gitstore.NewGitStoreRepoImpl(ctx, gs)
	require.NoError(t, err)
	repo, err := repograph.NewWithRepoImpl(ctx, ri)
	require.NoError(t, err)
	repos := repograph.Map{
		fakeRepo: repo,
	}

	// Skip tasks DB.
	fsClient, cleanupFS := fs_testutils.NewClientForTesting(context.Background(), t)
	skipDB, err := skip_tasks.New(context.Background(), fsClient)
	require.NoError(t, err)
	skipRule := &skip_tasks.Rule{
		AddedBy:          "test@google.com",
		TaskSpecPatterns: []string{"task"},
		Commits:          []string{hashes[0]},
		Description:      "Skip this!",
		Name:             "Skipper",
	}
	require.NoError(t, skipDB.AddRule(ctx, skipRule, repos))

	// Task config cache.
	btProject, btInstance, btCleanup := tcc_testutils.SetupBigTable(t)
	tcc, err := task_cfg_cache.NewTaskCfgCache(ctx, repos, btProject, btInstance, nil)
	require.NoError(t, err)
	for _, hash := range hashes {
		rs := types.RepoState{
			Repo:     fakeRepo,
			Revision: hash,
		}
		cfg := &specs.TasksCfg{
			Jobs: map[string]*specs.JobSpec{
				"job": {
					TaskSpecs: []string{"task"},
				},
			},
			Tasks: map[string]*specs.TaskSpec{
				"task": {
					Dimensions: []string{
						"os:linux",
					},
				},
			},
		}
		require.NoError(t, tcc.Set(ctx, rs, cfg, nil))
	}

	// Create a task and job.
	job := &types.Job{
		Created: time.Now(),
		Dependencies: map[string][]string{
			"task": {},
		},
		Name: "my-job",
		RepoState: types.RepoState{
			Repo:     fakeRepo,
			Revision: hashes[1],
		},
	}
	require.NoError(t, d.PutJob(ctx, job))

	task := &types.Task{
		Commits: hashes,
		Created: time.Now(),
		TaskKey: types.TaskKey{
			Name: "task",
			RepoState: types.RepoState{
				Repo:     fakeRepo,
				Revision: hashes[1],
			},
		},
	}
	require.NoError(t, d.PutTask(ctx, task))

	swarm := &mocks.SwarmingV2Client{}
	taskExec := swarmingv2.NewSwarmingV2TaskExecutor(swarm, "fake-swarming", "fake-cas-instance", "fake-pubsub-topic", "fake-realm", []string{"Skia"})
	taskExecs := types.TaskExecutors([]types.TaskExecutor{taskExec})

	// Create the service.
	srv := newTaskSchedulerServiceImpl(ctx, d, repos, skipDB, tcc, taskExecs)
	return ctx, srv, task, job, skipRule, swarm, func() {
		btCleanup()
		cleanupFS()
	}
}

func TestTriggerJobs(t *testing.T) {

	ctx, srv, _, _, _, _, cleanup := setup(t)
	defer cleanup()

	commit := srv.repos[fakeRepo].Get(git.MainBranch).Hash
	req := &TriggerJobsRequest{
		Jobs: []*TriggerJob{
			{
				JobName:    "job",
				CommitHash: commit,
			},
			{
				JobName:    "job",
				CommitHash: commit,
			},
		},
	}

	// Check authorization.
	ctx = alogin.FakeStatus(ctx, &unauthorizedStatus)
	res, err := srv.TriggerJobs(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"\" is not an authorized editor")

	ctx = alogin.FakeStatus(ctx, &viewerStatus)
	res, err = srv.TriggerJobs(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"viewer@google.com\" is not an authorized editor")

	// Check results.
	ctx = alogin.FakeStatus(ctx, &editorStatus)
	res, err = srv.TriggerJobs(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 2, len(res.JobIds))
	for _, id := range res.JobIds {
		require.NotEqual(t, "", id)
	}
}

func TestGetJob(t *testing.T) {

	ctx, srv, _, job, _, _, cleanup := setup(t)
	defer cleanup()

	req := &GetJobRequest{
		Id: job.Id,
	}

	// Check results.
	ctx = alogin.FakeStatus(ctx, &unauthorizedStatus)
	res, err := srv.GetJob(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res.Job)
	require.Equal(t, job.Id, res.Job.Id)
	// Don't bother checking other fields, since we have a separate test for
	// convertJob.
	require.Equal(t, 1, len(res.Job.TaskSpecSummaries))
	require.Equal(t, "task", res.Job.TaskSpecSummaries[0].TaskName)
	assertdeep.Equal(t, []string{"os:linux"}, res.Job.TaskSpecSummaries[0].Dimensions)
	require.Equal(t, "fake-swarming", res.Job.TaskSpecSummaries[0].TaskExecutor)
}

func TestGetJob_NotStarted(t *testing.T) {

	ctx, srv, _, _, _, _, cleanup := setup(t)
	defer cleanup()

	job := &types.Job{
		Created: time.Now(),
		Name:    "my-job",
		RepoState: types.RepoState{
			Repo:     fakeRepo,
			Revision: "",
		},
		Status: types.JOB_STATUS_REQUESTED,
	}
	require.NoError(t, srv.db.PutJob(ctx, job))

	req := &GetJobRequest{
		Id: job.Id,
	}

	// Check results.
	ctx = alogin.FakeStatus(ctx, &unauthorizedStatus)
	res, err := srv.GetJob(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res.Job)
	require.Equal(t, job.Id, res.Job.Id)
	// Don't bother checking other fields, since we have a separate test for
	// convertJob.
	require.Equal(t, 0, len(res.Job.TaskSpecSummaries))
}

func TestCancelJob(t *testing.T) {

	ctx, srv, _, job, _, _, cleanup := setup(t)
	defer cleanup()

	req := &CancelJobRequest{
		Id: job.Id,
	}

	// Check authorization.
	ctx = alogin.FakeStatus(ctx, &unauthorizedStatus)
	res, err := srv.CancelJob(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"\" is not an authorized editor")

	ctx = alogin.FakeStatus(ctx, &viewerStatus)
	res, err = srv.CancelJob(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"viewer@google.com\" is not an authorized editor")

	// Check results.
	ctx = alogin.FakeStatus(ctx, &editorStatus)
	res, err = srv.CancelJob(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res.Job)
	require.Equal(t, job.Id, res.Job.Id)
	require.Equal(t, JobStatus_JOB_STATUS_CANCELED, res.Job.Status)
	// Don't bother checking other fields, since we have a separate test for
	// convertJob.
}

func TestSearchJobs(t *testing.T) {

	ctx, srv, _, job, _, _, cleanup := setup(t)
	defer cleanup()

	req := &SearchJobsRequest{}

	// Check results.
	ctx = alogin.FakeStatus(ctx, &unauthorizedStatus)
	res, err := srv.SearchJobs(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 1, len(res.Jobs))
	require.Equal(t, job.Id, res.Jobs[0].Id)
	// Don't bother checking other fields, since we have a separate test for
	// convertJobs.
}

func TestGetTask(t *testing.T) {

	ctx, srv, task, _, _, swarm, cleanup := setup(t)
	defer cleanup()

	req := &GetTaskRequest{
		Id: task.Id,
	}

	// Check results.
	ctx = alogin.FakeStatus(ctx, &unauthorizedStatus)
	res, err := srv.GetTask(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res.Task)
	require.Equal(t, task.Id, res.Task.Id)
	require.Nil(t, res.Task.Stats)
	// Don't bother checking other fields, since we have a separate test for
	// convertTask.

	// Now, verify that we retrieve task stats when requested.
	swarm.On("GetResult", testutils.AnyContext, &apipb.TaskIdWithPerfRequest{
		TaskId:                  task.SwarmingTaskId,
		IncludePerformanceStats: true,
	}).Return(&apipb.TaskResultResponse{
		PerformanceStats: &apipb.PerformanceStats{
			BotOverhead: 10.0,
			IsolatedDownload: &apipb.CASOperationStats{
				Duration: 6.0,
			},
			IsolatedUpload: &apipb.CASOperationStats{
				Duration: 4.0,
			},
		},
	}, nil)
	req.IncludeStats = true
	res, err = srv.GetTask(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res.Task)
	require.Equal(t, task.Id, res.Task.Id)
	require.NotNil(t, res.Task.Stats)
	require.Equal(t, float32(10.0), res.Task.Stats.TotalOverheadS)
	require.Equal(t, float32(6.0), res.Task.Stats.DownloadOverheadS)
	require.Equal(t, float32(4.0), res.Task.Stats.UploadOverheadS)
}

func TestSearchTasks(t *testing.T) {

	ctx, srv, task, _, _, _, cleanup := setup(t)
	defer cleanup()

	req := &SearchTasksRequest{}

	// Check results.
	ctx = alogin.FakeStatus(ctx, &unauthorizedStatus)
	res, err := srv.SearchTasks(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 1, len(res.Tasks))
	require.Equal(t, task.Id, res.Tasks[0].Id)
	// Don't bother checking other fields, since we have a separate test for
	// convertTasks.
}

func TestGetSkipTaskRules(t *testing.T) {

	ctx, srv, _, _, skipRule, _, cleanup := setup(t)
	defer cleanup()

	req := &GetSkipTaskRulesRequest{}

	// Check results.
	ctx = alogin.FakeStatus(ctx, &unauthorizedStatus)
	res, err := srv.GetSkipTaskRules(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 1, len(res.Rules))
	require.Equal(t, skipRule.AddedBy, res.Rules[0].AddedBy)
	require.Equal(t, skipRule.Name, res.Rules[0].Name)
	require.Equal(t, skipRule.TaskSpecPatterns, res.Rules[0].TaskSpecPatterns)
	require.Equal(t, skipRule.Commits, res.Rules[0].Commits)
	require.Equal(t, skipRule.Description, res.Rules[0].Description)
}

func TestAddSkipTaskRule(t *testing.T) {

	ctx, srv, _, _, skipRule, _, cleanup := setup(t)
	defer cleanup()

	req := &AddSkipTaskRuleRequest{
		TaskSpecPatterns: []string{"*"},
		Name:             "StAaaaahp",
		Description:      "Skip everything!",
	}

	// Check authorization.
	ctx = alogin.FakeStatus(ctx, &unauthorizedStatus)
	res, err := srv.AddSkipTaskRule(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"\" is not an authorized editor")

	ctx = alogin.FakeStatus(ctx, &viewerStatus)
	res, err = srv.AddSkipTaskRule(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"viewer@google.com\" is not an authorized editor")

	// Check results.
	ctx = alogin.FakeStatus(ctx, &editorStatus)
	res, err = srv.AddSkipTaskRule(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 2, len(res.Rules))
	require.Equal(t, skipRule.AddedBy, res.Rules[0].AddedBy)
	require.Equal(t, skipRule.Name, res.Rules[0].Name)
	require.Equal(t, skipRule.TaskSpecPatterns, res.Rules[0].TaskSpecPatterns)
	require.Equal(t, skipRule.Commits, res.Rules[0].Commits)
	require.Equal(t, skipRule.Description, res.Rules[0].Description)

	require.Equal(t, editor, res.Rules[1].AddedBy)
	require.Equal(t, req.Name, res.Rules[1].Name)
	require.Equal(t, req.TaskSpecPatterns, res.Rules[1].TaskSpecPatterns)
	require.Equal(t, req.Commits, res.Rules[1].Commits)
	require.Equal(t, req.Description, res.Rules[1].Description)
}

func TestDeleteSkipTaskRule(t *testing.T) {

	ctx, srv, _, _, skipRule, _, cleanup := setup(t)
	defer cleanup()

	req := &DeleteSkipTaskRuleRequest{
		Id: skipRule.Name,
	}

	// Check authorization.
	ctx = alogin.FakeStatus(ctx, &unauthorizedStatus)
	res, err := srv.DeleteSkipTaskRule(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"\" is not an authorized editor")

	ctx = alogin.FakeStatus(ctx, &viewerStatus)
	res, err = srv.DeleteSkipTaskRule(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"viewer@google.com\" is not an authorized editor")

	// Check results.
	ctx = alogin.FakeStatus(ctx, &editorStatus)
	res, err = srv.DeleteSkipTaskRule(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 0, len(res.Rules))
}

func TestConvertRepoState(t *testing.T) {

	actual := convertRepoState(types.RepoState{
		Repo:     fakeRepo,
		Revision: "abc123",
		Patch: types.Patch{
			Issue:     "9999",
			PatchRepo: "patch.git",
			Patchset:  "2",
			Server:    "https://patch.com",
		},
	})
	assertdeep.Copy(t, &RepoState{
		Repo:     fakeRepo,
		Revision: "abc123",
		Patch: &RepoState_Patch{
			Issue:     "9999",
			PatchRepo: "patch.git",
			Patchset:  "2",
			Server:    "https://patch.com",
		},
	}, actual)
}

func TestConvertTaskStatus(t *testing.T) {

	test := func(input types.TaskStatus, expect TaskStatus) {
		actual, err := convertTaskStatus(input)
		require.NoError(t, err)
		require.Equal(t, expect, actual)
	}

	test(types.TASK_STATUS_PENDING, TaskStatus_TASK_STATUS_PENDING)
	test(types.TASK_STATUS_RUNNING, TaskStatus_TASK_STATUS_RUNNING)
	test(types.TASK_STATUS_SUCCESS, TaskStatus_TASK_STATUS_SUCCESS)
	test(types.TASK_STATUS_FAILURE, TaskStatus_TASK_STATUS_FAILURE)
	test(types.TASK_STATUS_MISHAP, TaskStatus_TASK_STATUS_MISHAP)

	_, err := convertTaskStatus(types.TaskStatus("bogus"))
	require.EqualError(t, err, "twirp error internal: Invalid task status.")
}

func TestConvertTask(t *testing.T) {

	actual, err := convertTask(&types.Task{
		Attempt:        1,
		Commits:        []string{"abc123"},
		Created:        time.Unix(1600181000, 0),
		DbModified:     time.Unix(1600182000, 0),
		Finished:       time.Unix(1600183000, 0),
		Id:             "my-task-id",
		IsolatedOutput: "outputhash",
		Jobs:           []string{"my-job"},
		MaxAttempts:    2,
		ParentTaskIds:  []string{"parent-task"},
		Properties: map[string]string{
			"key": "value",
		},
		RetryOf:        "previously-failed-task",
		Started:        time.Unix(1600181500, 0),
		Status:         types.TASK_STATUS_SUCCESS,
		SwarmingBotId:  "swarmbot",
		SwarmingTaskId: "swarmtask",
		TaskExecutor:   "fake-swarming",
		TaskKey: types.TaskKey{
			RepoState: types.RepoState{
				Repo:     fakeRepo,
				Revision: "abc123",
				Patch: types.Patch{
					Issue:     "9999",
					PatchRepo: "patch.git",
					Patchset:  "2",
					Server:    "https://patch.com",
				},
			},
			Name:        "my-task",
			ForcedJobId: "forced-job",
		},
	})
	require.NoError(t, err)

	// Fake the Stats to appease assertdeep.Copy; Stats aren't set in
	// convertTask.
	actual.Stats = &TaskStats{
		DownloadOverheadS: 5,
		UploadOverheadS:   4,
		TotalOverheadS:    9,
	}

	assertdeep.Copy(t, &Task{
		Attempt:        1,
		Commits:        []string{"abc123"},
		CreatedAt:      timestamppb.New(time.Unix(1600181000, 0)),
		DbModifiedAt:   timestamppb.New(time.Unix(1600182000, 0)),
		FinishedAt:     timestamppb.New(time.Unix(1600183000, 0)),
		Id:             "my-task-id",
		IsolatedOutput: "outputhash",
		Jobs:           []string{"my-job"},
		MaxAttempts:    2,
		ParentTaskIds:  []string{"parent-task"},
		Properties: map[string]string{
			"key": "value",
		},
		RetryOf:        "previously-failed-task",
		StartedAt:      timestamppb.New(time.Unix(1600181500, 0)),
		Status:         TaskStatus_TASK_STATUS_SUCCESS,
		SwarmingBotId:  "swarmbot",
		SwarmingTaskId: "swarmtask",
		TaskExecutor:   "fake-swarming",
		TaskKey: &TaskKey{
			RepoState: &RepoState{
				Repo:     fakeRepo,
				Revision: "abc123",
				Patch: &RepoState_Patch{
					Issue:     "9999",
					PatchRepo: "patch.git",
					Patchset:  "2",
					Server:    "https://patch.com",
				},
			},
			Name:        "my-task",
			ForcedJobId: "forced-job",
		},
		// Not set in convertTask.
		Stats: &TaskStats{
			DownloadOverheadS: 5,
			UploadOverheadS:   4,
			TotalOverheadS:    9,
		},
	}, actual)
}

func TestConvertJobStatus(t *testing.T) {

	test := func(input types.JobStatus, expect JobStatus) {
		actual, err := convertJobStatus(input)
		require.NoError(t, err)
		require.Equal(t, expect, actual)
	}

	test(types.JOB_STATUS_REQUESTED, JobStatus_JOB_STATUS_REQUESTED)
	test(types.JOB_STATUS_IN_PROGRESS, JobStatus_JOB_STATUS_IN_PROGRESS)
	test(types.JOB_STATUS_SUCCESS, JobStatus_JOB_STATUS_SUCCESS)
	test(types.JOB_STATUS_FAILURE, JobStatus_JOB_STATUS_FAILURE)
	test(types.JOB_STATUS_MISHAP, JobStatus_JOB_STATUS_MISHAP)
	test(types.JOB_STATUS_CANCELED, JobStatus_JOB_STATUS_CANCELED)

	_, err := convertJobStatus(types.JobStatus("bogus"))
	require.EqualError(t, err, "twirp error internal: Invalid job status.")
}

func TestConvertJob(t *testing.T) {
	actual, err := convertJob(&types.Job{
		BuildbucketBuildId:  12345,
		BuildbucketLeaseKey: 67890,
		Created:             time.Unix(1600181000, 0),
		DbModified:          time.Unix(1600182000, 0),
		Dependencies: map[string][]string{
			"taskA": {},
			"taskB": {},
			"taskC": {"taskB", "taskA"},
			"taskD": {"taskC"},
		},
		Finished: time.Unix(1600183000, 0),
		Id:       "fake-job-id",
		IsForce:  true,
		Name:     "My Job",
		Priority: 0.8,
		RepoState: types.RepoState{
			Repo:     fakeRepo,
			Revision: "abc123",
			Patch: types.Patch{
				Issue:     "9999",
				PatchRepo: "patch.git",
				Patchset:  "2",
				Server:    "https://patch.com",
			},
		},
		Requested: time.Unix(1600180000, 0),
		Started:   time.Unix(1600183000, 0),
		Status:    types.JOB_STATUS_FAILURE,
		Tasks: map[string][]*types.TaskSummary{
			"taskA": {
				{
					Attempt:        0,
					Id:             "taskA-id-0",
					MaxAttempts:    2,
					Status:         types.TASK_STATUS_FAILURE,
					SwarmingTaskId: "swarm0",
				},
				{
					Attempt:        1,
					Id:             "taskA-id-1",
					MaxAttempts:    2,
					Status:         types.TASK_STATUS_FAILURE,
					SwarmingTaskId: "swarm1",
				},
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, &Job{
		BuildbucketBuildId:  "12345",
		BuildbucketLeaseKey: "67890",
		CreatedAt:           timestamppb.New(time.Unix(1600181000, 0)),
		DbModifiedAt:        timestamppb.New(time.Unix(1600182000, 0)),
		Dependencies: []*TaskDependencies{
			{
				Task:         "taskA",
				Dependencies: []string{},
			},
			{
				Task:         "taskB",
				Dependencies: []string{},
			},
			{
				Task:         "taskC",
				Dependencies: []string{"taskB", "taskA"},
			},
			{
				Task:         "taskD",
				Dependencies: []string{"taskC"},
			},
		},
		FinishedAt: timestamppb.New(time.Unix(1600183000, 0)),
		Id:         "fake-job-id",
		IsForce:    true,
		Name:       "My Job",
		Priority:   0.8,
		RepoState: &RepoState{
			Repo:     fakeRepo,
			Revision: "abc123",
			Patch: &RepoState_Patch{
				Issue:     "9999",
				PatchRepo: "patch.git",
				Patchset:  "2",
				Server:    "https://patch.com",
			},
		},
		RequestedAt: timestamppb.New(time.Unix(1600180000, 0)),
		StartedAt:   timestamppb.New(time.Unix(1600183000, 0)),
		Status:      JobStatus_JOB_STATUS_FAILURE,
		Tasks: []*TaskSummaries{
			{
				Name: "taskA",
				Tasks: []*TaskSummary{
					{
						Attempt:        0,
						Id:             "taskA-id-0",
						MaxAttempts:    2,
						Status:         TaskStatus_TASK_STATUS_FAILURE,
						SwarmingTaskId: "swarm0",
					},
					{
						Attempt:        1,
						Id:             "taskA-id-1",
						MaxAttempts:    2,
						Status:         TaskStatus_TASK_STATUS_FAILURE,
						SwarmingTaskId: "swarm1",
					},
				},
			},
		},
	}, actual)
}

func TestConvertJob_NotStarted(t *testing.T) {
	actual, err := convertJob(&types.Job{
		BuildbucketBuildId:  12345,
		BuildbucketLeaseKey: 67890,
		Created:             time.Unix(1600181000, 0),
		DbModified:          time.Unix(1600182000, 0),
		Finished:            time.Unix(1600183000, 0),
		Id:                  "fake-job-id",
		IsForce:             true,
		Name:                "My Job",
		Priority:            0.8,
		RepoState: types.RepoState{
			Repo:     fakeRepo,
			Revision: "",
			Patch: types.Patch{
				Issue:     "9999",
				PatchRepo: "patch.git",
				Patchset:  "2",
				Server:    "https://patch.com",
			},
		},
		Requested: time.Unix(1600180000, 0),
		Status:    types.JOB_STATUS_REQUESTED,
	})
	require.NoError(t, err)

	require.Equal(t, &Job{
		BuildbucketBuildId:  "12345",
		BuildbucketLeaseKey: "67890",
		CreatedAt:           timestamppb.New(time.Unix(1600181000, 0)),
		DbModifiedAt:        timestamppb.New(time.Unix(1600182000, 0)),
		FinishedAt:          timestamppb.New(time.Unix(1600183000, 0)),
		Id:                  "fake-job-id",
		IsForce:             true,
		Name:                "My Job",
		Priority:            0.8,
		RepoState: &RepoState{
			Repo:     fakeRepo,
			Revision: "",
			Patch: &RepoState_Patch{
				Issue:     "9999",
				PatchRepo: "patch.git",
				Patchset:  "2",
				Server:    "https://patch.com",
			},
		},
		RequestedAt: timestamppb.New(time.Unix(1600180000, 0)),
		StartedAt:   timestamppb.New(time.Time{}),
		Status:      JobStatus_JOB_STATUS_REQUESTED,
	}, actual)
}
