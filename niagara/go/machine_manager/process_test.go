package machine_manager

import (
	"context"
	"testing"
	"time"

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
	m := newManagerForTesting(c)
	const testMachine = "machine-0001"

	err := m.process(ctx, machine.Description{
		ID: testMachine,
	}, map[string]string{
		machine.EventAttribute: string(machine.Booted),
	})
	require.NoError(t, err)
	expectToFindReadyMachine(t, c, testMachine)
}

func TestProcess_MachineStartedTask_MarkBusy(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := ifirestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	m := newManagerForTesting(c)
	const testMachine = "machine-0001"
	const testTask = "task-1234"

	createNewTask(t, c, testTask)
	err := m.process(ctx, machine.Description{
		ID: testMachine,
	}, map[string]string{
		machine.EventAttribute:       string(machine.StartedTask),
		machine.CurrentTaskAttribute: testTask,
	})
	require.NoError(t, err)
	expectToFindBusyMachine(t, c, testMachine, testTask)
	expectToFindRunningTask(t, c, testTask)
}

func TestProcess_MachineFinishedTaskSuccess_UpdateTask(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := ifirestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	m := newManagerForTesting(c)
	const testMachine = "machine-0001"
	const testTask = "task-1234"

	createNewTask(t, c, testTask)
	err := m.process(ctx, machine.Description{
		ID: testMachine,
	}, map[string]string{
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
	m := newManagerForTesting(c)
	const testMachine = "machine-0001"
	const testTask = "task-1234"

	createNewTask(t, c, testTask)
	err := m.process(ctx, machine.Description{
		ID: testMachine,
	}, map[string]string{
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
	m := newManagerForTesting(c)
	const testMachine = "machine-0001"
	const testTask = "task-1234"

	createNewTask(t, c, testTask)
	err := m.process(ctx, machine.Description{
		ID: testMachine,
	}, map[string]string{
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
	m := newManagerForTesting(c)
	const testMachine = "machine-0001"
	const testTask = "task-1234"

	createNewTask(t, c, testTask)
	err := m.process(ctx, machine.Description{
		ID: testMachine,
	}, map[string]string{
		machine.EventAttribute:       string(machine.FinishedTask),
		machine.CurrentTaskAttribute: testTask,
		machine.TaskStatusAttribute:  string(task.Cancelled),
	})
	require.NoError(t, err)
	expectToFindBusyMachine(t, c, testMachine, testTask)
	expectToFindEndedTaskWithState(t, c, testTask, task.Cancelled)
}

func TestProcess_MachineBigUptime_TriggerRebootTask(t *testing.T) {
	unittest.LargeTest(t)
	_, cleanup := ifirestore.NewClientForTesting(t)
	defer cleanup()

	require.Fail(t, "not impl")
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
	assert.Equal(t, taskID, m.CurrentTask)
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

func newManagerForTesting(c *ifirestore.Client) *Manager {
	m := New(c)
	m.now = fakeTimeSource
	return m
}

var mockNow = time.Date(2020, time.February, 2, 2, 2, 0, 0, time.UTC)

func fakeTimeSource() time.Time {
	return mockNow
}
