package buildbucket_taskbackend

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	buildbucketgrpcpb "go.chromium.org/luci/buildbucket/proto/grpcpb"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
	"google.golang.org/genproto/googleapis/rpc/status"
)

// TaskBackend implements TaskBackendServer in terms of Task Scheduler Jobs.
type TaskBackend struct {
	buildbucketgrpcpb.UnimplementedTaskBackendServer
	bb2                buildbucket.BuildBucketInterface
	buildbucketTarget  string
	db                 db.JobDB
	projectRepoMapping map[string]string
	taskSchedulerHost  string
}

// NewTaskBackend returns a TaskBackend instance.
func NewTaskBackend(buildbucketTarget, taskSchedulerHost string, projectRepoMapping map[string]string, d db.JobDB, bb2 buildbucket.BuildBucketInterface) *TaskBackend {
	return &TaskBackend{
		bb2:                bb2,
		buildbucketTarget:  buildbucketTarget,
		db:                 d,
		projectRepoMapping: projectRepoMapping,
		taskSchedulerHost:  taskSchedulerHost,
	}
}

// RunTask implements TaskBackendServer.
func (tb *TaskBackend) RunTask(ctx context.Context, req *buildbucketpb.RunTaskRequest) (*buildbucketpb.RunTaskResponse, error) {
	// Validation.
	if req.Target != tb.buildbucketTarget {
		return nil, skerr.Fmt("incorrect target for this scheduler; expected %s", tb.buildbucketTarget)
	}
	if req.Secrets == nil {
		return nil, skerr.Fmt("secrets not set on request")
	}
	if req.Secrets.StartBuildToken == "" {
		return nil, skerr.Fmt("missing StartBuildToken")
	}
	buildId, err := strconv.ParseInt(req.BuildId, 10, 64)
	if err != nil {
		return nil, skerr.Wrapf(err, "invalid build ID")
	}

	// Look for any Jobs which we might have already created for this Build.
	duplicates, err := tb.db.SearchJobs(ctx, &db.JobSearchParams{
		BuildbucketBuildID: &buildId,
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "failed looking for duplicate jobs")
	}
	if len(duplicates) > 0 {
		return &buildbucketpb.RunTaskResponse{
			Task: JobToBuildbucketTask(ctx, duplicates[0], tb.buildbucketTarget, tb.taskSchedulerHost),
		}, nil
	}

	// Get the build details from the v2 API.
	// TODO(borenet): It would be much better to avoid sending an extra request
	// back to Buildbucket. It seems like this information should be part of the
	// RunTaskRequest - maybe it's already in RunTaskRequest.BackendConfig?
	build, err := tb.bb2.GetBuild(ctx, buildId)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to retrieve build %d", buildId)
	}
	if build.Builder == nil {
		return nil, skerr.Fmt("builder isn't set on build %d", buildId)
	}

	// Obtain and validate the RepoState.
	if build.Input == nil || build.Input.GerritChanges == nil || len(build.Input.GerritChanges) != 1 {
		return nil, skerr.Fmt("invalid Build %d: input should have exactly one GerritChanges: %+v", buildId, build.Input)
	}
	gerritChange := build.Input.GerritChanges[0]
	repoUrl, ok := tb.projectRepoMapping[gerritChange.Project]
	if !ok {
		return nil, skerr.Fmt("unknown patch project %q", gerritChange.Project)
	}
	server := gerritChange.Host
	if !strings.Contains(server, "://") {
		server = fmt.Sprintf("https://%s", server)
	}
	rs := types.RepoState{
		Patch: types.Patch{
			Server:    server,
			Issue:     strconv.FormatInt(gerritChange.Change, 10),
			PatchRepo: repoUrl,
			Patchset:  strconv.FormatInt(gerritChange.Patchset, 10),
		},
		Repo: repoUrl,
		// We can't fill this out without retrieving the Gerrit ChangeInfo and
		// resolving the branch to a commit hash. Defer that work until later.
		Revision: "",
	}

	// Create the Job.
	j := &types.Job{
		Name:                   build.Builder.Builder,
		BuildbucketBuildId:     buildId,
		BuildbucketPubSubTopic: req.PubsubTopic,
		BuildbucketToken:       req.Secrets.StartBuildToken,
		Requested:              firestore.FixTimestamp(build.CreateTime.AsTime().UTC()),
		Created:                firestore.FixTimestamp(now.Now(ctx)),
		RepoState:              rs,
		Status:                 types.JOB_STATUS_REQUESTED,
	}
	if !j.Requested.Before(j.Created) {
		sklog.Errorf("Try job created time %s is before requested time %s! Setting equal.", j.Created, j.Requested)
		j.Requested = j.Created.Add(-firestore.TS_RESOLUTION)
	}

	// Insert the Job into the DB.
	if err := tb.db.PutJob(ctx, j); err != nil {
		return nil, skerr.Wrapf(err, "failed to insert Job into the DB")
	}
	return &buildbucketpb.RunTaskResponse{
		Task: JobToBuildbucketTask(ctx, j, tb.buildbucketTarget, tb.taskSchedulerHost),
	}, nil
}

