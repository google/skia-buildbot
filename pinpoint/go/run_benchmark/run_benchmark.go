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
	"slices"

	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/bot_configs"
)

// A RunBenchmarkRequest defines the request arguments of the performance test
// to swarming.
type RunBenchmarkRequest struct {
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

var runningStates = []string{
	swarming.TASK_STATE_PENDING,
	swarming.TASK_STATE_RUNNING,
}

// IsTaskStateFinished checks if a swarming task state is finished
func IsTaskStateFinished(state string) (bool, error) {
	if !slices.Contains(swarming.TASK_STATES, state) {
		return false, skerr.Fmt("Not a valid swarming task state %s", state)
	}
	return !slices.Contains(runningStates, state), nil
}

// IsTaskStateSuccess checks if a swarming task state is finished
func IsTaskStateSuccess(state string) bool {
	return state == swarming.TASK_STATE_COMPLETED
}

// Run schedules a swarming task to run the RunBenchmarkRequest.
func Run(ctx context.Context, sc backends.SwarmingClient, req RunBenchmarkRequest) (string, error) {
	swarmingReq, err := createSwarmingReq(req)
	if err != nil {
		return "", skerr.Wrapf(err, "Could not create run test request")
	}

	metadataResp, err := sc.TriggerTask(ctx, swarmingReq)
	if err != nil {
		return "", skerr.Fmt("trigger task %v\ncaused error: %s", req, err)
	}

	return metadataResp.TaskId, nil
}
