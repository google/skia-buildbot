// remote_db provides a client/server pair for accessing a db.RemoteDB over
// HTTP. The db.RemoteDB provided to NewServer will be accessible via RPC as the
// db.RemoteDB returned from NewClient.
package remote_db

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
)

const (
	// Server handles requests on these paths. See registerHandlers for detail.
	MODIFIED_TASKS_PATH     = "modified-tasks"
	TASKS_PATH              = "tasks"
	COMMENTS_PATH           = "comments"
	TASK_COMMENTS_PATH      = "comments/task-comments"
	TASK_SPEC_COMMENTS_PATH = "comments/task-spec-comments"
	COMMIT_COMMENTS_PATH    = "comments/commit-comments"

	// HTTP error codes used for defined DB errors. See reportDBError and
	// interpretStatusCode for detail.
	// None of the standard HTTP codes seem to apply for ErrAlreadyExists.
	ERR_ALREADY_EXISTS_CODE    = 475
	ERR_CONCURRENT_UPDATE_CODE = http.StatusConflict
	ERR_NOT_FOUND_CODE         = http.StatusNotFound
	ERR_TOO_MANY_USERS_CODE    = http.StatusTooManyRequests
	ERR_UNKNOWN_ID_CODE        = http.StatusGone
)

// server translates HTTP requests to method calls on d.
type server struct {
	d db.RemoteDB
}

// NewServer adds handlers to r that handle requests from a client created via
// NewClient.
//
// Currently no authentication is required, so r should not be exposed on a
// public port.
func NewServer(d db.RemoteDB, r *mux.Router) (io.Closer, error) {
	s := &server{
		d: d,
	}
	s.registerHandlers(r)
	return s, nil
}

// registerHandlers adds GET, POST, and DELETE handlers to r on various paths.
func (s *server) registerHandlers(r *mux.Router) {
	r.HandleFunc("/"+MODIFIED_TASKS_PATH, s.PostModifiedTasksHandler).Methods(http.MethodPost)
	r.HandleFunc("/"+MODIFIED_TASKS_PATH, s.DeleteModifiedTasksHandler).Methods(http.MethodDelete)
	r.HandleFunc("/"+MODIFIED_TASKS_PATH, s.GetModifiedTasksHandler).Methods(http.MethodGet)
	r.HandleFunc("/"+TASKS_PATH, s.GetTasksHandler).Methods(http.MethodGet)
	r.HandleFunc("/"+COMMENTS_PATH, s.GetCommentsHandler).Methods(http.MethodGet)
	r.HandleFunc("/"+TASK_COMMENTS_PATH, s.PostTaskCommentsHandler).Methods(http.MethodPost)
	r.HandleFunc("/"+TASK_COMMENTS_PATH, s.DeleteTaskCommentsHandler).Methods(http.MethodDelete)
	r.HandleFunc("/"+TASK_SPEC_COMMENTS_PATH, s.PostTaskSpecCommentsHandler).Methods(http.MethodPost)
	r.HandleFunc("/"+TASK_SPEC_COMMENTS_PATH, s.DeleteTaskSpecCommentsHandler).Methods(http.MethodDelete)
	r.HandleFunc("/"+COMMIT_COMMENTS_PATH, s.PostCommitCommentsHandler).Methods(http.MethodPost)
	r.HandleFunc("/"+COMMIT_COMMENTS_PATH, s.DeleteCommitCommentsHandler).Methods(http.MethodDelete)
}

// client translates db.RemoteDB method calls to HTTP requests.
type client struct {
	serverRoot string
	client     *http.Client
}

// NewClient returns a db.RemoteDB that connects to the server created by
// NewServer. serverRoot should end with a slash.
func NewClient(serverRoot string) (db.RemoteDB, error) {
	return &client{
		serverRoot: serverRoot,
		client:     httputils.NewTimeoutClient(),
	}, nil
}

// Close has no effect. Does not close the RemoteDB provided to NewServer.
func (s *server) Close() error {
	return nil
}

// Close has no effect. Does not close the remote DB.
func (c *client) Close() error {
	return nil
}

