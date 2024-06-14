package analyzer

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
)

func TestRunInfoFoTask(t *testing.T) {
	for i, test := range []struct {
		name        string
		task        *apipb.TaskRequestMetadataResponse
		expected    *runInfo
		expectError bool
	}{
		{
			name: "empty",
			task: &apipb.TaskRequestMetadataResponse{
				Request: &apipb.TaskRequestResponse{
					Properties: &apipb.TaskProperties{
						Dimensions: nil, // specifically, to make sure these get checked.
					},
				},
				TaskResult: &apipb.TaskResultResponse{},
			},
			expectError: true,
		},
		{
			name: "cpu and os but no synthetic_product_name",
			task: &apipb.TaskRequestMetadataResponse{
				Request: &apipb.TaskRequestResponse{
					Properties: &apipb.TaskProperties{
						Dimensions: []*apipb.StringPair{
							{Key: "os", Value: "mac"},
							{Key: "cpu", Value: "arm"},
						},
					},
				},
				TaskResult: &apipb.TaskResultResponse{},
			},
			expected: &runInfo{
				cpu:            "arm",
				os:             "mac",
				startTimestamp: "1970-01-01T00:00:00",
			},
		},
		{
			name: "cpu, os and synthetic_product_name",
			task: &apipb.TaskRequestMetadataResponse{
				Request: &apipb.TaskRequestResponse{
					Properties: &apipb.TaskProperties{
						Dimensions: []*apipb.StringPair{
							{Key: "os", Value: "mac"},
							{Key: "cpu", Value: "arm"},
						},
					},
				},
				TaskResult: &apipb.TaskResultResponse{
					BotDimensions: []*apipb.StringListPair{
						{Key: "synthetic_product_name", Value: []string{"Macmini9,1_arm64-64-Apple_M1_16384_1_4744421.0"}},
					},
				},
			},
			expected: &runInfo{
				syntheticProductName: "Macmini9,1_arm64-64-Apple_M1_16384_1_4744421.0",
				cpu:                  "arm",
				os:                   "mac",
				startTimestamp:       "1970-01-01T00:00:00",
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
			{resultOutput: &apipb.CASReference{}},
			{resultOutput: &apipb.CASReference{}},
			{resultOutput: &apipb.CASReference{}},
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
		taskInfo: &apipb.TaskRequestMetadataResponse{
			TaskId:     "ctask1",
			TaskResult: &apipb.TaskResultResponse{BotId: "bot1"},
		},
	}
	ct2 := &armTask{
		taskID:      "ctask2",
		buildConfig: "build1",
		runConfig:   "run1",
		taskInfo: &apipb.TaskRequestMetadataResponse{
			TaskId:     "ctask2",
			TaskResult: &apipb.TaskResultResponse{BotId: "bot1"},
		},
	}
	ct3 := &armTask{
		taskID:      "ctask3",
		buildConfig: "build2",
		runConfig:   "run2",
		taskInfo: &apipb.TaskRequestMetadataResponse{
			TaskId:     "ctask3",
			TaskResult: &apipb.TaskResultResponse{BotId: "bot2"},
		},
	}
	ct4 := &armTask{
		taskID:      "ctask4",
		buildConfig: "build4",
		runConfig:   "run4",
		taskInfo: &apipb.TaskRequestMetadataResponse{
			TaskId:     "ctask4",
			TaskResult: &apipb.TaskResultResponse{BotId: "bot4"},
		},
	}
	ct5 := &armTask{
		taskID:      "ctask5",
		buildConfig: "build5",
		runConfig:   "run6",
		taskInfo: &apipb.TaskRequestMetadataResponse{
			TaskId:     "ctask5",
			TaskResult: &apipb.TaskResultResponse{BotId: "bot6"},
		},
	}
	ct6 := &armTask{
		taskID:      "ctask6",
		buildConfig: "build7",
		runConfig:   "run6",
		taskInfo: &apipb.TaskRequestMetadataResponse{
			TaskId:     "ctask6",
			TaskResult: &apipb.TaskResultResponse{BotId: "bot7"},
		},
	}
	ct7 := &armTask{
		taskID:      "ctask7",
		buildConfig: "build8",
		runConfig:   "run8",
		taskInfo: &apipb.TaskRequestMetadataResponse{
			TaskId:     "ctask7",
			TaskResult: &apipb.TaskResultResponse{BotId: "bot8"},
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
		taskInfo: &apipb.TaskRequestMetadataResponse{
			TaskId:     "ttask1",
			TaskResult: &apipb.TaskResultResponse{BotId: "bot1"},
		},
	}
	tt2 := &armTask{
		taskID:      "ttask2",
		buildConfig: "build3",
		runConfig:   "run3",
		taskInfo: &apipb.TaskRequestMetadataResponse{
			TaskId:     "ttask2",
			TaskResult: &apipb.TaskResultResponse{BotId: "bot3"},
		},
	}
	tt3 := &armTask{
		taskID:      "ttask3",
		buildConfig: "build4",
		runConfig:   "run4",
		taskInfo: &apipb.TaskRequestMetadataResponse{
			TaskId:     "ttask3",
			TaskResult: &apipb.TaskResultResponse{BotId: "bot4"},
		},
	}
	tt4 := &armTask{
		taskID:      "ttask4",
		buildConfig: "build5",
		runConfig:   "run5",
		taskInfo: &apipb.TaskRequestMetadataResponse{
			TaskId:     "ttask4",
			TaskResult: &apipb.TaskResultResponse{BotId: "bot5"},
		},
	}
	tt5 := &armTask{
		taskID:      "ttask5",
		buildConfig: "build6",
		runConfig:   "run6",
		taskInfo: &apipb.TaskRequestMetadataResponse{
			TaskId:     "ttask5",
			TaskResult: &apipb.TaskResultResponse{BotId: "bot6"},
		},
	}
	tt6 := &armTask{
		taskID:      "ttask6",
		buildConfig: "build7",
		runConfig:   "run7",
		taskInfo: &apipb.TaskRequestMetadataResponse{
			TaskId:     "ttask6",
			TaskResult: &apipb.TaskResultResponse{BotId: "bot7"},
		},
	}
	tt7 := &armTask{
		taskID:      "ttask7",
		buildConfig: "build8",
		runConfig:   "run8",
		taskInfo: &apipb.TaskRequestMetadataResponse{
			TaskId:     "ttask7",
			TaskResult: &apipb.TaskResultResponse{BotId: "bot8"},
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
