/*
	Test utilities for mocking the V2 frontend.
*/
package frontend

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"

	"go.skia.org/infra/ct/go/ctfe/pending_tasks"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	"go.skia.org/infra/go/httputils"
	skutil "go.skia.org/infra/go/util"
)

// UpdateTaskReq includes the URL of an update request, the unmarshaled body, and any error
// encountered.
type UpdateTaskReq struct {
	Url   string
	Vars  task_common.UpdateTaskCommonVars
	Error error
}

// MockServer implements http.Handler and can be given a task with which to respond to
// ctfeutil.GET_OLDEST_PENDING_TASK_URI. It also collects any other requests and attempts to parse
// the body as task_common.UpdateTaskCommonVars JSON. Safe for use in multiple goroutines.
// Example usage:
//	mockServer := MockServer{}
//	mockServer.SetCurrentTask(&admin_tasks.RecreateWebpageArchivesDBTask{...})
//	defer CloseTestServer(InitTestServer(&mockServer))
//	...
//	expect.Equal(t, 1, mockServer.OldestPendingTaskReqCount())
//	assert.Len(t, mockServer.UpdateTaskReqs(), 1)
//	updateReq := mockServer.UpdateTaskReqs()[0]
//	expect.Equal(t, "/"+ctfeutil.UPDATE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, updateReq.Url)
//	expect.NoError(t, updateReq.Error)
//	expect.Equal(t, int64(42), updateReq.Vars.Id)
//	...
type MockServer struct {
	mutex                     sync.RWMutex
	currentTask               task_common.Task
	oldestPendingTaskReqCount int
	updateTaskReqs            []UpdateTaskReq
}

// SetCurrentTask provides the Task to be returned for a ctfeutil.GET_OLDEST_PENDING_TASK_URI
// request.
func (ms *MockServer) SetCurrentTask(currentTask task_common.Task) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()
	ms.currentTask = currentTask
}

func (ms *MockServer) OldestPendingTaskReqCount() int {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()
	return ms.oldestPendingTaskReqCount
}

// Returns all update requests seen thus far.
func (ms *MockServer) UpdateTaskReqs() []UpdateTaskReq {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()
	// TODO(benjaminwagner): Can I just return ms.updateTaskReqs?
	result := make([]UpdateTaskReq, len(ms.updateTaskReqs))
	copy(result, ms.updateTaskReqs)
	return result
}

func (ms *MockServer) HandleGetOldestPendingTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ms.mutex.Lock()
	defer ms.mutex.Unlock()
	ms.oldestPendingTaskReqCount++
	if err := pending_tasks.EncodeTask(w, ms.currentTask); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode JSON")
		return
	}
}

func (ms *MockServer) HandleUpdateTask(w http.ResponseWriter, r *http.Request) {
	updateTaskReq := UpdateTaskReq{Url: r.URL.Path}
	data, err := ioutil.ReadAll(r.Body)
	defer skutil.Close(r.Body)
	if err != nil {
		updateTaskReq.Error = err
		httputils.ReportError(w, r, err, "")
		return
	}
	err = json.Unmarshal(data, &updateTaskReq.Vars)
	if err != nil {
		updateTaskReq.Error = err
		httputils.ReportError(w, r, err, "")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	ms.mutex.Lock()
	defer ms.mutex.Unlock()
	ms.updateTaskReqs = append(ms.updateTaskReqs, updateTaskReq)
}

func (ms *MockServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/"+ctfeutil.GET_OLDEST_PENDING_TASK_URI {
		ms.HandleGetOldestPendingTask(w, r)
	} else {
		ms.HandleUpdateTask(w, r)
	}
}

// Creates an httptest.Server using h as its handler and calls Init to ensure
// GetOldestPendingTaskV2 and UpdateWebappTaskV2 use the test server. Also calls
// InitForTesting. Can be used as "defer CloseTestServer(InitTestServer(h))".
func InitTestServer(h http.Handler) *httptest.Server {
	ts := httptest.NewServer(h)
	MustInit(ts.URL+"/", ts.URL+"/")
	return ts
}

// Closes ts, resets CtfeV2, and resets the webapp Url for GetOldestPendingTaskV2 and
// UpdateWebappTaskV2. Can be used as "defer CloseTestServer(InitTestServer(h))".
func CloseTestServer(ts *httptest.Server) {
	ts.Close()
	MustInit(WEBAPP_ROOT, INTERNAL_WEBAPP_ROOT)
}