// FetchTasks implements TaskBackendServer.
func (tb *TaskBackend) FetchTasks(ctx context.Context, req *buildbucketpb.FetchTasksRequest) (*buildbucketpb.FetchTasksResponse, error) {
	resps := make([]*buildbucketpb.FetchTasksResponse_Response, 0, len(req.TaskIds))
	for _, id := range req.TaskIds {
		resp := &buildbucketpb.FetchTasksResponse_Response{}
		if id.Target != tb.buildbucketTarget {
			resp.Response = &buildbucketpb.FetchTasksResponse_Response_Error{
				Error: &status.Status{
					Code:    http.StatusBadRequest,
					Message: fmt.Sprintf("incorrect target for this scheduler; expected %s", tb.buildbucketTarget),
				},
			}
			resps = append(resps, resp)
			continue
		}
		job, err := tb.db.GetJobById(ctx, id.Id)
		if err != nil {
			resp.Response = &buildbucketpb.FetchTasksResponse_Response_Error{
				Error: &status.Status{
					Code:    http.StatusInternalServerError,
					Message: err.Error(),
				},
			}
			resps = append(resps, resp)
			continue
		} else if job == nil {
			resp.Response = &buildbucketpb.FetchTasksResponse_Response_Error{
				Error: &status.Status{
					Code:    http.StatusNotFound,
					Message: "unknown task",
				},
			}
			resps = append(resps, resp)
			continue
		}
		resp.Response = &buildbucketpb.FetchTasksResponse_Response_Task{
			Task: JobToBuildbucketTask(ctx, job, tb.buildbucketTarget, tb.taskSchedulerHost),
		}
		resps = append(resps, resp)
	}
	return &buildbucketpb.FetchTasksResponse{
		Responses: resps,
	}, nil
}

// CancelTasks implements TaskBackendServer.
func (tb *TaskBackend) CancelTasks(ctx context.Context, req *buildbucketpb.CancelTasksRequest) (*buildbucketpb.CancelTasksResponse, error) {
	// Note: According to the Buildbucket docs, we're supposed to be ensuring
	// that the tasks are fully canceled (ie. any underlying work is no longer
	// running) before we return with a "canceled" status. We have the
	// capability of canceling Swarming tasks, but given the complexity involved
	// (we might, for example be sharing a given Swarming task between multiple
	// jobs), I'm not sure we actually want to do that.
	jobs := make([]*types.Job, 0, len(req.TaskIds))
	for _, id := range req.TaskIds {
		if id.Target != tb.buildbucketTarget {
			return nil, skerr.Fmt("incorrect target for this scheduler; expected %s", tb.buildbucketTarget)
		}
		job, err := tb.db.GetJobById(ctx, id.Id)
		if err != nil {
			return nil, skerr.Wrap(err)
		} else if job == nil {
			return nil, skerr.Fmt("unknown job %q", id.Id)
		}
		jobs = append(jobs, job)
	}
	finished := now.Now(ctx)
	updated := make([]*types.Job, 0, len(jobs))
	for _, job := range jobs {
		if !job.Done() {
			job.Finished = finished
			job.Status = types.JOB_STATUS_CANCELED
			job.StatusDetails = "Canceled by Buildbucket"
			updated = append(updated, job)
		}
	}
	if len(updated) > 0 {
		if err := tb.db.PutJobs(ctx, updated); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	resps := make([]*buildbucketpb.Task, 0, len(req.TaskIds))
	for _, job := range jobs {
		resps = append(resps, JobToBuildbucketTask(ctx, job, tb.buildbucketTarget, tb.taskSchedulerHost))
	}
	return &buildbucketpb.CancelTasksResponse{
		Tasks: resps,
	}, nil
}

// ValidateConfigs implements TaskBackendServer.
func (tb *TaskBackend) ValidateConfigs(ctx context.Context, req *buildbucketpb.ValidateConfigsRequest) (*buildbucketpb.ValidateConfigsResponse, error) {
	// TODO(borenet): I'm not sure what we're actually supposed to be validating
	// in this method.
	var errs []*buildbucketpb.ValidateConfigsResponse_ErrorDetail
	for idx, cfg := range req.Configs {
		if cfg.Target != tb.buildbucketTarget {
			errs = append(errs, &buildbucketpb.ValidateConfigsResponse_ErrorDetail{
				Index: int32(idx),
				Error: fmt.Sprintf("incorrect target for this scheduler; expected %s", tb.buildbucketTarget),
			})
		}
	}
	return &buildbucketpb.ValidateConfigsResponse{
		ConfigErrors: errs,
	}, nil
}

// Assert that TaskBackend implements TaskBackendServer.
var _ buildbucketgrpcpb.TaskBackendServer = &TaskBackend{}

// JobStatusToBuildbucketStatus converts a types.JobStatus to a
// buildbucketpb.Status.
func JobStatusToBuildbucketStatus(status types.JobStatus) buildbucketpb.Status {
	switch status {
	case types.JOB_STATUS_CANCELED:
		return buildbucketpb.Status_CANCELED
	case types.JOB_STATUS_FAILURE:
		return buildbucketpb.Status_FAILURE
	case types.JOB_STATUS_IN_PROGRESS:
		return buildbucketpb.Status_STARTED
	case types.JOB_STATUS_MISHAP:
		return buildbucketpb.Status_INFRA_FAILURE
	case types.JOB_STATUS_REQUESTED:
		return buildbucketpb.Status_SCHEDULED
	case types.JOB_STATUS_SUCCESS:
		return buildbucketpb.Status_SUCCESS
	default:
		return buildbucketpb.Status_STATUS_UNSPECIFIED
	}
}

// JobToBuildbucketTask converts a types.Job to a buildbucketpb.Task.
func JobToBuildbucketTask(ctx context.Context, job *types.Job, buildbucketTarget, taskSchedulerHost string) *buildbucketpb.Task {
	return &buildbucketpb.Task{
		Id: &buildbucketpb.TaskID{
			Target: buildbucketTarget,
			Id:     job.Id,
		},
		Link:            job.URL(taskSchedulerHost),
		Status:          JobStatusToBuildbucketStatus(job.Status),
		SummaryMarkdown: job.StatusDetails,
		UpdateId:        now.Now(ctx).UnixNano(),
	}
}
