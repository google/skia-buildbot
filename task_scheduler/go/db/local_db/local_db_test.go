package local_db

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
)

func TestMain(m *testing.M) {
	db.AssertDeepEqual = deepequal.AssertDeepEqual
	os.Exit(m.Run())
}

// Check that formatId and ParseId are inverse operations and produce the
// expected result.
func TestFormatParseId(t *testing.T) {
	testutils.SmallTest(t)
	testCases := []struct {
		ts  time.Time
		seq uint64
		id  string
	}{
		{
			ts:  time.Date(2009, time.November, 10, 23, 45, 6, 1500, time.UTC),
			seq: 0,
			id:  "20091110T234506.000001500Z_0000000000000000",
		},
		{
			ts:  time.Date(2001, time.February, 3, 4, 5, 6, 0, time.FixedZone("fake", 45*60)),
			seq: 1,
			// Subtract 45 minutes due to zone.
			id: "20010203T032006.000000000Z_0000000000000001",
		},
		{
			ts:  time.Date(2001, time.January, 1, 1, 1, 1, 100000000, time.UTC),
			seq: 15,
			id:  "20010101T010101.100000000Z_000000000000000f",
		},
		{
			ts:  time.Date(2001, time.January, 1, 1, 1, 1, 100000000, time.UTC),
			seq: 16,
			id:  "20010101T010101.100000000Z_0000000000000010",
		},
		{
			ts:  time.Date(2001, time.January, 1, 1, 1, 1, 100000000, time.UTC),
			seq: 255,
			id:  "20010101T010101.100000000Z_00000000000000ff",
		},
		{
			ts:  time.Date(2001, time.January, 1, 1, 1, 1, 100000000, time.UTC),
			seq: 0xFFFFFFFFFFFFFFFF,
			id:  "20010101T010101.100000000Z_ffffffffffffffff",
		},
	}
	for _, testCase := range testCases {
		assert.Equal(t, testCase.id, formatId(testCase.ts, testCase.seq))
		ts, seq, err := ParseId(testCase.id)
		assert.NoError(t, err)
		assert.True(t, testCase.ts.Equal(ts))
		assert.Equal(t, testCase.seq, seq)
		assert.Equal(t, time.UTC, ts.Location())
	}

	// Invalid timestamps:
	for _, invalidId := range []string{
		// Missing seq num.
		"20091110T234506.000001500Z",
		// Two-digit year.
		"091110T234506.000001500Z_0000000000000000",
		// Invalid month.
		"20010001T010101.100000000Z_000000000000000f",
		// Missing T.
		"20010101010101.100000000Z_000000000000000f",
		// Missing Z.
		"20010101T010101.100000000_000000000000000f",
		// Empty seq num.
		"20010101T010101.100000000Z_",
		// Invalid char in seq num.
		"20010101T010101.100000000Z_000000000000000g",
		// Invalid char in seq num.
		"20010101T010101.100000000Z_g000000000000000",
		// Empty timestamp.
		"_000000000000000f",
		// Sequence num overflows.
		"20010101T010101.100000000Z_1ffffffffffffffff",
	} {
		_, _, err := ParseId(invalidId)
		assert.Error(t, err, fmt.Sprintf("No error for Id: %q", invalidId))
	}
}

