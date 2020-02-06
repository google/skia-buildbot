package machine_manager

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/niagara/go/fs_entries"
	"go.skia.org/infra/niagara/go/machine"
	"go.skia.org/infra/niagara/go/task"
)

func TestProcess_FirstTimeSeeingMachine_MarkReady(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := ifirestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	m, fl := newManagerForTesting(t, c)
	const testMachine = "machine-0001"

	desc := healthyMachineNoDevices(testMachine)
	extra := map[string]string{
		machine.EventAttribute: string(machine.Booted),
	}
	err := m.process(ctx, desc, extra)
	require.NoError(t, err)
	expectToFindReadyMachine(t, c, testMachine)
	expectToHaveLogged(t, fl, testMachine, []machineEvent{
		{Description: desc, Extra: extra, Status: machine.Ready},
	})
}

func TestProcess_MachineStartedTask_MarkBusy(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := ifirestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	m, fl := newManagerForTesting(t, c)
	const testMachine = "machine-0001"
	const testTask = "task-1234"

	createNewTask(t, c, testTask)
	desc := healthyMachineNoDevices(testMachine)
	extra := map[string]string{
		machine.EventAttribute:       string(machine.StartedTask),
		machine.CurrentTaskAttribute: testTask,
	}
	err := m.process(ctx, desc, extra)
	require.NoError(t, err)
	expectToFindBusyMachine(t, c, testMachine, testTask)
	expectToFindRunningTask(t, c, testTask)
	expectToHaveLogged(t, fl, testMachine, []machineEvent{
		{Description: desc, Extra: extra, Status: machine.Busy},
	})
}

func TestProcess_MachineFinishedTaskSuccess_UpdateTask(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := ifirestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	m, _ := newManagerForTesting(t, c)
	const testMachine = "machine-0001"
	const testTask = "task-1234"

	createNewTask(t, c, testTask)
	err := m.process(ctx, healthyMachineNoDevices(testMachine), map[string]string{
		machine.EventAttribute:       string(machine.FinishedTask),
		machine.CurrentTaskAttribute: testTask,
		machine.TaskStatusAttribute:  string(task.Success),
	})
	require.NoError(t, err)
	expectToFindBusyMachine(t, c, testMachine, testTask)
	expectToFindEndedTaskWithState(t, c, testTask, task.Success)

}

func TestProcess_MachineFinishedTaskFailure_UpdateTask(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := ifirestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	m, _ := newManagerForTesting(t, c)
	const testMachine = "machine-0001"
	const testTask = "task-1234"

	createNewTask(t, c, testTask)
	err := m.process(ctx, healthyMachineNoDevices(testMachine), map[string]string{
		machine.EventAttribute:       string(machine.FinishedTask),
		machine.CurrentTaskAttribute: testTask,
		machine.TaskStatusAttribute:  string(task.Failure),
	})
	require.NoError(t, err)
	expectToFindBusyMachine(t, c, testMachine, testTask)
	expectToFindEndedTaskWithState(t, c, testTask, task.Failure)
}

func TestProcess_MachineFinishedTaskTimedOut_UpdateTask(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := ifirestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	m, _ := newManagerForTesting(t, c)
	const testMachine = "machine-0001"
	const testTask = "task-1234"

	createNewTask(t, c, testTask)
	err := m.process(ctx, healthyMachineNoDevices(testMachine), map[string]string{
		machine.EventAttribute:       string(machine.FinishedTask),
		machine.CurrentTaskAttribute: testTask,
		machine.TaskStatusAttribute:  string(task.TimedOut),
	})
	require.NoError(t, err)
	expectToFindBusyMachine(t, c, testMachine, testTask)
	expectToFindEndedTaskWithState(t, c, testTask, task.TimedOut)
}

func TestProcess_MachineFinishedTaskCancelled_UpdateTask(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := ifirestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	m, _ := newManagerForTesting(t, c)
	const testMachine = "machine-0001"
	const testTask = "task-1234"

	createNewTask(t, c, testTask)
	err := m.process(ctx, healthyMachineNoDevices(testMachine), map[string]string{
		machine.EventAttribute:       string(machine.FinishedTask),
		machine.CurrentTaskAttribute: testTask,
		machine.TaskStatusAttribute:  string(task.Cancelled),
	})
	require.NoError(t, err)
	expectToFindBusyMachine(t, c, testMachine, testTask)
	expectToFindEndedTaskWithState(t, c, testTask, task.Cancelled)
}

