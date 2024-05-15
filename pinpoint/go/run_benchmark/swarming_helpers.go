package run_benchmark

import (
	"fmt"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
)

const ExecutionTimeoutSecs = 2700 // 45 min
const PendingTimeoutSecs = 86400  // 1 day

func convertDimensions(dimensions []map[string]string) []*apipb.StringPair {
	// TODO(b/318863812): add mapping from device + benchmark to the specific run test
	// currently catapult maps the device + benchmark to the target and then
	// the target dictates what test to run. We can map to target if that info
	// is useful on the UI, but for this, it's not relevant.
	// see GetIsolateTarget and _GenerateQuests here:
	// https://source.chromium.org/chromium/chromium/src/+/main:third_party/catapult/dashboard/dashboard/pinpoint/handlers/new.py;drc=8fe602e47f11cfdd79225696f1f6a5556b57c58c;l=466
	// TODO(b/321299939): create an interface for different runBenchmark types
	// and refactor telemetryExp to use that interface
	dim := make([]*apipb.StringPair, len(dimensions))
	for i, kv := range dimensions {
		dim[i] = &apipb.StringPair{
			Key:   kv["key"],
			Value: kv["value"],
		}
	}
	return dim
}

func generateProperties(command []string, casRef *apipb.CASReference, dim []*apipb.StringPair) *apipb.TaskProperties {
	return &apipb.TaskProperties{
		CasInputRoot:         casRef,
		Command:              command,
		Dimensions:           dim,
		ExecutionTimeoutSecs: ExecutionTimeoutSecs,
		IoTimeoutSecs:        ExecutionTimeoutSecs,
		RelativeCwd:          "out/Release",
	}
}

func generateTags(jobID string, hash string, sizeBytes int64) []string {
	// TODO(b/318863812): update swarming task tags to more appropriate tags.
	return []string{
		fmt.Sprintf("pinpoint_job_id:%s", jobID),
		fmt.Sprintf("build_cas:%s/%d", hash, sizeBytes),
	}
}

func createSwarmingRequest(jobID string, command []string, casRef *apipb.CASReference, dimensions []map[string]string) *apipb.NewTaskRequest {
	return &apipb.NewTaskRequest{
		BotPingToleranceSecs: 1200,
		ExpirationSecs:       PendingTimeoutSecs,
		// EvaluateOnly omitted
		Name: "Pinpoint bisection run benchmark task",
		// ParentTaskId omitted
		// PoolTaskTemplate omitted
		Priority: 100,
		// define properties later
		PubsubTopic: "projects/chromeperf/topics/pinpoint-swarming-updates",
		// can populate later, see example swarming call log
		PubsubUserdata: "UNUSED",
		Realm:          "chrome:pinpoint",
		// RequestUuid: omitted
		// Resultdb: omitted
		ServiceAccount: "chrome-tester@chops-service-accounts.iam.gserviceaccount.com",
		// define tags later
		// TaskSlices optional if properties defined
		User: "Pinpoint",
		// ForceSendFields: omitted
		// NullFields: omitted
		Properties: generateProperties(command, casRef, convertDimensions(dimensions)),
		Tags:       generateTags(jobID, casRef.Digest.Hash, casRef.Digest.SizeBytes),
	}
}
