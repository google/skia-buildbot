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
}

func New(client *ifirestore.Client) *Manager {
	return &Manager{
		client: client,
	}
}

func (m *Manager) ProcessPubsubMessage(ctx context.Context, msg *pubsub.Message) {
	msg.Ack()

	var state machine.State
	if err := json.Unmarshal(msg.Data, &state); err != nil {
		sklog.Errorf("Invalid message data %s: %s", err, string(msg.Data))
		return
	}
	if state.ID == "" {
		sklog.Warningf("Machine state had no id")
		return
	}
	event := machine.Event(msg.Attributes[machine.EventAttribute])
	if event == "" {
		sklog.Warningf("Machine event was empty")
		return
	}
	sklog.Infof("Got %s from machine %s", event, state.ID)
	err := m.process(ctx, state, event)
	if err != nil {
		sklog.Errorf("Problem processing machine event: %s \nstate: %v", err, state)
	}
}

func (m *Manager) process(ctx context.Context, state machine.State, event machine.Event) error {
	switch event {
	default:
		return skerr.Fmt("Unknown event %s", event)
	case machine.Booted:
		// TODO(kjlubick) Check that either we haven't seen this machine before, or the previous
		//  state was rebooting or sitting idle
		// TODO(kjlubick) check the health of this machine.
		if err := m.updateMachine(ctx, machine.Ready, machine.Booted, state); err != nil {
			return skerr.Wrapf(err, "updating machine entry to Ready (Booted)")
		}
	case machine.StartedTask:
		// TODO(kjlubick) Check that the machine had previously finished a task or booted.
		if err := m.updateMachine(ctx, machine.Busy, machine.StartedTask, state); err != nil {
			return skerr.Wrapf(err, "updating machine entry to Busy (StartedTask)")
		}
		now := time.Now()
		if err := m.updateTask(ctx, state.CurrentTask, task.Running, firestore.Update{Path: "started", Value: now}); err != nil {
			return skerr.Wrapf(err, "updating task entry to Running")
		}
	case machine.FinishedTask:
		// TODO(kjlubick) Somehow need to know if a task failed, had an infra failure, or something
		//   else when horribly wrong (niagara failure).
		// TODO(kjlubick) Check that the machine was expected to be running a task
		// TODO(kjlubick) check the health of this machine.

		// FIXME(kjlubick) : we shouldn't actually quarantine machines when they are done.
		if err := m.updateMachine(ctx, machine.Quarantined, machine.FinishedTask, state); err != nil {
			return skerr.Wrapf(err, "updating machine entry to Quarantined")
		}
		sklog.Infof("Quarantined machine %s", state.ID)
		now := time.Now()
		if err := m.updateTask(ctx, state.CurrentTask, task.Success, firestore.Update{Path: "completed", Value: now}); err != nil {
			return skerr.Wrapf(err, "updating task entry to Success")
		}
	}
	return nil
}
