package fs_entries

import (
	"time"

	"go.skia.org/infra/niagara/go/machine"
	"go.skia.org/infra/niagara/go/task"
)

const (
	// MachinesCollection is the Firestore collection which holds the machines.
	MachinesCollection = "machines"
	// TasksCollection is the Firestore collection to hold the tasks.
	TasksCollection = "tasks"

	// MaintenanceTasksCollection is the Firestore collection to hold the tasks.
	MaintenanceTasksCollection = "maintenance_tasks"
)

const (
	MachineStatusField = "status"

	TaskMachineAssignedField = "machine_assigned"
	TaskStatusField          = "status"
	TaskStartedField         = "started"
	TaskUpdatedField         = "updated"
	TaskEndedField           = "ended"
)

// Machine represents an individual machine that can run tasks based on its Dimensions.
type Machine struct {
	Status           machine.Status       `firestore:"status"`
	StatusReason     machine.StatusReason `firestore:"status_reason"`
	CurrentTask      string               `firestore:"current_task"`
	LastEvent        machine.Event        `firestore:"last_event"`
	Updated          time.Time            `firestore:"updated"`
	LastStatusChange time.Time            `firestore:"last_status_change"` // TODO(kjlubick)
	Description      machine.Description  `firestore:"description"`        // can be unindexed
}

// MachineEvent represents a single 'ping' from a machine.
type MachineEvent struct {
	Event machine.Event `firestore:"event"`
	TS    time.Time     `firestore:"ts"`
}

// Task represents a command that is to be run on a machine.
type Task struct {
	MachineAssigned string          `firestore:"machine_assigned"`
	Command         string          `firestore:"command"`
	Status          task.TaskStatus `firestore:"status"`

	Created time.Time `firestore:"created"`
	Started time.Time `firestore:"started"`
	Updated time.Time `firestore:"updated"`
	Ended   time.Time `firestore:"ended"`
}

type MaintenanceTask struct {
	Task            task.MaintenanceTaskType `firestore:"task"`
	Config          string                   `firestore:"config"`
	MachineAssigned string                   `firestore:"machine_assigned"`
}
