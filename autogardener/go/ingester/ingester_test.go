package ingester

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	db_mocks "go.skia.org/infra/autogardener/go/db/mocks"
	gemini_mocks "go.skia.org/infra/autogardener/go/gemini/mocks"
	"go.skia.org/infra/autogardener/go/types"
	ts_db "go.skia.org/infra/task_scheduler/go/db"
	ts_mocks "go.skia.org/infra/task_scheduler/go/mocks"
	ts_types "go.skia.org/infra/task_scheduler/go/types"
)

func TestTaskProcessingRegistry(t *testing.T) {
	r := newTaskProcessingRegistry()

	// 1. First claim succeeds
	release1, ok := r.TryClaimTask("task1")
	require.True(t, ok)
	require.NotNil(t, release1)

	// 2. Second concurrent claim for same task ID fails
	release2, ok := r.TryClaimTask("task1")
	require.False(t, ok)
	require.Nil(t, release2)

	// 3. Releasing first claim allows claiming again
	release1()
	release3, ok := r.TryClaimTask("task1")
	require.True(t, ok)
	require.NotNil(t, release3)
	release3()
}

func TestIngestTask(t *testing.T) {
	ctx := t.Context()
	mockDB := db_mocks.NewAutoGardenerDB(t)
	mockG := gemini_mocks.NewClient(t)
	i := &Ingester{
		db:     mockDB,
		gemini: mockG,
	}

	task := &ts_types.Task{
		Id:       "task1",
		Finished: time.Now().Add(-5 * time.Minute),
	}

	// 1. Task already has a summary in the DB.
	t.Run("already ingested", func(t *testing.T) {
		registry := newTaskProcessingRegistry()
		existing := &types.TaskSummary{
			ErrorMessage: "error",
			Analysis:     "analysis",
		}
		mockDB.On("GetTaskSummary", ctx, task.Id).Return(existing, nil).Once()
		taskSummary, err := i.ingestTask(ctx, registry, task)
		require.NoError(t, err)
		require.Equal(t, existing, taskSummary)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})

	// 2. Task needs to be summarized.
	t.Run("not yet ingested", func(t *testing.T) {
		registry := newTaskProcessingRegistry()
		summary := &types.TaskSummary{
			Analysis:     "analysis",
			ErrorMessage: "error",
		}
		mockDB.On("GetTaskSummary", ctx, task.Id).Return(nil, nil).Once()
		mockG.On("GetTaskSummary", ctx, task).Return(summary, nil).Once()
		mockDB.On("PutTaskSummary", ctx, task.Id, summary).Return(nil).Once()

		taskSummary, err := i.ingestTask(ctx, registry, task)
		require.NoError(t, err)
		require.Equal(t, summary, taskSummary)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})

	// 3. Task is already being processed, return nil, nil.
	t.Run("already being processed", func(t *testing.T) {
		registry := newTaskProcessingRegistry()
		release, ok := registry.TryClaimTask(task.Id)
		require.True(t, ok)
		defer release()

		taskSummary, err := i.ingestTask(ctx, registry, task)
		require.NoError(t, err)
		require.Nil(t, taskSummary)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})
}

