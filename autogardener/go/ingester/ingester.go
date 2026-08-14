package ingester

import (
	"context"
	"net/http"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/autogardener/go/db"
	"go.skia.org/infra/autogardener/go/gemini"
	"go.skia.org/infra/autogardener/go/types"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	ts_db "go.skia.org/infra/task_scheduler/go/db"
	ts_types "go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2/google"
)

const workerPoolSize = 10

type Ingester struct {
	db         db.AutoGardenerDB
	gemini     gemini.Client
	httpClient *http.Client
	repos      repograph.Map
	tsDB       ts_db.TaskReader
}

func New(ctx context.Context, db db.AutoGardenerDB, gemini gemini.Client, repos repograph.Map, tsDB ts_db.TaskReader) (*Ingester, error) {
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, datastore.ScopeDatastore)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &Ingester{
		db:         db,
		gemini:     gemini,
		httpClient: httputils.DefaultClientConfig().WithTokenSource(ts).Client(),
		repos:      repos,
		tsDB:       tsDB,
	}, nil
}

func (i *Ingester) StartGeneratingReportsForRepo(ctx context.Context, repoURL, branch string, numCommits int, interval time.Duration) {
	lv := metrics2.NewLiveness("liveness_autogardener_report_generation", map[string]string{
		"repo":   repoURL,
		"branch": branch,
	})
	go util.RepeatCtx(ctx, interval, func(ctx context.Context) {
		sklog.Infof("Generating report for repo %s @ %s", repoURL, branch)
		report, err := i.gemini.GenerateReport(ctx, repoURL, branch, numCommits)
		if err != nil {
			sklog.Errorf("Failed generating report for repo %s @ %s: %s", repoURL, branch, err)
			return
		}
		if err := i.db.PutReport(ctx, repoURL, branch, report); err != nil {
			sklog.Errorf("Failed storing report for repo %s @ %s: %s", repoURL, branch, err)
		} else {
			sklog.Infof("Successfully generated and stored report for %s @ %s", repoURL, branch)
			lv.Reset()
		}
	})
}