// Check that packV1 and unpackV1 are inverse operations and produce the
// expected result.
func TestPackUnpackV1(t *testing.T) {
	testutils.SmallTest(t)
	testCases := []struct {
		ts     time.Time
		data   []byte
		packed []byte
	}{
		{
			ts:     time.Unix(0, 0x1174f263b54399dc),
			data:   []byte{0xab, 0xcd, 0xef, 0x01, 0x23},
			packed: []byte{0x01, 0x11, 0x74, 0xf2, 0x63, 0xb5, 0x43, 0x99, 0xdc, 0xab, 0xcd, 0xef, 0x01, 0x23},
		},
		{
			ts:     time.Date(2262, time.April, 11, 23, 47, 16, 854775807, time.UTC),
			data:   []byte("Hi Mom!"),
			packed: append([]byte{0x01, 0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, "Hi Mom!"...),
		},
		{
			ts:     time.Unix(0, 0),
			data:   []byte{},
			packed: []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
	}
	for _, testCase := range testCases {
		assert.Equal(t, testCase.packed, packV1(testCase.ts, testCase.data))
		ts, data, err := unpackV1(testCase.packed)
		assert.NoError(t, err)
		assert.True(t, testCase.ts.Equal(ts))
		assert.Equal(t, testCase.data, data)
		assert.Equal(t, time.UTC, ts.Location())
	}

	for _, invalid := range [][]byte{
		{},
		{0x00},
		{0x01, 0x00, 0x00},
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	} {
		_, _, err := unpackV1(invalid)
		assert.Error(t, err)
	}
}

// Create a localDB for testing. Call defer util.RemoveAll() on the second
// return value.
func makeDB(t *testing.T, name string) (db.BackupDBCloser, string) {
	tmpdir, err := ioutil.TempDir("", name)
	assert.NoError(t, err)
	d, err := NewDB(name, filepath.Join(tmpdir, "task.db"))
	assert.NoError(t, err)
	return d, tmpdir
}

// Test that AssignId returns an error if Id is set.
func TestAssignIdAlreadyAssigned(t *testing.T) {
	testutils.MediumTest(t)
	d, tmpdir := makeDB(t, "TestAssignIdAlreadyAssigned")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)

	task := &types.Task{}
	assert.NoError(t, d.AssignId(task))
	assert.Error(t, d.AssignId(task))
}

// Test that AssignId uses created timestamp when set, and generates unique IDs
// for the same timestamp.
func TestAssignIdsFromCreatedTs(t *testing.T) {
	testutils.LargeTest(t) // Creates a lot of tasks.

	d, tmpdir := makeDB(t, "TestAssignIdsFromCreatedTs")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)

	tasks := []*types.Task{}
	addTask := func(ts time.Time) {
		task := &types.Task{
			Created: ts,
		}
		assert.NoError(t, d.AssignId(task))
		tasks = append(tasks, task)
	}

	// Add tasks with various creation timestamps.
	addTask(time.Date(2008, time.August, 8, 8, 8, 8, 8, time.UTC))
	addTask(time.Date(1776, time.July, 4, 13, 0, 0, 0, time.UTC))
	addTask(time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC))
	addTask(time.Date(2016, time.December, 31, 23, 59, 59, 999999999, time.UTC))
	// Repeated timestamps.
	addTask(time.Date(2008, time.August, 8, 8, 8, 8, 8, time.UTC))
	addTask(time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC))
	for i := 0; i < 256; i++ {
		addTask(time.Date(2008, time.August, 8, 8, 8, 8, 8, time.UTC))
	}

	// Collect IDs. Assert Id is set.
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		assert.NotEqual(t, "", task.Id)
		ids = append(ids, task.Id)
	}

	// Stable-sort tasks.
	sort.Stable(types.TaskSlice(tasks))

	// Sort IDs.
	sort.Strings(ids)

	// Validate that sorted IDs match sorted Tasks. Check that there are no
	// duplicate IDs. Check that ID timestamp matches created timestamp.
	prevId := ""
	for i := 0; i < len(tasks); i++ {
		assert.Equal(t, ids[i], tasks[i].Id)
		assert.NotEqual(t, prevId, ids[i])
		ts, _, err := ParseId(ids[i])
		assert.NoError(t, err)
		assert.True(t, ts.Equal(tasks[i].Created))
		prevId = ids[i]
	}
}

