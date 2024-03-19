package backends

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/swarming"

	"golang.org/x/oauth2/google"

	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
)

const (
	DefaultSwarmingServiceAddress = "chrome-swarming.appspot.com:443"
	TaskStateFailure              = "FAILURE"
)

// SwarmingClient
type SwarmingClient interface {
	// CancelTasks tells Swarming to cancel the given tasks.
	CancelTasks(ctx context.Context, taskIDs []string) error

	// GetCASOutput returns the CAS output of a swarming task.
	GetCASOutput(ctx context.Context, taskID string) (*swarmingV1.SwarmingRpcsCASReference, error)

	// GetStates returns the state of each task in a list of tasks.
	GetStates(ctx context.Context, taskIDs []string) ([]string, error)

	// GetStatus gets the current status of a swarming task.
	GetStatus(ctx context.Context, taskID string) (string, error)

	// ListPinpointTasks lists the Pinpoint swarming tasks.
	ListPinpointTasks(ctx context.Context, jobID string, buildArtifact *swarmingV1.SwarmingRpcsCASReference) ([]string, error)

	// TriggerTask is a literal wrapper around swarming.ApiClient TriggerTask
	// TODO(jeffyoon@) remove once run_benchmark is refactored if no longer needed.
	TriggerTask(ctx context.Context, req *swarmingV1.SwarmingRpcsNewTaskRequest) (*swarmingV1.SwarmingRpcsTaskRequestMetadata, error)
}

// SwarmingClientImpl
// TODO(jeffyoon@) make this private once run_benchmark doesn't rely on this in testing.
type SwarmingClientImpl struct {
	swarming.ApiClient
}

func NewSwarmingClient(ctx context.Context, server string) (*SwarmingClientImpl, error) {
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, skerr.Wrapf(err, "Problem setting up default token source")
	}
	c := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).With2xxOnly().Client()

	sc, err := swarming.NewApiClient(c, server)
	if err != nil {
		return nil, err
	}

	return &SwarmingClientImpl{
		ApiClient: sc,
	}, nil
}

// CancelTasks tells Swarming to cancel the given tasks.
func (s *SwarmingClientImpl) CancelTasks(ctx context.Context, taskIDs []string) error {
	for _, id := range taskIDs {
		err := s.CancelTask(ctx, id, true)
		if err != nil {
			return skerr.Fmt("Could not cancel task %s due to %s", id, err)
		}
	}
	return nil
}

// GetCASOutput returns the CAS output of a swarming task in the form of a RBE CAS hash.
// This function assumes the task is finished, or it throws an error.
func (s *SwarmingClientImpl) GetCASOutput(ctx context.Context, taskID string) (*swarmingV1.SwarmingRpcsCASReference, error) {
	task, err := s.GetTask(ctx, taskID, false)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not retrieve CAS of task %s", taskID)
	}
	if task.State != swarming.TASK_STATE_COMPLETED {
		return nil, skerr.Fmt("cannot get result of task %s because it is %s and not COMPLETED", taskID, task.State)
	}
	rbe := &swarmingV1.SwarmingRpcsCASReference{
		CasInstance: task.CasOutputRoot.CasInstance,
		Digest: &swarmingV1.SwarmingRpcsDigest{
			Hash:      task.CasOutputRoot.Digest.Hash,
			SizeBytes: task.CasOutputRoot.Digest.SizeBytes,
		},
	}

	return rbe, nil
}

// func (s *SwarmingClientImpl) GetStates(ctx context.Context, taskIDs []string) ([]string, error) {
// 	return s.GetStates(ctx, taskIDs)
// }

// GetStatus gets the current status of a swarming task.
func (s *SwarmingClientImpl) GetStatus(ctx context.Context, taskID string) (string, error) {
	res, err := s.GetTask(ctx, taskID, false)
	if err != nil {
		return "", skerr.Fmt("failed to get swarming task ID %s due to err: %v", taskID, err)
	}
	// Swarming tasks can COMPLETE but fail. In this case, we need to
	// differentiate a successful task from an unsuccessful task.
	if res.State == swarming.TASK_STATE_COMPLETED && res.Failure {
		return TaskStateFailure, nil
	}
	return res.State, nil
}

// ListPinpointTasks lists the Pinpoint swarming tasks of a given job and build identified by Swarming tags.
func (s *SwarmingClientImpl) ListPinpointTasks(ctx context.Context, jobID string, buildArtifact *swarmingV1.SwarmingRpcsCASReference) ([]string, error) {
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
	tasks, err := s.ListTasks(ctx, start, time.Now(), tags, "")
	if err != nil {
		return nil, fmt.Errorf("error retrieving tasks %s", err)
	}
	taskIDs := make([]string, len(tasks))
	for i, t := range tasks {
		taskIDs[i] = t.TaskId
	}
	return taskIDs, nil
}

// func (s *SwarmingClientImpl) TriggerTask(ctx context.Context, req *swarmingV1.SwarmingRpcsNewTaskRequest) (*swarmingV1.SwarmingRpcsTaskRequestMetadata, error) {
// 	return s.TriggerTask(ctx, req)
// }
