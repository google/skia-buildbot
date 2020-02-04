package machine_manager

import (
	"context"

	"cloud.google.com/go/firestore"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/niagara/go/fs_entries"
	"go.skia.org/infra/niagara/go/task"
)

func (m *Manager) updateMachine(ctx context.Context, machineID string, data fs_entries.Machine) error {
	data.Updated = m.now()
	md := m.client.Collection(fs_entries.MachinesCollection).Doc(machineID)
	_, err := m.client.Set(ctx, md, data, maxFirestoreWriteAttempts, maxFirestoreOperationTime)
	if err != nil {
		return skerr.Wrapf(err, "updating machine entry for %s", machineID)
	}
	return nil
}

func (m *Manager) updateTask(ctx context.Context, taskID string, status task.TaskStatus, ups ...firestore.Update) error {
	ups = append(ups, firestore.Update{Path: fs_entries.TaskStatusField, Value: status},
		firestore.Update{Path: fs_entries.TaskUpdatedField, Value: m.now()})
	doc := m.client.Collection(fs_entries.TasksCollection).Doc(taskID)
	if _, err := m.client.Update(ctx, doc, maxFirestoreWriteAttempts, maxFirestoreOperationTime, ups); err != nil {
		return skerr.Wrapf(err, "could not update task %s", taskID)
	}
	return nil
}
