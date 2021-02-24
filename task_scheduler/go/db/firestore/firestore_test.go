package firestore

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_scheduler/go/db"
)

func TestMain(m *testing.M) {
	db.AssertDeepEqual = assertdeep.Equal
	os.Exit(m.Run())
}

func setup(t *testing.T) (db.DBCloser, func()) {
	unittest.LargeTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	c, cleanup := firestore.NewClientForTesting(ctx, t)
	d, err := NewDB(ctx, c)
	require.NoError(t, err)
	return d, func() {
		cancel()
		cleanup()
	}
}

func TestFirestoreDBTaskDB(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestTaskDB(t, d)
}

func TestFirestoreDBTaskDBConcurrentUpdate(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestTaskDBConcurrentUpdate(t, d)
}

func TestFirestoreDBTaskDBUpdateTasksWithRetries(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestUpdateTasksWithRetries(t, d)
}

func TestFirestoreDBTaskDBGetTasksFromDateRangeByRepo(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestTaskDBGetTasksFromDateRangeByRepo(t, d)
}

func TestFirestoreDBTaskDBGetTasksFromWindow(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestTaskDBGetTasksFromWindow(t, d)
}

func TestFirestoreDBTaskReaderSearch(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestTaskDBSearch(t, d)
}

func TestFirestoreDBJobDB(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestJobDB(t, d)
}

func TestFirestoreDBJobReaderSearch(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestJobDBSearch(t, d)
}

func TestFirestoreDBJobDBConcurrentUpdate(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestJobDBConcurrentUpdate(t, d)
}

func TestFirestoreDBCommentDB(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestCommentDB(t, d)
}