// flush allows the client to begin reading the response before the entire
// response is written.
func flush(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// reportDBError writes an appropriate HTTP status code and status message to w
// for known DB errors. Writes a generic status for other errors.
func reportDBError(w http.ResponseWriter, r *http.Request, err error, msg string) {
	switch true {
	case db.IsAlreadyExists(err):
		http.Error(w, err.Error(), ERR_ALREADY_EXISTS_CODE)
	case db.IsConcurrentUpdate(err):
		http.Error(w, err.Error(), ERR_CONCURRENT_UPDATE_CODE)
	case db.IsNotFound(err):
		http.Error(w, err.Error(), ERR_NOT_FOUND_CODE)
	case db.IsTooManyUsers(err):
		http.Error(w, err.Error(), ERR_TOO_MANY_USERS_CODE)
	case db.IsUnknownId(err):
		http.Error(w, err.Error(), ERR_UNKNOWN_ID_CODE)
	default:
		httputils.ReportError(w, r, err, msg)
	}
}

// interpretStatusCode returns a known DB error for specific error codes, or nil
// for OK response, or a generic error for other status codes.
func interpretStatusCode(r *http.Response) error {
	switch r.StatusCode {
	case http.StatusOK:
		return nil
	case ERR_ALREADY_EXISTS_CODE:
		return db.ErrAlreadyExists
	case ERR_CONCURRENT_UPDATE_CODE:
		return db.ErrConcurrentUpdate
	case ERR_NOT_FOUND_CODE:
		return db.ErrNotFound
	case ERR_TOO_MANY_USERS_CODE:
		return db.ErrTooManyUsers
	case ERR_UNKNOWN_ID_CODE:
		return db.ErrUnknownId
	default:
		return fmt.Errorf("Received status code %d: %s", r.StatusCode, r.Status)
	}
}

// PostModifiedTasksHandler translates a POST request with empty body to
// StartTrackingModifiedTasks.
//   - format: must be "gob"; default "gob"
// Response is GOB of string id.
func (s *server) PostModifiedTasksHandler(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format != "" && format != "gob" {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Unsupported format %q", format))
		return
	}
	id, err := s.d.StartTrackingModifiedTasks()
	if err != nil {
		reportDBError(w, r, err, "Unable to start tracking tasks")
		return
	}
	w.Header().Set("Content-Type", "application/gob")
	enc := gob.NewEncoder(w)
	if err := enc.Encode(id); err != nil {
		s.d.StopTrackingModifiedTasks(id)
		httputils.ReportError(w, r, err, "Unable to encode start id")
		return
	}
}

// See documentation for db.TaskReader.
func (c *client) StartTrackingModifiedTasks() (string, error) {
	req, err := http.NewRequest(http.MethodPost, c.serverRoot+MODIFIED_TASKS_PATH+"?format=gob", nil)
	if err != nil {
		return "", err
	}
	r, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	if err := interpretStatusCode(r); err != nil {
		return "", err
	}
	dec := gob.NewDecoder(r.Body)
	var id string
	if err := dec.Decode(&id); err != nil {
		return "", err
	}
	return id, nil
}

// DeleteModifiedTasksHandler translates a DELETE request with empty body to
// StopTrackingModifiedTasks.
//   - id: id returned from PostModifiedTasksHandler
// No response body.
func (s *server) DeleteModifiedTasksHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		httputils.ReportError(w, r, nil, "Missing id param")
		return
	}
	s.d.StopTrackingModifiedTasks(id)
	w.WriteHeader(http.StatusOK)
}

// See documentation for db.TaskReader.
// TODO(benjaminwagner): Should this method be changed to return an error?
func (c *client) StopTrackingModifiedTasks(id string) {
	params := url.Values{}
	params.Set("id", id)
	req, err := http.NewRequest(http.MethodDelete, c.serverRoot+MODIFIED_TASKS_PATH+"?"+params.Encode(), nil)
	if err != nil {
		glog.Error(err)
		return
	}
	r, err := c.client.Do(req)
	if err != nil {
		glog.Error(err)
		return
	}
	defer r.Body.Close()
	if err := interpretStatusCode(r); err != nil {
		glog.Error(err)
		return
	}
}

// GetModifiedTasksHandler translates a GET request to GetModifiedTasks.
//   - format: must be "gob"; default "gob"
//   - id: id returned from PostModifiedTasksHandler
// Response is GOB stream; first object is the number of tasks, the remaining
// objects are db.Tasks.
// Warning: not RESTful: the same URI will return different results each time.
func (s *server) GetModifiedTasksHandler(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format != "" && format != "gob" {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Unsupported format %q", format))
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		httputils.ReportError(w, r, nil, "Missing id param")
		return
	}
	tasks, err := s.d.GetModifiedTasks(id)
	if err != nil {
		reportDBError(w, r, err, "Unable to retrieve tasks")
		return
	}
	w.Header().Set("Content-Type", "application/gob")
	enc := gob.NewEncoder(w)
	if err := enc.Encode(len(tasks)); err != nil {
		s.d.StopTrackingModifiedTasks(id)
		httputils.ReportError(w, r, err, "Unable to encode task count")
		return
	}
	for _, t := range tasks {
		if err := enc.Encode(t); err != nil {
			s.d.StopTrackingModifiedTasks(id)
			httputils.ReportError(w, r, err, "Unable to encode task")
			return
		}
		flush(w)
	}
}

