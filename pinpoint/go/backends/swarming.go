package backends

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	swarmingv2 "go.skia.org/infra/go/swarming/v2"
	"go.skia.org/infra/pinpoint/go/bot_configs"
	"google.golang.org/protobuf/types/known/timestamppb"

	"golang.org/x/oauth2/google"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
)

const (
	DefaultSwarmingServiceAddress = "chrome-swarming.appspot.com:443"
	RunBenchmarkFailure           = "BENCHMARK_FAILURE" // the task completed but benchmark run failed
)

// SwarmingClient
type SwarmingClient interface {
	// CancelTasks tells Swarming to cancel the given tasks.
	CancelTasks(ctx context.Context, taskIDs []string) error

	// GetCASOutput returns the CAS output of a swarming task.
	GetCASOutput(ctx context.Context, taskID string) (*apipb.CASReference, error)

	// GetStates returns the state of each task in a list of tasks.
	GetStates(ctx context.Context, taskIDs []string) ([]string, error)

	// GetStatus gets the current status of a swarming task.
	GetStatus(ctx context.Context, taskID string) (string, error)

	// ListPinpointTasks lists the Pinpoint swarming tasks.
	ListPinpointTasks(ctx context.Context, jobID string, buildArtifact *apipb.CASReference) ([]string, error)

	// TriggerTask is a literal wrapper around swarming.ApiClient TriggerTask
	// TODO(jeffyoon@) remove once run_benchmark is refactored if no longer needed.
	TriggerTask(ctx context.Context, req *apipb.NewTaskRequest) (*apipb.TaskRequestMetadataResponse, error)

	// FetchFreeBots gets a list of available bots per specified builder configuration.
	FetchFreeBots(ctx context.Context, builder string) ([]*apipb.BotInfo, error)
}

// SwarmingClientImpl
// TODO(jeffyoon@) make this private once run_benchmark doesn't rely on this in testing.
type SwarmingClientImpl struct {
	swarmingv2.SwarmingV2Client
}

func NewSwarmingClient(ctx context.Context, server string) (*SwarmingClientImpl, error) {
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, skerr.Wrapf(err, "Problem setting up default token source")
	}
	c := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).With2xxOnly().Client()
	sc := swarmingv2.NewDefaultClient(c, server)
	return &SwarmingClientImpl{
		SwarmingV2Client: sc,
	}, nil
}

// CancelTasks tells Swarming to cancel the given tasks.
func (s *SwarmingClientImpl) CancelTasks(ctx context.Context, taskIDs []string) error {
	for _, id := range taskIDs {
		_, err := s.CancelTask(ctx, &apipb.TaskCancelRequest{
			TaskId:      id,
			KillRunning: true,
		})
		if err != nil {
			return skerr.Fmt("Could not cancel task %s due to %s", id, err)
		}
	}
	return nil
}

// GetCASOutput returns the CAS output of a swarming task in the form of a RBE CAS hash.
// This function assumes the task is finished, or it throws an error.
func (s *SwarmingClientImpl) GetCASOutput(ctx context.Context, taskID string) (*apipb.CASReference, error) {
	task, err := s.GetResult(ctx, &apipb.TaskIdWithPerfRequest{
		TaskId:                  taskID,
		IncludePerformanceStats: false,
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "could not retrieve CAS of task %s", taskID)
	}
	if task.State != apipb.TaskState_COMPLETED {
		return nil, skerr.Fmt("cannot get result of task %s because it is %s and not COMPLETED", taskID, task.State)
	}
	return task.CasOutputRoot, nil
}

func (s *SwarmingClientImpl) GetStates(ctx context.Context, taskIDs []string) ([]string, error) {
	resp, err := s.SwarmingV2Client.ListTaskStates(ctx, &apipb.TaskStatesRequest{
		TaskId: taskIDs,
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rv := make([]string, 0, len(resp.States))
	for _, state := range resp.States {
		rv = append(rv, state.String())
	}
	return rv, nil
}

// GetStatus gets the current status of a swarming task.
func (s *SwarmingClientImpl) GetStatus(ctx context.Context, taskID string) (string, error) {
	res, err := s.GetResult(ctx, &apipb.TaskIdWithPerfRequest{
		TaskId:                  taskID,
		IncludePerformanceStats: false,
	})
	if err != nil {
		return "", skerr.Fmt("failed to get swarming task ID %s due to err: %v", taskID, err)
	}
	// Swarming tasks can COMPLETE but fail. In this case, we need to
	// differentiate a successful task from an unsuccessful task.
	if res.State == apipb.TaskState_COMPLETED && res.Failure {
		return RunBenchmarkFailure, nil
	}
	return res.State.String(), nil
}

// ListPinpointTasks lists the Pinpoint swarming tasks of a given job and build identified by Swarming tags.
func (s *SwarmingClientImpl) ListPinpointTasks(ctx context.Context, jobID string, buildArtifact *apipb.CASReference) ([]string, error) {
	if jobID == "" {
		return nil, skerr.Fmt("Cannot list tasks because request is missing JobID")
	}
	if buildArtifact == nil || buildArtifact.Digest == nil {
		return nil, skerr.Fmt("Cannot list tasks because request is missing cas isolate")
	}
	start := time.Now().Add(-24 * time.Hour)
	tags := []string{
		fmt.Sprintf("pinpoint_job_id:%s", jobID),
		fmt.Sprintf("build_cas:%s/%d", buildArtifact.Digest.Hash, buildArtifact.Digest.SizeBytes),
	}
	tasks, err := swarmingv2.ListTasksHelper(ctx, s.SwarmingV2Client, &apipb.TasksWithPerfRequest{
		Start: timestamppb.New(start),
		Tags:  tags,
		State: apipb.StateQuery_QUERY_ALL,
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "error retrieving tasks")
	}
	taskIDs := make([]string, len(tasks))
	for i, t := range tasks {
		taskIDs[i] = t.TaskId
	}
	return taskIDs, nil
}

func (s *SwarmingClientImpl) TriggerTask(ctx context.Context, req *apipb.NewTaskRequest) (*apipb.TaskRequestMetadataResponse, error) {
	return s.SwarmingV2Client.NewTask(ctx, req)
}

// FetchFreeBots gets a list of available bots per specified builder configuration.
func (s *SwarmingClientImpl) FetchFreeBots(ctx context.Context, builder string) ([]*apipb.BotInfo, error) {
	var botConfig bot_configs.BotConfig
	botConfig, err := bot_configs.GetBotConfig(builder, false)
	if err != nil {
		return nil, skerr.Wrapf(err, "error retrieving bots")
	}

	dims := map[string]string{}
	for _, d := range botConfig.Dimensions {
		dims[d["key"]] = d["value"]
	}

	bots, err := swarmingv2.ListBotsHelper(ctx, s.SwarmingV2Client, &apipb.BotsRequest{
		Dimensions: swarmingv2.StringMapToTaskDimensions(dims),
	})

	return bots, skerr.Wrap(err)
}
