package machine_manager

import (
	"context"
	"encoding/json"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/pubsub"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/niagara/go/fs_entries"
	"go.skia.org/infra/niagara/go/machine"
	"go.skia.org/infra/niagara/go/task"
)

const (
	maxFirestoreWriteAttempts = 5
	maxFirestoreOperationTime = 2 * time.Minute
)

type Manager struct {
	client *ifirestore.Client
	now    func() time.Time
}

func New(client *ifirestore.Client) *Manager {
	return &Manager{
		client: client,
		now:    time.Now,
	}
}

func (m *Manager) ProcessPubsubMessage(ctx context.Context, msg *pubsub.Message) {
	msg.Ack()

	var desc machine.Description
	if err := json.Unmarshal(msg.Data, &desc); err != nil {
		sklog.Errorf("Invalid message data %s: %s", err, string(msg.Data))
		return
	}
	err := m.process(ctx, desc, msg.Attributes)
	if err != nil {
		sklog.Errorf("Problem processing machine event: %s \ndescription: %v", err, desc)
	}
}

func (m *Manager) process(ctx context.Context, desc machine.Description, extraData map[string]string) error {
	if desc.ID == "" {
		return skerr.Fmt("Machine description had no id")
	}
	event := machine.Event(extraData[machine.EventAttribute])
	if event == "" {
		return skerr.Fmt("Machine event was empty")
	}
	sklog.Infof("Processing %s from machine %s", event, desc.ID)
	currentTask := extraData[machine.CurrentTaskAttribute]

	switch event {
	default:
		return skerr.Fmt("Unknown event %s", event)
	case machine.Booted:
		// TODO(kjlubick) Check that either we haven't seen this machine before, or the previous
		//  event was rebooting or sitting idle
		// TODO(kjlubick) check the health of this machine.
		if err := m.setMachine(ctx, desc.ID, fs_entries.Machine{
			Status:      machine.Ready,
			LastEvent:   machine.Booted,
			Description: desc,
		}); err != nil {
			return skerr.Wrapf(err, "updating machine entry to Ready (Booted)")
		}
	case machine.Idle:
		newStatus, newReason := checkHealth(desc)
		if err := m.setMachine(ctx, desc.ID, fs_entries.Machine{
			Status:       newStatus,
			StatusReason: newReason,
			LastEvent:    machine.Booted,
			Description:  desc,
		}); err != nil {
			return skerr.Wrapf(err, "updating machine entry to %s (Idle)", newStatus)
		}
		if newReason == machine.ExcessiveUptimeReason {
			if err := m.scheduleMaintenanceTask(ctx, fs_entries.MaintenanceTask{
				Task:            task.RebootMachine,
				MachineAssigned: desc.ID,
			}); err != nil {
				return skerr.Wrapf(err, "attempting to automatically reboot machine %s", desc.ID)
			}
		}
	case machine.StartedTask:
		if currentTask == "" {
			return skerr.Fmt("no current_task")
		}
		// TODO(kjlubick) Check that the machine had previously finished a task or booted.
		if err := m.setMachine(ctx, desc.ID, fs_entries.Machine{
			Status:      machine.Busy,
			LastEvent:   machine.StartedTask,
			CurrentTask: currentTask,
			Description: desc,
		}); err != nil {
			return skerr.Wrapf(err, "updating machine entry to Busy (StartedTask)")
		}
		now := m.now()
		if err := m.updateTask(ctx, currentTask, task.Running, firestore.Update{Path: fs_entries.TaskStartedField, Value: now}); err != nil {
			return skerr.Wrapf(err, "updating task entry to Running")
		}
	case machine.FinishedTask:
		// A FinishedTask event means the machine is still doing post-task hooks. It might
		// reboot at this point (either host or device) and so we won't check its health
		// until we see an Idle or a Booted.
		if currentTask == "" {
			return skerr.Fmt("no current_task")
		}
		taskStatus := task.TaskStatus(extraData[machine.TaskStatusAttribute])
		if taskStatus == "" {
			return skerr.Fmt("task finished, but we didn't get a status")
		}
		// TODO(kjlubick) Check that the machine was expected to be running a task

		if err := m.setMachine(ctx, desc.ID, fs_entries.Machine{
			Status:      machine.Busy,
			LastEvent:   machine.FinishedTask,
			CurrentTask: currentTask,
			Description: desc,
		}); err != nil {
			return skerr.Wrapf(err, "updating machine entry to Busy (FinishedTask)")
		}
		sklog.Infof("machine %s finished task %s", desc.ID, currentTask)
		now := m.now()
		if err := m.updateTask(ctx, currentTask, taskStatus, firestore.Update{Path: fs_entries.TaskEndedField, Value: now}); err != nil {
			return skerr.Wrapf(err, "updating task entry to Success")
		}
	}
	return nil
}
