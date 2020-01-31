package db

import (
	"time"

	"go.skia.org/infra/niagara/go/messages"
)

type FirestoreMachineEntry struct {
	State       messages.MachineStatus `firestore:"state"`
	LastEvent   messages.MachineEvent  `firestore:"last_event"`
	Updated     time.Time              `firestore:"updated"`
	Dimensions  map[string][]string    `firestore:"dimensions"`
	CurrentTask string                 `firestore:"current_task"`
}

type FirestoreTaskEntry struct {
	MachineAssigned string `firestore:"machine_assigned"`
	Command         string `firestore:"command"`
}
