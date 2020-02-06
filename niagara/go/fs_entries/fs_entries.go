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
)

const (
	MachineStatusField = "status"

	TaskMachineAssignedField = "machine_assigned"
	TaskStatusField          = "status"
	TaskStartedField         = "started"
	TaskUpdatedField         = "updated"
	TaskEndedField           = "ended"
	MaintenanceTaskField     = "maintenance_task"
)

// Machine represents an individual machine that can run tasks based on its Dimensions.
type Machine struct {
	Status       machine.Status       `firestore:"status"`
	StatusReason machine.StatusReason `firestore:"status_reason"`
	CurrentTask  string               `firestore:"current_task"`
	LastEvent    machine.Event        `firestore:"last_event"`
	Updated      time.Time            `firestore:"updated"`
	Description  machine.Description  `firestore:"description"` // can be unindexed
}

// Task represents a command that is to be run on a machine.
type Task struct {
	MachineAssigned string          `firestore:"machine_assigned"`
	Status          task.TaskStatus `firestore:"status"`

	// If this is set, MaintenanceTask should not be set.
	Command string `firestore:"command"`

	// If this is set, Command should not be set.
	MaintenanceTask task.MaintenanceTaskType `firestore:"maintenance_task"`
	Config          string                   `firestore:"config"`

	Created time.Time `firestore:"created"`
	Started time.Time `firestore:"started"`
	Updated time.Time `firestore:"updated"`
	Ended   time.Time `firestore:"ended"`
}
