package machinetest

import (
	"time"

	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/task_scheduler/go/types"
)

var MockTime = time.Date(2020, 1, 1, 1, 1, 1, 1, time.UTC)

var MockDuration = int32(5) // Seconds

// FullyFilledInDescription is a Description filled in with non-default values,
// useful in tests that round-trip Descriptions.
var FullyFilledInDescription = machine.Description{
	MaintenanceMode: "jcgregorio 2022-11-08",
	IsQuarantined:   true,
	Recovering:      "too hot",
	AttachedDevice:  machine.AttachedDevice(machine.AttachedDeviceAdb),
	Annotation: machine.Annotation{
		Message:   "take offline",
		User:      "barney@example.com",
		Timestamp: MockTime,
	},
	Note: machine.Annotation{
		Message:   "Battery swollen.",
		User:      "wilma@example.com",
		Timestamp: MockTime,
	},
	Dimensions: machine.SwarmingDimensions{
		machine.DimID:   []string{"skia-e-linux-202"},
		machine.DimPool: []string{machine.PoolSkia},
		"foo":           []string{"bar"},
		"alpha":         []string{"beta", "gamma"},
		"task_type":     []string{"swarming"},
	},
	SuppliedDimensions: machine.SwarmingDimensions{
		"gpu": []string{"some-gpu"},
	},
	Version:             "v1.2",
	LastUpdated:         MockTime,
	Battery:             91,
	Temperature:         map[string]float64{"cpu": 26.4},
	RunningSwarmingTask: true,
	LaunchedSwarming:    true,
	PowerCycle:          true,
	PowerCycleState:     machine.Available,
	RecoveryStart:       MockTime,
	DeviceUptime:        MockDuration,
	SSHUserIP:           "root@skia-sparky360-03",
	TaskRequest: &types.TaskRequest{
		Command: []string{"./helloworld"},
	},
	TaskStarted: MockTime,
}
