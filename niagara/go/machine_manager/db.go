package machine_manager

import (
	"context"

	"cloud.google.com/go/firestore"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/niagara/go/fs_entries"
	"go.skia.org/infra/niagara/go/task"
)

func (m *Manager) setMachine(ctx context.Context, machineID string, data fs_entries.Machine) error {
	data.Updated = m.now()
	md := m.fsClient.Collection(fs_entries.MachinesCollection).Doc(machineID)
	_, err := m.fsClient.Set(ctx, md, data, maxFirestoreWriteAttempts, maxFirestoreOperationTime)
	if err != nil {
		return skerr.Wrapf(err, "updating machine entry for %s", machineID)
	}
	return nil
}

func (m *Manager) updateTask(ctx context.Context, taskID string, status task.TaskStatus, ups ...firestore.Update) error {
	ups = append(ups, firestore.Update{Path: fs_entries.TaskStatusField, Value: status},
		firestore.Update{Path: fs_entries.TaskUpdatedField, Value: m.now()})
	doc := m.fsClient.Collection(fs_entries.TasksCollection).Doc(taskID)
	if _, err := m.fsClient.Update(ctx, doc, maxFirestoreWriteAttempts, maxFirestoreOperationTime, ups); err != nil {
		return skerr.Wrapf(err, "could not update task %s", taskID)
	}
	return nil
}

type maintenanceTask struct {
	Task            task.MaintenanceTaskType `firestore:"task"`
	Config          string                   `firestore:"config"`
	MachineAssigned string                   `firestore:"machine_assigned"`
}

func (m *Manager) scheduleMaintenanceTask(ctx context.Context, mt maintenanceTask) error {
	q := m.fsClient.Collection(fs_entries.TasksCollection).Where(fs_entries.TaskMachineAssignedField, "==", mt.MachineAssigned).
		Where(fs_entries.MaintenanceTaskField, "==", mt.Task).Where(fs_entries.TaskStatusField, "==", task.New)
	if xd, err := q.Documents(ctx).GetAll(); err != nil {
		return skerr.Wrapf(err, "checking for other MaintenanceTasks conflicting with %v", mt)
	} else if len(xd) > 0 {
		sklog.Infof("not creating another MaintenanceTask %v - one is already scheduled", mt)
		return nil
	}
	// There is a small race condition here between checking for another already existing
	// MaintenanceTask and creating this new one. I'm not sure if that matters too much -
	// a machine might rarely get rebooted twice in a row, or have an extra update task
	// that it ignores. If it becomes a problem, a field could be added to the fs_entries.Machine
	// and a transactional update could be used.

	doc := m.fsClient.Collection(fs_entries.TasksCollection).NewDoc()
	if _, err := m.fsClient.Create(ctx, doc, fs_entries.Task{
		MachineAssigned: mt.MachineAssigned,
		Status:          task.New,
		Command:         "",
		MaintenanceTask: mt.Task,
		Config:          mt.Config,
		Created:         m.now(),
	}, maxFirestoreWriteAttempts, maxFirestoreOperationTime); err != nil {
		return skerr.Wrapf(err, "creating new maintenance task %v", mt)
	}
	return nil
}
