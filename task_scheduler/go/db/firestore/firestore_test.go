package firestore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/shared_tests"
)

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
	shared_tests.TestTaskDB(t, d)
}

func TestFirestoreDBTaskDBConcurrentUpdate(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	shared_tests.TestTaskDBConcurrentUpdate(t, d)
}

func TestFirestoreDBTaskDBUpdateTasksWithRetries(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	shared_tests.TestUpdateTasksWithRetries(t, d)
}

func TestFirestoreDBTaskDBGetTasksFromDateRangeByRepo(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	shared_tests.TestTaskDBGetTasksFromDateRangeByRepo(t, d)
}

func TestFirestoreDBTaskDBGetTasksFromWindow(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	shared_tests.TestTaskDBGetTasksFromWindow(t, d)
}

func TestFirestoreDBJobDB(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	shared_tests.TestJobDB(t, d)
}

func TestFirestoreDBJobDBConcurrentUpdate(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	shared_tests.TestJobDBConcurrentUpdate(t, d)
}

func TestFirestoreDBCommentDB(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	shared_tests.TestCommentDB(t, d)
}
