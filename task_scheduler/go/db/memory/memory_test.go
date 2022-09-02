package memory

import (
	"os"
	"testing"

	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/task_scheduler/go/db"
)

func TestMain(m *testing.M) {
	db.AssertDeepEqual = assertdeep.Equal
	os.Exit(m.Run())
}

func TestInMemoryTaskDB(t *testing.T) {
	db.TestTaskDB(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBConcurrentUpdate(t *testing.T) {
	db.TestTaskDBConcurrentUpdate(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBUpdateTasksWithRetries(t *testing.T) {
	db.TestUpdateTasksWithRetries(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBGetTasksFromDateRangeByRepo(t *testing.T) {
	db.TestTaskDBGetTasksFromDateRangeByRepo(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBGetTasksFromWindow(t *testing.T) {
	db.TestTaskDBGetTasksFromWindow(t, NewInMemoryTaskDB())
}

func TestInMemoryUpdateDBFromSwarmingTask(t *testing.T) {
	db.TestUpdateDBFromSwarmingTask(t, NewInMemoryTaskDB())
}

func TestInMemoryUpdateDBFromSwarmingTaskTryjob(t *testing.T) {
	db.TestUpdateDBFromSwarmingTaskTryJob(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBSearch(t *testing.T) {
	db.TestTaskDBSearch(t, NewInMemoryTaskDB())
}

func TestInMemoryJobDB(t *testing.T) {
	db.TestJobDB(t, NewInMemoryJobDB())
}

func TestInMemoryJobDBConcurrentUpdate(t *testing.T) {
	db.TestJobDBConcurrentUpdate(t, NewInMemoryJobDB())
}

func TestInMemoryJobDBSearch(t *testing.T) {
	db.TestJobDBSearch(t, NewInMemoryJobDB())
}
