package ingester

import (
	"context"
	"net/http"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/autogardener/go/db"
	"go.skia.org/infra/autogardener/go/gemini"
	"go.skia.org/infra/go/auth"
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

func (i *Ingester) StartIngestingTaskSummariesForRepo(ctx context.Context, repoURL string, period time.Duration) {
	queue := newIngestionQueue(ctx)

	// Primary ingestion mechanism: receive tasks from DB as they are updated.
	modCh := i.tsDB.ModifiedTasksCh(ctx)
	go func() {
		for {
			select {
			case tasks := <-modCh:
				for _, task := range tasks {
					if task.Done() && !task.Success() && task.Repo == repoURL {
						queue.Push(task)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Secondary: fall back to periodically loading all failed tasks.
	lv := metrics2.NewLiveness("liveness_autogardener_task_ingestion_fallback")
	go util.RepeatCtx(ctx, 5*time.Minute, func(ctx context.Context) {
		tasks, err := i.getFailedTasks(ctx, repoURL, period)
		if err != nil {
			sklog.Errorf("Failed to retrieve tasks: %s", err)
			return
		}
		lv.Reset()
		for _, task := range tasks {
			queue.Push(task)
		}
	})

	// Start up a worker pool to ingest the tasks.
	for range workerPoolSize {
		go func() {
			for {
				select {
				case task := <-queue.Pop():
					if err := i.ingestTask(ctx, task); err != nil {
						sklog.Errorf("Failed to ingest task: %s", err)
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

func (i *Ingester) ingestTask(ctx context.Context, task *ts_types.Task) error {
	// If we already have a summary for this task, skip it.
	taskSummary, err := i.db.GetTaskSummary(ctx, task.Id)
	if err != nil {
		return skerr.Wrapf(err, "failed to ingest task %s", task.Id)
	}
	if taskSummary != nil {
		return nil
	}
	// Use Gemini to find the error summary for this task and insert it
	// into the DB.
	taskSummary, err = i.gemini.GetTaskSummary(ctx, task)
	if err != nil {
		return skerr.Wrapf(err, "failed to ingest task %s", task.Id)
	}
	if err := i.db.PutTaskSummary(ctx, task.Id, taskSummary); err != nil {
		return skerr.Wrapf(err, "failed to ingest task %s", task.Id)
	}

	latency := time.Since(task.Finished).Seconds()
	metrics2.GetFloat64SummaryMetric("autogardener_task_ingest_latency").Observe(latency)
	sklog.Infof("Ingested task %s with latency of %2f seconds", task.Id, latency)
	return nil
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

type ingestionQueue struct {
	pushCh chan *ts_types.Task
	popCh  chan *ts_types.Task
}

func newIngestionQueue(ctx context.Context) *ingestionQueue {
	// There are two main motivations for this design:
	//
	// 1. Reducing duplicated work. We already check the DB for an existing
	//    TaskSummary before sending requests to the agent, but it still costs
	//    us a DB lookup. We want to be able to discard tasks that are already
	//    in the queue.
	// 2. Maintaining a FIFO ordering.
	//
	// Plain channels would not allow us to do either of those things. Using
	// unbuffered channels for pushing and popping combined with a buffer of
	// tasks within the queue allows us to maintain ordering, and adding a map
	// allows for efficient tracking of which tasks are in the queue.
	q := &ingestionQueue{
		pushCh: make(chan *ts_types.Task),
		popCh:  make(chan *ts_types.Task),
	}
	go func() {
		inQueue := map[string]bool{}
		var buffer []*ts_types.Task
		for {
			var next *ts_types.Task
			var popCh chan *ts_types.Task
			if len(buffer) > 0 {
				next = buffer[0]
				popCh = q.popCh
			}

			select {
			case task := <-q.pushCh:
				if !inQueue[task.Id] {
					inQueue[task.Id] = true
					buffer = append(buffer, task)
				}
			case popCh <- next:
				delete(inQueue, next.Id)
				buffer = buffer[1:]
			case <-ctx.Done():
				return
			}
		}
	}()
	return q
}

func (q *ingestionQueue) Push(task *ts_types.Task) {
	q.pushCh <- task
}

func (q *ingestionQueue) Pop() <-chan *ts_types.Task {
	return q.popCh
}
