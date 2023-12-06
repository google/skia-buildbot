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
	"go.skia.org/infra/cabe/go/backends"
	"go.skia.org/infra/go/swarming"
)

// A RunBenchmarkRequest defines the request arguments of the performance test
// to swarming.
type RunBenchmarkRequest struct {
	// the Pinpoint job id
	JobID string
	// the swarming instance and cas digest hash and bytes location for the build
	Build swarmingV1.SwarmingRpcsCASReference
}

// swarming request to run task
// define static, constant fields here
var swarmingReq = swarmingV1.SwarmingRpcsNewTaskRequest{
	BotPingToleranceSecs: 1200,
	ExpirationSecs:       86400,
	// EvaluateOnly omitted
	Name: "Dummy pinpoint bisection task",
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
func DialSwarming(ctx context.Context) (swarming.ApiClient, error) {
	return backends.DialSwarming(ctx)
}

// ListPinpointTasks lists the Pinpoint swarming tasks of a given
// job and build.
func ListPinpointTasks(ctx context.Context, client swarming.ApiClient, req RunBenchmarkRequest) ([]string, error) {
	start := time.Now().Add(-24 * time.Hour)
	// TODO(b/299675000): update swarming task tags to more appropriate tags.
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
		return "", fmt.Errorf("failed to get swarming task ID: %s\ndue to error: %v", taskID, err)
	}
	return res.State, nil
}

// GetStates returns the state of each task in a list of tasks.
func GetStates(ctx context.Context, client swarming.ApiClient, taskIDs []string) ([]string, error) {
	return client.GetStates(ctx, taskIDs)
}

// GetCASOutput returns the CAS output of a swarming task in the
// form of a RBE CAS hash.
// GetCASOutput assumes the task is finished, or it throws an error.
func GetCASOutput(ctx context.Context, client swarming.ApiClient, taskID string) (swarmingV1.SwarmingRpcsCASReference, error) {
	rbe := swarmingV1.SwarmingRpcsCASReference{}
	rbe.Digest = &swarmingV1.SwarmingRpcsDigest{}

	task, err := client.GetTask(ctx, taskID, false)
	if err != nil {
		return rbe, fmt.Errorf("error retrieving result of task %s: %s", taskID, err)
	}
	if task.State != "COMPLETED" {
		return rbe, fmt.Errorf("cannot get result of task %s because it is %s and not COMPLETED", taskID, task.State)
	}
	rbe = swarmingV1.SwarmingRpcsCASReference{
		CasInstance: task.CasOutputRoot.CasInstance,
		Digest: &swarmingV1.SwarmingRpcsDigest{
			Hash:      task.CasOutputRoot.Digest.Hash,
			SizeBytes: task.CasOutputRoot.Digest.SizeBytes,
		},
	}

	return rbe, nil
}

// RunTest runs the performance test as a swarming task.
func RunTest(ctx context.Context, client swarming.ApiClient, req RunBenchmarkRequest) (string, error) {
	// based off of Pinpoint job https://pinpoint-dot-chromeperf.appspot.com/job/1226ecbef60000
	// and task https://chrome-swarming.appspot.com/task?id=65efa8b022be7910
	properties := swarmingV1.SwarmingRpcsTaskProperties{
		CasInputRoot: &swarmingV1.SwarmingRpcsCASReference{
			CasInstance: req.Build.CasInstance,
			Digest: &swarmingV1.SwarmingRpcsDigest{
				Hash:      req.Build.Digest.Hash,
				SizeBytes: req.Build.Digest.SizeBytes,
			},
		},
		Command: []string{
			"luci-auth",
			"context",
			"--",
			"vpython3",
			"../../testing/test_env.py",
			"../../testing/scripts/run_performance_tests.py",
			"../../tools/perf/run_benchmark",
			"-d",
			"--benchmarks",
			"blink_perf.bindings",
			"--story-filter",
			"^node.type.html$",
			"--pageset-repeat",
			"1",
			"--browser",
			"release",
			"-v",
			"--upload-results",
			"--output-format",
			"histograms",
			"--isolated-script-test-output",
			"${ISOLATED_OUTDIR}/output.json",
			"--results-label",
			"chromium@faf089b",
			"--run-full-story-set",
		},
		Dimensions: []*swarmingV1.SwarmingRpcsStringPair{
			{
				Key:   "mac_model",
				Value: "Macmini9,1",
			},
			{
				Key:   "os",
				Value: "Mac",
			},
			{
				Key:   "cpu",
				Value: "arm",
			},
			{
				Key:   "pool",
				Value: "chrome.tests.pinpoint",
			},
		},
		ExecutionTimeoutSecs: 2700,
		IoTimeoutSecs:        2700,
		RelativeCwd:          "out/Release",
	}

	swarmingReq.Properties = &properties
	swarmingReq.Tags = []string{
		fmt.Sprintf("pinpoint_job_id:%s", req.JobID),
		fmt.Sprintf("build_cas:%s/%d", req.Build.Digest.Hash, req.Build.Digest.SizeBytes),
	}

	// kick off task
	metadataResp, err := client.TriggerTask(ctx, &swarmingReq)
	if err != nil {
		return "", fmt.Errorf("trigger task %v caused error: %v", req, err)
	}
	fmt.Printf("\nCreated swarming task: %s\n", metadataResp.TaskId)

	return metadataResp.TaskId, nil
}
