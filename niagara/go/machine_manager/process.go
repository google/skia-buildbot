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
	if desc.ID == "" {
		sklog.Warningf("Machine description had no id")
		return
	}
	err := m.process(ctx, desc, msg.Attributes)
	if err != nil {
		sklog.Errorf("Problem processing machine event: %s \ndescription: %v", err, desc)
	}
}

func (m *Manager) process(ctx context.Context, desc machine.Description, extraData map[string]string) error {
	event := machine.Event(extraData[machine.EventAttribute])
	if event == "" {
		return skerr.Fmt("Machine event was empty")
	}
	sklog.Infof("Processing %s from machine %s", event, desc.ID)

	switch event {
	default:
		return skerr.Fmt("Unknown event %s", event)
	case machine.Booted:
		// TODO(kjlubick) Check that either we haven't seen this machine before, or the previous
		//  event was rebooting or sitting idle
		// TODO(kjlubick) check the health of this machine.
		if err := m.updateMachine(ctx, machine.Ready, machine.Booted, desc); err != nil {
			return skerr.Wrapf(err, "updating machine entry to Ready (Booted)")
		}
	case machine.StartedTask:
		if desc.CurrentTask == "" {
			return skerr.Fmt("invalid description - no CurrentTask")
		}
		// TODO(kjlubick) Check that the machine had previously finished a task or booted.
		if err := m.updateMachine(ctx, machine.Busy, machine.StartedTask, desc); err != nil {
			return skerr.Wrapf(err, "updating machine entry to Busy (StartedTask)")
		}
		now := m.now()
		if err := m.updateTask(ctx, desc.CurrentTask, task.Running, firestore.Update{Path: "started", Value: now}); err != nil {
			return skerr.Wrapf(err, "updating task entry to Running")
		}
	case machine.FinishedTask:
		taskStatus := task.TaskStatus(extraData[machine.TaskStatusAttribute])
		if taskStatus == "" {
			return skerr.Fmt("task finished, but we didn't get a status")
		}
		// TODO(kjlubick) Check that the machine was expected to be running a task
		// TODO(kjlubick) check the health of this machine.

		if err := m.updateMachine(ctx, machine.Busy, machine.FinishedTask, desc); err != nil {
			return skerr.Wrapf(err, "updating machine entry to Busy (FinishedTask)")
		}
		sklog.Infof("Quarantined machine %s", desc.ID)
		now := m.now()
		if err := m.updateTask(ctx, desc.CurrentTask, taskStatus, firestore.Update{Path: "completed", Value: now}); err != nil {
			return skerr.Wrapf(err, "updating task entry to Success")
		}

	case machine.AbandonedTask:
		taskStatus := task.TaskStatus(extraData[machine.TaskStatusAttribute])
		if taskStatus == "" {
			return skerr.Fmt("task abandoned, but we didn't get a status")
		}
		// TODO(kjlubick) Check that the machine was expected to be running a task
		// TODO(kjlubick) check the health of this machine.

		if err := m.updateMachine(ctx, machine.Busy, machine.AbandonedTask, desc); err != nil {
			return skerr.Wrapf(err, "updating machine entry to Busy (AbandonedTask)")
		}
		sklog.Infof("Quarantined machine %s", desc.ID)
		now := m.now()
		if err := m.updateTask(ctx, desc.CurrentTask, taskStatus, firestore.Update{Path: "abandoned", Value: now}); err != nil {
			return skerr.Wrapf(err, "updating task entry to Success")
		}
	}
	return nil
}