// processTaskList decodes a list of db.Task from r. r must be a GOB stream
// where the first object is the count of tasks and the remaining objects are
// db.Tasks.
func processTaskList(r io.Reader) ([]*db.Task, error) {
	dec := gob.NewDecoder(r)
	var count int
	if err := dec.Decode(&count); err != nil {
		return nil, err
	}
	rv := make([]*db.Task, count)
	for i, _ := range rv {
		var t db.Task
		if err := dec.Decode(&t); err != nil {
			return nil, err
		}
		rv[i] = &t
	}
	return rv, nil
}

// See documentation for db.TaskReader.
func (c *client) GetModifiedTasks(id string) ([]*db.Task, error) {
	params := url.Values{}
	params.Set("format", "gob")
	params.Set("id", id)
	r, err := c.client.Get(c.serverRoot + MODIFIED_TASKS_PATH + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if err := interpretStatusCode(r); err != nil {
		return nil, err
	}
	return processTaskList(r.Body)
}

// GetTasksHandler translates a GET request to GetTasksFromDateRange or
// GetTaskById.
//   - format: must be "gob"; default "gob"
//   - id: Task.Id; may not be repeated
//   - from, to: nanoseconds since the Unix epoch. (base-10 string)
// Response is GOB stream; first object is the number of tasks, the remaining
// objects are db.Tasks.
func (s *server) GetTasksHandler(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format != "" && format != "gob" {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Unsupported format %q", format))
		return
	}
	id := r.URL.Query().Get("id")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if id == "" && (fromStr == "" || toStr == "") {
		httputils.ReportError(w, r, nil, "Only lookup by id or date range is implemented. Missing id/from/to params")
		return
	}
	var tasks []*db.Task
	if id != "" {
		task, err := s.d.GetTaskById(id)
		if err != nil {
			reportDBError(w, r, err, "Unable to retrieve task")
			return
		}
		if task == nil {
			tasks = []*db.Task{}
		} else {
			tasks = []*db.Task{task}
		}
	} else {
		fromInt, err := strconv.ParseInt(fromStr, 10, 64)
		if err != nil {
			httputils.ReportError(w, r, err, fmt.Sprintf("Invalid from param %q", fromStr))
			return
		}
		toInt, err := strconv.ParseInt(toStr, 10, 64)
		if err != nil {
			httputils.ReportError(w, r, err, fmt.Sprintf("Invalid to param %q", toStr))
			return
		}
		tasks, err = s.d.GetTasksFromDateRange(time.Unix(0, fromInt), time.Unix(0, toInt))
		if err != nil {
			reportDBError(w, r, err, "Unable to retrieve tasks")
			return
		}
	}
	w.Header().Set("Content-Type", "application/gob")
	enc := gob.NewEncoder(w)
	if err := enc.Encode(len(tasks)); err != nil {
		httputils.ReportError(w, r, err, "Unable to encode task count")
		return
	}
	for _, t := range tasks {
		if err := enc.Encode(t); err != nil {
			httputils.ReportError(w, r, err, "Unable to encode task")
			return
		}
		flush(w)
	}
}

// See documentation for db.TaskReader.
func (c *client) GetTaskById(id string) (*db.Task, error) {
	params := url.Values{}
	params.Set("format", "gob")
	params.Set("id", id)
	r, err := c.client.Get(c.serverRoot + TASKS_PATH + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if err := interpretStatusCode(r); err != nil {
		return nil, err
	}
	tasks, err := processTaskList(r.Body)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, nil
	}
	if len(tasks) > 1 {
		return nil, fmt.Errorf("Unexpected multiple tasks for id query. %q %v", id, tasks)
	}
	return tasks[0], nil
}

// See documentation for db.TaskReader.
func (c *client) GetTasksFromDateRange(from time.Time, to time.Time) ([]*db.Task, error) {
	params := url.Values{}
	params.Set("format", "gob")
	params.Set("from", strconv.FormatInt(from.UnixNano(), 10))
	params.Set("to", strconv.FormatInt(to.UnixNano(), 10))
	r, err := c.client.Get(c.serverRoot + TASKS_PATH + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if err := interpretStatusCode(r); err != nil {
		return nil, err
	}
	return processTaskList(r.Body)
}

// GetCommentsHandler translates a GET request to GetCommentsForRepos
//   - format: must be "gob"; default "gob"
//   - repo: repo for which to return comments (may be repeated)
//   - from (optional): nanoseconds since the Unix epoch. (base-10 string)
// Response is GOB stream; first object is the number of db.RepoComments, the
// remaining objects are db.RepoComments.
func (s *server) GetCommentsHandler(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format != "" && format != "gob" {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Unsupported format %q", format))
		return
	}
	repos := r.URL.Query()["repo"]
	if len(repos) == 0 {
		httputils.ReportError(w, r, nil, "Only listing comments by repos and from is implemented. Missing repo param")
		return
	}
	from := time.Time{}
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		fromInt, err := strconv.ParseInt(fromStr, 10, 64)
		if err != nil {
			httputils.ReportError(w, r, err, fmt.Sprintf("Invalid from param %q", fromStr))
			return
		}
		from = time.Unix(0, fromInt)
	}
	comments, err := s.d.GetCommentsForRepos(repos, from)
	if err != nil {
		reportDBError(w, r, err, "Unable to retrieve comments")
		return
	}
	w.Header().Set("Content-Type", "application/gob")
	enc := gob.NewEncoder(w)
	if err := enc.Encode(len(comments)); err != nil {
		httputils.ReportError(w, r, err, "Unable to encode RepoComment count")
		return
	}
	for _, c := range comments {
		if err := enc.Encode(c); err != nil {
			httputils.ReportError(w, r, err, "Unable to encode RepoComments")
			return
		}
		flush(w)
	}
}

// See documentation for db.CommentDB.
func (c *client) GetCommentsForRepos(repos []string, from time.Time) ([]*db.RepoComments, error) {
	params := url.Values{"repo": repos}
	params.Set("format", "gob")
	if !util.TimeIsZero(from) {
		params.Add("from", strconv.FormatInt(from.UnixNano(), 10))
	}
	r, err := c.client.Get(c.serverRoot + COMMENTS_PATH + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if err := interpretStatusCode(r); err != nil {
		return nil, err
	}
	dec := gob.NewDecoder(r.Body)
	var count int
	if err := dec.Decode(&count); err != nil {
		return nil, err
	}
	rv := make([]*db.RepoComments, count)
	for i, _ := range rv {
		var c db.RepoComments
		if err := dec.Decode(&c); err != nil {
			return nil, err
		}
		rv[i] = &c
	}
	return rv, nil
}

// sharedTaskCommentsHandler translates a POST or DELETE request where the body
// is a GOB-encoded db.TaskComment to PutTaskComment or DeleteTaskComment.
//   - format: must be "gob"; default "gob"
// No response body.
func (s *server) sharedTaskCommentsHandler(w http.ResponseWriter, r *http.Request, isPut bool) {
	format := r.URL.Query().Get("format")
	if format != "" && format != "gob" {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Unsupported format %q", format))
		return
	}
	dec := gob.NewDecoder(r.Body)
	var c db.TaskComment
	if err := dec.Decode(&c); err != nil {
		httputils.ReportError(w, r, err, "Unable to decode TaskComment")
		return
	}
	var err error
	var errMsg string
	if isPut {
		err = s.d.PutTaskComment(&c)
		errMsg = "Unable to create comment"
	} else {
		err = s.d.DeleteTaskComment(&c)
		errMsg = "Unable to delete comment"
	}
	if err != nil {
		reportDBError(w, r, err, errMsg)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// PostTaskCommentsHandler translates a POST or DELETE request where the body
// is a GOB-encoded db.TaskComment to PutTaskComment.
//   - format: must be "gob"; default "gob"
// No response body.
func (s *server) PostTaskCommentsHandler(w http.ResponseWriter, r *http.Request) {
	s.sharedTaskCommentsHandler(w, r, true)
}

// DeleteTaskCommentsHandler translates a DELETE request where the body is a
// GOB-encoded db.TaskComment to DeleteTaskComment.
//   - format: must be "gob"; default "gob"
// No response body.
func (s *server) DeleteTaskCommentsHandler(w http.ResponseWriter, r *http.Request) {
	s.sharedTaskCommentsHandler(w, r, false)
}

// See documentation for sharedTaskCommentsHandler; this method is the same for
// db.TaskSpecComment.
func (s *server) sharedTaskSpecCommentsHandler(w http.ResponseWriter, r *http.Request, isPut bool) {
	format := r.URL.Query().Get("format")
	if format != "" && format != "gob" {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Unsupported format %q", format))
		return
	}
	dec := gob.NewDecoder(r.Body)
	var c db.TaskSpecComment
	if err := dec.Decode(&c); err != nil {
		httputils.ReportError(w, r, err, "Unable to decode TaskSpecComment")
		return
	}
	var err error
	var errMsg string
	if isPut {
		err = s.d.PutTaskSpecComment(&c)
		errMsg = "Unable to create comment"
	} else {
		err = s.d.DeleteTaskSpecComment(&c)
		errMsg = "Unable to delete comment"
	}
	if err != nil {
		reportDBError(w, r, err, errMsg)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// See documentation for PostTaskCommentsHandler; this method is the same for
// db.TaskSpecComment.
func (s *server) PostTaskSpecCommentsHandler(w http.ResponseWriter, r *http.Request) {
	s.sharedTaskSpecCommentsHandler(w, r, true)
}

// See documentation for DeleteTaskCommentsHandler; this method is the same for
// db.TaskSpecComment.
func (s *server) DeleteTaskSpecCommentsHandler(w http.ResponseWriter, r *http.Request) {
	s.sharedTaskSpecCommentsHandler(w, r, false)
}

// See documentation for sharedTaskCommentsHandler; this method is the same for
// db.CommitComment.
func (s *server) sharedCommitCommentsHandler(w http.ResponseWriter, r *http.Request, isPut bool) {
	format := r.URL.Query().Get("format")
	if format != "" && format != "gob" {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Unsupported format %q", format))
		return
	}
	dec := gob.NewDecoder(r.Body)
	var c db.CommitComment
	if err := dec.Decode(&c); err != nil {
		httputils.ReportError(w, r, err, "Unable to decode CommitComment")
		return
	}
	var err error
	var errMsg string
	if isPut {
		err = s.d.PutCommitComment(&c)
		errMsg = "Unable to create comment"
	} else {
		err = s.d.DeleteCommitComment(&c)
		errMsg = "Unable to delete comment"
	}
	if err != nil {
		reportDBError(w, r, err, errMsg)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// See documentation for PostTaskCommentsHandler; this method is the same for
// db.CommitComment.
func (s *server) PostCommitCommentsHandler(w http.ResponseWriter, r *http.Request) {
	s.sharedCommitCommentsHandler(w, r, true)
}

// See documentation for DeleteTaskCommentsHandler; this method is the same for
// db.CommitComment.
func (s *server) DeleteCommitCommentsHandler(w http.ResponseWriter, r *http.Request) {
	s.sharedCommitCommentsHandler(w, r, false)
}

// doCommentRequest implements the client side of
// (Put|Delete)(Task|TaskSpec|Commit)Comment. Sends an HTTP request with the
// given method and path, with the request body given by gob. If an error status
// code is returned, translates into a DB error.
func (c *client) doCommentRequest(method, path string, gob io.Reader) error {
	req, err := http.NewRequest(method, c.serverRoot+path+"?format=gob", gob)
	if err != nil {
		return err
	}
	r, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if err := interpretStatusCode(r); err != nil {
		return err
	}
	return nil
}

// See documentation for db.CommentDB.
func (c *client) PutTaskComment(tc *db.TaskComment) error {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(tc); err != nil {
		return err
	}
	return c.doCommentRequest(http.MethodPost, TASK_COMMENTS_PATH, &buf)
}

// See documentation for db.CommentDB.
func (c *client) DeleteTaskComment(tc *db.TaskComment) error {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(tc); err != nil {
		return err
	}
	return c.doCommentRequest(http.MethodDelete, TASK_COMMENTS_PATH, &buf)
}

// See documentation for db.CommentDB.
func (c *client) PutTaskSpecComment(sc *db.TaskSpecComment) error {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(sc); err != nil {
		return err
	}
	return c.doCommentRequest(http.MethodPost, TASK_SPEC_COMMENTS_PATH, &buf)
}

// See documentation for db.CommentDB.
func (c *client) DeleteTaskSpecComment(sc *db.TaskSpecComment) error {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(sc); err != nil {
		return err
	}
	return c.doCommentRequest(http.MethodDelete, TASK_SPEC_COMMENTS_PATH, &buf)
}

// See documentation for db.CommentDB.
func (c *client) PutCommitComment(cc *db.CommitComment) error {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(cc); err != nil {
		return err
	}
	return c.doCommentRequest(http.MethodPost, COMMIT_COMMENTS_PATH, &buf)
}

// See documentation for db.CommentDB.
func (c *client) DeleteCommitComment(cc *db.CommitComment) error {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(cc); err != nil {
		return err
	}
	return c.doCommentRequest(http.MethodDelete, COMMIT_COMMENTS_PATH, &buf)
}

// Compile-time assert that client is a db.RemoteDB.
var _ db.RemoteDB = &client{}
