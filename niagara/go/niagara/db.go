package niagara

import (
	"time"
)

type FirestoreMachineEntry struct {
	State       MachineStatus       `firestore:"state"`
	LastEvent   MachineEvent        `firestore:"last_event"`
	Updated     time.Time           `firestore:"updated"`
	Dimensions  map[string][]string `firestore:"dimensions"`
	CurrentTask string              `firestore:"current_task"`
}

type FirestoreTaskEntry struct {
	MachineAssigned string     `firestore:"machine_assigned"`
	Command         string     `firestore:"command"`
	Status          TaskStatus `firestore:"status"`

	Created   time.Time `firestore:"created"`
	Started   time.Time `firestore:"started"`
	Completed time.Time `firestore:"completed"`
	Abandoned time.Time `firestore:"abandoned"`
	Updated   time.Time `firestore:"updated"`
	// TODO(kjlubick) Expires
}
