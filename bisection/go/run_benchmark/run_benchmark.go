// Package run_benchmark runs the benchmark, story, metric as a
// on a build of chrome via swarming tasks.
//
// Package run_benchmark also supports various utility functions
// that make it easy to get the performance measuring tasks of
// a Pinpoint job and check their statuses.
package run_benchmark

import (
	"context"
	"fmt"
	"time"

	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/bisection/go/bot_configs"
	"go.skia.org/infra/cabe/go/backends"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/swarming"
)

// A RunBenchmarkRequest defines the request arguments of the performance test
// to swarming.
type RunBenchmarkRequest struct {
	// the Swarming client
	Client swarming.ApiClient
	// the Pinpoint job id
	JobID string
	// the swarming instance and cas digest hash and bytes location for the build
	Build *swarmingV1.SwarmingRpcsCASReference
	// commit hash
	Commit string
	// device configuration
	Config bot_configs.BotConfig
	// benchmark to test
	Benchmark string
	// story to test
	Story string
	// test target of the job
	Target string
}

// swarming request to run task
// define static, constant fields here
var swarmingReq = swarmingV1.SwarmingRpcsNewTaskRequest{
	BotPingToleranceSecs: 1200,
	ExpirationSecs:       86400,
	// EvaluateOnly omitted
	Name: "Pinpoint bisection run benchmark task",
	// ParentTaskId omitted
	// PoolTaskTemplate omitted
	Priority: 100,
	// define properties later
	PubsubTopic:    "projects/chromeperf/topics/pinpoint-swarming-updates",
	PubsubUserdata: "UNUSED", // can populate later, see example swarming call log
	Realm:          "chrome:pinpoint",
	// RequestUuid: omitted
	// Resultdb: omitted
	ServiceAccount: "chrome-tester@chops-service-accounts.iam.gserviceaccount.com",
	// define tags later
	// TaskSlices optional if properties defined
	User: "Pinpoint",
	// ForceSendFields: omitted
	// NullFields: omitted
}

// DialSwarming dials a swarming API client.
// TODO(sunxiaodi@) migrate swarming components to backends/ folder
func DialSwarming(ctx context.Context) (swarming.ApiClient, error) {
	return backends.DialSwarming(ctx)
}

// ListPinpointTasks lists the Pinpoint swarming tasks of a given
// job and build.
func ListPinpointTasks(ctx context.Context, client swarming.ApiClient, req RunBenchmarkRequest) ([]string, error) {
	if req.JobID == "" {
		return nil, skerr.Fmt("Cannot list tasks because request is missing JobID")
	}
	if req.Build == nil || req.Build.Digest == nil {
		return nil, skerr.Fmt("Cannot list tasks because request is missing cas isolate")
	}
	start := time.Now().Add(-24 * time.Hour)
	tags := []string{
		fmt.Sprintf("pinpoint_job_id:%s", req.JobID),
		fmt.Sprintf("build_cas:%s/%d", req.Build.Digest.Hash, req.Build.Digest.SizeBytes),
	}
	tasks, err := client.ListTasks(ctx, start, time.Now(), tags, "")
	if err != nil {
		return nil, fmt.Errorf("error retrieving tasks %s", err)
	}
	taskIDs := make([]string, len(tasks))
	for i, t := range tasks {
		taskIDs[i] = t.TaskId
	}
	return taskIDs, nil
}

// GetStatus gets the current status of a swarming task.
func GetStatus(ctx context.Context, client swarming.ApiClient, taskID string) (string, error) {
	res, err := client.GetTask(ctx, taskID, false)
	if err != nil {
		return "", skerr.Fmt("failed to get swarming task ID %s due to err: %v", taskID, err)
	}
	return res.State, nil
}

// GetStates returns the state of each task in a list of tasks.
func GetStates(ctx context.Context, client swarming.ApiClient, taskIDs []string) ([]string, error) {
	return client.GetStates(ctx, taskIDs)
}

