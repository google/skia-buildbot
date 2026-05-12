package ingester

import (
	"context"
	"net/http"
	"sync"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/autogardener/go/db"
	"go.skia.org/infra/autogardener/go/gemini"
	"go.skia.org/infra/autogardener/go/types"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/go/workerpool"
	ts_db "go.skia.org/infra/task_scheduler/go/db"
	ts_types "go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2/google"
	"golang.org/x/sync/errgroup"
)

const workerPoolSize = 10

type Ingester struct {
	db         db.AutoGardenerDB
	gemini     *gemini.Client
	httpClient *http.Client
	tsDB       ts_db.TaskReader
}

func New(ctx context.Context, db db.AutoGardenerDB, gemini *gemini.Client, tsDB ts_db.TaskReader) (*Ingester, error) {
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, datastore.ScopeDatastore)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &Ingester{
		db:         db,
		gemini:     gemini,
		httpClient: httputils.DefaultClientConfig().WithTokenSource(ts).Client(),
		tsDB:       tsDB,
	}, nil
}

func (i *Ingester) GetTaskSummariesForRepo(ctx context.Context, repoURL, branch string, numCommits int) ([]*types.TaskAndSummary, error) {
	tasks, err := i.getFailedTasks(ctx, repoURL, branch, numCommits)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Retrieve summaries from the DB.
	var eg errgroup.Group
	results := make([]*types.TaskAndSummary, len(tasks))
	for idx, task := range tasks {
		results[idx] = &types.TaskAndSummary{
			Task: task,
		}
		eg.Go(func() error {
			taskSummary, err := i.db.GetTaskSummary(ctx, task.Id)
			if err != nil {
				return skerr.Wrap(err)
			}
			results[idx].Summary = taskSummary
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, skerr.Wrap(err)
	}
	return results, nil
}

func (i *Ingester) IngestTaskSummariesForRepo(ctx context.Context, repoURL, branch string, numCommits int) error {
	sklog.Infof("IngestTaskSummariesForRepo(%s, %s, %d)", repoURL, branch, numCommits)

	tasks, err := i.getFailedTasks(ctx, repoURL, branch, numCommits)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Extract and analyze the errors for the failed tasks, inserting them into
	// the DB.
	pool := workerpool.New(workerPoolSize)
	var mtx sync.Mutex
	results := make([]*types.TaskAndSummary, 0, len(tasks))
	errs := []error{}
	for _, task := range tasks {
		pool.Go(func() {
			// If we already have a summary for this task, skip it.
			taskSummary, err := i.db.GetTaskSummary(ctx, task.Id)
			if err != nil {
				errs = append(errs, err)
				return
			}
			if taskSummary == nil {
				// Use Gemini to find the error summary for this task and insert it
				// into the DB.
				taskSummary, err = i.gemini.GetTaskSummary(ctx, task)
				if err != nil {
					errs = append(errs, err)
					return
				}
				if err := i.db.PutTaskSummary(ctx, task.Id, taskSummary); err != nil {
					errs = append(errs, err)
					return
				}
			}
			mtx.Lock()
			defer mtx.Unlock()
			results = append(results, &types.TaskAndSummary{
				Summary: taskSummary,
				Task:    task,
			})
			sklog.Infof("  %d/%d tasks ingested", len(results), len(tasks))
		})
	}
	pool.Wait()
	if len(errs) > 0 {
		return skerr.Wrap(errs[0])
	}
	sklog.Infof("  Updated summaries for %d tasks.", len(tasks))
	return nil
}

func (i *Ingester) getFailedTasks(ctx context.Context, repoURL, branch string, numCommits int) ([]*ts_types.Task, error) {
	// TODO(borenet): We should be able to reuse the TaskCache that task
	// scheduler uses, which would keep more tasks in memory but would be more
	// efficient across the running time of the service.

	// Retrieve the last N commits in the repo.
	repo := gitiles.NewRepo(repoURL, i.httpClient)
	commits, err := repo.Log(ctx, branch, gitiles.LogLimit(numCommits))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	sklog.Infof("  Retrieved %d commits", len(commits))
	for _, commit := range commits {
		sklog.Infof("    %s", commit.Hash)
	}

	var eg errgroup.Group
	var mtx sync.Mutex

	// Retrieve failed tasks for all commits.
	var tasks []*ts_types.Task
	for _, commit := range commits {
		eg.Go(func() error {
			t, err := i.getFailedTasksForCommit(ctx, commit)
			if err != nil {
				return skerr.Wrap(err)
			}
			mtx.Lock()
			defer mtx.Unlock()
			tasks = append(tasks, t...)
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, skerr.Wrap(err)
	}
	sklog.Infof("  Found %d failing tasks.", len(tasks))
	return tasks, nil
}

func (i *Ingester) getFailedTasksForCommit(ctx context.Context, commit *vcsinfo.LongCommit) ([]*ts_types.Task, error) {
	var results []*ts_types.Task
	searchResultsLimit := 0
	empty := ""
	for _, status := range []ts_types.TaskStatus{ts_types.TASK_STATUS_FAILURE, ts_types.TASK_STATUS_MISHAP} {
		params := &ts_db.TaskSearchParams{
			Revision:  &commit.Hash,
			Status:    &status,
			Issue:     &empty, // Exclude try jobs.
			Patchset:  &empty, // Exclude try jobs.
			TimeStart: &commit.Timestamp,
			Limit:     &searchResultsLimit,
		}
		tasks, err := i.tsDB.SearchTasks(ctx, params)
		if err != nil {
			return nil, err
		}
		results = append(results, tasks...)
	}
	return results, nil
}
