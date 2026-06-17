package task_scheduler

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2/google"
)

type TaskSchedulerClient struct {
	client *http.Client
	db     db.DBCloser
}

func NewClient(ctx context.Context, firestoreInstance string) (*TaskSchedulerClient, error) {
	ts, err := google.DefaultTokenSource(ctx, datastore.ScopeDatastore)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	db, err := firestore.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, firestoreInstance, ts)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &TaskSchedulerClient{
		client: client,
		db:     db,
	}, nil
}

func (c *TaskSchedulerClient) Close() error {
	return skerr.Wrap(c.db.Close())
}

func (c *TaskSchedulerClient) SearchTasksHandler(ctx context.Context, req mcp.CallToolRequest) (fmt.Stringer, error) {
	defer timer.New("SearchTasksHandler").Stop()
	startTime, err := parseTimeOrNil(req, argStartTime)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	endTime, err := parseTimeOrNil(req, argEndTime)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// If issue and patchset aren't provided, assume the caller doesn't want try jobs included.
	issue := req.GetString(argIssue, "")
	patchset := req.GetString(argPatchset, "")

	status := getStringOrNil(req, argTaskStatus)
	if status != nil && *status == "PENDING" {
		*status = string(types.TASK_STATUS_PENDING)
	}

	limit := req.GetInt(argLimit, db.SearchResultLimit)

	searchParams := &db.TaskSearchParams{
		Status:    (*types.TaskStatus)(status),
		Issue:     &issue,
		Name:      getStringOrNil(req, argTaskName),
		Patchset:  &patchset,
		Repo:      getStringOrNil(req, argRepo),
		Revision:  getStringOrNil(req, argRevision),
		TimeStart: startTime,
		TimeEnd:   endTime,
		Limit:     &limit,
	}
	tasks, err := c.db.SearchTasks(ctx, searchParams)

	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return TaskList(tasks), nil
}

func (c *TaskSchedulerClient) GetTaskHealthReportHandler(ctx context.Context, req mcp.CallToolRequest) (fmt.Stringer, error) {
	defer timer.New("GetTaskHealthReportHandler").Stop()
	repoUrl, err := req.RequireString(argRepo)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	revision, err := req.RequireString(argRevision)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	limit, err := req.RequireInt(argLimit)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	taskName := req.GetString(argTaskName, "")
	includeStable := req.GetBool(argIncludeStable, false)
	includeSuccessful := req.GetBool(argIncludeSuccessful, true)

	repo, err := gitiles.NewRepoWithClient(repoUrl, c.client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	commits, err := repo.Log(ctx, revision, gitiles.LogLimit(limit))
	if err != nil {
		return nil, err
	}

	var resp TaskHealthReport
	resp.Commits = make([]*vcsinfo.ShortCommit, 0, len(commits))
	for _, c := range commits {
		resp.Commits = append(resp.Commits, c.ShortCommit)
	}

	// map[taskName]map[commitHash]*types.Task
	resp.Tasks = map[string]map[string]*types.Task{}
	searchResultsLimit := 0
	empty := ""
	for _, commit := range commits {
		params := &db.TaskSearchParams{
			Repo:      &repoUrl,
			Revision:  &commit.Hash,
			Issue:     &empty, // Exclude try jobs.
			Patchset:  &empty, // Exclude try jobs.
			TimeStart: &commit.Timestamp,
			Limit:     &searchResultsLimit,
		}
		if taskName != "" {
			params.Name = &taskName
		}
		tasks, err := c.db.SearchTasks(ctx, params)
		if err != nil {
			return nil, err
		}

		for _, t := range tasks {
			// Ignore in-progress tasks.
			if !t.Done() {
				continue
			}
			history, ok := resp.Tasks[t.Name]
			if !ok {
				history = map[string]*types.Task{}
				resp.Tasks[t.Name] = history
			}
			// TODO(borenet): Include retries?
			if existing, ok := history[commit.Hash]; !ok || existing.Created.Before(t.Created) {
				history[commit.Hash] = t
			}
		}
	}

	// Filter out stable and successful tasks.
	if taskName == "" {
		for taskName, tasksByCommit := range resp.Tasks {
			stable := true
			succeededLatestRun := false
			var firstStatus types.TaskStatus = "(unset)"
			for _, task := range tasksByCommit {
				if firstStatus == "(unset)" {
					if task.Status == types.TASK_STATUS_SUCCESS {
						succeededLatestRun = true
					}
					firstStatus = task.Status
				}
				if task.Status != firstStatus {
					stable = false
					break
				}
			}
			if !includeStable && stable {
				delete(resp.Tasks, taskName)
			} else if !includeSuccessful && succeededLatestRun {
				delete(resp.Tasks, taskName)
			}
		}
	}

	return &resp, nil
}
func (c *TaskSchedulerClient) GetTaskHandler(ctx context.Context, req mcp.CallToolRequest) (fmt.Stringer, error) {
	defer timer.New("GetTaskHandler").Stop()
	taskID, err := req.RequireString(argTaskId)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	task, err := c.db.GetTaskById(ctx, taskID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if task == nil {
		return nil, skerr.Fmt("No such task with ID %q", taskID)
	}
	return &TaskWrapper{task}, nil
}

type TaskList []*types.Task

func (l TaskList) String() string {
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "| ID | Name | Status | Revision | Created |\n")
	_, _ = fmt.Fprintf(&sb, "|----|------|--------|----------|---------|\n")
	for _, t := range l {
		_, _ = fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s |\n", t.Id, t.Name, t.Status, t.Revision, t.Created.Format(time.RFC3339))
	}
	return sb.String()
}

type TaskWrapper struct {
	*types.Task
}

func (w TaskWrapper) String() string {
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "**ID:** %s\n", w.Id)
	_, _ = fmt.Fprintf(&sb, "**Name:** %s\n", w.Name)
	_, _ = fmt.Fprintf(&sb, "**Status:** %s\n", w.Status)
	_, _ = fmt.Fprintf(&sb, "**Revision:** %s\n", w.Revision)
	_, _ = fmt.Fprintf(&sb, "**Created:** %s\n", w.Created.Format(time.RFC3339))
	_, _ = fmt.Fprintf(&sb, "**Started:** %s\n", w.Started.Format(time.RFC3339))
	_, _ = fmt.Fprintf(&sb, "**Finished:** %s\n", w.Finished.Format(time.RFC3339))
	_, _ = fmt.Fprintf(&sb, "**Swarming Task ID:** %s\n", w.SwarmingTaskId)
	_, _ = fmt.Fprintf(&sb, "**Swarming Bot ID:** %s\n", w.SwarmingBotId)
	return sb.String()
}