func TestClassifyTaskSummary(t *testing.T) {
	ctx := t.Context()

	setup := func(t *testing.T) (*db_mocks.AutoGardenerDB, *gemini_mocks.Client, *Ingester, *ts_types.Task, *types.TaskSummary) {
		mockDB := db_mocks.NewAutoGardenerDB(t)
		mockG := gemini_mocks.NewClient(t)
		i := &Ingester{
			db:     mockDB,
			gemini: mockG,
		}

		task := &ts_types.Task{
			Id: "task1",
			TaskKey: ts_types.TaskKey{
				RepoState: ts_types.RepoState{
					Repo: "my_repo",
				},
			},
		}
		taskSummary := &types.TaskSummary{
			Analysis:     "analysis",
			ErrorMessage: "Compilation failed due to missing semicolon in SkCanvas.cpp:123",
		}
		return mockDB, mockG, i, task, taskSummary
	}

	// Case 1: No previous matching failure classes exist.
	// This results in a new failure class being registered, without calling into Gemini.
	t.Run("no existing failure class", func(t *testing.T) {
		mockDB, mockG, i, task, taskSummary := setup(t)

		mockDB.On("GetTaskSummary", ctx, task.Id).Return(taskSummary, nil).Once()
		mockDB.On("GetRecentFailureClasses", mock.Anything, task.Repo, mock.Anything, 0).Return([]*types.FailureClass{}, nil).Once()
		mockDB.On("PutFailureClass", mock.Anything, mock.MatchedBy(func(fc *types.FailureClass) bool {
			return fc.Repo == task.Repo && fc.ErrorMessage == taskSummary.ErrorMessage && fc.Analysis == taskSummary.Analysis && fc.Id != ""
		})).Return(nil).Once()
		mockDB.On("PutTaskSummary", mock.Anything, task.Id, mock.MatchedBy(func(ts *types.TaskSummary) bool {
			return ts.FailureClassId != ""
		})).Return(nil).Once()

		err := i.classifyTaskSummary(ctx, task, taskSummary)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})

	// Case 2: Failure class with exactly-matching error message. No need to
	// call into Gemini.
	t.Run("has exactly-matching failure class", func(t *testing.T) {
		mockDB, mockG, i, task, taskSummary := setup(t)

		fc := &types.FailureClass{
			Id:           "class_abc",
			Repo:         task.Repo,
			ErrorMessage: taskSummary.ErrorMessage,
		}
		failureClasses := []*types.FailureClass{fc}
		mockDB.On("GetTaskSummary", ctx, task.Id).Return(taskSummary, nil).Once()
		mockDB.On("GetRecentFailureClasses", mock.Anything, task.Repo, mock.Anything, 0).Return(failureClasses, nil).Once()
		mockDB.On("PutFailureClass", mock.Anything, fc).Return(nil).Once()
		mockDB.On("PutTaskSummary", mock.Anything, task.Id, mock.MatchedBy(func(ts *types.TaskSummary) bool {
			return ts.FailureClassId == "class_abc"
		})).Return(nil).Once()

		err := i.classifyTaskSummary(ctx, task, taskSummary)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})

	// Case 3: Matching failure class exists, Gemini returns its ID.
	t.Run("gemini assigned failure class", func(t *testing.T) {
		mockDB, mockG, i, task, taskSummary := setup(t)

		fc := &types.FailureClass{
			Id:           "class_abc",
			Repo:         task.Repo,
			ErrorMessage: "task failed with: " + taskSummary.ErrorMessage,
		}
		failureClasses := []*types.FailureClass{fc}
		mockDB.On("GetTaskSummary", ctx, task.Id).Return(taskSummary, nil).Once()
		mockDB.On("GetRecentFailureClasses", mock.Anything, task.Repo, mock.Anything, 0).Return(failureClasses, nil).Once()
		mockG.On("ClassifyFailure", mock.Anything, taskSummary, failureClasses, task.Repo).Return("class_abc", nil).Once()
		mockDB.On("PutFailureClass", mock.Anything, fc).Return(nil).Once()
		mockDB.On("PutTaskSummary", mock.Anything, task.Id, mock.MatchedBy(func(ts *types.TaskSummary) bool {
			return ts.FailureClassId == "class_abc"
		})).Return(nil).Once()

		err := i.classifyTaskSummary(ctx, task, taskSummary)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})

	// Case 4: Shortcut if we already classified the TaskSummary.
	t.Run("already classified", func(t *testing.T) {
		mockDB, mockG, i, task, taskSummary := setup(t)

		alreadyClassified := &types.TaskSummary{
			ErrorMessage:   taskSummary.ErrorMessage,
			Analysis:       taskSummary.Analysis,
			FailureClassId: "some-failure-class",
		}
		mockDB.On("GetTaskSummary", ctx, task.Id).Return(alreadyClassified, nil).Once()
		err := i.classifyTaskSummary(ctx, task, taskSummary)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})

	// Case 4: Generic error message shortcuts classification to generic-unidentified-failure.
	t.Run("generic error message", func(t *testing.T) {
		mockDB, mockG, i, task, _ := setup(t)

		genericTaskSummary := &types.TaskSummary{
			Analysis:     "analysis",
			ErrorMessage: "exit status 1",
		}

		mockDB.On("GetTaskSummary", ctx, task.Id).Return(genericTaskSummary, nil).Once()
		mockDB.On("PutFailureClass", mock.Anything, mock.MatchedBy(func(fc *types.FailureClass) bool {
			return fc.Id == genericFailureClassID
		})).Return(nil).Once()
		mockDB.On("PutTaskSummary", mock.Anything, task.Id, mock.MatchedBy(func(ts *types.TaskSummary) bool {
			return ts.FailureClassId == genericFailureClassID
		})).Return(nil).Once()

		err := i.classifyTaskSummary(ctx, task, genericTaskSummary)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})
}