// Test that AssignId can generate ids when created timestamp is not set, and
// generates unique IDs for PutTasks.
func TestAssignIdsFromCurrentTime(t *testing.T) {
	testutils.MediumTest(t)
	d, tmpdir := makeDB(t, "TestAssignIdsFromCurrentTime")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)

	tasks := []*types.Task{}
	for i := 0; i < 260; i++ {
		tasks = append(tasks, &types.Task{})
	}

	begin := time.Now()

	// Test AssignId.
	assert.NoError(t, d.AssignId(tasks[5]))
	assert.NoError(t, d.AssignId(tasks[6]))
	id5, id6 := tasks[5].Id, tasks[6].Id

	// Created time is required.
	for _, task := range tasks {
		task.Created = time.Now()
	}

	// Test PutTasks.
	assert.NoError(t, d.PutTasks(tasks))

	end := time.Now()

	// Check that PutTasks did not change existing Ids.
	assert.Equal(t, id5, tasks[5].Id)
	assert.Equal(t, id6, tasks[6].Id)

	// Order tasks by time of ID assignment.
	first2 := []*types.Task{tasks[5], tasks[6]}
	copy(tasks[2:7], tasks[0:5])
	copy(tasks[0:2], first2)

	// Collect IDs. Assert Id is set.
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		assert.NotEqual(t, "", task.Id)
		ids = append(ids, task.Id)
	}

	// Sort IDs.
	sort.Strings(ids)

	// Validate that sorted IDs match Tasks by insertion order. Check that there
	// are no duplicate IDs. Check that begin <= ID timestamp <= end.
	prevId := ""
	for i := 0; i < len(tasks); i++ {
		assert.Equal(t, ids[i], tasks[i].Id)
		assert.NotEqual(t, prevId, ids[i])
		ts, _, err := ParseId(ids[i])
		assert.NoError(t, err)
		assert.True(t, begin.Before(ts) || begin.Equal(ts))
		assert.True(t, ts.Before(end) || ts.Equal(end))
		prevId = ids[i]
	}
}

