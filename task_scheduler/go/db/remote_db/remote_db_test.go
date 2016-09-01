package remote_db

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/task_scheduler/go/db"
)

// clientWithBackdoor allows us to test the client/server pair as a
// db.TaskAndCommentDB, using the generic DB test utils. All method calls
// supported by RemoteDB use the client/server implementation; other methods
// have "backdoor" access to the underlying DB to allow the tests to modify the
// DB.
type clientWithBackdoor struct {
	// *client; implements the methods being tested.
	db.RemoteDB
	// The DB passed to NewServer.
	backdoor db.DB
	// *server; keep a reference so that we can call Close().
	dbserver io.Closer
	// The test HTTP server listening on the loopback address.
	httpserver *httptest.Server
}

func (b *clientWithBackdoor) Close() error {
	if err := b.RemoteDB.Close(); err != nil {
		return err
	}
	b.httpserver.Close()
	if err := b.dbserver.Close(); err != nil {
		return err
	}
	if err := b.backdoor.Close(); err != nil {
		return err
	}
	return nil
}

func (b *clientWithBackdoor) AssignId(t *db.Task) error {
	return b.backdoor.AssignId(t)
}
func (b *clientWithBackdoor) PutTask(t *db.Task) error {
	return b.backdoor.PutTask(t)
}
func (b *clientWithBackdoor) PutTasks(t []*db.Task) error {
	return b.backdoor.PutTasks(t)
}

// makeDB sets up a client/server pair wrapped in a clientWithBackdoor.
func makeDB(t *testing.T) db.TaskAndCommentDB {
	baseDB := db.NewInMemoryDB()
	r := mux.NewRouter()
	dbserver, err := NewServer(baseDB, r.PathPrefix("/db").Subrouter())
	assert.NoError(t, err)
	ts := httptest.NewServer(r)
	dbclient, err := NewClient(ts.URL + "/db/")
	assert.NoError(t, err)
	return &clientWithBackdoor{
		RemoteDB:   dbclient,
		backdoor:   baseDB,
		dbserver:   dbserver,
		httpserver: ts,
	}
}

func TestRemoteDB(t *testing.T) {
	d := makeDB(t)
	db.TestDB(t, d)
}

func TestRemoteDBTooManyUsers(t *testing.T) {
	d := makeDB(t)
	db.TestTooManyUsers(t, d)
}

func TestRemoteDBConcurrentUpdate(t *testing.T) {
	d := makeDB(t)
	db.TestConcurrentUpdate(t, d)
}

func TestRemoteDBUpdateWithRetries(t *testing.T) {
	d := makeDB(t)
	db.TestUpdateWithRetries(t, d)
}

func TestRemoteDBCommentDB(t *testing.T) {
	d := makeDB(t)
	db.TestCommentDB(t, d)
}
