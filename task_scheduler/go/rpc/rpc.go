package rpc

import (
	context "context"
	fmt "fmt"
	http "net/http"
	"sort"
	"time"

	twirp "github.com/twitchtv/twirp"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/twirp_auth"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/skip_tasks"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Generate the go code from the protocol buffer definitions.
//go:generate protoc --go_opt=paths=source_relative --twirp_out=. --go_out=. ./rpc.proto
//go:generate mv ./go.skia.org/infra/task_scheduler/go/rpc/rpc.twirp.go ./rpc.twirp.go
//go:generate rm -rf ./go.skia.org
//go:generate goimports -w rpc.pb.go
//go:generate goimports -w rpc.twirp.go
//go:generate protoc --twirp_typescript_out=../../modules/rpc ./rpc.proto

// NewTaskSchedulerServer creates and returns a Twirp HTTP server.
func NewTaskSchedulerServer(ctx context.Context, db db.DB, repos repograph.Map, skipTasks *skip_tasks.DB, taskCfgCache *task_cfg_cache.TaskCfgCache, viewers, editors, admins allowed.Allow) http.Handler {
	impl := newTaskSchedulerServiceImpl(ctx, db, repos, skipTasks, taskCfgCache, viewers, editors, admins)
	srv := NewTaskSchedulerServiceServer(impl, nil)
	return twirp_auth.Middleware(srv)
}

// taskSchedulerServiceImpl implements TaskSchedulerService.
type taskSchedulerServiceImpl struct {
	*twirp_auth.AuthHelper
	db           db.DB
	repos        repograph.Map
	skipTasks    *skip_tasks.DB
	taskCfgCache *task_cfg_cache.TaskCfgCache
}

// newTaskSchedulerServiceImpl returns a taskSchedulerServiceImpl instance.
func newTaskSchedulerServiceImpl(ctx context.Context, db db.DB, repos repograph.Map, skipTasks *skip_tasks.DB, taskCfgCache *task_cfg_cache.TaskCfgCache, viewers, editors, admins allowed.Allow) *taskSchedulerServiceImpl {
	return &taskSchedulerServiceImpl{
		AuthHelper:   twirp_auth.NewAuthHelper(viewers, editors, admins),
		db:           db,
		repos:        repos,
		skipTasks:    skipTasks,
		taskCfgCache: taskCfgCache,
	}
}

// TriggerJobs triggers the given jobs.
func (s *taskSchedulerServiceImpl) TriggerJobs(ctx context.Context, req *TriggerJobsRequest) (*TriggerJobsResponse, error) {
	if _, err := s.GetEditor(ctx); err != nil {
		return nil, err
	}
	jobs := make([]*types.Job, 0, len(req.Jobs))
	for _, j := range req.Jobs {
		_, repoName, _, err := s.repos.FindCommit(j.CommitHash)
		if err != nil {
			sklog.Error(err)
			return nil, twirp.NotFoundError("Unable to find the given commit in any repo.")
		}
		job, err := s.taskCfgCache.MakeJob(ctx, types.RepoState{
			Repo:     repoName,
			Revision: j.CommitHash,
		}, j.JobName)
		if err != nil {
			sklog.Error(err)
			return nil, twirp.InternalError("Failed to create job.")
		}
		job.Requested = job.Created
		job.IsForce = true
		if err != nil {
			return nil, twirp.InternalError("Failed to trigger jobs.")
		}
		jobs = append(jobs, job)
	}
	if err := s.db.PutJobsInChunks(jobs); err != nil {
		sklog.Error(err)
		return nil, twirp.InternalError("Failed to insert jobs into DB.")
	}
	ids := make([]string, 0, len(jobs))
	for _, job := range jobs {
		ids = append(ids, job.Id)
	}
	return &TriggerJobsResponse{
		JobIds: ids,
	}, nil
}

// getJob returns the given job.
func (s *taskSchedulerServiceImpl) getJob(ctx context.Context, id string) (*Job, *types.Job, error) {
	if _, err := s.GetViewer(ctx); err != nil {
		return nil, nil, err
	}
	dbJob, err := s.db.GetJobById(id)
	if err == db.ErrNotFound {
		return nil, nil, twirp.NotFoundError("Unknown job")
	} else if err != nil {
		sklog.Error(err)
		return nil, nil, twirp.InternalError("Failed to retrieve job")
	}
	rv, err := convertJob(dbJob)
	if err != nil {
		return nil, nil, err
	}

	// Retrieve the task specs, so that we can include the task dimensions
	// in the results.
	cfg, err := s.taskCfgCache.Get(ctx, dbJob.RepoState)
	if err != nil {
		sklog.Error(err)
		return nil, nil, twirp.InternalError("Failed to retrieve job dependencies")
	}
	taskDimensions := make([]*TaskDimensions, 0, len(rv.Dependencies))
	for _, task := range rv.Dependencies {
		taskSpec, ok := cfg.Tasks[task.Task]
		if !ok {
			err := fmt.Errorf("Job %s (%s) points to unknown task %q at repo state: %+v", rv.Id, rv.Name, task.Task, rv.RepoState)
			sklog.Error(err)
			return nil, nil, twirp.InternalError(err.Error())
		}
		taskDimensions = append(taskDimensions, &TaskDimensions{
			TaskName:   task.Task,
			Dimensions: taskSpec.Dimensions,
		})
	}
	rv.TaskDimensions = taskDimensions

	return rv, dbJob, nil
}

// GetJob returns the given job.
func (s *taskSchedulerServiceImpl) GetJob(ctx context.Context, req *GetJobRequest) (*GetJobResponse, error) {
	job, _, err := s.getJob(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &GetJobResponse{
		Job: job,
	}, nil
}

// CancelJob cancels the given job.
func (s *taskSchedulerServiceImpl) CancelJob(ctx context.Context, req *CancelJobRequest) (*CancelJobResponse, error) {
	if _, err := s.GetEditor(ctx); err != nil {
		return nil, err
	}
	_, job, err := s.getJob(ctx, req.Id)
	if err != nil {
		sklog.Error(err)
		return nil, twirp.InternalError("Failed to retrieve job.")
	}
	if job.Done() {
		err := fmt.Errorf("Job %s is already finished with status %s", req.Id, job.Status)
		return nil, twirp.InvalidArgumentError("id", err.Error())
	}
	job.Finished = time.Now()
	job.Status = types.JOB_STATUS_CANCELED
	if err := s.db.PutJob(job); err != nil {
		sklog.Error(err)
		return nil, twirp.InternalError("Failed to update job")
	}
	rv, _, err := s.getJob(ctx, req.Id)
	return &CancelJobResponse{
		Job: rv,
	}, err
}

// SearchJobs searches the DB and returns jobs matching the given criteria.
func (s *taskSchedulerServiceImpl) SearchJobs(ctx context.Context, req *SearchJobsRequest) (*SearchJobsResponse, error) {
	if _, err := s.GetViewer(ctx); err != nil {
		return nil, err
	}
	bbid := int64(req.BuildbucketBuildId)
	status := types.JobStatus("")
	switch req.Status {
	case JobStatus_JOB_STATUS_IN_PROGRESS:
		status = types.JOB_STATUS_IN_PROGRESS
	case JobStatus_JOB_STATUS_SUCCESS:
		status = types.JOB_STATUS_IN_PROGRESS
	case JobStatus_JOB_STATUS_FAILURE:
		status = types.JOB_STATUS_FAILURE
	case JobStatus_JOB_STATUS_MISHAP:
		status = types.JOB_STATUS_MISHAP
	case JobStatus_JOB_STATUS_CANCELED:
		status = types.JOB_STATUS_CANCELED
	}
	params := &db.JobSearchParams{
		BuildbucketBuildId: &bbid,
		IsForce:            &req.IsForce,
		Name:               req.Name,
		Status:             status,
		TimeStart:          req.TimeStart.AsTime(),
		TimeEnd:            req.TimeEnd.AsTime(),
	}
	if req.RepoState != nil {
		params.RepoState = types.RepoState{
			Repo:     req.RepoState.Repo,
			Revision: req.RepoState.Revision,
		}
		if req.RepoState.Patch != nil {
			params.RepoState.Patch = types.Patch{
				Issue:     req.RepoState.Patch.Issue,
				PatchRepo: req.RepoState.Patch.PatchRepo,
				Patchset:  req.RepoState.Patch.Patchset,
				Server:    req.RepoState.Patch.Server,
			}
		}
	}
	results, err := db.SearchJobs(s.db, params)
	if err != nil {
		sklog.Error(err)
		return nil, twirp.InternalError("Failed to search jobs")
	}
	jobs, err := convertJobs(results)
	if err != nil {
		return nil, err
	}
	return &SearchJobsResponse{
		Jobs: jobs,
	}, nil
}

// getTask returns the given task.
func (s *taskSchedulerServiceImpl) getTask(ctx context.Context, id string) (*Task, *types.Task, error) {
	if _, err := s.GetViewer(ctx); err != nil {
		return nil, nil, err
	}
	dbTask, err := s.db.GetTaskById(id)
	if err == db.ErrNotFound {
		return nil, nil, twirp.NotFoundError("Unknown task")
	} else if err != nil {
		sklog.Error(err)
		return nil, nil, twirp.InternalError("Failed to retrieve task")
	}
	rv, err := convertTask(dbTask)
	if err != nil {
		return nil, nil, err
	}
	return rv, dbTask, nil
}

// GetTask returns the given task.
func (s *taskSchedulerServiceImpl) GetTask(ctx context.Context, req *GetTaskRequest) (*GetTaskResponse, error) {
	task, _, err := s.getTask(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &GetTaskResponse{
		Task: task,
	}, nil
}

// SearchTasks searches the DB and returns tasks matching the given
// criteria.
func (s *taskSchedulerServiceImpl) SearchTasks(ctx context.Context, req *SearchTasksRequest) (*SearchTasksResponse, error) {
	if _, err := s.GetViewer(ctx); err != nil {
		return nil, err
	}
	attempt := int64(req.Attempt)
	status := types.TaskStatus("")
	switch req.Status {
	case TaskStatus_TASK_STATUS_PENDING:
		status = types.TASK_STATUS_PENDING
	case TaskStatus_TASK_STATUS_RUNNING:
		status = types.TASK_STATUS_RUNNING
	case TaskStatus_TASK_STATUS_SUCCESS:
		status = types.TASK_STATUS_SUCCESS
	case TaskStatus_TASK_STATUS_FAILURE:
		status = types.TASK_STATUS_FAILURE
	case TaskStatus_TASK_STATUS_MISHAP:
		status = types.TASK_STATUS_MISHAP
	}
	params := &db.TaskSearchParams{
		Attempt:   &attempt, // TODO(borenet): nil vs empty?
		Status:    status,
		TimeStart: req.TimeStart.AsTime(),
		TimeEnd:   req.TimeEnd.AsTime(),
	}
	if req.TaskKey != nil {
		params.TaskKey = types.TaskKey{
			Name:        req.TaskKey.Name,
			ForcedJobId: req.TaskKey.ForcedJobId,
		}
		if req.TaskKey.RepoState != nil {
			params.RepoState = types.RepoState{
				Repo:     req.TaskKey.RepoState.Repo,
				Revision: req.TaskKey.RepoState.Revision,
			}
			if req.TaskKey.RepoState.Patch != nil {
				params.RepoState.Patch = types.Patch{
					Issue:     req.TaskKey.RepoState.Patch.Issue,
					PatchRepo: req.TaskKey.RepoState.Patch.PatchRepo,
					Patchset:  req.TaskKey.RepoState.Patch.Patchset,
					Server:    req.TaskKey.RepoState.Patch.Server,
				}
			}
		}
	}
	results, err := db.SearchTasks(s.db, params)
	if err != nil {
		sklog.Error(err)
		return nil, twirp.InternalError("Failed to search jobs")
	}
	tasks, err := convertTasks(results)
	if err != nil {
		return nil, err
	}
	return &SearchTasksResponse{
		Tasks: tasks,
	}, nil
}

func (s *taskSchedulerServiceImpl) getSkipTaskRules() []*SkipTaskRule {
	rules := s.skipTasks.GetRules()
	rv := make([]*SkipTaskRule, 0, len(rules))
	for _, rule := range rules {
		rv = append(rv, &SkipTaskRule{
			AddedBy:          rule.AddedBy,
			TaskSpecPatterns: rule.TaskSpecPatterns,
			Commits:          rule.Commits,
			Description:      rule.Description,
			Name:             rule.Name,
		})
	}
	return rv
}

// GetSkipTaskRules returns all active rules for skipping tasks.
func (s *taskSchedulerServiceImpl) GetSkipTaskRules(ctx context.Context, req *GetSkipTaskRulesRequest) (*GetSkipTaskRulesResponse, error) {
	if _, err := s.GetViewer(ctx); err != nil {
		return nil, err
	}
	return &GetSkipTaskRulesResponse{
		Rules: s.getSkipTaskRules(),
	}, nil
}

// AddSkipTaskRule adds a rule for skipping tasks.
func (s *taskSchedulerServiceImpl) AddSkipTaskRule(ctx context.Context, req *AddSkipTaskRuleRequest) (*AddSkipTaskRuleResponse, error) {
	user, err := s.GetEditor(ctx)
	if err != nil {
		return nil, err
	}
	rule := &skip_tasks.Rule{
		AddedBy:          user,
		TaskSpecPatterns: req.TaskSpecPatterns,
		Commits:          req.Commits,
		Description:      req.Description,
		Name:             req.Name,
	}
	if len(rule.Commits) == 2 {
		rangeRule, err := skip_tasks.NewCommitRangeRule(context.Background(), rule.Name, rule.AddedBy, rule.Description, rule.TaskSpecPatterns, rule.Commits[0], rule.Commits[1], s.repos)
		if err != nil {
			sklog.Error(err)
			return nil, twirp.InvalidArgumentError("commits", "Failed to create commit range rule")
		}
		rule = rangeRule
	}
	if err := s.skipTasks.AddRule(rule, s.repos); err != nil {
		sklog.Error(err)
		return nil, twirp.InternalError("Failed to add skip task rule")
	}
	return &AddSkipTaskRuleResponse{
		Rules: s.getSkipTaskRules(),
	}, nil
}

// DeleteSkipTaskRule deletes the given rule for skipping tasks.
func (s *taskSchedulerServiceImpl) DeleteSkipTaskRule(ctx context.Context, req *DeleteSkipTaskRuleRequest) (*DeleteSkipTaskRuleResponse, error) {
	if _, err := s.GetEditor(ctx); err != nil {
		return nil, err
	}
	if err := s.skipTasks.RemoveRule(req.Id); err != nil {
		sklog.Error(err)
		return nil, twirp.InternalError("Failed to remove rule")
	}
	return &DeleteSkipTaskRuleResponse{
		Rules: s.getSkipTaskRules(),
	}, nil
}

// convertRepoState converts a types.RepoState to rpc.RepoState.
func convertRepoState(rs types.RepoState) *RepoState {
	return &RepoState{
		Patch: &RepoState_Patch{
			Issue:     rs.Issue,
			PatchRepo: rs.PatchRepo,
			Patchset:  rs.Patchset,
			Server:    rs.Server,
		},
		Repo:     rs.Repo,
		Revision: rs.Revision,
	}
}

// convertTaskStatus converts a types.TaskStatus to rpc.TaskStatus.
func convertTaskStatus(st types.TaskStatus) (TaskStatus, error) {
	switch st {
	case types.TASK_STATUS_PENDING:
		return TaskStatus_TASK_STATUS_PENDING, nil
	case types.TASK_STATUS_RUNNING:
		return TaskStatus_TASK_STATUS_RUNNING, nil
	case types.TASK_STATUS_SUCCESS:
		return TaskStatus_TASK_STATUS_SUCCESS, nil
	case types.TASK_STATUS_FAILURE:
		return TaskStatus_TASK_STATUS_FAILURE, nil
	case types.TASK_STATUS_MISHAP:
		return TaskStatus_TASK_STATUS_MISHAP, nil
	default:
		return TaskStatus_TASK_STATUS_PENDING, twirp.InternalError("Invalid task status.")
	}
}

// convertTask converts a types.Task to rpc.Task.
func convertTask(task *types.Task) (*Task, error) {
	st, err := convertTaskStatus(task.Status)
	if err != nil {
		return nil, err
	}
	return &Task{
		Attempt:        int32(task.Attempt),
		Commits:        task.Commits,
		CreatedAt:      timestamppb.New(task.Created),
		DbModifiedAt:   timestamppb.New(task.DbModified),
		FinishedAt:     timestamppb.New(task.Finished),
		Id:             task.Id,
		IsolatedOutput: task.IsolatedOutput,
		Jobs:           task.Jobs,
		MaxAttempts:    int32(task.MaxAttempts),
		ParentTaskIds:  task.ParentTaskIds,
		Properties:     task.Properties,
		RetryOf:        task.RetryOf,
		StartedAt:      timestamppb.New(task.Started),
		Status:         st,
		SwarmingBotId:  task.SwarmingBotId,
		SwarmingTaskId: task.SwarmingTaskId,
		TaskKey: &TaskKey{
			RepoState:   convertRepoState(task.RepoState),
			Name:        task.Name,
			ForcedJobId: task.ForcedJobId,
		},
	}, nil
}

// convertTasks converts a slice of types.Task to rpc.Task.
func convertTasks(tasks []*types.Task) ([]*Task, error) {
	rv := make([]*Task, 0, len(tasks))
	for _, task := range tasks {
		t, err := convertTask(task)
		if err != nil {
			return nil, err
		}
		rv = append(rv, t)
	}
	return rv, nil
}

// convertJobStatus converts a types.JobStatus to rpc.JobStatus.
func convertJobStatus(st types.JobStatus) (JobStatus, error) {
	switch st {
	case types.JOB_STATUS_IN_PROGRESS:
		return JobStatus_JOB_STATUS_IN_PROGRESS, nil
	case types.JOB_STATUS_SUCCESS:
		return JobStatus_JOB_STATUS_SUCCESS, nil
	case types.JOB_STATUS_FAILURE:
		return JobStatus_JOB_STATUS_FAILURE, nil
	case types.JOB_STATUS_MISHAP:
		return JobStatus_JOB_STATUS_MISHAP, nil
	case types.JOB_STATUS_CANCELED:
		return JobStatus_JOB_STATUS_CANCELED, nil
	default:
		return JobStatus_JOB_STATUS_IN_PROGRESS, twirp.InternalError("Invalid job status.")
	}
}

// convertJob converts a types.Job to rpc.Job.
func convertJob(job *types.Job) (*Job, error) {
	depNames := make([]string, 0, len(job.Dependencies))
	for name := range job.Dependencies {
		depNames = append(depNames, name)
	}
	sort.Strings(depNames)
	deps := make([]*TaskDependencies, 0, len(job.Dependencies))
	for _, name := range depNames {
		taskDeps := job.Dependencies[name]
		deps = append(deps, &TaskDependencies{
			Task:         name,
			Dependencies: taskDeps,
		})
	}
	// Sort the deps by task name for determinism.
	sort.Slice(deps, func(i, j int) bool {
		return deps[i].Task < deps[j].Task
	})
	status, err := convertJobStatus(job.Status)
	if err != nil {
		return nil, err
	}
	taskNames := make([]string, 0, len(job.Tasks))
	for name := range job.Tasks {
		taskNames = append(taskNames, name)
	}
	sort.Strings(taskNames)
	tasks := make([]*TaskSummaries, 0, len(job.Tasks))
	for _, name := range taskNames {
		taskSummaries := job.Tasks[name]
		ts := make([]*TaskSummary, 0, len(tasks))
		for _, taskSummary := range taskSummaries {
			st, err := convertTaskStatus(taskSummary.Status)
			if err != nil {
				return nil, err
			}
			ts = append(ts, &TaskSummary{
				Id:             taskSummary.Id,
				Attempt:        int32(taskSummary.Attempt),
				MaxAttempts:    int32(taskSummary.MaxAttempts),
				Status:         st,
				SwarmingTaskId: taskSummary.SwarmingTaskId,
			})
		}
		tasks = append(tasks, &TaskSummaries{
			Name:  name,
			Tasks: ts,
		})
	}
	return &Job{
		BuildbucketBuildId:  job.BuildbucketBuildId,
		BuildbucketLeaseKey: job.BuildbucketLeaseKey,
		CreatedAt:           timestamppb.New(job.Created),
		DbModifiedAt:        timestamppb.New(job.DbModified),
		Dependencies:        deps,
		FinishedAt:          timestamppb.New(job.Finished),
		Id:                  job.Id,
		IsForce:             job.IsForce,
		Name:                job.Name,
		Priority:            float32(job.Priority),
		RepoState:           convertRepoState(job.RepoState),
		RequestedAt:         timestamppb.New(job.Requested),
		Status:              status,
		Tasks:               tasks,
	}, nil
}

// convertJobs converts a slice of types.Job to rpc.Job.
func convertJobs(jobs []*types.Job) ([]*Job, error) {
	rv := make([]*Job, 0, len(jobs))
	for _, job := range jobs {
		j, err := convertJob(job)
		if err != nil {
			return nil, err
		}
		rv = append(rv, j)
	}
	return rv, nil
}

var _ TaskSchedulerService = &taskSchedulerServiceImpl{}