// Test that PutTask returns an error when AssignId time is too far before (or
// after) the value subsequently assigned to Task.Created.
func TestPutTaskValidateCreatedTime(t *testing.T) {
	testutils.MediumTest(t)
	d, tmpdir := makeDB(t, "TestPutTaskValidateCreatedTime")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)

	task := &types.Task{}
	beforeAssignId := time.Now().Add(-time.Nanosecond)
	assert.NoError(t, d.AssignId(task))
	afterAssignId := time.Now().Add(time.Nanosecond)

	// Test "not set".
	{
		err := d.PutTask(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Created not set.")
	}

	// Test "too late".
	{
		task.Created = afterAssignId.Add(MAX_CREATED_TIME_SKEW)
		err := d.PutTask(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Created too late.")
	}

	// Test "too early".
	{
		task.Created = beforeAssignId.Add(-MAX_CREATED_TIME_SKEW)
		err := d.PutTask(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Created too early.")

		// Verify not in DB.
		noTask, err := d.GetTaskById(task.Id)
		assert.NoError(t, err)
		assert.Nil(t, noTask)
	}

	// Test late but within range.
	{
		task.Created = beforeAssignId.Add(MAX_CREATED_TIME_SKEW)
		err := d.PutTask(task)
		assert.NoError(t, err)

		// Verify added to DB.
		taskCopy, err := d.GetTaskById(task.Id)
		assert.NoError(t, err)
		deepequal.AssertDeepEqual(t, task, taskCopy)
	}

	// We can even change the Created time if we want. (Not necessarily supported
	// by all DB implementations.)
	// Test early but within range.
	{
		task.Created = afterAssignId.Add(-MAX_CREATED_TIME_SKEW)
		err := d.PutTask(task)
		assert.NoError(t, err)

		// Verify added to DB.
		taskCopy, err := d.GetTaskById(task.Id)
		assert.NoError(t, err)
		deepequal.AssertDeepEqual(t, task, taskCopy)
	}

	// But we can't change it to be out of range.
	{
		prevCreated := task.Created
		task.Created = beforeAssignId.Add(-MAX_CREATED_TIME_SKEW)
		err := d.PutTask(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Created too early.")

		taskCopy, err := d.GetTaskById(task.Id)
		assert.NoError(t, err)
		assert.True(t, prevCreated.Equal(taskCopy.Created))
	}
}

// Test that PutTask/s does not modify the passed-in Tasks when there is an
// error.
func TestPutTaskLeavesTasksUnchanged(t *testing.T) {
	testutils.MediumTest(t)
	d, tmpdir := makeDB(t, "TestPutTaskLeavesTasksUnchanged")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)

	begin := time.Now().Add(-time.Nanosecond)

	// Create and insert a task that will cause ErrConcurrentUpdate.
	task1 := &types.Task{
		Created: time.Now(),
	}
	assert.NoError(t, d.PutTask(task1))

	// Retrieve a copy, modify original.
	task1Cached, err := d.GetTaskById(task1.Id)
	assert.NoError(t, err)
	task1.Status = types.TASK_STATUS_RUNNING
	assert.NoError(t, d.PutTask(task1))
	task1InDb := task1.Copy()

	// Create and insert a task to check PutTasks doesn't change DbModified.
	task2 := &types.Task{
		Created: time.Now(),
	}
	assert.NoError(t, d.PutTask(task2))
	task2InDb := task2.Copy()
	task2.Status = types.TASK_STATUS_MISHAP

	// Create a task with an Id already set.
	task3 := &types.Task{}
	assert.NoError(t, d.AssignId(task3))
	task3.Created = time.Now()

	// Create a task without an Id set.
	task4 := &types.Task{
		Created: time.Now(),
	}

	// Make an update to task1Cached.
	task1Cached.Commits = []string{"a", "b"}

	// Copy to compare later.
	expectedTasks := []*types.Task{task1Cached.Copy(), task2.Copy(), task3.Copy(), task4.Copy()}

	// Attempt to insert; put task1Cached last so that the error comes last.
	err = d.PutTasks([]*types.Task{task2, task3, task4, task1Cached})
	assert.True(t, db.IsConcurrentUpdate(err))
	deepequal.AssertDeepEqual(t, expectedTasks, []*types.Task{task1Cached, task2, task3, task4})

	// Check that nothing was updated in the DB.
	tasksInDb, err := d.GetTasksFromDateRange(begin, time.Now(), "")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(tasksInDb))
	for _, task := range tasksInDb {
		switch task.Id {
		case task1.Id:
			deepequal.AssertDeepEqual(t, task1InDb, task)
		case task2.Id:
			deepequal.AssertDeepEqual(t, task2InDb, task)
		default:
			assert.Fail(t, "Unexpected task in DB: %v", task)
		}
	}
}

// Test that PutJob uses Created timestamp, and generates unique IDs for the
// same timestamp.
func TestJobIdsFromCreatedTs(t *testing.T) {
	testutils.LargeTest(t) // Creates a lot of jobs.

	d, tmpdir := makeDB(t, "TestJobIdsFromCreatedTs")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)

	jobs := []*types.Job{}
	addJob := func(ts time.Time) {
		job := &types.Job{
			Created: ts,
		}
		assert.NoError(t, d.PutJob(job))
		jobs = append(jobs, job)
	}

	// Add jobs with various creation timestamps.
	addJob(time.Date(2008, time.August, 8, 8, 8, 8, 8, time.UTC))
	addJob(time.Date(1776, time.July, 4, 13, 0, 0, 0, time.UTC))
	addJob(time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC))
	addJob(time.Date(2016, time.December, 31, 23, 59, 59, 999999999, time.UTC))
	// Repeated timestamps.
	addJob(time.Date(2008, time.August, 8, 8, 8, 8, 8, time.UTC))
	addJob(time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC))
	for i := 0; i < 256; i++ {
		addJob(time.Date(2008, time.August, 8, 8, 8, 8, 8, time.UTC))
	}

	// Collect IDs. Assert Id is set.
	ids := make([]string, 0, len(jobs))
	for _, job := range jobs {
		assert.NotEqual(t, "", job.Id)
		ids = append(ids, job.Id)
	}

	// Stable-sort jobs.
	sort.Stable(types.JobSlice(jobs))

	// Sort IDs.
	sort.Strings(ids)

	// Validate that sorted IDs match sorted Jobs. Check that there are no
	// duplicate IDs. Check that ID timestamp matches created timestamp.
	prevId := ""
	for i := 0; i < len(jobs); i++ {
		assert.Equal(t, ids[i], jobs[i].Id)
		assert.NotEqual(t, prevId, ids[i])
		ts, _, err := ParseId(ids[i])
		assert.NoError(t, err)
		assert.True(t, ts.Equal(jobs[i].Created))
		prevId = ids[i]
	}
}