func CancelTasks(ctx context.Context, client swarming.ApiClient, taskIDs []string) error {
	for _, id := range taskIDs {
		err := client.CancelTask(ctx, id, true)
		if err != nil {
			return skerr.Fmt("Could not cancel task %s due to %s", id, err)
		}
	}
	return nil
}

// GetCASOutput returns the CAS output of a swarming task in the
// form of a RBE CAS hash.
// GetCASOutput assumes the task is finished, or it throws an error.
func GetCASOutput(ctx context.Context, client swarming.ApiClient, taskID string) (
	*swarmingV1.SwarmingRpcsCASReference, error) {
	task, err := client.GetTask(ctx, taskID, false)
	if err != nil {
		return nil, fmt.Errorf("error retrieving result of task %s: %s", taskID, err)
	}
	if task.State != "COMPLETED" {
		return nil, fmt.Errorf("cannot get result of task %s because it is %s and not COMPLETED", taskID, task.State)
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

func createSwarmingReq(req RunBenchmarkRequest) (
	*swarmingV1.SwarmingRpcsNewTaskRequest, error) {
	// TODO(b/318863812): add mapping from device + benchmark to the specific run test
	// currently catapult maps the device + benchmark to the target and then
	// the target dictates what test to run. We can map to target if that info
	// is useful on the UI, but for this, it's not relevant.
	// see GetIsolateTarget and _GenerateQuests here:
	// https://source.chromium.org/chromium/chromium/src/+/main:third_party/catapult/dashboard/dashboard/pinpoint/handlers/new.py;drc=8fe602e47f11cfdd79225696f1f6a5556b57c58c;l=466
	// TODO(b/321299939): create an interface for different runBenchmark types
	// and refactor telemetryExp to use that interface
	exp := telemetryExp{}
	cmd, err := exp.createCmd(req)
	if err != nil {
		return nil, skerr.Fmt("Unable to create run benchmark command due to %s\n", err)
	}
	dim := make([]*swarmingV1.SwarmingRpcsStringPair,
		len(req.Config.Dimensions))
	for i, kv := range req.Config.Dimensions {
		dim[i] = &swarmingV1.SwarmingRpcsStringPair{
			Key:   kv["key"],
			Value: kv["value"],
		}
	}

	swarmingReq.Properties = &swarmingV1.SwarmingRpcsTaskProperties{
		CasInputRoot: &swarmingV1.SwarmingRpcsCASReference{
			CasInstance: req.Build.CasInstance,
			Digest: &swarmingV1.SwarmingRpcsDigest{
				Hash:      req.Build.Digest.Hash,
				SizeBytes: req.Build.Digest.SizeBytes,
			},
		},
		// TODO(b/318863812): support user submitted extra_args.
		// This support is needed for pairwise executions, not bisection.
		Command:              cmd,
		Dimensions:           dim,
		ExecutionTimeoutSecs: 2700,
		IoTimeoutSecs:        2700,
		RelativeCwd:          "out/Release",
	}

	// TODO(b/318863812): update swarming task tags to more appropriate tags.
	swarmingReq.Tags = []string{
		fmt.Sprintf("pinpoint_job_id:%s", req.JobID),
		fmt.Sprintf("build_cas:%s/%d", req.Build.Digest.Hash, req.Build.Digest.SizeBytes),
	}

	return &swarmingReq, nil
}

// Run schedules a swarming task to run the RunBenchmarkRequest.
func Run(ctx context.Context, client swarming.ApiClient, req RunBenchmarkRequest) (string, error) {
	swarmingReq, err := createSwarmingReq(req)
	if err != nil {
		return "", skerr.Wrapf(err, "Could not create run test request")
	}

	metadataResp, err := client.TriggerTask(ctx, swarmingReq)
	if err != nil {
		return "", skerr.Fmt("trigger task %v\ncaused error: %s", req, err)
	}

	return metadataResp.TaskId, nil
}
