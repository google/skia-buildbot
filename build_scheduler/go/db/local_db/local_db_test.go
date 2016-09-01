package local_db

import (
	"io/ioutil"
	"path/filepath"
	"sort"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/build_scheduler/go/db"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

// Check that formatId and parseId are inverse operations and produce the
// expected result.
func TestFormatParseId(t *testing.T) {
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
		ts, seq, err := parseId(testCase.id)
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
		_, _, err := parseId(invalidId)
		assert.Error(t, err, "No error for Id: %q", invalidId)
	}
}

// Check that packV1 and unpackV1 are inverse operations and produce the
// expected result.
func TestPackUnpackV1(t *testing.T) {
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
		[]byte{},
		[]byte{0x00},
		[]byte{0x01, 0x00, 0x00},
		[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	} {
		_, _, err := unpackV1(invalid)
		assert.Error(t, err)
	}
}

// Create a localDB for testing. Call defer util.RemoveAll() on the second
// return value.
func makeDB(t *testing.T, name string) (db.TaskAndCommentDB, string) {
	//testutils.SkipIfShort(t)
	tmpdir, err := ioutil.TempDir("", name)
	assert.NoError(t, err)
	d, err := NewDB(name, filepath.Join(tmpdir, "task.db"))
	assert.NoError(t, err)
	return d, tmpdir
}

// Test that AssignId returns an error if Id is set.
func TestAssignIdAlreadyAssigned(t *testing.T) {
	d, tmpdir := makeDB(t, "TestAssignIdsFromCreatedTs")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)

	task := &db.Task{}
	assert.NoError(t, d.AssignId(task))
	assert.Error(t, d.AssignId(task))
}

// Test that AssignId uses created timestamp when set, and generates unique IDs
// for the same timestamp.
func TestAssignIdsFromCreatedTs(t *testing.T) {
	d, tmpdir := makeDB(t, "TestAssignIdsFromCreatedTs")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)

	tasks := []*db.Task{}
	addTask := func(ts time.Time) {
		task := &db.Task{
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
	sort.Stable(db.TaskSlice(tasks))

	// Sort IDs.
	sort.Strings(ids)

	// Validate that sorted IDs match sorted Tasks. Check that there are no
	// duplicate IDs. Check that ID timestamp matches created timestamp.
	prevId := ""
	for i := 0; i < len(tasks); i++ {
		assert.Equal(t, ids[i], tasks[i].Id)
		assert.NotEqual(t, prevId, ids[i])
		ts, _, err := parseId(ids[i])
		assert.NoError(t, err)
		assert.True(t, ts.Equal(tasks[i].Created))
		prevId = ids[i]
	}
}

// Test that AssignId can generate ids when created timestamp is not set, and
// generates unique IDs for PutTasks.
func TestAssignIdsFromCurrentTime(t *testing.T) {
	d, tmpdir := makeDB(t, "TestAssignIdsFromCurrentTime")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)

	tasks := []*db.Task{}
	for i := 0; i < 260; i++ {
		tasks = append(tasks, &db.Task{})
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
	first2 := []*db.Task{tasks[5], tasks[6]}
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
		ts, _, err := parseId(ids[i])
		assert.NoError(t, err)
		assert.True(t, begin.Before(ts) || begin.Equal(ts))
		assert.True(t, ts.Before(end) || ts.Equal(end))
		prevId = ids[i]
	}
}

