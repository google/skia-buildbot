package fs_entries

import (
	"time"

	"go.skia.org/infra/niagara/go/machine"
	"go.skia.org/infra/niagara/go/task"
)

const (
	// MachinesCollection is the Firestore collection which holds the machines.
	MachinesCollection = "machines"
	// MachineEventsCollection is a Firestore collection (a subcollection of a given machine) which
	// holds the events received by a specific machine.
	MachineEventsCollection = "events"
	// TasksCollection is the Firestore collection to hold the tasks.
	TasksCollection = "tasks"
)

const (
	MachineStatusField = "status"

	TaskStatusField = "status"
)

// Machine represents an individual machine that can run tasks based on its Dimensions.
type Machine struct {
	Status           machine.Status      `firestore:"status"`
	LastEvent        machine.Event       `firestore:"last_event"`
	Updated          time.Time           `firestore:"updated"`
	LastStatusChange time.Time           `firestore:"last_status_change"` // TODO(kjlubick)
	Dimensions       map[string][]string `firestore:"dimensions"`
	CurrentTask      string              `firestore:"current_task"`
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

	Created   time.Time `firestore:"created"`
	Started   time.Time `firestore:"started"`
	Completed time.Time `firestore:"completed"`
	Abandoned time.Time `firestore:"abandoned"`
	Updated   time.Time `firestore:"updated"`
}
