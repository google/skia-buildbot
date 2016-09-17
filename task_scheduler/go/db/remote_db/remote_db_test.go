package remote_db

import (
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
)

// clientWithBackdoor allows us to test the client/server pair as a db.DB, using
// the generic DB test utils. All method calls supported by RemoteDB use the
// client/server implementation; other methods have "backdoor" access to the
// underlying DB to allow the tests to modify the DB.
type clientWithBackdoor struct {
	// *client; implements the methods being tested.
	db.RemoteDB
	// The DB passed to NewServer.
	backdoor db.DB
	// The test HTTP server listening on the loopback address.
	httpserver *httptest.Server
	// TODO(benjaminwagner): Adding this to get things to compile until RemoteDB
	// implements JobReader.
	db.JobReader
}

func (b *clientWithBackdoor) Close() error {
	b.httpserver.Close()
	return nil
}

func (b *clientWithBackdoor) AssignId(task *db.Task) error {
	return b.backdoor.AssignId(task)
}
func (b *clientWithBackdoor) PutTask(task *db.Task) error {
	return b.backdoor.PutTask(task)
}
func (b *clientWithBackdoor) PutTasks(tasks []*db.Task) error {
	return b.backdoor.PutTasks(tasks)
}
func (b *clientWithBackdoor) PutJob(job *db.Job) error {
	return b.backdoor.PutJob(job)
}
func (b *clientWithBackdoor) PutJobs(jobs []*db.Job) error {
	return b.backdoor.PutJobs(jobs)
}

// makeDB sets up a client/server pair wrapped in a clientWithBackdoor.
func makeDB(t *testing.T) db.DBCloser {
	baseDB := db.NewInMemoryDB()
	r := mux.NewRouter()
	err := RegisterServer(baseDB, r.PathPrefix("/db").Subrouter())
	assert.NoError(t, err)
	ts := httptest.NewServer(r)
	dbclient, err := NewClient(ts.URL + "/db/")
	assert.NoError(t, err)
	return &clientWithBackdoor{
		RemoteDB:   dbclient,
		backdoor:   baseDB,
		httpserver: ts,
	}
}

func TestRemoteDB(t *testing.T) {
	d := makeDB(t)
	defer testutils.AssertCloses(t, d)
	db.TestTaskDB(t, d)
}

func TestRemoteDBTooManyUsers(t *testing.T) {
	d := makeDB(t)
	defer testutils.AssertCloses(t, d)
	db.TestTaskDBTooManyUsers(t, d)
}

func TestRemoteDBConcurrentUpdate(t *testing.T) {
	d := makeDB(t)
	defer testutils.AssertCloses(t, d)
	db.TestTaskDBConcurrentUpdate(t, d)
}

func TestRemoteDBUpdateTasksWithRetries(t *testing.T) {
	d := makeDB(t)
	defer testutils.AssertCloses(t, d)
	db.TestUpdateTasksWithRetries(t, d)
}

func TestRemoteDBCommentDB(t *testing.T) {
	d := makeDB(t)
	defer testutils.AssertCloses(t, d)
	db.TestCommentDB(t, d)
}
