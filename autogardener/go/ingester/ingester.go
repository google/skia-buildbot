package ingester

import (
	"context"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/autogardener/go/db"
	"go.skia.org/infra/autogardener/go/gemini"
	"go.skia.org/infra/autogardener/go/types"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/mcp/services/skia/task_details"
	td_db "go.skia.org/infra/task_driver/go/db"
	ts_db "go.skia.org/infra/task_scheduler/go/db"
	ts_types "go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2/google"
)

const (
	getUnclassifiedExtractedErrorsBatchSize = 100
)

type Ingester struct {
	db         db.AutoGardenerDB
	gemini     gemini.Client
	tsDB       ts_db.TaskReader
	extractor  *errorExtractor
	classifier *errorClassifier
}

func New(ctx context.Context, db db.AutoGardenerDB, gemini gemini.Client, repos repograph.Map, tsDB ts_db.TaskReader, tdDB td_db.DB) (*Ingester, error) {
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, datastore.ScopeDatastore)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &Ingester{
		db:         db,
		gemini:     gemini,
		tsDB:       tsDB,
		extractor:  newErrorExtractor(db, gemini, tdDB, task_details.NewLogDogClient(ts)),
		classifier: newErrorClassifier(db, gemini),
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

func (i *Ingester) periodicUnclassifiedExtractedErrorPollingFallback(ctx context.Context, classifyCh chan<- *types.TaskAndError) {
	util.RepeatCtx(ctx, 5*time.Minute, func(ctx context.Context) {
		// TODO(borenet): This retrieves extracted errors for ALL repos.
		unclassified, err := i.db.GetUnclassifiedExtractedErrors(ctx, getUnclassifiedExtractedErrorsBatchSize)
		if err != nil {
			sklog.Errorf("Failed to retrieve unclassified extracted errors: %s", err)
			return
		}
		for taskID, extractedErr := range unclassified {
			task, err := i.tsDB.GetTaskById(ctx, taskID)
			if err != nil {
				sklog.Errorf("Failed to retrieve task from the DB: %s", err)
			}
			classifyCh <- &types.TaskAndError{
				Task:  task,
				Error: extractedErr,
			}
		}
	})
}

func (i *Ingester) StartIngestingForRepo(ctx context.Context, repoURL string, period time.Duration) {
	taskCh := make(chan *ts_types.Task)

	// Primary: receive tasks from DB as they are updated.
	go i.enqueueModifiedTasks(ctx, repoURL, taskCh)

	// Secondary: fall back to periodically loading all failed tasks, in case
	// any are missed by the above.
	go i.periodicTaskPollingFallback(ctx, repoURL, period, taskCh)

	// We classify failures sequentially to prevent the race condition in which
	// multiple task failures with the same, new, failure class are processed at
	// the same time and thus add duplicates to the DB.
	classifyCh := make(chan *types.TaskAndError, getUnclassifiedExtractedErrorsBatchSize)

	// Extract error messages in parallel.
	go i.extractor.extractErrorMessages(ctx, taskCh, classifyCh)

	// Backup: periodically fetch all unclassified extracted errors and add them
	// to the queue.
	go i.periodicUnclassifiedExtractedErrorPollingFallback(ctx, classifyCh)

	// Classify extracted errors.
	go i.classifier.classifyExtractedErrors(ctx, classifyCh)
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
