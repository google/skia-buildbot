package memory

import (
	"os"
	"testing"

	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_scheduler/go/db"
)

func TestMain(m *testing.M) {
	db.AssertDeepEqual = assertdeep.Equal
	os.Exit(m.Run())
}

func TestInMemoryTaskDB(t *testing.T) {
	unittest.SmallTest(t)
	db.TestTaskDB(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBConcurrentUpdate(t *testing.T) {
	unittest.SmallTest(t)
	db.TestTaskDBConcurrentUpdate(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBUpdateTasksWithRetries(t *testing.T) {
	unittest.SmallTest(t)
	db.TestUpdateTasksWithRetries(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBGetTasksFromDateRangeByRepo(t *testing.T) {
	unittest.SmallTest(t)
	db.TestTaskDBGetTasksFromDateRangeByRepo(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBGetTasksFromWindow(t *testing.T) {
	unittest.LargeTest(t)
	db.TestTaskDBGetTasksFromWindow(t, NewInMemoryTaskDB())
}

func TestInMemoryUpdateDBFromSwarmingTask(t *testing.T) {
	unittest.SmallTest(t)
	db.TestUpdateDBFromSwarmingTask(t, NewInMemoryTaskDB())
}

func TestInMemoryUpdateDBFromSwarmingTaskTryjob(t *testing.T) {
	unittest.SmallTest(t)
	db.TestUpdateDBFromSwarmingTaskTryJob(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBSearch(t *testing.T) {
	unittest.SmallTest(t)
	db.TestTaskDBSearch(t, NewInMemoryTaskDB())
}

func TestInMemoryJobDB(t *testing.T) {
	unittest.SmallTest(t)
	db.TestJobDB(t, NewInMemoryJobDB())
}

func TestInMemoryJobDBConcurrentUpdate(t *testing.T) {
	unittest.SmallTest(t)
	db.TestJobDBConcurrentUpdate(t, NewInMemoryJobDB())
}

func TestInMemoryJobDBSearch(t *testing.T) {
	unittest.SmallTest(t)
	db.TestJobDBSearch(t, NewInMemoryJobDB())
}
