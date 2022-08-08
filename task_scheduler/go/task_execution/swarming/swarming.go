package swarming

import (
	"context"
	"fmt"
	"strings"
	"time"

	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.opencensus.io/trace"

	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/timeout"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/types"
)

const (
	// SwarmingUser is the user associated with Swarming tasks triggered by
	// this package.
	SwarmingUser = "skiabot@google.com"
)

// SwarmingTaskExecutor implements types.TaskExecutor.
type SwarmingTaskExecutor struct {
	swarming swarming.ApiClient

	casInstance string
	pubSubTopic string
}

// NewSwarmingTaskExecutor returns a SwarmingTaskExecutor instance.
func NewSwarmingTaskExecutor(s swarming.ApiClient, casInstance, pubSubTopic string) *SwarmingTaskExecutor {
	return &SwarmingTaskExecutor{
		swarming:    s,
		casInstance: casInstance,
		pubSubTopic: pubSubTopic,
	}
}

// GetFreeMachines implements types.TaskExecutor.
func (s *SwarmingTaskExecutor) GetFreeMachines(ctx context.Context, pool string) ([]*types.Machine, error) {
	ctx, span := trace.StartSpan(ctx, "swarming_GetFreeMachines")
	span.AddAttributes(trace.StringAttribute("pool", pool))
	defer span.End()
	free, err := s.swarming.ListFreeBots(ctx, pool)
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
func (s *SwarmingTaskExecutor) GetPendingTasks(ctx context.Context, pool string) ([]*types.TaskResult, error) {
	ctx, span := trace.StartSpan(ctx, "swarming_GetPendingTasks")
	span.AddAttributes(trace.StringAttribute("pool", pool))
	defer span.End()
	// We want to put a bound on how far Swarming has to search to get our request, otherwise Swarming can timeout,
	// which stops the whole scheduling loop. 2 days was arbitrarily chosen as a result that is higher than the
	// pending timeout we use for Swarming (typically 4 hours).
	end := now.Now(ctx)
	start := end.Add(-2 * 24 * time.Hour)
	tasks, err := s.swarming.ListTaskResults(ctx, start, end, []string{fmt.Sprintf("pool:%s", pool)}, swarming.TASK_STATE_PENDING, false)
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
func (s *SwarmingTaskExecutor) GetTaskResult(ctx context.Context, taskID string) (*types.TaskResult, error) {
	ctx, span := trace.StartSpan(ctx, "swarming_GetTaskResult")
	defer span.End()
	swarmTask, err := s.swarming.GetTask(ctx, taskID, false)
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
func (s *SwarmingTaskExecutor) GetTaskCompletionStatuses(ctx context.Context, taskIDs []string) ([]bool, error) {
	ctx, span := trace.StartSpan(ctx, "swarming_GetTaskCompletionStatuses")
	defer span.End()
	states, err := s.swarming.GetStates(ctx, taskIDs)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rv := make([]bool, 0, len(states))
	for _, state := range states {
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
func (s *SwarmingTaskExecutor) TriggerTask(ctx context.Context, req *types.TaskRequest) (*types.TaskResult, error) {
	ctx, span := trace.StartSpan(ctx, "swarming_TriggerTask")
	defer span.End()
	sReq, err := s.convertTaskRequest(req)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var resp *swarming_api.SwarmingRpcsTaskRequestMetadata
	if err := timeout.Run(func() error {
		var err error
		resp, err = s.swarming.TriggerTask(ctx, sReq)
		return err
	}, time.Minute); err != nil {
		return nil, skerr.Wrap(err)
	}
	if resp.TaskResult != nil {
		if resp.TaskResult.State == swarming.TASK_STATE_NO_RESOURCE {
			return nil, skerr.Fmt("No bots available to run %s with dimensions: %s", req.Name, strings.Join(req.Dimensions, ", "))
		}
		return convertTaskResult(resp.TaskResult)
	}
	created, err := swarming.ParseTimestamp(resp.Request.CreatedTs)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to parse Swarming timestamp")
	}
	t := &types.TaskResult{
		ID:      resp.TaskId,
		Created: created,
		Status:  types.TASK_STATUS_PENDING,
	}
	return t, nil
}

// convertTaskRequest converts a types.TaskRequest to a
// swarming_api.SwarmingRpcsNewTaskRequest.
func (s *SwarmingTaskExecutor) convertTaskRequest(req *types.TaskRequest) (*swarming_api.SwarmingRpcsNewTaskRequest, error) {
	var caches []*swarming_api.SwarmingRpcsCacheEntry
	if len(req.Caches) > 0 {
		caches = make([]*swarming_api.SwarmingRpcsCacheEntry, 0, len(req.Caches))
		for _, cache := range req.Caches {
			caches = append(caches, &swarming_api.SwarmingRpcsCacheEntry{
				Name: cache.Name,
				Path: cache.Path,
			})
		}
	}
	casInput, err := swarming.MakeCASReference(req.CasInput, s.casInstance)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var cipdInput *swarming_api.SwarmingRpcsCipdInput
	if len(req.CipdPackages) > 0 {
		cipdInput = &swarming_api.SwarmingRpcsCipdInput{
			Packages: make([]*swarming_api.SwarmingRpcsCipdPackage, 0, len(req.CipdPackages)),
		}
		for _, p := range req.CipdPackages {
			cipdInput.Packages = append(cipdInput.Packages, &swarming_api.SwarmingRpcsCipdPackage{
				PackageName: p.Name,
				Path:        p.Path,
				Version:     p.Version,
			})
		}
	}

	dims := make([]*swarming_api.SwarmingRpcsStringPair, 0, len(req.Dimensions))
	for _, d := range req.Dimensions {
		split := strings.SplitN(d, ":", 2)
		key := split[0]
		val := split[1]
		dims = append(dims, &swarming_api.SwarmingRpcsStringPair{
			Key:   key,
			Value: val,
		})
	}

	var env []*swarming_api.SwarmingRpcsStringPair
	if len(req.Env) > 0 {
		env = make([]*swarming_api.SwarmingRpcsStringPair, 0, len(req.Env))
		for k, v := range req.Env {
			env = append(env, &swarming_api.SwarmingRpcsStringPair{
				Key:   k,
				Value: v,
			})
		}
	}

	var envPrefixes []*swarming_api.SwarmingRpcsStringListPair
	if len(req.EnvPrefixes) > 0 {
		envPrefixes = make([]*swarming_api.SwarmingRpcsStringListPair, 0, len(req.EnvPrefixes))
		for k, v := range req.EnvPrefixes {
			envPrefixes = append(envPrefixes, &swarming_api.SwarmingRpcsStringListPair{
				Key:   k,
				Value: util.CopyStringSlice(v),
			})
		}
	}

	expirationSecs := int64(req.Expiration.Seconds())
	if expirationSecs == int64(0) {
		expirationSecs = int64(swarming.RECOMMENDED_EXPIRATION.Seconds())
	}
	executionTimeoutSecs := int64(req.ExecutionTimeout.Seconds())
	if executionTimeoutSecs == int64(0) {
		executionTimeoutSecs = int64(swarming.RECOMMENDED_HARD_TIMEOUT.Seconds())
	}
	ioTimeoutSecs := int64(req.IoTimeout.Seconds())
	if ioTimeoutSecs == int64(0) {
		ioTimeoutSecs = int64(swarming.RECOMMENDED_IO_TIMEOUT.Seconds())
	}
	outputs := util.CopyStringSlice(req.Outputs)
	rv := &swarming_api.SwarmingRpcsNewTaskRequest{
		Name:           req.Name,
		Priority:       swarming.RECOMMENDED_PRIORITY,
		PubsubTopic:    fmt.Sprintf(swarming.PUBSUB_FULLY_QUALIFIED_TOPIC_TMPL, common.PROJECT_ID, s.pubSubTopic),
		PubsubUserdata: req.TaskSchedulerTaskID,
		ServiceAccount: req.ServiceAccount,
		Tags:           req.Tags,
		TaskSlices: []*swarming_api.SwarmingRpcsTaskSlice{
			{
				ExpirationSecs: expirationSecs,
				Properties: &swarming_api.SwarmingRpcsTaskProperties{
					Caches:               caches,
					CasInputRoot:         casInput,
					CipdInput:            cipdInput,
					Command:              req.Command,
					Dimensions:           dims,
					Env:                  env,
					EnvPrefixes:          envPrefixes,
					ExecutionTimeoutSecs: executionTimeoutSecs,
					ExtraArgs:            req.ExtraArgs,
					Idempotent:           req.Idempotent,
					IoTimeoutSecs:        ioTimeoutSecs,
					Outputs:              outputs,
				},
				WaitForCapacity: false,
			},
		},
		User: SwarmingUser,
	}
	return rv, nil
}

// convertTaskResult converts a swarming_api.SwarmingRpcsTaskResult to a
// types.TaskResult.
func convertTaskResult(res *swarming_api.SwarmingRpcsTaskResult) (*types.TaskResult, error) {
	// Isolated output.
	var casOutput string
	if res.OutputsRef != nil {
		casOutput = res.OutputsRef.Isolated
	} else if res.CasOutputRoot != nil && res.CasOutputRoot.Digest.Hash != "" {
		casOutput = rbe.DigestToString(res.CasOutputRoot.Digest.Hash, res.CasOutputRoot.Digest.SizeBytes)
	}

	status, err := convertTaskStatus(res.State, res.Failure)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	var created time.Time
	if res.CreatedTs != "" {
		created, err = swarming.ParseTimestamp(res.CreatedTs)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to parse Swarming timestamp")
		}
	}

	var started time.Time
	if res.StartedTs != "" {
		started, err = swarming.ParseTimestamp(res.StartedTs)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to parse Swarming timestamp")
		}
	}

	var finished time.Time
	if res.CompletedTs != "" {
		finished, err = swarming.ParseTimestamp(res.CompletedTs)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	} else if status == types.TASK_STATUS_MISHAP && res.AbandonedTs != "" {
		finished, err = swarming.ParseTimestamp(res.AbandonedTs)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}

	tags, err := swarming.ParseTags(res.Tags)
	if err != nil {
		return nil, skerr.Wrap(err)
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
func convertTaskStatus(state string, failure bool) (types.TaskStatus, error) {
	switch state {
	case swarming.TASK_STATE_BOT_DIED, swarming.TASK_STATE_CANCELED, swarming.TASK_STATE_CLIENT_ERROR, swarming.TASK_STATE_EXPIRED, swarming.TASK_STATE_NO_RESOURCE, swarming.TASK_STATE_TIMED_OUT, swarming.TASK_STATE_KILLED:
		return types.TASK_STATUS_MISHAP, nil
	case swarming.TASK_STATE_PENDING:
		return types.TASK_STATUS_PENDING, nil
	case swarming.TASK_STATE_RUNNING:
		return types.TASK_STATUS_RUNNING, nil
	case swarming.TASK_STATE_COMPLETED:
		if failure {
			// TODO(borenet): Choose FAILURE or MISHAP depending on ExitCode?
			return types.TASK_STATUS_FAILURE, nil
		}
		return types.TASK_STATUS_SUCCESS, nil
	default:
		return types.TASK_STATUS_MISHAP, skerr.Fmt("Unknown Swarming State %v", state)
	}
}

// convertMachine converts a swarming_api.SwarmingRpcsBotInfo to a
// types.Machine.
func convertMachine(bot *swarming_api.SwarmingRpcsBotInfo) *types.Machine {
	return &types.Machine{
		ID:            bot.BotId,
		Dimensions:    swarming.BotDimensionsToStringSlice(bot.Dimensions),
		IsDead:        bot.IsDead,
		IsQuarantined: bot.Quarantined,
		CurrentTaskID: bot.TaskId,
	}
}
