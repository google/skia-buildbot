package analyzer

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"go.chromium.org/luci/common/api/swarming/swarming/v1"
)

func TestRunInfoFoTask(t *testing.T) {
	for i, test := range []struct {
		name        string
		task        *swarming.SwarmingRpcsTaskRequestMetadata
		expected    *runInfo
		expectError bool
	}{
		{
			name: "empty",
			task: &swarming.SwarmingRpcsTaskRequestMetadata{
				Request: &swarming.SwarmingRpcsTaskRequest{
					Properties: &swarming.SwarmingRpcsTaskProperties{
						Dimensions: nil, // specifically, to make sure these get checked.
					},
				},
				TaskResult: &swarming.SwarmingRpcsTaskResult{},
			},
			expectError: true,
		},
		{
			name: "cpu and os but no synthetic_product_name",
			task: &swarming.SwarmingRpcsTaskRequestMetadata{
				Request: &swarming.SwarmingRpcsTaskRequest{
					Properties: &swarming.SwarmingRpcsTaskProperties{
						Dimensions: []*swarming.SwarmingRpcsStringPair{
							{Key: "os", Value: "mac"},
							{Key: "cpu", Value: "arm"},
						},
					},
				},
				TaskResult: &swarming.SwarmingRpcsTaskResult{
					StartedTs: "0",
				},
			},
			expected: &runInfo{
				cpu:            "arm",
				os:             "mac",
				startTimestamp: "0",
			},
		},
		{
			name: "cpu, os and synthetic_product_name",
			task: &swarming.SwarmingRpcsTaskRequestMetadata{
				Request: &swarming.SwarmingRpcsTaskRequest{
					Properties: &swarming.SwarmingRpcsTaskProperties{
						Dimensions: []*swarming.SwarmingRpcsStringPair{
							{Key: "os", Value: "mac"},
							{Key: "cpu", Value: "arm"},
						},
					},
				},
				TaskResult: &swarming.SwarmingRpcsTaskResult{
					BotDimensions: []*swarming.SwarmingRpcsStringListPair{
						{Key: "synthetic_product_name", Value: []string{"Macmini9,1_arm64-64-Apple_M1_16384_1_4744421.0"}},
					},
					StartedTs: "0",
				},
			},
			expected: &runInfo{
				syntheticProductName: "Macmini9,1_arm64-64-Apple_M1_16384_1_4744421.0",
				cpu:                  "arm",
				os:                   "mac",
				startTimestamp:       "0",
			},
		},
	} {
		got, err := runInfoForTask(test.task)
		if err != nil && !test.expectError {
			t.Errorf("[%d] %q unexpected error: %v", i, test.name, err)
		}
		if err == nil && test.expectError {
			t.Errorf("[%d] %q did not return expected error", i, test.name)
		}
		diff := cmp.Diff(test.expected, got, cmpopts.EquateEmpty(), cmp.AllowUnexported(runInfo{}))
		if diff != "" {
			t.Errorf("[%d] %q results didn't match expected value. Diff:\n%s", i, test.name, diff)
		}
	}
}

func TestProcessedArmTasks_outputDigests(t *testing.T) {
	pat := &processedArmTasks{
		tasks: []*armTask{
			{resultOutput: &swarming.SwarmingRpcsCASReference{}},
			{resultOutput: &swarming.SwarmingRpcsCASReference{}},
			{resultOutput: &swarming.SwarmingRpcsCASReference{}},
		},
	}
	res := pat.outputDigests()
	if len(pat.tasks) != len(res) {
		t.Errorf("expected %d output digests but got %d", len(pat.tasks), len(res))
	}
}
