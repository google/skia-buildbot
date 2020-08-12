package memory

import (
	"testing"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_scheduler/go/db/shared_tests"
)

func TestInMemoryTaskDB(t *testing.T) {
	unittest.SmallTest(t)
	shared_tests.TestTaskDB(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBConcurrentUpdate(t *testing.T) {
	unittest.SmallTest(t)
	shared_tests.TestTaskDBConcurrentUpdate(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBUpdateTasksWithRetries(t *testing.T) {
	unittest.SmallTest(t)
	shared_tests.TestUpdateTasksWithRetries(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBGetTasksFromDateRangeByRepo(t *testing.T) {
	unittest.SmallTest(t)
	shared_tests.TestTaskDBGetTasksFromDateRangeByRepo(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBGetTasksFromWindow(t *testing.T) {
	unittest.LargeTest(t)
	shared_tests.TestTaskDBGetTasksFromWindow(t, NewInMemoryTaskDB())
}

func TestInMemoryUpdateDBFromSwarmingTask(t *testing.T) {
	unittest.SmallTest(t)
	shared_tests.TestUpdateDBFromSwarmingTask(t, NewInMemoryTaskDB())
}

func TestInMemoryUpdateDBFromSwarmingTaskTryjob(t *testing.T) {
	unittest.SmallTest(t)
	shared_tests.TestUpdateDBFromSwarmingTaskTryJob(t, NewInMemoryTaskDB())
}

func TestInMemoryJobDB(t *testing.T) {
	unittest.SmallTest(t)
	shared_tests.TestJobDB(t, NewInMemoryJobDB())
}

func TestInMemoryJobDBConcurrentUpdate(t *testing.T) {
	unittest.SmallTest(t)
	shared_tests.TestJobDBConcurrentUpdate(t, NewInMemoryJobDB())
}