func TestEnqueueModifiedTasks(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mockTSDB := ts_mocks.NewRemoteDB(t)
	repoURL := "http://my-repo.git"
	i := &Ingester{
		tsDB: mockTSDB,
	}

	modCh := make(chan []*ts_types.Task)
	mockTSDB.On("ModifiedTasksCh", mock.Anything).Return((<-chan []*ts_types.Task)(modCh))

	taskCh := make(chan *ts_types.Task)

	taskValid := &ts_types.Task{
		Id:       "valid-task",
		Status:   ts_types.TASK_STATUS_FAILURE,
		Finished: time.Now().Add(-10 * time.Minute),
		TaskKey: ts_types.TaskKey{
			RepoState: ts_types.RepoState{
				Repo: repoURL,
			},
		},
	}

	taskMismatchedRepo := &ts_types.Task{
		Id:       "mismatched-repo-task",
		Status:   ts_types.TASK_STATUS_FAILURE,
		Finished: time.Now().Add(-10 * time.Minute),
		TaskKey: ts_types.TaskKey{
			RepoState: ts_types.RepoState{
				Repo: "http://other-repo.git",
			},
		},
	}

	taskRunning := &ts_types.Task{
		Id:       "running-task",
		Status:   ts_types.TASK_STATUS_RUNNING,
		Finished: time.Time{},
		TaskKey: ts_types.TaskKey{
			RepoState: ts_types.RepoState{
				Repo: repoURL,
			},
		},
	}

	taskSuccess := &ts_types.Task{
		Id:       "success-task",
		Status:   ts_types.TASK_STATUS_SUCCESS,
		Finished: time.Now().Add(-10 * time.Minute),
		TaskKey: ts_types.TaskKey{
			RepoState: ts_types.RepoState{
				Repo: repoURL,
			},
		},
	}

	go i.enqueueModifiedTasks(ctx, repoURL, taskCh)

	// Send tasks to modCh. enqueueModifiedTasks processes tasks in the order
	// that they arrive, so sending taskValid last allows us to use its arrival
	// from taskCh to signal that all other tasks have been processed as well.
	modCh <- []*ts_types.Task{taskMismatchedRepo, taskRunning, taskSuccess}
	modCh <- []*ts_types.Task{taskValid}

	// Verify only the valid task is written to taskCh
	popped := <-taskCh
	require.Equal(t, taskValid.Id, popped.Id)
}

func TestPeriodicTaskPollingFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mockDB := db_mocks.NewAutoGardenerDB(t)
	mockG := gemini_mocks.NewClient(t)
	mockTSDB := ts_mocks.NewRemoteDB(t)
	repoURL := "http://my-repo.git"
	period := 24 * time.Hour

	i := &Ingester{
		db:     mockDB,
		gemini: mockG,
		tsDB:   mockTSDB,
	}

	task := &ts_types.Task{
		Id:     "failed-task",
		Status: ts_types.TASK_STATUS_FAILURE,
	}

	// Query fallback tasks: SearchTasks is called twice (for FAILURE and MISHAP status)
	mockTSDB.On("SearchTasks", mock.Anything, mock.MatchedBy(func(p *ts_db.TaskSearchParams) bool {
		return *p.Status == ts_types.TASK_STATUS_FAILURE
	})).Return([]*ts_types.Task{task}, nil).Once()
	mockTSDB.On("SearchTasks", mock.Anything, mock.MatchedBy(func(p *ts_db.TaskSearchParams) bool {
		return *p.Status == ts_types.TASK_STATUS_MISHAP
	})).Return([]*ts_types.Task{}, nil).Once()

	taskCh := make(chan *ts_types.Task)

	go i.periodicTaskPollingFallback(ctx, repoURL, period, taskCh)

	popped := <-taskCh
	require.Equal(t, task.Id, popped.Id)
}

func TestIngestTasks(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mockDB := db_mocks.NewAutoGardenerDB(t)
	mockG := gemini_mocks.NewClient(t)
	i := &Ingester{
		db:     mockDB,
		gemini: mockG,
	}

	task1 := &ts_types.Task{
		Id: "task1",
	}
	summary := &types.TaskSummary{
		Analysis: "analysis",
	}

	mockDB.On("GetTaskSummary", mock.Anything, task1.Id).Return(nil, nil).Once()
	mockG.On("GetTaskSummary", mock.Anything, task1).Return(summary, nil).Once()
	mockDB.On("PutTaskSummary", mock.Anything, task1.Id, summary).Return(nil).Once()

	task2 := &ts_types.Task{
		Id: "task2",
	}
	someError := errors.New("uh oh")
	mockDB.On("GetTaskSummary", mock.Anything, task2.Id).Return(nil, nil).Once()
	mockG.On("GetTaskSummary", mock.Anything, task2).Return(nil, someError).Once()

	inputCh := make(chan *ts_types.Task)
	outputCh := make(chan *taskIngestionResult)

	go i.ingestTasks(ctx, inputCh, outputCh)

	inputCh <- task1
	result1 := <-outputCh
	require.NoError(t, result1.err)
	require.Equal(t, task1.Id, result1.Task.Id)
	require.Equal(t, summary.Analysis, result1.Summary.Analysis)

	inputCh <- task2
	result2 := <-outputCh
	require.ErrorContains(t, result2.err, someError.Error())
	require.Nil(t, result2.Task)
	require.Nil(t, result2.Summary)
}

