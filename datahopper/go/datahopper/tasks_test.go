package main

import (
	"bytes"
	"encoding/gob"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/metrics2/events"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
)

// Create a db.TaskDB and taskEventDB.
func setupTasks(t *testing.T, now time.Time) (*taskEventDB, db.TaskDB) {
	tdb := db.NewInMemoryTaskDB()
	edb := &taskEventDB{
		cached: []*events.Event{},
		db:     tdb,
	}
	return edb, tdb
}

// makeTask returns a fake task with only the fields relevant to this test set.
func makeTask(created time.Time, name string, status db.TaskStatus) *db.Task {
	task := &db.Task{
		Created: created,
		TaskKey: db.TaskKey{
			Name: name,
		},
		Status: status,
	}
	return task
}

// assertTaskEvent checks that ev.Data contains task.
func assertTaskEvent(t *testing.T, ev *events.Event, task *db.Task) {
	assert.Equal(t, TASK_STREAM, ev.Stream)
	var other db.Task
	assert.NoError(t, gob.NewDecoder(bytes.NewReader(ev.Data)).Decode(&other))
	deepequal.AssertDeepEqual(t, task, &other)
	assert.True(t, task.Created.Equal(ev.Timestamp))
}

// TestTaskUpdate checks that taskEventDB.update creates the correct Events from Tasks in the DB.
func TestTaskUpdate(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now()
	edb, tdb := setupTasks(t, now)
	start := now.Add(-TIME_PERIODS[len(TIME_PERIODS)-1])
	tasks := []*db.Task{
		// 0: Filtered out -- too early.
		makeTask(start.Add(-time.Minute), "A", db.TASK_STATUS_SUCCESS),
		makeTask(start.Add(time.Minute), "A", db.TASK_STATUS_SUCCESS),
		makeTask(start.Add(2*time.Minute), "A", db.TASK_STATUS_FAILURE),
		// 3: Filtered out -- not Done.
		makeTask(start.Add(3*time.Minute), "A", db.TASK_STATUS_RUNNING),
		makeTask(start.Add(4*time.Minute), "A", db.TASK_STATUS_MISHAP),
		makeTask(start.Add(5*time.Minute), "A", db.TASK_STATUS_FAILURE),
		makeTask(start.Add(6*time.Minute), "B", db.TASK_STATUS_SUCCESS),
		makeTask(start.Add(7*time.Minute), "A", db.TASK_STATUS_SUCCESS),
	}
	assert.NoError(t, tdb.PutTasks(tasks))
	assert.NoError(t, edb.update())
	evs, err := edb.Range(TASK_STREAM, start.Add(-time.Hour), start.Add(time.Hour))
	assert.NoError(t, err)

	expected := append(tasks[1:3], tasks[4:8]...)
	assert.Len(t, evs, len(expected))
	for i, ev := range evs {
		assertTaskEvent(t, ev, expected[i])
	}
}

// TestTaskRange checks that taskEventDB.Range returns Events within the given range.
func TestTaskRange(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now()
	edb, tdb := setupTasks(t, now)
	base := now.Add(-time.Hour)
	tasks := []*db.Task{
		makeTask(base.Add(-time.Nanosecond), "A", db.TASK_STATUS_SUCCESS),
		makeTask(base, "A", db.TASK_STATUS_SUCCESS),
		makeTask(base.Add(time.Nanosecond), "A", db.TASK_STATUS_SUCCESS),
		makeTask(base.Add(time.Minute), "A", db.TASK_STATUS_SUCCESS),
	}
	assert.NoError(t, tdb.PutTasks(tasks))
	assert.NoError(t, edb.update())

	test := func(start, end time.Time, startIdx, count int) {
		evs, err := edb.Range(TASK_STREAM, start, end)
		assert.NoError(t, err)
		assert.Len(t, evs, count)
		for i, ev := range evs {
			assertTaskEvent(t, ev, tasks[startIdx+i])
		}
	}
	before := base.Add(-time.Hour)
	after := base.Add(time.Hour)
	test(before, before, -1, 0)
	test(before, tasks[0].Created, -1, 0)
	test(before, tasks[1].Created, 0, 1)
	test(before, tasks[2].Created, 0, 2)
	test(before, tasks[3].Created, 0, 3)
	test(before, after, 0, 4)
	test(tasks[0].Created, before, -1, 0)
	test(tasks[0].Created, tasks[0].Created, -1, 0)
	test(tasks[0].Created, tasks[1].Created, 0, 1)
	test(tasks[0].Created, tasks[2].Created, 0, 2)
	test(tasks[0].Created, tasks[3].Created, 0, 3)
	test(tasks[0].Created, after, 0, 4)
	test(tasks[1].Created, tasks[0].Created, -1, 0)
	test(tasks[1].Created, tasks[1].Created, -1, 0)
	test(tasks[1].Created, tasks[2].Created, 1, 1)
	test(tasks[1].Created, tasks[3].Created, 1, 2)
	test(tasks[1].Created, after, 1, 3)
	test(tasks[2].Created, tasks[2].Created, -1, 0)
	test(tasks[2].Created, tasks[3].Created, 2, 1)
	test(tasks[2].Created, after, 2, 2)
	test(tasks[3].Created, tasks[3].Created, -1, 0)
	test(tasks[3].Created, after, 3, 1)
	test(after, after, -1, 0)
}

