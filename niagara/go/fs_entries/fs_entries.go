package fs_entries

import (
	"time"

	"go.skia.org/infra/niagara/go/machine"
	"go.skia.org/infra/niagara/go/task"
)

type Machine struct {
	State           machine.Status      `firestore:"state"`
	LastEvent       machine.Event       `firestore:"last_event"`
	Updated         time.Time           `firestore:"updated"`
	LastStateChange time.Time           `firestore:"last_state_change"` // TODO(kjlubick)
	Dimensions      map[string][]string `firestore:"dimensions"`
	CurrentTask     string              `firestore:"current_task"`
}

type Task struct {
	MachineAssigned string          `firestore:"machine_assigned"`
	Command         string          `firestore:"command"`
	Status          task.TaskStatus `firestore:"status"`

	Created   time.Time `firestore:"created"`
	Started   time.Time `firestore:"started"`
	Completed time.Time `firestore:"completed"`
	Abandoned time.Time `firestore:"abandoned"`
	Updated   time.Time `firestore:"updated"`
	// TODO(kjlubick) Expires
}
