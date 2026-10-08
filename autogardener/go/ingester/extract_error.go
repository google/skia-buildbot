package ingester

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/autogardener/go/db"
	"go.skia.org/infra/autogardener/go/gemini"
	"go.skia.org/infra/autogardener/go/types"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/mcp/services/skia/task_details"
	td_db "go.skia.org/infra/task_driver/go/db"
	ts_types "go.skia.org/infra/task_scheduler/go/types"
)

const (
	workerPoolSize             = 10
	unfinishedTaskMaxWait      = 5 * time.Minute
	unfinishedTaskRequeueDelay = 5 * time.Second
)

// sleepFn allows overriding time.Sleep in tests.
var sleepFn = time.Sleep

type errorExtractor struct {
	db         db.AutoGardenerDB
	gemini     gemini.Client
	tdDB       td_db.DB
	logdog     task_details.LogDogClient
	processing *taskProcessingRegistry
}

func newErrorExtractor(db db.AutoGardenerDB, gemini gemini.Client, tdDB td_db.DB, logdog task_details.LogDogClient) *errorExtractor {
	return &errorExtractor{
		db:         db,
		gemini:     gemini,
		tdDB:       tdDB,
		logdog:     logdog,
		processing: newTaskProcessingRegistry(),
	}
}

func (e *errorExtractor) extractErrorMessages(ctx context.Context, input chan *ts_types.Task, output chan<- *types.TaskAndError) {
	for range workerPoolSize {
		go func() {
			for {
				select {
				case task := <-input:
					extractedErr, err := e.extractErrorMessage(ctx, task, input)
					if err != nil {
						sklog.Errorf("Failed to extract error message for task %s: %s", task.Id, err)
						continue
					}
					if extractedErr == nil || extractedErr.FailureClassId != "" {
						continue
					}
					output <- &types.TaskAndError{
						Task:  task,
						Error: extractedErr,
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	<-ctx.Done()
}

func (e *errorExtractor) extractErrorMessage(ctx context.Context, task *ts_types.Task, taskCh chan *ts_types.Task) (*types.ExtractedError, error) {
	// If another worker is already processing this task, skip it.
	release, ok := e.processing.TryClaimTask(task.Id)
	if !ok {
		return nil, nil
	}
	defer release()

	// If we already have an extracted error for this task, skip it.
	extractedErr, err := e.db.GetExtractedError(ctx, task.Id)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to ingest task %s", task.Id)
	}
	if extractedErr != nil {
		return extractedErr, nil
	}

	// Occasionally task drivers and recipes have propagation delay to the DB
	// which causes some steps and/or their logs to be missing. If this task has
	// any unfinished steps or logs, re-enqueue it after a short wait.
	isUnfinished, err := isUnfinished(ctx, e.tdDB, e.logdog, task)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if isUnfinished {
		if now.Now(ctx).Sub(task.Finished) < unfinishedTaskMaxWait {
			sklog.Infof("Task %s has unfinished steps or logs; re-enqueuing after %s", task.Id, unfinishedTaskRequeueDelay)
			go func(t *ts_types.Task) {
				sleepFn(unfinishedTaskRequeueDelay)
				taskCh <- t
			}(task)
			return nil, nil
		}
		sklog.Warningf("Task %s still has unfinished steps or logs after %s; proceeding anyway.", task.Id, now.Now(ctx).Sub(task.Finished))
	}

	// Use Gemini to extract the error message for this task and insert it
	// into the DB.
	extractedErr, err = e.gemini.ExtractErrorMessage(ctx, task)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to ingest task %s", task.Id)
	}
	if err := e.db.PutExtractedError(ctx, task.Id, extractedErr); err != nil {
		return nil, skerr.Wrapf(err, "failed to save extracted error %s: %s", task.Id, err)
	}
	latency := now.Now(ctx).Sub(task.Finished).Seconds()
	metrics2.GetFloat64SummaryMetric("autogardener_task_ingest_latency").Observe(latency)
	sklog.Infof("Ingested task %s with latency of %2f seconds", task.Id, latency)
	return extractedErr, nil
}

func isUnfinished(ctx context.Context, tdDB td_db.DB, logdog task_details.LogDogClient, task *ts_types.Task) (bool, error) {
	td, err := tdDB.GetTaskDriver(ctx, task.Id)
	if err != nil {
		return false, skerr.Wrapf(err, "failed to get task driver for %s", task.Id)
	}
	if td != nil {
		return isUnfinishedTaskDriver(td), nil
	}
	return isUnfinishedRecipe(ctx, logdog, task.SwarmingTaskId)
}

// isUnfinishedTaskDriver returns true if the given task driver run has any
// unfinished steps.
func isUnfinishedTaskDriver(td *td_db.TaskDriverRun) bool {
	if len(td.Steps) == 0 {
		return false
	}
	for _, step := range td.Steps {
		if step.Finished.IsZero() {
			return true
		}
	}
	return false
}

// isUnfinishedRecipe returns true if the given Swarming task ID is a recipe
// task whose steps or failed step logs have not finished propagating to LogDog.
func isUnfinishedRecipe(ctx context.Context, logdog task_details.LogDogClient, swarmingTaskID string) (bool, error) {
	fixedTaskID := task_details.FixupSwarmingTaskID(swarmingTaskID)
	step, finished, err := logdog.GetBuildSteps(ctx, task_details.LogDogProject, fixedTaskID)
	if err != nil {
		if strings.Contains(err.Error(), "coordinator: no access") {
			// Not a recipe task (no LogDog annotations stream).
			return false, nil
		}
		if strings.Contains(err.Error(), "no annotation entries found in stream") {
			return true, nil
		}
		return false, skerr.Wrapf(err, "failed to get build steps for %s", swarmingTaskID)
	}
	if !finished {
		return true, nil
	}
	recipeStep := task_details.ToRecipeStep(step)
	if recipeStep == nil {
		return false, nil
	}
	if !recipeStep.Finished {
		return true, nil
	}

	var checkStepLogs func(s *task_details.RecipeStep) (bool, error)
	checkStepLogs = func(s *task_details.RecipeStep) (bool, error) {
		if s == nil {
			return false, nil
		}
		if s.Status != "SUCCESS" && s.Name != "" {
			for _, logStream := range []string{s.StdoutStream, s.StderrStream} {
				if logStream == "" {
					continue
				}
				logPath := fmt.Sprintf(task_details.LogDogPathTmplStepLogs, fixedTaskID, logStream)
				_, finished, err := logdog.GetLastEntry(ctx, task_details.LogDogProject, logPath)
				if err != nil {
					if strings.Contains(err.Error(), "coordinator: no access") {
						return true, nil
					}
					return false, skerr.Wrapf(err, "failed to get last log entry for %s (%s)", swarmingTaskID, logStream)
				}
				if !finished {
					return true, nil
				}
			}
		}
		for _, sub := range s.Substeps {
			unfinished, err := checkStepLogs(sub)
			if err != nil || unfinished {
				return unfinished, err
			}
		}
		return false, nil
	}

	return checkStepLogs(recipeStep)
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
