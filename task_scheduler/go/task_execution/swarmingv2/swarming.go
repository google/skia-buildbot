package swarmingv2

import (
	"context"
	"fmt"
	"strings"
	"time"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/swarming"
	swarmingv2 "go.skia.org/infra/go/swarming/v2"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// swarmingUser is the user associated with Swarming tasks triggered by
	// this package.
	swarmingUser = "skiabot@google.com"
)

// SwarmingV2TaskExecutor implements types.TaskExecutor.
type SwarmingV2TaskExecutor struct {
	casInstance string
	pubSubTopic string
	realm       string
	client      swarmingv2.SwarmingV2Client
}

// NewSwarmingV2TaskExecutor returns a SwarmingTaskExecutor instance.
func NewSwarmingV2TaskExecutor(client swarmingv2.SwarmingV2Client, casInstance, pubSubTopic, realm string) *SwarmingV2TaskExecutor {
	return &SwarmingV2TaskExecutor{
		casInstance: casInstance,
		pubSubTopic: pubSubTopic,
		realm:       realm,
		client:      client,
	}
}

// GetFreeMachines implements types.TaskExecutor.
func (s *SwarmingV2TaskExecutor) GetFreeMachines(ctx context.Context, pool string) ([]*types.Machine, error) {
	ctx, span := trace.StartSpan(ctx, "swarming_GetFreeMachines")
	span.AddAttributes(trace.StringAttribute("pool", pool))
	defer span.End()

	free, err := swarmingv2.ListBotsHelper(ctx, s.client, &apipb.BotsRequest{
		Dimensions: []*apipb.StringPair{
			{Key: "pool", Value: pool},
		},
		IsBusy:        apipb.NullableBool_FALSE,
		IsDead:        apipb.NullableBool_FALSE,
		InMaintenance: apipb.NullableBool_FALSE,
		Quarantined:   apipb.NullableBool_FALSE,
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rv := make([]*types.Machine, 0, len(free))
	for _, bot := range free {
		rv = append(rv, convertMachine(bot))
	}
	return rv, nil
}

// GetPendingTasks implements types.TaskExecutor.
func (s *SwarmingV2TaskExecutor) GetPendingTasks(ctx context.Context, pool string) ([]*types.TaskResult, error) {
	ctx, span := trace.StartSpan(ctx, "swarming_GetPendingTasks")
	span.AddAttributes(trace.StringAttribute("pool", pool))
	defer span.End()

	// We want to put a bound on how far Swarming has to search to get our request, otherwise Swarming can timeout,
	// which stops the whole scheduling loop. 2 days was arbitrarily chosen as a result that is higher than the
	// pending timeout we use for Swarming (typically 4 hours).
	end := now.Now(ctx)
	start := end.Add(-2 * 24 * time.Hour)
	tasks, err := swarmingv2.ListTasksHelper(ctx, s.client, &apipb.TasksWithPerfRequest{
		Start:                   timestamppb.New(start),
		End:                     timestamppb.New(end),
		State:                   apipb.StateQuery_QUERY_PENDING,
		Tags:                    []string{fmt.Sprintf("pool:%s", pool)},
		IncludePerformanceStats: false,
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rv := make([]*types.TaskResult, 0, len(tasks))
	for _, task := range tasks {
		conv, err := convertTaskResult(task)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, conv)
	}
	return rv, nil
}

// GetTaskResult implements types.TaskExecutor.
func (s *SwarmingV2TaskExecutor) GetTaskResult(ctx context.Context, taskID string) (*types.TaskResult, error) {
	ctx, span := trace.StartSpan(ctx, "swarming_GetTaskResult", trace.WithSampler(trace.ProbabilitySampler(0.01)))
	defer span.End()
	swarmTask, err := s.client.GetResult(ctx, &apipb.TaskIdWithPerfRequest{
		TaskId:                  taskID,
		IncludePerformanceStats: false,
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	conv, err := convertTaskResult(swarmTask)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return conv, nil
}

// GetTaskCompletionStatuses implements types.TaskExecutor.
func (s *SwarmingV2TaskExecutor) GetTaskCompletionStatuses(ctx context.Context, taskIDs []string) ([]bool, error) {
	ctx, span := trace.StartSpan(ctx, "swarming_GetTaskCompletionStatuses")
	span.AddAttributes(trace.Int64Attribute("num_tasks", int64(len(taskIDs))))
	defer span.End()
	resp, err := s.client.ListTaskStates(ctx, &apipb.TaskStatesRequest{
		TaskId: taskIDs,
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rv := make([]bool, 0, len(resp.States))
	for _, state := range resp.States {
		conv, err := convertTaskStatus(state, false)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		finished := true
		if conv == types.TASK_STATUS_PENDING || conv == types.TASK_STATUS_RUNNING {
			finished = false
		}
		rv = append(rv, finished)
	}
	return rv, nil
}

// TriggerTask implements types.TaskExecutor.
func (s *SwarmingV2TaskExecutor) TriggerTask(ctx context.Context, req *types.TaskRequest) (*types.TaskResult, error) {
	ctx, span := trace.StartSpan(ctx, "swarming_TriggerTask")
	defer span.End()
	sReq, err := s.convertTaskRequest(req)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	resp, err := s.client.NewTask(ctx, sReq)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if resp.TaskResult != nil {
		if resp.TaskResult.State == apipb.TaskState_NO_RESOURCE {
			return nil, skerr.Fmt("No bots available to run %s with dimensions: %s", req.Name, strings.Join(req.Dimensions, ", "))
		}
		return convertTaskResult(resp.TaskResult)
	}
	var created time.Time
	if resp.Request != nil && resp.Request.CreatedTs != nil {
		created = resp.Request.CreatedTs.AsTime()
	}
	t := &types.TaskResult{
		ID:      resp.TaskId,
		Created: created,
		Status:  types.TASK_STATUS_PENDING,
	}
	return t, nil
}

// convertTaskRequest converts a types.TaskRequest to a
// apipb.NewTaskRequest.
func (s *SwarmingV2TaskExecutor) convertTaskRequest(req *types.TaskRequest) (*apipb.NewTaskRequest, error) {
	var caches []*apipb.CacheEntry
	if len(req.Caches) > 0 {
		caches = make([]*apipb.CacheEntry, 0, len(req.Caches))
		for _, cache := range req.Caches {
			caches = append(caches, &apipb.CacheEntry{
				Name: cache.Name,
				Path: cache.Path,
			})
		}
	}
	casInput, err := swarmingv2.MakeCASReference(req.CasInput, s.casInstance)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var cipdInput *apipb.CipdInput
	if len(req.CipdPackages) > 0 {
		cipdInput = &apipb.CipdInput{
			Packages: make([]*apipb.CipdPackage, 0, len(req.CipdPackages)),
		}
		for _, p := range req.CipdPackages {
			cipdInput.Packages = append(cipdInput.Packages, &apipb.CipdPackage{
				PackageName: p.Name,
				Path:        p.Path,
				Version:     p.Version,
			})
		}
	}

	var dims []*apipb.StringPair
	if len(req.Dimensions) > 0 {
		dims = make([]*apipb.StringPair, 0, len(req.Dimensions))
		for _, d := range req.Dimensions {
			split := strings.SplitN(d, ":", 2)
			key := split[0]
			val := split[1]
			dims = append(dims, &apipb.StringPair{
				Key:   key,
				Value: val,
			})
		}
	}

	var env []*apipb.StringPair
	if len(req.Env) > 0 {
		env = make([]*apipb.StringPair, 0, len(req.Env))
		for k, v := range req.Env {
			env = append(env, &apipb.StringPair{
				Key:   k,
				Value: v,
			})
		}
	}

	var envPrefixes []*apipb.StringListPair
	if len(req.EnvPrefixes) > 0 {
		envPrefixes = make([]*apipb.StringListPair, 0, len(req.EnvPrefixes))
		for k, v := range req.EnvPrefixes {
			envPrefixes = append(envPrefixes, &apipb.StringListPair{
				Key:   k,
				Value: util.CopyStringSlice(v),
			})
		}
	}

	expirationSecs := int32(req.Expiration.Seconds())
	if expirationSecs <= int32(0) {
		expirationSecs = int32(swarming.RECOMMENDED_EXPIRATION.Seconds())
	}
	executionTimeoutSecs := int32(req.ExecutionTimeout.Seconds())
	if executionTimeoutSecs <= int32(0) {
		executionTimeoutSecs = int32(swarming.RECOMMENDED_HARD_TIMEOUT.Seconds())
	}
	ioTimeoutSecs := int32(req.IoTimeout.Seconds())
	if ioTimeoutSecs <= int32(0) {
		ioTimeoutSecs = int32(swarming.RECOMMENDED_IO_TIMEOUT.Seconds())
	}
	outputs := util.CopyStringSlice(req.Outputs)
	rv := &apipb.NewTaskRequest{
		Name:           req.Name,
		Priority:       swarming.RECOMMENDED_PRIORITY,
		PubsubTopic:    fmt.Sprintf(swarming.PUBSUB_FULLY_QUALIFIED_TOPIC_TMPL, common.PROJECT_ID, s.pubSubTopic),
		PubsubUserdata: req.TaskSchedulerTaskID,
		Realm:          s.realm,
		ServiceAccount: req.ServiceAccount,
		Tags:           req.Tags,
		TaskSlices: []*apipb.TaskSlice{
			{
				ExpirationSecs: expirationSecs,
				Properties: &apipb.TaskProperties{
					Caches:               caches,
					CasInputRoot:         casInput,
					CipdInput:            cipdInput,
					Command:              req.Command,
					Dimensions:           dims,
					Env:                  env,
					EnvPrefixes:          envPrefixes,
					ExecutionTimeoutSecs: executionTimeoutSecs,
					Idempotent:           req.Idempotent,
					IoTimeoutSecs:        ioTimeoutSecs,
					Outputs:              outputs,
				},
				WaitForCapacity: false,
			},
		},
		User: swarmingUser,
	}
	return rv, nil
}

// convertTaskResult converts a apipb.TaskResultResponse to a
// types.TaskResult.
func convertTaskResult(res *apipb.TaskResultResponse) (*types.TaskResult, error) {
	var casOutput string
	if res.CasOutputRoot != nil && res.CasOutputRoot.Digest.Hash != "" {
		casOutput = rbe.DigestToString(res.CasOutputRoot.Digest.Hash, res.CasOutputRoot.Digest.SizeBytes)
	}

	status, err := convertTaskStatus(res.State, res.Failure)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	tags, err := swarming.ParseTags(res.Tags)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Note: timestamppb.Timestamp.AsTime() works for a nil Timestamp, but it
	// uses time.Unix() to create the time.Time which differs from time.Time{}.
	// The if-statements here help preserve the zero-value of time.Time.
	var created time.Time
	if res.CreatedTs != nil {
		created = res.CreatedTs.AsTime()
	}
	var started time.Time
	if res.StartedTs != nil {
		started = res.StartedTs.AsTime()
	}
	var finished time.Time
	if !util.TimeIsZero(res.CompletedTs.AsTime()) {
		finished = res.CompletedTs.AsTime().UTC()
	} else if status == types.TASK_STATUS_MISHAP && !util.TimeIsZero(res.AbandonedTs.AsTime()) {
		finished = res.AbandonedTs.AsTime().UTC()
	}

	return &types.TaskResult{
		CasOutput: casOutput,
		Created:   created,
		Finished:  finished,
		ID:        res.TaskId,
		MachineID: res.BotId,
		Started:   started,
		Status:    status,
		Tags:      tags,
	}, nil
}

// convertTaskStatus converts a Swarming task state to a types.TaskStatus.
func convertTaskStatus(state apipb.TaskState, failure bool) (types.TaskStatus, error) {
	switch state {
	case apipb.TaskState_BOT_DIED, apipb.TaskState_CANCELED, apipb.TaskState_CLIENT_ERROR, apipb.TaskState_EXPIRED, apipb.TaskState_NO_RESOURCE, apipb.TaskState_TIMED_OUT, apipb.TaskState_KILLED, apipb.TaskState_INVALID:
		return types.TASK_STATUS_MISHAP, nil
	case apipb.TaskState_PENDING:
		return types.TASK_STATUS_PENDING, nil
	case apipb.TaskState_RUNNING:
		return types.TASK_STATUS_RUNNING, nil
	case apipb.TaskState_COMPLETED:
		if failure {
			// TODO(borenet): Choose FAILURE or MISHAP depending on ExitCode?
			return types.TASK_STATUS_FAILURE, nil
		}
		return types.TASK_STATUS_SUCCESS, nil
	default:
		return types.TASK_STATUS_MISHAP, skerr.Fmt("Unknown Swarming State %v", state)
	}
}

// convertMachine converts a apipb.BotInfo to a
// types.Machine.
func convertMachine(bot *apipb.BotInfo) *types.Machine {
	return &types.Machine{
		ID:            bot.BotId,
		Dimensions:    swarmingv2.BotDimensionsToStringSlice(bot.Dimensions),
		IsDead:        bot.IsDead,
		IsQuarantined: bot.Quarantined,
		CurrentTaskID: bot.TaskId,
	}
}

var _ types.TaskExecutor = &SwarmingV2TaskExecutor{}