func (i *Ingester) enqueueModifiedTasks(ctx context.Context, repoURL string, taskCh chan<- *ts_types.Task) {
	modCh := i.tsDB.ModifiedTasksCh(ctx)
	for {
		select {
		case tasks := <-modCh:
			for _, task := range tasks {
				if task.Done() && !task.Success() && task.Repo == repoURL {
					taskCh <- task
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (i *Ingester) periodicTaskPollingFallback(ctx context.Context, repoURL string, period time.Duration, taskCh chan<- *ts_types.Task) {
	lv := metrics2.NewLiveness("liveness_autogardener_task_ingestion_fallback", map[string]string{
		"repo": repoURL,
	})
	util.RepeatCtx(ctx, 5*time.Minute, func(ctx context.Context) {
		tasks, err := i.getFailedTasks(ctx, repoURL, period)
		if err != nil {
			sklog.Errorf("Failed to retrieve tasks: %s", err)
			return
		}
		lv.Reset()
		for _, task := range tasks {
			taskCh <- task
		}
	})
}

type taskIngestionResult struct {
	types.TaskAndSummary
	err error
}

func (i *Ingester) ingestTasks(ctx context.Context, input <-chan *ts_types.Task, output chan<- *taskIngestionResult) {
	processing := newTaskProcessingRegistry()
	for range workerPoolSize {
		go func() {
			for {
				select {
				case task := <-input:
					taskSummary, err := i.ingestTask(ctx, processing, task)
					result := &taskIngestionResult{}
					if err != nil {
						result.err = err
					} else {
						result.Task = task
						result.Summary = taskSummary
					}
					output <- result
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	<-ctx.Done()
}

func (i *Ingester) periodicUnclassifiedTaskSummaryPollingFallback(ctx context.Context, classifyCh chan<- *types.TaskAndSummary) {
	util.RepeatCtx(ctx, 5*time.Minute, func(ctx context.Context) {
		// TODO(borenet): This retrieves task summaries for ALL repos.
		unclassified, err := i.db.GetUnclassifiedTaskSummaries(ctx, 100)
		if err != nil {
			sklog.Errorf("Failed to retrieve unclassified task summaries: %s", err)
			return
		}
		for taskID, taskSummary := range unclassified {
			task, err := i.tsDB.GetTaskById(ctx, taskID)
			if err != nil {
				sklog.Errorf("Failed to retrieve task from the DB: %s", err)
			}
			classifyCh <- &types.TaskAndSummary{
				Task:    task,
				Summary: taskSummary,
			}
		}
	})
}

func (i *Ingester) StartIngestingTaskSummariesForRepo(ctx context.Context, repoURL string, period time.Duration) {
	taskCh := make(chan *ts_types.Task)

	// Primary: receive tasks from DB as they are updated.
	go i.enqueueModifiedTasks(ctx, repoURL, taskCh)

	// Secondary: fall back to periodically loading all failed tasks, in case
	// any are missed by the above.
	go i.periodicTaskPollingFallback(ctx, repoURL, period, taskCh)

	// Start up a worker pool to ingest the tasks in parallel.
	taskIngestResultCh := make(chan *taskIngestionResult)
	go i.ingestTasks(ctx, taskCh, taskIngestResultCh)

	// We classify failures sequentially to prevent the race condition in which
	// multiple task failures with the same, new, failure class are processed at
	// the same time and thus add duplicates to the DB.
	classifyCh := make(chan *types.TaskAndSummary, 100)

	// Consume task ingestion results from that channel and enqueue them into
	// the classification channel.
	go func() {
		for {
			select {
			case result := <-taskIngestResultCh:
				if result.err != nil {
					sklog.Errorf("failed to ingest task: %s", result.err)
					continue
				} else if result.Summary == nil {
					// This occurs if the worker received a task which was
					// already being processed by another worker. Ignore.
					continue
				} else if result.Summary.FailureClassId != "" {
					continue
				}

				// Enqueue the classification of the task summary. Do so in
				// a separate goroutine to prevent blocking workers.
				go func() {
					classifyCh <- &types.TaskAndSummary{
						Task:    result.Task,
						Summary: result.Summary,
					}
				}()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Backup: periodically fetch all unclassified task summaries and add them
	// to the queue.
	go i.periodicUnclassifiedTaskSummaryPollingFallback(ctx, classifyCh)

	// Classify task summaries.
	go func() {
		for {
			select {
			case job := <-classifyCh:
				if err := i.classifyTaskSummary(ctx, job.Task, job.Summary); err != nil {
					sklog.Errorf("Failed to classify task summary: %s", err)
					continue
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (i *Ingester) ingestTask(ctx context.Context, processing *taskProcessingRegistry, task *ts_types.Task) (*types.TaskSummary, error) {
	// If another worker is already processing this task, skip it.
	release, ok := processing.TryClaimTask(task.Id)
	if !ok {
		return nil, nil
	}
	defer release()

	// If we already have a summary for this task, skip it.
	taskSummary, err := i.db.GetTaskSummary(ctx, task.Id)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to ingest task %s", task.Id)
	}
	if taskSummary != nil {
		return taskSummary, nil
	}
	// Use Gemini to find the error summary for this task and insert it
	// into the DB.
	taskSummary, err = i.gemini.GetTaskSummary(ctx, task)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to ingest task %s", task.Id)
	}
	if err := i.db.PutTaskSummary(ctx, task.Id, taskSummary); err != nil {
		return nil, skerr.Wrapf(err, "failed to save task summary %s: %s", task.Id, err)
	}
	latency := time.Since(task.Finished).Seconds()
	metrics2.GetFloat64SummaryMetric("autogardener_task_ingest_latency").Observe(latency)
	sklog.Infof("Ingested task %s with latency of %2f seconds", task.Id, latency)
	return taskSummary, nil
}

func (i *Ingester) classifyTaskSummary(ctx context.Context, task *ts_types.Task, taskSummary *types.TaskSummary) error {
	// Retrieve the TaskSummary from the DB to ensure that we haven't already
	// classified it (since the primary and backup queuing mechanisms may
	// overlap and enqueue the same Task[Summary] multiple times).
	if exist, err := i.db.GetTaskSummary(ctx, task.Id); err != nil {
		return skerr.Wrap(err)
	} else if exist.FailureClassId != "" {
		return nil
	}

	// TODO(borenet): We should probably do this per-repo and ensure
	// that we include the last N commits.
	windowStart := time.Now().Add(-4 * 24 * time.Hour)
	// TODO(borenet): These need to be cached, rather than hitting the DB for
	// every single failed task.
	failureClasses, err := i.db.GetRecentFailureClasses(ctx, task.Repo, windowStart, 10)
	if err != nil {
		sklog.Errorf("Failed to retrieve failure class")
	}
	classID, err := i.gemini.ClassifyFailure(ctx, taskSummary, failureClasses, task.Repo)
	if err != nil {
		return skerr.Wrapf(err, "Failed to classify failure for task %s", task.Id)
	}
	var assignedFailureClass *types.FailureClass
	if classID == "" {
		assignedFailureClass = &types.FailureClass{
			Id:           firestore.AlphaNumID(),
			ErrorMessage: taskSummary.ErrorMessage,
			Analysis:     taskSummary.Analysis,
			LastSeen:     time.Now(),
			Repo:         task.Repo,
		}
	} else {
		for _, fc := range failureClasses {
			if fc.Id == classID {
				fc.LastSeen = time.Now()
				assignedFailureClass = fc
			}
		}
	}
	if assignedFailureClass == nil {
		return skerr.Fmt("Failed to classify failure for task %s: unknown failure class with ID %q", task.Id, classID)
	}

	if err := i.db.PutFailureClass(ctx, assignedFailureClass); err != nil {
		return skerr.Wrapf(err, "failed to save failure class %s", classID)
	}
	if classID == "" {
		sklog.Infof("Successfully registered new FailureClass: %s", assignedFailureClass.Id)
	}

	// Store the task summary with the resolved FailureClassId.
	taskSummary.FailureClassId = classID
	return skerr.Wrap(i.db.PutTaskSummary(ctx, task.Id, taskSummary))
}

func (i *Ingester) getFailedTasks(ctx context.Context, repoURL string, period time.Duration) ([]*ts_types.Task, error) {
	// Retrieve tasks for all commits.
	var allTasks []*ts_types.Task
	end := time.Now()
	start := end.Add(-period)
	for _, status := range []ts_types.TaskStatus{ts_types.TASK_STATUS_FAILURE, ts_types.TASK_STATUS_MISHAP} {
		tasks, err := i.tsDB.SearchTasks(ctx, &ts_db.TaskSearchParams{
			TimeStart: &start,
			TimeEnd:   &end,
			Repo:      &repoURL,
			Status:    &status,
		})
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		allTasks = append(allTasks, tasks...)
	}
	sklog.Infof("Found %d failing tasks for %s in last %s.", len(allTasks), repoURL, period)
	return allTasks, nil
}

type taskProcessingRegistry struct {
	processing map[string]bool
	mtx        sync.Mutex
}

func newTaskProcessingRegistry() *taskProcessingRegistry {
	return &taskProcessingRegistry{
		processing: map[string]bool{},
	}
}

func (r *taskProcessingRegistry) TryClaimTask(taskID string) (func(), bool) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if r.processing[taskID] {
		return nil, false
	}
	r.processing[taskID] = true
	return func() {
		r.mtx.Lock()
		defer r.mtx.Unlock()
		delete(r.processing, taskID)
	}, true
}
