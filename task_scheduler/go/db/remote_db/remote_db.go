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
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/pubsub"
	"go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2"
)

const (
	// Server handles requests on these paths. See registerHandlers for detail.
	TASKS_PATH              = "tasks"
	JOBS_PATH               = "jobs"
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

	// Maximum time range of tasks to load at once in GetTasksFromDateRange.
	MAX_TASK_TIME_RANGE = 7 * 24 * time.Hour
)

// server translates HTTP requests to method calls on d.
type server struct {
	d db.RemoteDB
}

// RegisterServer adds handlers to r that handle requests from a client created
// via NewClient.
//
// It is recommended that the caller restrict DB access to a specific set of
// users.
func RegisterServer(d db.RemoteDB, r *mux.Router) error {
	s := &server{
		d: d,
	}
	s.registerHandlers(r)
	return nil
}

// registerHandlers adds GET, POST, and DELETE handlers to r on various paths.
func (s *server) registerHandlers(r *mux.Router) {
	r.HandleFunc("/"+TASKS_PATH, s.GetTasksHandler).Methods(http.MethodGet)
	r.HandleFunc("/"+JOBS_PATH, s.GetJobsHandler).Methods(http.MethodGet)
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
	db.ModifiedData
}

// NewClient returns a db.RemoteDB that connects to the server created by
// NewServer. serverRoot should end with a slash.
func NewClient(serverRoot, topicSet, label string, ts oauth2.TokenSource) (db.RemoteDB, error) {
	c := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	mod, err := pubsub.NewModifiedData(topicSet, label, ts)
	if err != nil {
		return nil, err
	}
	return &client{
		serverRoot:   serverRoot,
		client:       c,
		ModifiedData: mod,
	}, nil
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

// writeTaskList encodes a list of types.Task to w. Writes a GOB stream; first
// object is the number of tasks, the remaining objects are types.Tasks.
func writeTaskList(w http.ResponseWriter, tasks []*types.Task) error {
	w.Header().Set("Content-Type", "application/gob")
	enc := gob.NewEncoder(w)
	if err := enc.Encode(len(tasks)); err != nil {
		return fmt.Errorf("Unable to encode task count: %s", err)
	}
	for _, task := range tasks {
		if err := enc.Encode(task); err != nil {
			return fmt.Errorf("Unable to encode task: %s", err)
		}
		flush(w)
	}
	return nil
}

// getTaskList sends an HTTP request to the given URL and decodes the response
// body as a list of types.Task, as written by writeTaskList.
func (c *client) getTaskList(url string) ([]*types.Task, error) {
	r, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer util.Close(r.Body)
	if err := interpretStatusCode(r); err != nil {
		return nil, err
	}
	dec := gob.NewDecoder(r.Body)
	var count int
	if err := dec.Decode(&count); err != nil {
		return nil, err
	}
	rv := make([]*types.Task, count)
	for i := range rv {
		var t types.Task
		if err := dec.Decode(&t); err != nil {
			return nil, err
		}
		rv[i] = &t
	}
	return rv, nil
}

// writeJobList encodes a list of types.Job to w. Writes a GOB stream; first object
// is the number of jobs, the remaining objects are types.Jobs.
func writeJobList(w http.ResponseWriter, jobs []*types.Job) error {
	w.Header().Set("Content-Type", "application/gob")
	enc := gob.NewEncoder(w)
	if err := enc.Encode(len(jobs)); err != nil {
		return fmt.Errorf("Unable to encode job count: %s", err)
	}
	for _, job := range jobs {
		if err := enc.Encode(job); err != nil {
			return fmt.Errorf("Unable to encode job: %s", err)
		}
		flush(w)
	}
	return nil
}

// getJobList sends an HTTP request to the given URL and decodes the response
// body as a list of types.Job, as written by writeJobList.
func (c *client) getJobList(url string) ([]*types.Job, error) {
	r, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer util.Close(r.Body)
	if err := interpretStatusCode(r); err != nil {
		return nil, err
	}
	dec := gob.NewDecoder(r.Body)
	var count int
	if err := dec.Decode(&count); err != nil {
		return nil, err
	}
	rv := make([]*types.Job, count)
	for i := range rv {
		var t types.Job
		if err := dec.Decode(&t); err != nil {
			return nil, err
		}
		rv[i] = &t
	}
	return rv, nil
}

// GetTasksHandler translates a GET request to GetTasksFromDateRange or
// GetTaskById.
//   - format: must be "gob"; default "gob"
//   - id: Task.Id; may not be repeated
//   - from, to: nanoseconds since the Unix epoch. (base-10 string)
// Response is GOB stream; first object is the number of tasks, the remaining
// objects are types.Tasks.
func (s *server) GetTasksHandler(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format != "" && format != "gob" {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Unsupported format %q", format))
		return
	}
	id := r.URL.Query().Get("id")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	repo := r.URL.Query().Get("repo")
	if id == "" && (fromStr == "" || toStr == "") {
		httputils.ReportError(w, r, nil, "Only lookup by id or date range is implemented. Missing id/from/to params")
		return
	}
	var tasks []*types.Task
	if id != "" {
		task, err := s.d.GetTaskById(id)
		if err != nil {
			reportDBError(w, r, err, "Unable to retrieve task")
			return
		}
		if task == nil {
			tasks = []*types.Task{}
		} else {
			tasks = []*types.Task{task}
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
		from := time.Unix(0, fromInt)
		to := time.Unix(0, toInt)
		tasks, err = s.d.GetTasksFromDateRange(from, to, repo)
		if err != nil {
			reportDBError(w, r, err, "Unable to retrieve tasks")
			return
		}
		if len(tasks) >= 10000 {
			sklog.Debugf("Loaded %d tasks for request from %s for time range from %s to %s.", len(tasks), r.RemoteAddr, from, to)
		}
	}
	if err := writeTaskList(w, tasks); err != nil {
		httputils.ReportError(w, r, err, "")
		return
	}
}

// See documentation for types.TaskReader.
func (c *client) GetTaskById(id string) (*types.Task, error) {
	params := url.Values{}
	params.Set("format", "gob")
	params.Set("id", id)
	tasks, err := c.getTaskList(c.serverRoot + TASKS_PATH + "?" + params.Encode())
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

// See documentation for types.TaskReader.
func (c *client) GetTasksFromDateRange(from time.Time, to time.Time, repo string) ([]*types.Task, error) {
	params := url.Values{}
	params.Set("format", "gob")
	if repo != "" {
		params.Set("repo", repo)
	}
	tasks := make([]*types.Task, 0, 1024)
	if err := util.IterTimeChunks(from, to, MAX_TASK_TIME_RANGE, func(chunkStart, chunkEnd time.Time) error {
		params.Set("from", strconv.FormatInt(chunkStart.UnixNano(), 10))
		params.Set("to", strconv.FormatInt(chunkEnd.UnixNano(), 10))
		sklog.Debugf("Retrieving tasks from (%s, %s)", chunkStart, chunkEnd)
		chunkTasks, err := c.getTaskList(c.serverRoot + TASKS_PATH + "?" + params.Encode())
		if err != nil {
			return err
		}
		tasks = append(tasks, chunkTasks...)
		return nil
	}); err != nil {
		return nil, err
	}
	return tasks, nil
}

// GetJobsHandler translates a GET request to GetJobsFromDateRange or
// GetJobById.
//   - format: must be "gob"; default "gob"
//   - id: Job.Id; may not be repeated
//   - from, to: nanoseconds since the Unix epoch. (base-10 string)
// Response is GOB stream; first object is the number of jobs, the remaining
// objects are types.Jobs.
func (s *server) GetJobsHandler(w http.ResponseWriter, r *http.Request) {
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
	var jobs []*types.Job
	if id != "" {
		job, err := s.d.GetJobById(id)
		if err != nil {
			reportDBError(w, r, err, "Unable to retrieve job")
			return
		}
		if job == nil {
			jobs = []*types.Job{}
		} else {
			jobs = []*types.Job{job}
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
		jobs, err = s.d.GetJobsFromDateRange(time.Unix(0, fromInt), time.Unix(0, toInt))
		if err != nil {
			reportDBError(w, r, err, "Unable to retrieve jobs")
			return
		}
	}
	if err := writeJobList(w, jobs); err != nil {
		httputils.ReportError(w, r, err, "")
		return
	}
}

// See documentation for types.JobReader.
func (c *client) GetJobById(id string) (*types.Job, error) {
	params := url.Values{}
	params.Set("format", "gob")
	params.Set("id", id)
	jobs, err := c.getJobList(c.serverRoot + JOBS_PATH + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, nil
	}
	if len(jobs) > 1 {
		return nil, fmt.Errorf("Unexpected multiple jobs for id query. %q %v", id, jobs)
	}
	return jobs[0], nil
}

// See documentation for types.JobReader.
func (c *client) GetJobsFromDateRange(from time.Time, to time.Time) ([]*types.Job, error) {
	params := url.Values{}
	params.Set("format", "gob")
	params.Set("from", strconv.FormatInt(from.UnixNano(), 10))
	params.Set("to", strconv.FormatInt(to.UnixNano(), 10))
	return c.getJobList(c.serverRoot + JOBS_PATH + "?" + params.Encode())
}

// GetCommentsHandler translates a GET request to GetCommentsForRepos
//   - format: must be "gob"; default "gob"
//   - repo: repo for which to return comments (may be repeated)
//   - from (optional): nanoseconds since the Unix epoch. (base-10 string)
// Response is GOB stream; first object is the number of types.RepoComments, the
// remaining objects are types.RepoComments.
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
func (c *client) GetCommentsForRepos(repos []string, from time.Time) ([]*types.RepoComments, error) {
	params := url.Values{"repo": repos}
	params.Set("format", "gob")
	if !util.TimeIsZero(from) {
		params.Add("from", strconv.FormatInt(from.UnixNano(), 10))
	}
	r, err := c.client.Get(c.serverRoot + COMMENTS_PATH + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer util.Close(r.Body)
	if err := interpretStatusCode(r); err != nil {
		return nil, err
	}
	dec := gob.NewDecoder(r.Body)
	var count int
	if err := dec.Decode(&count); err != nil {
		return nil, err
	}
	rv := make([]*types.RepoComments, count)
	for i := range rv {
		var c types.RepoComments
		if err := dec.Decode(&c); err != nil {
			return nil, err
		}
		rv[i] = &c
	}
	return rv, nil
}

// sharedTaskCommentsHandler translates a POST or DELETE request where the body
// is a GOB-encoded types.TaskComment to PutTaskComment or DeleteTaskComment.
//   - format: must be "gob"; default "gob"
// No response body.
func (s *server) sharedTaskCommentsHandler(w http.ResponseWriter, r *http.Request, isPut bool) {
	format := r.URL.Query().Get("format")
	if format != "" && format != "gob" {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Unsupported format %q", format))
		return
	}
	dec := gob.NewDecoder(r.Body)
	var c types.TaskComment
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
// is a GOB-encoded types.TaskComment to PutTaskComment.
//   - format: must be "gob"; default "gob"
// No response body.
func (s *server) PostTaskCommentsHandler(w http.ResponseWriter, r *http.Request) {
	s.sharedTaskCommentsHandler(w, r, true)
}

// DeleteTaskCommentsHandler translates a DELETE request where the body is a
// GOB-encoded types.TaskComment to DeleteTaskComment.
//   - format: must be "gob"; default "gob"
// No response body.
func (s *server) DeleteTaskCommentsHandler(w http.ResponseWriter, r *http.Request) {
	s.sharedTaskCommentsHandler(w, r, false)
}

// See documentation for sharedTaskCommentsHandler; this method is the same for
// types.TaskSpecComment.
func (s *server) sharedTaskSpecCommentsHandler(w http.ResponseWriter, r *http.Request, isPut bool) {
	format := r.URL.Query().Get("format")
	if format != "" && format != "gob" {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Unsupported format %q", format))
		return
	}
	dec := gob.NewDecoder(r.Body)
	var c types.TaskSpecComment
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
// types.TaskSpecComment.
func (s *server) PostTaskSpecCommentsHandler(w http.ResponseWriter, r *http.Request) {
	s.sharedTaskSpecCommentsHandler(w, r, true)
}

// See documentation for DeleteTaskCommentsHandler; this method is the same for
// types.TaskSpecComment.
func (s *server) DeleteTaskSpecCommentsHandler(w http.ResponseWriter, r *http.Request) {
	s.sharedTaskSpecCommentsHandler(w, r, false)
}

// See documentation for sharedTaskCommentsHandler; this method is the same for
// types.CommitComment.
func (s *server) sharedCommitCommentsHandler(w http.ResponseWriter, r *http.Request, isPut bool) {
	format := r.URL.Query().Get("format")
	if format != "" && format != "gob" {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Unsupported format %q", format))
		return
	}
	dec := gob.NewDecoder(r.Body)
	var c types.CommitComment
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
// types.CommitComment.
func (s *server) PostCommitCommentsHandler(w http.ResponseWriter, r *http.Request) {
	s.sharedCommitCommentsHandler(w, r, true)
}

// See documentation for DeleteTaskCommentsHandler; this method is the same for
// types.CommitComment.
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
	defer util.Close(r.Body)
	if err := interpretStatusCode(r); err != nil {
		return err
	}
	return nil
}

// See documentation for db.CommentDB.
func (c *client) PutTaskComment(tc *types.TaskComment) error {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(tc); err != nil {
		return err
	}
	return c.doCommentRequest(http.MethodPost, TASK_COMMENTS_PATH, &buf)
}

// See documentation for db.CommentDB.
func (c *client) DeleteTaskComment(tc *types.TaskComment) error {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(tc); err != nil {
		return err
	}
	return c.doCommentRequest(http.MethodDelete, TASK_COMMENTS_PATH, &buf)
}

// See documentation for db.CommentDB.
func (c *client) PutTaskSpecComment(sc *types.TaskSpecComment) error {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(sc); err != nil {
		return err
	}
	return c.doCommentRequest(http.MethodPost, TASK_SPEC_COMMENTS_PATH, &buf)
}

// See documentation for db.CommentDB.
func (c *client) DeleteTaskSpecComment(sc *types.TaskSpecComment) error {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(sc); err != nil {
		return err
	}
	return c.doCommentRequest(http.MethodDelete, TASK_SPEC_COMMENTS_PATH, &buf)
}

// See documentation for db.CommentDB.
func (c *client) PutCommitComment(cc *types.CommitComment) error {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(cc); err != nil {
		return err
	}
	return c.doCommentRequest(http.MethodPost, COMMIT_COMMENTS_PATH, &buf)
}

// See documentation for db.CommentDB.
func (c *client) DeleteCommitComment(cc *types.CommitComment) error {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(cc); err != nil {
		return err
	}
	return c.doCommentRequest(http.MethodDelete, COMMIT_COMMENTS_PATH, &buf)
}

// Compile-time assert that client is a db.RemoteDB.
var _ db.RemoteDB = &client{}