// Test that PutJob returns an error when Job.Created is not set or when
// modified after insertion.
func TestPutJobValidateCreatedTime(t *testing.T) {
	testutils.MediumTest(t)
	d, tmpdir := makeDB(t, "TestPutJobValidateCreatedTime")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)

	job := &types.Job{}

	// Test "not set".
	{
		err := d.PutJob(job)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Created not set.")
	}

	job.Created = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	assert.NoError(t, d.PutJob(job))

	{
		// Verify added to DB.
		jobCopy, err := d.GetJobById(job.Id)
		assert.NoError(t, err)
		deepequal.AssertDeepEqual(t, job, jobCopy)
	}

	// Test changing Created time.
	{
		jobBefore := job.Copy()

		job.Created = job.Created.Add(time.Nanosecond)
		err := d.PutJob(job)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Created time has changed")

		jobAfter, err := d.GetJobById(job.Id)
		assert.NoError(t, err)
		deepequal.AssertDeepEqual(t, jobBefore, jobAfter)
	}
}

// Test that PutJob/s does not modify the passed-in Jobs when there is an error.
func TestPutJobLeavesJobsUnchanged(t *testing.T) {
	testutils.MediumTest(t)
	d, tmpdir := makeDB(t, "TestPutJobLeavesJobsUnchanged")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)

	begin := time.Now().Add(-time.Nanosecond)

	// Create and insert a job that will cause ErrConcurrentUpdate.
	job1 := &types.Job{
		Created: time.Now(),
	}
	assert.NoError(t, d.PutJob(job1))

	// Retrieve a copy, modify original.
	job1Cached, err := d.GetJobById(job1.Id)
	assert.NoError(t, err)
	job1.Status = types.JOB_STATUS_SUCCESS
	assert.NoError(t, d.PutJob(job1))
	job1InDb := job1.Copy()

	// Create and insert a job to check PutJobs doesn't change DbModified.
	job2 := &types.Job{
		Created: time.Now(),
	}
	assert.NoError(t, d.PutJob(job2))
	job2InDb := job2.Copy()
	job2.Status = types.JOB_STATUS_MISHAP

	// Create a job without an Id set.
	job3 := &types.Job{
		Created: time.Now(),
	}

	// Make an update to job1Cached.
	job1Cached.Status = types.JOB_STATUS_FAILURE

	// Copy to compare later.
	expectedJobs := []*types.Job{job1Cached.Copy(), job2.Copy(), job3.Copy()}

	// Attempt to insert; put job1Cached last so that the error comes last.
	err = d.PutJobs([]*types.Job{job2, job3, job1Cached})
	assert.True(t, db.IsConcurrentUpdate(err))
	deepequal.AssertDeepEqual(t, expectedJobs, []*types.Job{job1Cached, job2, job3})

	// Check that nothing was updated in the DB.
	jobsInDb, err := d.GetJobsFromDateRange(begin, time.Now())
	assert.NoError(t, err)
	assert.Equal(t, 2, len(jobsInDb))
	for _, job := range jobsInDb {
		switch job.Id {
		case job1.Id:
			deepequal.AssertDeepEqual(t, job1InDb, job)
		case job2.Id:
			deepequal.AssertDeepEqual(t, job2InDb, job)
		default:
			assert.Fail(t, "Unexpected job in DB: %v", job)
		}
	}
}

func TestLocalDBTaskDB(t *testing.T) {
	testutils.MediumTest(t)
	d, tmpdir := makeDB(t, "TestLocalDBTaskDB")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)
	db.TestTaskDB(t, d)
}

func TestLocalDBTaskDBTooManyUsers(t *testing.T) {
	testutils.MediumTest(t)
	d, tmpdir := makeDB(t, "TestLocalDBTaskDBTooManyUsers")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)
	db.TestTaskDBTooManyUsers(t, d)
}