// Test that PutTask returns an error when AssignId time is too far before (or
// after) the value subsequently assigned to Task.Created.
func TestPutTaskValidateCreatedTime(t *testing.T) {
	d, tmpdir := makeDB(t, "TestPutTaskValidateCreatedTime")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)

	task := &db.Task{}
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
		task.Created = beforeAssignId
		err := d.PutTask(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Created too early.")

		// Verify not in DB.
		noTask, err := d.GetTaskById(task.Id)
		assert.NoError(t, err)
		assert.Nil(t, noTask)
	}

	// Test in range.
	{
		task.Created = beforeAssignId.Add(MAX_CREATED_TIME_SKEW)
		err := d.PutTask(task)
		assert.NoError(t, err)

		// Verify added to DB.
		taskCopy, err := d.GetTaskById(task.Id)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, task, taskCopy)
	}

	// We can even change the Created time if we want. (Not necessarily supported
	// by all DB implementations.)
	{
		task.Created = afterAssignId
		err := d.PutTask(task)
		assert.NoError(t, err)

		taskCopy, err := d.GetTaskById(task.Id)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, task, taskCopy)
	}

	// But we can't change it to be out of range.
	{
		prevCreated := task.Created
		task.Created = beforeAssignId
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
	d, tmpdir := makeDB(t, "TestPutTaskLeavesTasksUnchanged")
	defer util.RemoveAll(tmpdir)
	defer testutils.AssertCloses(t, d)

	begin := time.Now().Add(-time.Nanosecond)

	// Create and insert a task that will cause ErrConcurrentUpdate.
	task1 := &db.Task{
		Created: time.Now(),
	}
	assert.NoError(t, d.PutTask(task1))

	// Retrieve a copy, modify original.
	task1Cached, err := d.GetTaskById(task1.Id)
	assert.NoError(t, err)
	task1.Status = db.TASK_STATUS_RUNNING
	assert.NoError(t, d.PutTask(task1))
	task1InDb := task1.Copy()

	// Create and insert a task to check PutTasks doesn't change DbModified.
	task2 := &db.Task{
		Created: time.Now(),
	}
	assert.NoError(t, d.PutTask(task2))
	task2InDb := task2.Copy()
	task2.Status = db.TASK_STATUS_MISHAP

	// Create a task with an Id already set.
	task3 := &db.Task{}
	assert.NoError(t, d.AssignId(task3))
	task3.Created = time.Now()

	// Create a task without an Id set.
	task4 := &db.Task{
		Created: time.Now(),
	}

	// Make an update to task1Cached.
	task1Cached.Commits = []string{"a", "b"}

	// Copy to compare later.
	expectedTasks := []*db.Task{task1Cached.Copy(), task2.Copy(), task3.Copy(), task4.Copy()}

	// Attempt to insert; put task1Cached last so that the error comes last.
	err = d.PutTasks([]*db.Task{task2, task3, task4, task1Cached})
	assert.True(t, db.IsConcurrentUpdate(err))
	testutils.AssertDeepEqual(t, expectedTasks, []*db.Task{task1Cached, task2, task3, task4})

	// Check that nothing was updated in the DB.
	tasksInDb, err := d.GetTasksFromDateRange(begin, time.Now())
	assert.NoError(t, err)
	assert.Equal(t, 2, len(tasksInDb))
	for _, task := range tasksInDb {
		switch task.Id {
		case task1.Id:
			testutils.AssertDeepEqual(t, task1InDb, task)
		case task2.Id:
			testutils.AssertDeepEqual(t, task2InDb, task)
		default:
			assert.Fail(t, "Unexpected task in DB: %v", task)
		}
	}
}

func TestLocalDB(t *testing.T) {
	d, tmpdir := makeDB(t, "TestLocalDB")
	defer util.RemoveAll(tmpdir)
	db.TestDB(t, d)
}

func TestLocalDBTooManyUsers(t *testing.T) {
	d, tmpdir := makeDB(t, "TestLocalDBTooManyUsers")
	defer util.RemoveAll(tmpdir)
	db.TestTooManyUsers(t, d)
}

func TestLocalDBConcurrentUpdate(t *testing.T) {
	d, tmpdir := makeDB(t, "TestLocalDBConcurrentUpdate")
	defer util.RemoveAll(tmpdir)
	db.TestConcurrentUpdate(t, d)
}

func TestLocalDBUpdateWithRetries(t *testing.T) {
	d, tmpdir := makeDB(t, "TestLocalDBUpdateWithRetries")
	defer util.RemoveAll(tmpdir)
	db.TestUpdateWithRetries(t, d)
}

func TestLocalDBCommentDB(t *testing.T) {
	d, tmpdir := makeDB(t, "TestLocalDBCommentDB")
	defer util.RemoveAll(tmpdir)
	db.TestCommentDB(t, d)
}
