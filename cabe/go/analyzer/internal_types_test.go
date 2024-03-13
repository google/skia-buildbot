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

func TestPairedTasks(t *testing.T) {
	ct1 := &armTask{
		taskID:      "ctask1",
		buildConfig: "build1",
		runConfig:   "run1",
		taskInfo: &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskId:     "ctask1",
			TaskResult: &swarming.SwarmingRpcsTaskResult{BotId: "bot1"},
		},
	}
	ct2 := &armTask{
		taskID:      "ctask2",
		buildConfig: "build1",
		runConfig:   "run1",
		taskInfo: &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskId:     "ctask2",
			TaskResult: &swarming.SwarmingRpcsTaskResult{BotId: "bot1"},
		},
	}
	ct3 := &armTask{
		taskID:      "ctask3",
		buildConfig: "build2",
		runConfig:   "run2",
		taskInfo: &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskId:     "ctask3",
			TaskResult: &swarming.SwarmingRpcsTaskResult{BotId: "bot2"},
		},
	}
	ct4 := &armTask{
		taskID:      "ctask4",
		buildConfig: "build4",
		runConfig:   "run4",
		taskInfo: &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskId:     "ctask4",
			TaskResult: &swarming.SwarmingRpcsTaskResult{BotId: "bot4"},
		},
	}
	ct5 := &armTask{
		taskID:      "ctask5",
		buildConfig: "build5",
		runConfig:   "run6",
		taskInfo: &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskId:     "ctask5",
			TaskResult: &swarming.SwarmingRpcsTaskResult{BotId: "bot6"},
		},
	}
	ct6 := &armTask{
		taskID:      "ctask6",
		buildConfig: "build7",
		runConfig:   "run6",
		taskInfo: &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskId:     "ctask6",
			TaskResult: &swarming.SwarmingRpcsTaskResult{BotId: "bot7"},
		},
	}
	ct7 := &armTask{
		taskID:      "ctask7",
		buildConfig: "build8",
		runConfig:   "run8",
		taskInfo: &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskId:     "ctask7",
			TaskResult: &swarming.SwarmingRpcsTaskResult{BotId: "bot8"},
		},
	}
	control := &processedArmTasks{
		tasks: []*armTask{
			ct1,
			ct2,
			ct3,
			ct4,
			ct5,
			ct6,
			ct7,
		},
	}

	tt1 := &armTask{
		taskID:      "ttask1",
		buildConfig: "build1",
		runConfig:   "run1",
		taskInfo: &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskId:     "ttask1",
			TaskResult: &swarming.SwarmingRpcsTaskResult{BotId: "bot1"},
		},
	}
	tt2 := &armTask{
		taskID:      "ttask2",
		buildConfig: "build3",
		runConfig:   "run3",
		taskInfo: &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskId:     "ttask2",
			TaskResult: &swarming.SwarmingRpcsTaskResult{BotId: "bot3"},
		},
	}
	tt3 := &armTask{
		taskID:      "ttask3",
		buildConfig: "build4",
		runConfig:   "run4",
		taskInfo: &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskId:     "ttask3",
			TaskResult: &swarming.SwarmingRpcsTaskResult{BotId: "bot4"},
		},
	}
	tt4 := &armTask{
		taskID:      "ttask4",
		buildConfig: "build5",
		runConfig:   "run5",
		taskInfo: &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskId:     "ttask4",
			TaskResult: &swarming.SwarmingRpcsTaskResult{BotId: "bot5"},
		},
	}
	tt5 := &armTask{
		taskID:      "ttask5",
		buildConfig: "build6",
		runConfig:   "run6",
		taskInfo: &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskId:     "ttask5",
			TaskResult: &swarming.SwarmingRpcsTaskResult{BotId: "bot6"},
		},
	}
	tt6 := &armTask{
		taskID:      "ttask6",
		buildConfig: "build7",
		runConfig:   "run7",
		taskInfo: &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskId:     "ttask6",
			TaskResult: &swarming.SwarmingRpcsTaskResult{BotId: "bot7"},
		},
	}
	tt7 := &armTask{
		taskID:      "ttask7",
		buildConfig: "build8",
		runConfig:   "run8",
		taskInfo: &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskId:     "ttask7",
			TaskResult: &swarming.SwarmingRpcsTaskResult{BotId: "bot8"},
		},
	}
	treatment := &processedArmTasks{
		tasks: []*armTask{
			tt1,
			tt2,
			tt3,
			tt4,
			tt5,
			tt6,
			tt7,
		},
	}

	tasks := &processedExperimentTasks{
		control:   control,
		treatment: treatment,
	}

	expectedPairedTasks := [3]pairedTasks{
		{
			control:   ct1,
			treatment: tt1,
		},
		{
			control:   ct4,
			treatment: tt3,
		},
		{
			control:   ct7,
			treatment: tt7,
		},
	}

	actualPairedTasks, err := tasks.pairedTasks(make(map[string]*SwarmingTaskDiagnostics))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(actualPairedTasks) != len(expectedPairedTasks) {
		t.Errorf("Unexpected error: %v", err)
	}

	for i, actualPairedTask := range actualPairedTasks {
		expectedPairedTask := expectedPairedTasks[i]
		if actualPairedTask.control.taskID != expectedPairedTask.control.taskID ||
			actualPairedTask.treatment.taskID != expectedPairedTask.treatment.taskID {
			t.Errorf("Results %v didn't match expected value %v", actualPairedTask, expectedPairedTask)
		}
	}
}