func TestComputeTaskFlakeRate(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now()
	edb, tdb := setupTasks(t, now)
	created := now.Add(-time.Hour)

	tester := newDynamicAggregateFnTester(t, computeTaskFlakeRate)
	expect := func(taskName string, metric string, numer, denom int) {
		tester.AddAssert(map[string]string{
			"task_name": taskName,
			"task_type": "",
			"metric":    metric,
		}, float64(numer)/float64(denom))
	}

	taskCount := 0
	addedTasks := map[string]*db.Task{}
	addTask := func(name string, status db.TaskStatus, retryOf string) string {
		taskCount++
		task := makeTask(created, name, status)
		if retryOf != "" {
			task.RetryOf = retryOf
			if prev, ok := addedTasks[retryOf]; ok {
				task.Attempt = prev.Attempt + 1
			} else {
				// For the "missing-task" case below.
				task.Attempt = 1
			}
		}
		addedTasks[task.Id] = task
		assert.NoError(t, tdb.PutTask(task))
		return task.Id
	}
	{
		name := "NoFlakes"
		addTask(name, db.TASK_STATUS_SUCCESS, "")
		addTask(name, db.TASK_STATUS_SUCCESS, "")
		addTask(name, db.TASK_STATUS_SUCCESS, "")
		addTask(name, db.TASK_STATUS_FAILURE, "")
		addTask(name, db.TASK_STATUS_FAILURE, "")
		expect(name, "flake-rate", 0, 5)
	}
	{
		name := "Mishaps"
		addTask(name, db.TASK_STATUS_FAILURE, "")
		addTask(name, db.TASK_STATUS_FAILURE, "")
		addTask(name, db.TASK_STATUS_FAILURE, "")
		addTask(name, db.TASK_STATUS_MISHAP, "")
		expect(name, "flake-rate", 1, 4)
	}
	{
		name := "RetrySucceeded"
		addTask(name, db.TASK_STATUS_SUCCESS, "")
		orig := addTask(name, db.TASK_STATUS_FAILURE, "")
		addTask(name, db.TASK_STATUS_SUCCESS, orig)
		expect(name, "flake-rate", 1, 3)
	}
	{
		name := "RetryFailed"
		orig := addTask(name, db.TASK_STATUS_FAILURE, "")
		addTask(name, db.TASK_STATUS_FAILURE, orig)
		expect(name, "flake-rate", 0, 2)
	}
	{
		name := "Mix"
		addTask(name, db.TASK_STATUS_SUCCESS, "")
		orig1 := addTask(name, db.TASK_STATUS_FAILURE, "")
		orig2 := addTask(name, db.TASK_STATUS_FAILURE, "")
		addTask(name, db.TASK_STATUS_FAILURE, orig1)
		addTask(name, db.TASK_STATUS_SUCCESS, orig2)
		orig3 := addTask(name, db.TASK_STATUS_MISHAP, "")
		addTask(name, db.TASK_STATUS_SUCCESS, orig3)
		expect(name, "flake-rate", 2, 7)
		sklog.Infof("Pass")
	}
	{
		name := "RetryOriginalMissing"
		addTask(name, db.TASK_STATUS_FAILURE, "")
		addTask(name, db.TASK_STATUS_FAILURE, "missing-task")
		expect(name, "flake-rate", 0, 2)
	}
	{
		name := "LongRetryChain"
		a := addTask(name, db.TASK_STATUS_FAILURE, "")
		b := addTask(name, db.TASK_STATUS_FAILURE, a)
		c := addTask(name, db.TASK_STATUS_FAILURE, b)
		d := addTask(name, db.TASK_STATUS_FAILURE, c)
		e := addTask(name, db.TASK_STATUS_FAILURE, d)
		f := addTask(name, db.TASK_STATUS_FAILURE, e)
		addTask(name, db.TASK_STATUS_SUCCESS, f)
		expect(name, "flake-rate", 6, 7)
	}

	assert.NoError(t, edb.update())
	evs, err := edb.Range(TASK_STREAM, created.Add(-time.Hour), created.Add(time.Hour))
	assert.NoError(t, err)
	assert.Len(t, evs, taskCount)

	tester.Run(evs)
}
