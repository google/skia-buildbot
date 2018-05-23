package db

import (
	"os"
	"testing"

	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
)

func TestMain(m *testing.M) {
	AssertDeepEqual = deepequal.AssertDeepEqual
	os.Exit(m.Run())
}

func TestInMemoryTaskDB(t *testing.T) {
	testutils.SmallTest(t)
	TestTaskDB(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBTooManyUsers(t *testing.T) {
	testutils.SmallTest(t)
	TestTaskDBTooManyUsers(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBConcurrentUpdate(t *testing.T) {
	testutils.SmallTest(t)
	TestTaskDBConcurrentUpdate(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBUpdateTasksWithRetries(t *testing.T) {
	testutils.SmallTest(t)
	TestUpdateTasksWithRetries(t, NewInMemoryTaskDB())
}

func TestInMemoryJobDB(t *testing.T) {
	testutils.SmallTest(t)
	TestJobDB(t, NewInMemoryJobDB())
}

func TestInMemoryJobDBTooManyUsers(t *testing.T) {
	testutils.SmallTest(t)
	TestJobDBTooManyUsers(t, NewInMemoryJobDB())
}

func TestInMemoryJobDBConcurrentUpdate(t *testing.T) {
	testutils.SmallTest(t)
	TestJobDBConcurrentUpdate(t, NewInMemoryJobDB())
}

func TestInMemoryJobDBUpdateJobsWithRetries(t *testing.T) {
	testutils.SmallTest(t)
	TestUpdateJobsWithRetries(t, NewInMemoryJobDB())
}