type TaskHealthReport struct {
	Commits []*vcsinfo.ShortCommit            `json:"commits"`
	Tasks   map[string]map[string]*types.Task `json:"tasks"`
}

func (r *TaskHealthReport) String() string {
	var sb strings.Builder
	fmt.Fprint(&sb, Commits(r.Commits).String())

	// Ensure that the results are in a predictable order.
	taskNames := make([]string, 0, len(r.Tasks))
	for taskName := range r.Tasks {
		taskNames = append(taskNames, taskName)
	}
	sort.Strings(taskNames)

	_, _ = fmt.Fprintf(&sb, "\n\n# Task Results\n\n")
	for _, taskName := range taskNames {
		_, _ = fmt.Fprintf(&sb, "## %s\n\n", taskName)
		fmt.Fprint(&sb, TaskHistory(r.Tasks[taskName]).String(r.Commits))
		fmt.Fprintln(&sb, "")
	}
	return sb.String()
}

type Commits []*vcsinfo.ShortCommit

func (c Commits) String() string {
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "| Commit  | Subject |\n")
	_, _ = fmt.Fprintf(&sb, "|---------|---------|\n")
	for _, commit := range c {
		_, _ = fmt.Fprintf(&sb, "| %s | %s |\n", commit.Hash[:7], commit.Subject)
	}
	return sb.String()
}

type TaskHistory map[string]*types.Task

func (h TaskHistory) String(commits []*vcsinfo.ShortCommit) string {
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "| Commit  | Result  | Task ID |\n")
	_, _ = fmt.Fprintf(&sb, "|---------|---------|---------|\n")
	var currentTask *types.Task
	for _, commit := range commits {
		task, ok := h[commit.Hash]
		if ok {
			_, _ = fmt.Fprintf(&sb, "| %s | %s | %s |\n", commit.Hash[:7], task.Status, task.Id)
			currentTask = task
		} else if currentTask != nil {
			// TODO(borenet): we could display something different to
			// indicate that we're in the blamelist of the currentTask.
			_, _ = fmt.Fprintf(&sb, "| %s | %s | %s |\n", commit.Hash[:7], strings.Repeat(" ", len(string(currentTask.Status))), strings.Repeat(" ", len(currentTask.Id)))
		} else {
			_, _ = fmt.Fprintf(&sb, "| %s |         |         |\n", commit.Hash[:7])
		}
	}
	return sb.String()
}

func getStringOrNil(req mcp.CallToolRequest, arg string) *string {
	// Using RequireString is cleaner than choosing a placeholder to use when
	// the arg is not provided, even if we don't really require it.
	str, err := req.RequireString(arg)
	if err != nil {
		return nil
	}
	return &str
}

func parseTimeOrNil(req mcp.CallToolRequest, arg string) (*time.Time, error) {
	str, err := req.RequireString(arg)
	if err != nil {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, str)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &parsed, nil
}