func TestProcess_MachineIdleBigUptime_TriggerRebootTask(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := ifirestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	m, fl := newManagerForTesting(t, c)
	const testMachine = "machine-0001"

	createMaintenanceTask(t, c, maintenanceTask{
		Task:            task.RebootMachine,
		MachineAssigned: testMachine,
	}, task.Success, mockNow.Add(-time.Hour))

	needsUpdating := healthyMachineNoDevices(testMachine)
	// 14 hours is on the outer cusp of the reboot logic, so this tests the RNG calculation,
	// but p = 1.0 and will always reboot.
	needsUpdating.Uptime = 14 * time.Hour
	extra := map[string]string{
		machine.EventAttribute: string(machine.Idle),
	}
	err := m.process(ctx, needsUpdating, extra)
	require.NoError(t, err)
	expectToFindQuarantinedMachine(t, c, testMachine, machine.ExcessiveUptimeReason)
	expectToFindOneNewMaintenanceTask(t, c, maintenanceTask{
		Task:            task.RebootMachine,
		MachineAssigned: testMachine,
	}, mockNow)
	expectToHaveLogged(t, fl, testMachine, []machineEvent{
		{Description: needsUpdating, Extra: extra, Status: machine.Quarantined, StatusReason: machine.ExcessiveUptimeReason},
	})
}

func TestProcess_MachineIdleBigUptimeTaskAlreadyTriggered_JustLog(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := ifirestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	m, fl := newManagerForTesting(t, c)
	const testMachine = "machine-0001"

	var preExistingTaskTime = mockNow.Add(-5 * time.Second)
	createMaintenanceTask(t, c, maintenanceTask{
		Task:            task.RebootMachine,
		MachineAssigned: testMachine,
	}, task.New, preExistingTaskTime)

	needsUpdating := healthyMachineNoDevices(testMachine)
	needsUpdating.Uptime = 15 * time.Hour
	extra := map[string]string{
		machine.EventAttribute: string(machine.Idle),
	}
	err := m.process(ctx, needsUpdating, extra)
	require.NoError(t, err)
	expectToFindQuarantinedMachine(t, c, testMachine, machine.ExcessiveUptimeReason)
	expectToFindOneNewMaintenanceTask(t, c, maintenanceTask{
		Task:            task.RebootMachine,
		MachineAssigned: testMachine,
	}, preExistingTaskTime)
	expectToHaveLogged(t, fl, testMachine, []machineEvent{
		{Description: needsUpdating, Extra: extra, Status: machine.Quarantined, StatusReason: machine.ExcessiveUptimeReason},
	})
}

func TestProcess_MachineIdleDeviceDisconnected_Quarantine(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := ifirestore.NewClientForTesting(t)
	defer cleanup()
	const testMachine = "machine-0001"
	const testDevice = "crosshatch"

	createMachine(t, c, fs_entries.Machine{
		Status:      machine.Busy,
		CurrentTask: "does not matter",
		LastEvent:   machine.FinishedTask,
		Updated:     mockNow.Add(-30 * time.Second),
		Description: machine.Description{
			ID:             testMachine,
			DeviceAttached: testDevice,
			// crosshatch would also be in the dimensions in a real machine, but we would prefer
			// to keep Dimensions an opaque blob to Machine Manager.
		},
	})

	ctx := context.Background()
	m, fl := newManagerForTesting(t, c)
	reportedWithoutDevice := healthyMachineNoDevices(testMachine)
	extra := map[string]string{
		machine.EventAttribute: string(machine.Idle),
	}
	err := m.process(ctx, reportedWithoutDevice, extra)
	require.NoError(t, err)
	actualMachine := expectToFindQuarantinedMachine(t, c, testMachine, machine.DeviceMissing)
	// make sure the Machine stored preserves the expected device from the cached version.
	assert.Equal(t, testDevice, actualMachine.Description.DeviceAttached)
	// Machine manager should not trigger a MaintenanceTask or anything
	expectToFindNoTasks(t, c)
	// We log the event/description we received, which does not have the DeviceAttached
	// set to what it was previous.
	expectToHaveLogged(t, fl, testMachine, []machineEvent{
		{Description: reportedWithoutDevice, Extra: extra, Status: machine.Quarantined, StatusReason: machine.DeviceMissing},
	})

}

func createMachine(t *testing.T, client *ifirestore.Client, m fs_entries.Machine) {
	doc := client.Collection(fs_entries.MachinesCollection).Doc(m.Description.ID)
	_, err := doc.Create(context.Background(), m)
	require.NoError(t, err)
}

func createMaintenanceTask(t *testing.T, client *ifirestore.Client, mt maintenanceTask, status task.TaskStatus, ts time.Time) {
	doc := client.Collection(fs_entries.TasksCollection).NewDoc()
	newT := fs_entries.Task{
		MachineAssigned: mt.MachineAssigned,
		Status:          status,
		MaintenanceTask: mt.Task,
		Config:          mt.Config,
		Created:         ts,
	}
	if status == task.Success {
		newT.Started = ts
		newT.Ended = ts
	}
	_, err := doc.Create(context.Background(), newT)
	require.NoError(t, err)
}

func createNewTask(t *testing.T, client *ifirestore.Client, taskID string) {
	doc := client.Collection(fs_entries.TasksCollection).Doc(taskID)
	_, err := doc.Create(context.Background(), fs_entries.Task{
		MachineAssigned: "",
		Command:         "docker version",
		Status:          task.New,
		Created:         mockNow.Add(-time.Minute),
	})
	require.NoError(t, err)
}

