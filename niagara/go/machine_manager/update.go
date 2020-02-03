package machine_manager

import (
	"context"

	"cloud.google.com/go/firestore"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/niagara/go/fs_entries"
	"go.skia.org/infra/niagara/go/machine"
	"go.skia.org/infra/niagara/go/task"
)

func (m *Manager) updateMachine(ctx context.Context, newStatus machine.Status, currEvent machine.Event, desc machine.Description) error {
	now := m.now()
	updatedMachine := fs_entries.Machine{
		Status:      newStatus,
		LastEvent:   currEvent,
		Updated:     now,
		Dimensions:  desc.Dimensions,
		CurrentTask: desc.CurrentTask,
	}
	newEvent := fs_entries.MachineEvent{
		Event: currEvent,
		TS:    now,
	}
	md := m.client.Collection(fs_entries.MachinesCollection).Doc(desc.ID)

	err := m.client.RunTransaction(ctx, "update-machine", desc.ID, maxFirestoreWriteAttempts, maxFirestoreOperationTime, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := tx.Set(md, &updatedMachine); err != nil {
			return skerr.Wrap(err)
		}
		evt := md.Collection(fs_entries.MachineEventsCollection).NewDoc()
		if err := tx.Create(evt, newEvent); err != nil {
			return skerr.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return skerr.Wrapf(err, "updating machine entry for %s", desc.ID)
	}
	return nil
}

func (m *Manager) updateTask(ctx context.Context, taskID string, status task.TaskStatus, ups ...firestore.Update) error {
	ups = append(ups, firestore.Update{Path: "status", Value: status}, firestore.Update{Path: "updated", Value: m.now()})
	doc := m.client.Collection(fs_entries.TasksCollection).Doc(taskID)
	if _, err := m.client.Update(ctx, doc, maxFirestoreWriteAttempts, maxFirestoreOperationTime, ups); err != nil {
		return skerr.Wrapf(err, "could not update task %s", taskID)
	}
	return nil
}
