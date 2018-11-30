package memory

import (
	"os"
	"testing"

	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
)

func TestMain(m *testing.M) {
	db.AssertDeepEqual = deepequal.AssertDeepEqual
	os.Exit(m.Run())
}

func TestInMemoryTaskDB(t *testing.T) {
	testutils.SmallTest(t)
	db.TestTaskDB(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBTooManyUsers(t *testing.T) {
	testutils.SmallTest(t)
	db.TestTaskDBTooManyUsers(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBConcurrentUpdate(t *testing.T) {
	testutils.SmallTest(t)
	db.TestTaskDBConcurrentUpdate(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBUpdateTasksWithRetries(t *testing.T) {
	testutils.SmallTest(t)
	db.TestUpdateTasksWithRetries(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBGetTasksFromDateRangeByRepo(t *testing.T) {
	testutils.SmallTest(t)
	db.TestTaskDBGetTasksFromDateRangeByRepo(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBGetTasksFromWindow(t *testing.T) {
	testutils.LargeTest(t)
	db.TestTaskDBGetTasksFromWindow(t, NewInMemoryTaskDB())
}

func TestInMemoryUpdateDBFromSwarmingTask(t *testing.T) {
	testutils.SmallTest(t)
	db.TestUpdateDBFromSwarmingTask(t, NewInMemoryTaskDB())
}

func TestInMemoryUpdateDBFromSwarmingTaskTryjob(t *testing.T) {
	testutils.SmallTest(t)
	db.TestUpdateDBFromSwarmingTaskTryJob(t, NewInMemoryTaskDB())
}

func TestInMemoryJobDB(t *testing.T) {
	testutils.SmallTest(t)
	db.TestJobDB(t, NewInMemoryJobDB())
}

func TestInMemoryJobDBTooManyUsers(t *testing.T) {
	testutils.SmallTest(t)
	db.TestJobDBTooManyUsers(t, NewInMemoryJobDB())
}

func TestInMemoryJobDBConcurrentUpdate(t *testing.T) {
	testutils.SmallTest(t)
	db.TestJobDBConcurrentUpdate(t, NewInMemoryJobDB())
}

func TestInMemoryJobDBUpdateJobsWithRetries(t *testing.T) {
	testutils.SmallTest(t)
	db.TestUpdateJobsWithRetries(t, NewInMemoryJobDB())
}
