package firestore

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
)

func TestMain(m *testing.M) {
	db.AssertDeepEqual = deepequal.AssertDeepEqual
	os.Exit(m.Run())
}

func setup(t *testing.T) (db.DBCloser, func()) {
	testutils.MediumTest(t)
	testutils.ManualTest(t)
	instance := fmt.Sprintf("test-%s", uuid.New())
	d, err := NewDB(context.Background(), "skia-firestore", instance, nil)
	assert.NoError(t, err)
	cleanup := func() {
		c := d.(*firestoreDB).client
		assert.NoError(t, firestore.RecursiveDelete(c, c.ParentDoc, 5, 30*time.Second))
		assert.NoError(t, d.Close())
	}
	return d, cleanup
}

func TestFirestoreDBTaskDB(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestTaskDB(t, d)
}

func TestFirestoreDBTaskDBTooManyUsers(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestTaskDBTooManyUsers(t, d)
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

func TestFirestoreDBJobDBTooManyUsers(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestJobDBTooManyUsers(t, d)
}

func TestFirestoreDBJobDBConcurrentUpdate(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestJobDBConcurrentUpdate(t, d)
}

func TestFirestoreDBJobDBUpdateJobsWithRetries(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestUpdateJobsWithRetries(t, d)
}

func TestFirestoreDBCommentDB(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestCommentDB(t, d)
}
