package remote_db

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
	memory "go.skia.org/infra/task_scheduler/go/db/memory"
	"go.skia.org/infra/task_scheduler/go/types"
)

func TestMain(m *testing.M) {
	db.AssertDeepEqual = deepequal.AssertDeepEqual
	os.Exit(m.Run())
}

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
}

func (b *clientWithBackdoor) Close() error {
	b.httpserver.Close()
	return nil
}

func (b *clientWithBackdoor) AssignId(task *types.Task) error {
	return b.backdoor.AssignId(task)
}
func (b *clientWithBackdoor) PutTask(task *types.Task) error {
	return b.backdoor.PutTask(task)
}
func (b *clientWithBackdoor) PutTasks(tasks []*types.Task) error {
	return b.backdoor.PutTasks(tasks)
}
func (b *clientWithBackdoor) PutJob(job *types.Job) error {
	return b.backdoor.PutJob(job)
}
func (b *clientWithBackdoor) PutJobs(jobs []*types.Job) error {
	return b.backdoor.PutJobs(jobs)
}

type reqCountingTransport struct {
	count    int
	countMtx sync.RWMutex
	rt       http.RoundTripper
}

func (t *reqCountingTransport) Inc() {
	t.countMtx.Lock()
	defer t.countMtx.Unlock()
	t.count++
}

func (t *reqCountingTransport) Get() int {
	t.countMtx.RLock()
	defer t.countMtx.RUnlock()
	return t.count
}

func (t *reqCountingTransport) Reset() {
	t.countMtx.Lock()
	defer t.countMtx.Unlock()
	t.count = 0
}

func (t *reqCountingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.Inc()
	return t.rt.RoundTrip(req)
}

func newReqCountingTransport(rt http.RoundTripper) http.RoundTripper {
	return &reqCountingTransport{
		rt: rt,
	}
}

// makeDB sets up a client/server pair wrapped in a clientWithBackdoor.
func makeDB(t *testing.T) db.DBCloser {
	baseDB := memory.NewInMemoryDB()
	r := mux.NewRouter()
	err := RegisterServer(baseDB, r.PathPrefix("/db").Subrouter())
	assert.NoError(t, err)
	ts := httptest.NewServer(r)
	c := httputils.NewTimeoutClient()
	c.Transport = newReqCountingTransport(c.Transport)
	dbclient, err := NewClient(ts.URL+"/db/", c)
	assert.NoError(t, err)
	return &clientWithBackdoor{
		RemoteDB:   dbclient,
		backdoor:   baseDB,
		httpserver: ts,
	}
}

func TestRemoteDBTaskDB(t *testing.T) {
	testutils.SmallTest(t)
	d := makeDB(t)
	defer testutils.AssertCloses(t, d)
	db.TestTaskDB(t, d)
}

func TestRemoteDBTaskDBTooManyUsers(t *testing.T) {
	testutils.SmallTest(t)
	d := makeDB(t)
	defer testutils.AssertCloses(t, d)
	db.TestTaskDBTooManyUsers(t, d)
}

func TestRemoteDBTaskDBConcurrentUpdate(t *testing.T) {
	testutils.SmallTest(t)
	d := makeDB(t)
	defer testutils.AssertCloses(t, d)
	db.TestTaskDBConcurrentUpdate(t, d)
}

func TestRemoteDBTaskDBUpdateTasksWithRetries(t *testing.T) {
	testutils.SmallTest(t)
	d := makeDB(t)
	defer testutils.AssertCloses(t, d)
	db.TestUpdateTasksWithRetries(t, d)
}

func TestRemoteDBTaskDBGetTasksFromDateRangeByRepo(t *testing.T) {
	testutils.SmallTest(t)
	d := makeDB(t)
	defer testutils.AssertCloses(t, d)
	db.TestTaskDBGetTasksFromDateRangeByRepo(t, d)
}

func TestRemoteDBTaskDBGetTasksFromWindow(t *testing.T) {
	testutils.LargeTest(t)
	d := makeDB(t)
	defer testutils.AssertCloses(t, d)
	db.TestTaskDBGetTasksFromWindow(t, d)
}

func TestRemoteDBJobDB(t *testing.T) {
	testutils.SmallTest(t)
	d := makeDB(t)
	defer testutils.AssertCloses(t, d)
	db.TestJobDB(t, d)
}

func TestRemoteDBJobDBTooManyUsers(t *testing.T) {
	testutils.SmallTest(t)
	d := makeDB(t)
	defer testutils.AssertCloses(t, d)
	db.TestJobDBTooManyUsers(t, d)
}

func TestRemoteDBJobDBConcurrentUpdate(t *testing.T) {
	testutils.SmallTest(t)
	d := makeDB(t)
	defer testutils.AssertCloses(t, d)
	db.TestJobDBConcurrentUpdate(t, d)
}

func TestRemoteDBUpdateJobsWithRetries(t *testing.T) {
	testutils.SmallTest(t)
	d := makeDB(t)
	defer testutils.AssertCloses(t, d)
	db.TestUpdateJobsWithRetries(t, d)
}

func TestRemoteDBCommentDB(t *testing.T) {
	testutils.SmallTest(t)
	d := makeDB(t)
	defer testutils.AssertCloses(t, d)
	db.TestCommentDB(t, d)
}

func TestRemoteDBGetTasksFromDateRange(t *testing.T) {
	testutils.SmallTest(t)
	d := makeDB(t)
	defer testutils.AssertCloses(t, d)

	tp := d.(*clientWithBackdoor).RemoteDB.(*client).client.Transport.(*reqCountingTransport)

	timeStart := time.Now().Add(-3 * MAX_TASK_TIME_RANGE)
	t1 := types.MakeTestTask(timeStart.Add(time.Nanosecond), []string{"a", "b"})
	assert.NoError(t, d.PutTask(t1))
	t2 := types.MakeTestTask(t1.Created.Add(MAX_TASK_TIME_RANGE), []string{"c"})
	assert.NoError(t, d.PutTask(t2))
	t3 := types.MakeTestTask(t2.Created.Add(MAX_TASK_TIME_RANGE), []string{"d"})
	assert.NoError(t, d.PutTask(t3))

	// Request time ranges, and ensure that we get back the correct number
	// of tasks and made the correct number of HTTP requests.
	test := func(start, end time.Time, expectTasks, expectReqs int) {
		tp.Reset()
		tasks, err := d.GetTasksFromDateRange(start, end, "")
		assert.NoError(t, err)
		assert.Equal(t, expectTasks, len(tasks))
		assert.Equal(t, expectReqs, tp.Get())
	}
	test(timeStart, t1.Created.Add(time.Nanosecond), 1, 1)
	test(timeStart, t2.Created.Add(time.Nanosecond), 2, 2)
	test(timeStart, t3.Created.Add(time.Nanosecond), 3, 3)
	test(timeStart, timeStart.Add(MAX_TASK_TIME_RANGE), 1, 1)
	test(timeStart, timeStart.Add(MAX_TASK_TIME_RANGE).Add(time.Nanosecond), 1, 2)
}
