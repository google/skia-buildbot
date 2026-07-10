package orphaned_tasks_machines

import (
	"context"
	gotesting "testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	swarming_mocks "go.skia.org/infra/go/swarming/v2/mocks"
	"go.skia.org/infra/task_scheduler/go/specs"
)

func TestGenerateReport(t *gotesting.T) {
	ctx := context.Background()

	// Define tasks configuration
	tasksCfg := &specs.TasksCfg{
		Tasks: map[string]*specs.TaskSpec{
			"Test-Task-A": {
				Dimensions: []string{"pool:Skia", "os:Ubuntu-20.04"},
			},
			"Test-Task-B": {
				Dimensions: []string{"pool:Skia", "os:Windows"},
			},
		},
	}

	swarmMock := &swarming_mocks.SwarmingV2Client{}

	// Mock ListBots for "Skia" pool.
	// We'll return machine-1 matching Test-Task-A dimensions, but no machine matching Test-Task-B.
	swarmMock.On("ListBots", mock.Anything, mock.Anything, mock.Anything).Return(&apipb.BotInfoListResponse{
		Items: []*apipb.BotInfo{
			{
				BotId: "machine-1",
				Dimensions: []*apipb.StringListPair{
					{Key: "pool", Value: []string{"Skia"}},
					{Key: "os", Value: []string{"Ubuntu-20.04"}},
				},
			},
			{
				BotId: "machine-2-unused", // This matches neither task directly as its OS is different
				Dimensions: []*apipb.StringListPair{
					{Key: "pool", Value: []string{"Skia"}},
					{Key: "os", Value: []string{"macOS"}},
				},
			},
		},
	}, nil)

	// Mock ListTasks for Test-Task-B (which has no matching machine)
	swarmMock.On("ListTasks", mock.Anything, mock.Anything, mock.Anything).Return(&apipb.TaskListResponse{
		Items: []*apipb.TaskResultResponse{
			{
				TaskId: "task-b-last",
			},
		},
	}, nil)

	report, err := GenerateReport(ctx, tasksCfg, swarmMock)
	require.NoError(t, err)
	require.NotNil(t, report)

	// Verify Tasks with no matching machines
	// Test-Task-B has pool:Skia and os:Windows, no machines returned for it.
	require.Len(t, report.NoMatchingMachines, 1)
	require.Equal(t, []string{"Test-Task-B"}, report.NoMatchingMachines[0].Tasks)
	require.Equal(t, []string{"os:Windows", "pool:Skia"}, report.NoMatchingMachines[0].Dimensions)

	// Verify Unused machines (machines with no matching tasks)
	// machine-2-unused matches pool:Skia and os:macOS, which matches no tasks in tasksCfg
	require.Len(t, report.NoMatchingTasks, 1)
	require.Equal(t, []string{"machine-2-unused"}, report.NoMatchingTasks[0].Machines)
	require.Equal(t, []string{"os:macOS", "pool:Skia"}, report.NoMatchingTasks[0].Dimensions)

}