func TestPeriodicUnclassifiedTaskSummaryPollingFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mockDB := db_mocks.NewAutoGardenerDB(t)
	mockTSDB := ts_mocks.NewRemoteDB(t)
	i := &Ingester{
		db:   mockDB,
		tsDB: mockTSDB,
	}

	task := &ts_types.Task{
		Id: "unclassified-task",
	}
	summary := &types.TaskSummary{
		Analysis: "unclassified analysis",
	}

	mockDB.On("GetUnclassifiedTaskSummaries", mock.Anything, getUnclassifiedTaskSummariesBatchSize).Return(map[string]*types.TaskSummary{
		task.Id: summary,
	}, nil).Once()
	mockTSDB.On("GetTaskById", mock.Anything, task.Id).Return(task, nil).Once()

	classifyCh := make(chan *types.TaskAndSummary)

	go i.periodicUnclassifiedTaskSummaryPollingFallback(ctx, classifyCh)

	item := <-classifyCh
	require.Equal(t, task.Id, item.Task.Id)
	require.Equal(t, summary.Analysis, item.Summary.Analysis)
}

func TestStartIngestingTaskSummariesForRepo(t *testing.T) {
	repoURL := "http://my-repo.git"

	task := &ts_types.Task{
		Id:       "failed-task-123",
		Status:   ts_types.TASK_STATUS_FAILURE,
		Finished: time.Now().Add(-10 * time.Minute),
		TaskKey: ts_types.TaskKey{
			RepoState: ts_types.RepoState{
				Repo: repoURL,
			},
		},
	}

	summary := &types.TaskSummary{
		Analysis:     "test analysis",
		ErrorMessage: "test error",
	}
	failureClass := &types.FailureClass{
		Id: "failure-class-1",
		// Use an error message which doesn't exactly match, to ensure that we
		// call into Gemini.
		ErrorMessage: "task failed with: " + summary.ErrorMessage,
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mockDB := db_mocks.NewAutoGardenerDB(t)
	mockG := gemini_mocks.NewClient(t)
	mockTSDB := ts_mocks.NewRemoteDB(t)

	modCh := make(chan []*ts_types.Task)
	mockTSDB.On("ModifiedTasksCh", mock.Anything).Return((<-chan []*ts_types.Task)(modCh))
	mockTSDB.On("SearchTasks", mock.Anything, mock.Anything).Return([]*ts_types.Task{}, nil).Maybe()

	// We must use .Maybe() because the backup unclassified repeat loop runs asynchronously in a concurrent
	// background goroutine immediately upon StartIngestingTaskSummariesForRepo starting.
	// It may or may not execute before the test's context is canceled and asserts expectations,
	// making its execution non-deterministic during the test run.
	mockDB.On("GetUnclassifiedTaskSummaries", mock.Anything, getUnclassifiedTaskSummariesBatchSize).Return(map[string]*types.TaskSummary{}, nil).Maybe()

	i := &Ingester{
		db:     mockDB,
		gemini: mockG,
		tsDB:   mockTSDB,
	}

	// Step 1: Ingest task summary
	mockDB.On("GetTaskSummary", mock.Anything, task.Id).Return(nil, nil).Once()
	mockG.On("GetTaskSummary", mock.Anything, task).Return(summary, nil).Once()
	mockDB.On("PutTaskSummary", mock.Anything, task.Id, summary).Return(nil).Once()

	// Step 2: Classify task summary
	mockDB.On("GetTaskSummary", mock.Anything, task.Id).Return(summary, nil).Once()
	mockDB.On("GetRecentFailureClasses", mock.Anything, task.Repo, mock.Anything, 0).Return([]*types.FailureClass{failureClass}, nil).Once()
	mockG.On("ClassifyFailure", mock.Anything, summary, []*types.FailureClass{failureClass}, task.Repo).Return(failureClass.Id, nil).Once()
	mockDB.On("PutFailureClass", mock.Anything, mock.Anything).Return(nil).Once()

	// The final call to PutTaskSummary indicates that the ingestion is
	// finished. We'll close doneCh to reflect that.
	doneCh := make(chan struct{})
	mockDB.On("PutTaskSummary", mock.Anything, task.Id, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		close(doneCh)
	}).Once()

	i.StartIngestingTaskSummariesForRepo(ctx, repoURL, 24*time.Hour)

	// Send the task to modCh to trigger ingestion
	modCh <- []*ts_types.Task{task}

	// Wait for the whole pipeline to finish
	<-doneCh // Wait for ingestion to complete.
	mockDB.AssertExpectations(t)
	mockG.AssertExpectations(t)
	mockTSDB.AssertExpectations(t)
}
