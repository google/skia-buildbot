package analyzer

import (
	"testing"

	cpb "go.skia.org/infra/cabe/go/proto"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"

	"github.com/stretchr/testify/assert"
)

func TestDiagnostics_AnalysisDiagnostics(t *testing.T) {
	d := newDiagnostics()

	d.excludeSwarmingTask(&apipb.TaskRequestMetadataResponse{
		TaskId: "task-id-0",
	}, "missing data")
	d.excludeSwarmingTask(&apipb.TaskRequestMetadataResponse{
		TaskId: "task-id-0",
	}, "an extra reason to exclude this same task")

	d.includeSwarmingTask(&apipb.TaskRequestMetadataResponse{
		TaskId: "task-id-1",
	})
	d.includeSwarmingTask(&apipb.TaskRequestMetadataResponse{
		TaskId: "task-id-3",
	})
	d.includeSwarmingTask(&apipb.TaskRequestMetadataResponse{
		TaskId: "task-id-4",
	})

	d.excludeReplica(7, pairedTasks{
		control: &armTask{
			taskID: "task-id-0",
		},
		treatment: &armTask{
			taskID: "task-id-1",
		},
	}, "bad data in one or both tasks")

	d.excludeReplica(7, pairedTasks{
		control: &armTask{
			taskID: "task-id-0",
		},
		treatment: &armTask{
			taskID: "task-id-1",
		},
	}, "an extra reason to exclude this same replica")

	d.includeReplica(1, pairedTasks{
		control: &armTask{
			taskID: "task-id-2",
		},
		treatment: &armTask{
			taskID: "task-id-3",
		},
	})

	ad := d.AnalysisDiagnostics()
	assert.NotNil(t, ad)
	diff := cmp.Diff(&cpb.AnalysisDiagnostics{
		ExcludedSwarmingTasks: []*cpb.SwarmingTaskDiagnostics{{
			Id: &cpb.SwarmingTaskId{
				TaskId: "task-id-0",
			},
			Message: []string{"missing data", "an extra reason to exclude this same task"},
		}},
		ExcludedReplicas: []*cpb.ReplicaDiagnostics{{
			ReplicaNumber: int32(7),
			ControlTask: &cpb.SwarmingTaskId{
				TaskId: "task-id-0",
			},
			TreatmentTask: &cpb.SwarmingTaskId{
				TaskId: "task-id-1",
			},
			Message: []string{"bad data in one or both tasks", "an extra reason to exclude this same replica"},
		}},
		IncludedSwarmingTasks: []*cpb.SwarmingTaskDiagnostics{
			{
				Id: &cpb.SwarmingTaskId{
					TaskId: "task-id-1",
				},
			},
			{
				Id: &cpb.SwarmingTaskId{
					TaskId: "task-id-3",
				},
			},
			{
				Id: &cpb.SwarmingTaskId{
					TaskId: "task-id-4",
				},
			},
		},
		IncludedReplicas: []*cpb.ReplicaDiagnostics{{
			ReplicaNumber: int32(1),
			ControlTask: &cpb.SwarmingTaskId{
				TaskId: "task-id-2",
			},
			TreatmentTask: &cpb.SwarmingTaskId{
				TaskId: "task-id-3",
			},
		}},
	},
		ad,
		cmpopts.EquateEmpty(),
		protocmp.Transform(),
	)
	assert.Equal(t, "", diff)
}