func expectToFindReadyMachine(t *testing.T, client *ifirestore.Client, machineID string) {
	doc := client.Collection(fs_entries.MachinesCollection).Doc(machineID)
	ds, err := doc.Get(context.Background())
	require.NoError(t, err)
	assert.True(t, ds.Exists())
	var m fs_entries.Machine
	err = ds.DataTo(&m)
	require.NoError(t, err)
	assert.Equal(t, machine.Ready, m.Status)
	assert.Equal(t, machine.NoReason, m.StatusReason)
	assert.Equal(t, "", m.CurrentTask)
}

func expectToFindBusyMachine(t *testing.T, client *ifirestore.Client, machineID, taskID string) {
	doc := client.Collection(fs_entries.MachinesCollection).Doc(machineID)
	ds, err := doc.Get(context.Background())
	require.NoError(t, err)
	assert.True(t, ds.Exists())
	var m fs_entries.Machine
	err = ds.DataTo(&m)
	require.NoError(t, err)
	assert.Equal(t, machine.Busy, m.Status)
	assert.Equal(t, machine.NoReason, m.StatusReason)
	assert.Equal(t, taskID, m.CurrentTask)
}

func expectToFindQuarantinedMachine(t *testing.T, client *ifirestore.Client, machineID string, reason machine.StatusReason) fs_entries.Machine {
	doc := client.Collection(fs_entries.MachinesCollection).Doc(machineID)
	ds, err := doc.Get(context.Background())
	require.NoError(t, err)
	assert.True(t, ds.Exists())
	var m fs_entries.Machine
	err = ds.DataTo(&m)
	require.NoError(t, err)
	assert.Equal(t, machine.Quarantined, m.Status)
	assert.Equal(t, reason, m.StatusReason)
	assert.Equal(t, "", m.CurrentTask)
	return m
}

func expectToFindRunningTask(t *testing.T, client *ifirestore.Client, taskID string) {
	doc := client.Collection(fs_entries.TasksCollection).Doc(taskID)
	ds, err := doc.Get(context.Background())
	require.NoError(t, err)
	var ft fs_entries.Task
	err = ds.DataTo(&ft)
	require.NoError(t, err)
	assert.Equal(t, task.Running, ft.Status)
	assert.Equal(t, mockNow, ft.Updated)
	assert.Zero(t, ft.Ended)
}

func expectToFindEndedTaskWithState(t *testing.T, client *ifirestore.Client, taskID string, s task.TaskStatus) {
	doc := client.Collection(fs_entries.TasksCollection).Doc(taskID)
	ds, err := doc.Get(context.Background())
	require.NoError(t, err)
	var ft fs_entries.Task
	err = ds.DataTo(&ft)
	require.NoError(t, err)
	assert.Equal(t, s, ft.Status)
	assert.Equal(t, mockNow, ft.Updated)
	assert.Equal(t, mockNow, ft.Ended)
}

func expectToFindOneNewMaintenanceTask(t *testing.T, client *ifirestore.Client, expected maintenanceTask, created time.Time) {
	q := client.Collection(fs_entries.TasksCollection).Where(fs_entries.TaskMachineAssignedField, "==", expected.MachineAssigned).
		Where(fs_entries.MaintenanceTaskField, "==", expected.Task).Where(fs_entries.TaskStatusField, "==", task.New)
	di := q.Documents(context.Background())
	xd, err := di.GetAll()
	require.NoError(t, err)
	require.Len(t, xd, 1)
	var mt fs_entries.Task
	err = xd[0].DataTo(&mt)
	require.NoError(t, err)
	assert.Equal(t, fs_entries.Task{
		MachineAssigned: expected.MachineAssigned,
		Status:          task.New,
		Command:         "",
		MaintenanceTask: expected.Task,
		Config:          expected.Config,
		Created:         created,
	}, mt)
}

func expectToFindNoTasks(t *testing.T, client *ifirestore.Client) {
	di := client.Collection(fs_entries.TasksCollection).Documents(context.Background())
	xd, err := di.GetAll()
	require.NoError(t, err)
	assert.Empty(t, xd)
}

func newManagerForTesting(t *testing.T, c *ifirestore.Client) (*Manager, *fakeLogger) {
	fl := &fakeLogger{}
	m := New(c, fl)
	m.now = func() time.Time {
		return mockNow
	}
	require.NoError(t, m.StartMachineFirestoreQuery(context.Background()))
	return m, fl
}

var mockNow = time.Date(2020, time.February, 2, 2, 2, 0, 0, time.UTC)

func healthyMachineNoDevices(machineID string) machine.Description {
	return machine.Description{
		ID:     machineID,
		Uptime: 1 * time.Hour, // this is under the reboot threshold
	}
}

type fakeLogger struct {
	entries []logging.Entry
}

func (f *fakeLogger) Log(e logging.Entry) {
	f.entries = append(f.entries, e)
}

func expectToHaveLogged(t *testing.T, fl *fakeLogger, machineID string, evts []machineEvent) {
	require.Len(t, fl.entries, len(evts))
	for i, e := range evts {
		actual := fl.entries[i]
		assert.Equal(t, e, actual.Payload)
		assert.Equal(t, machineID, actual.Labels["machineID"])
	}
}
