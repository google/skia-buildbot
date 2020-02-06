package machine_manager

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/logging"
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
	fsClient *ifirestore.Client
	logger   Logger
	now      func() time.Time

	machines sync.Map
}

type Logger interface {
	Log(e logging.Entry)
}

func New(client *ifirestore.Client, logger Logger) *Manager {
	return &Manager{
		fsClient: client,
		logger:   logger,
		now:      time.Now,
	}
}

func (m *Manager) StartMachineFirestoreQuery(ctx context.Context) error {
	snap := m.fsClient.Collection(fs_entries.MachinesCollection).Snapshots(ctx)
	err := m.machineSnapshotCycle(snap)
	if err != nil {
		return skerr.Wrapf(err, "getting initial machines")
	}
	go func() {
		for {
			if err := ctx.Err(); err != nil {
				sklog.Debugf("Stopping due to context error: %s", err)
				snap.Stop()
				return
			}
			err := m.machineSnapshotCycle(snap)
			if err != nil {
				sklog.Errorf("Could not update machine cache: %s", err)
				return
			}
		}
	}()
	return nil
}

func (m *Manager) machineSnapshotCycle(snap *firestore.QuerySnapshotIterator) error {
	qs, err := snap.Next()
	if err != nil {
		// TODO(kjlubick) auto-heal this and be less noisy about connection closing.
		return skerr.Wrap(err)
	}
	for _, dc := range qs.Changes {
		id := dc.Doc.Ref.ID
		if dc.Kind == firestore.DocumentRemoved {
			m.machines.Delete(id)
			continue
		}
		entry := fs_entries.Machine{}
		if err := dc.Doc.DataTo(&entry); err != nil {
			sklog.Errorf("corrupt data in firestore, could not unmarshal machine entry with id %s", id)
			continue
		}
		m.machines.Store(id, entry)
	}
	return nil
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

// TODO(kjlubick) account for PubSub events that are out of order. Maybe add a previous event ts
//   field to the machine and log, but don't process those that are too old.
func (m *Manager) process(ctx context.Context, desc machine.Description, extraData map[string]string) error {
	if desc.ID == "" {
		return skerr.Fmt("Machine description had no id")
	}
	event := machine.Event(extraData[machine.EventAttribute])
	if event == "" {
		return skerr.Fmt("Machine event was empty")
	}
	sklog.Infof("Processing %s from machine %s", event, desc.ID)

	prevMachine := m.getMachine(desc.ID)

	currentTask := extraData[machine.CurrentTaskAttribute]
	status := machine.Status("<invalid>")
	reason := machine.NoReason
	switch event {
	default:
		return skerr.Fmt("Unknown event %s", event)
	case machine.Booted:
		// TODO(kjlubick) Is this the place to detect a machine dying in the middle of a task?
		var updatedDesc machine.Description
		updatedDesc, status, reason = checkHealth(desc, prevMachine)
		if err := m.setMachine(ctx, desc.ID, fs_entries.Machine{
			Status:       status,
			StatusReason: reason,
			LastEvent:    event,
			Description:  updatedDesc,
		}); err != nil {
			return skerr.Wrapf(err, "updating machine entry to Ready (Booted)")
		}
	case machine.Idle:
		var updatedDesc machine.Description
		updatedDesc, status, reason = checkHealth(desc, prevMachine)
		if err := m.setMachine(ctx, desc.ID, fs_entries.Machine{
			Status:       status,
			StatusReason: reason,
			LastEvent:    event,
			Description:  updatedDesc,
		}); err != nil {
			return skerr.Wrapf(err, "updating machine entry to %s (Idle)", status)
		}
		if reason == machine.ExcessiveUptimeReason {
			if err := m.scheduleMaintenanceTask(ctx, maintenanceTask{
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
		status = machine.Busy
		if err := m.setMachine(ctx, desc.ID, fs_entries.Machine{
			Status:      status,
			LastEvent:   event,
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
		// TODO(kjlubick) Maybe check that the machine was expected to be running a task
		status = machine.Busy
		if err := m.setMachine(ctx, desc.ID, fs_entries.Machine{
			Status:      status,
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

	case machine.Rebooting:
		if err := m.setMachine(ctx, desc.ID, fs_entries.Machine{
			Status:      status,
			LastEvent:   machine.Rebooting,
			Description: desc,
		}); err != nil {
			return skerr.Wrapf(err, "updating machine entry to Rebooting")
		}
	}
	m.logger.Log(logging.Entry{
		Payload: machineEvent{
			Description:  desc,
			Extra:        extraData,
			Status:       status,
			StatusReason: reason,
		},
		Timestamp: m.now(),
		Severity:  logging.Debug,
		Labels: map[string]string{
			"machineID": desc.ID,
		},
	})
	return nil
}

func (m *Manager) getMachine(machineID string) *fs_entries.Machine {
	raw, inCache := m.machines.Load(machineID)
	if !inCache {
		return nil
	}
	cm, ok := raw.(fs_entries.Machine)
	if !ok {
		sklog.Infof("Deleting corrupt entry in machine cache for id %s", machineID)
		m.machines.Delete(machineID)
		return nil
	}
	return &cm
}

type machineEvent struct {
	Description  machine.Description
	Extra        map[string]string
	Status       machine.Status
	StatusReason machine.StatusReason
}
