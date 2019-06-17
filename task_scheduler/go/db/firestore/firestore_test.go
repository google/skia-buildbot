package firestore

import (
	"context"
	"os"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_scheduler/go/db"
)

func TestMain(m *testing.M) {
	db.AssertDeepEqual = deepequal.AssertDeepEqual
	os.Exit(m.Run())
}

func setup(t *testing.T) (db.DBCloser, func()) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	d, err := NewDB(context.Background(), c, nil)
	assert.NoError(t, err)
	return d, cleanup
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

func TestFirestoreDBJobDB(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestJobDB(t, d)
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