func TestLocalDBTaskDBConcurrentUpdate(t *testing.T) {
	testutils.MediumTest(t)
	d, tmpdir := makeDB(t, "TestLocalDBTaskDBConcurrentUpdate")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)
	db.TestTaskDBConcurrentUpdate(t, d)
}

func TestLocalDBTaskDBUpdateTasksWithRetries(t *testing.T) {
	testutils.MediumTest(t)
	d, tmpdir := makeDB(t, "TestLocalDBTaskDBUpdateTasksWithRetries")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)
	db.TestUpdateTasksWithRetries(t, d)
}

func TestLocalDBTaskDBGetTasksFromDateRangeByRepo(t *testing.T) {
	testutils.MediumTest(t)
	d, tmpdir := makeDB(t, "TestLocalDBTaskDBGetTasksFromDateRangeByRepo")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)
	db.TestTaskDBGetTasksFromDateRangeByRepo(t, d)
}

func TestLocalDBTaskDBGetTasksFromWindow(t *testing.T) {
	testutils.LargeTest(t)
	d, tmpdir := makeDB(t, "TestLocalDBTaskDBGetTasksFromWindow")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)
	db.TestTaskDBGetTasksFromWindow(t, d)
}

func TestLocalDBUpdateDBFromSwarmingTask(t *testing.T) {
	testutils.LargeTest(t)
	d, tmpdir := makeDB(t, "TestLocalDBUpdateDBFromSwarmingTask")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)
	db.TestUpdateDBFromSwarmingTask(t, d)
}

func TestLocalDBUpdateDBFromSwarmingTaskTryjob(t *testing.T) {
	testutils.LargeTest(t)
	d, tmpdir := makeDB(t, "TestLocalDBUpdateFromSwarmingTaskTryjob")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)
	db.TestUpdateDBFromSwarmingTaskTryJob(t, d)
}

func TestLocalDBJobDB(t *testing.T) {
	testutils.MediumTest(t)
	d, tmpdir := makeDB(t, "TestLocalDBJobDB")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)
	db.TestJobDB(t, d)
}

func TestLocalDBJobDBTooManyUsers(t *testing.T) {
	testutils.MediumTest(t)
	d, tmpdir := makeDB(t, "TestLocalDBJobDBTooManyUsers")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)
	db.TestJobDBTooManyUsers(t, d)
}

func TestLocalDBJobDBConcurrentUpdate(t *testing.T) {
	testutils.MediumTest(t)
	d, tmpdir := makeDB(t, "TestLocalDBJobDBConcurrentUpdate")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)
	db.TestJobDBConcurrentUpdate(t, d)
}

func TestLocalDBJobDBUpdateJobsWithRetries(t *testing.T) {
	testutils.MediumTest(t)
	d, tmpdir := makeDB(t, "TestLocalDBJobDBUpdateJobsWithRetries")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)
	db.TestUpdateJobsWithRetries(t, d)
}

func TestLocalDBCommentDB(t *testing.T) {
	testutils.MediumTest(t)
	d, tmpdir := makeDB(t, "TestLocalDBCommentDB")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)
	db.TestCommentDB(t, d)
}

func TestLocalDBIncrementalBackupTime(t *testing.T) {
	testutils.MediumTest(t)
	d, tmpdir := makeDB(t, "TestLocalDBIncrementalBackupTime")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)

	test := func(ts time.Time) {
		assert.NoError(t, d.SetIncrementalBackupTime(ts))
		actual, err := d.GetIncrementalBackupTime()
		assert.NoError(t, err)
		assert.True(t, ts.Equal(actual))
	}
	test(time.Date(2008, time.August, 8, 8, 8, 8, 8, time.UTC))
	test(time.Date(1776, time.July, 4, 13, 0, 0, 0, time.UTC))
	test(time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC))
	test(time.Date(2016, time.December, 31, 23, 59, 59, 999999999, time.UTC))
	test(time.Date(2008, time.August, 8, 8, 8, 8, 8, time.UTC))
}
