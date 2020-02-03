package machine_manager

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/niagara/go/fs_entries"
	"go.skia.org/infra/niagara/go/machine"
	"go.skia.org/infra/niagara/go/task"
)

func (m *Manager) updateMachine(ctx context.Context, newStatus machine.Status, currEvent machine.Event, state machine.State) error {
	fme := fs_entries.Machine{
		State:       newStatus,
		LastEvent:   currEvent,
		Updated:     time.Now(),
		Dimensions:  state.Dimensions,
		CurrentTask: state.CurrentTask,
	}
	// store to ifirestore
	doc := m.client.Collection("machines").Doc(state.ID)
	if _, err := m.client.Set(ctx, doc, &fme, maxFirestoreWriteAttempts, maxFirestoreOperationTime); err != nil {
		return skerr.Wrapf(err, "updating machine entry for %s", state.ID)
	}
	return nil
}

func (m *Manager) updateTask(ctx context.Context, taskID string, status task.TaskStatus, ups ...firestore.Update) error {
	ups = append(ups, firestore.Update{Path: "status", Value: status}, firestore.Update{Path: "updated", Value: time.Now()})
	doc := m.client.Collection("tasks").Doc(taskID)
	if _, err := m.client.Update(ctx, doc, maxFirestoreWriteAttempts, maxFirestoreOperationTime, ups); err != nil {
		sklog.Warningf("Could not update task %s", taskID)
	}
	return nil
}
