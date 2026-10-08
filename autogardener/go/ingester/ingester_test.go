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
	"go.skia.org/infra/mcp/services/skia/task_details"
	task_details_mocks "go.skia.org/infra/mcp/services/skia/task_details/mocks"
	td_mocks "go.skia.org/infra/task_driver/go/db/mocks"
	ts_db "go.skia.org/infra/task_scheduler/go/db"
	ts_mocks "go.skia.org/infra/task_scheduler/go/mocks"
	ts_types "go.skia.org/infra/task_scheduler/go/types"
)

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

	modCh <- []*ts_types.Task{taskMismatchedRepo, taskRunning, taskSuccess}
	modCh <- []*ts_types.Task{taskValid}

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

func TestPeriodicUnclassifiedExtractedErrorPollingFallback(t *testing.T) {
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
	extractedErr := &types.ExtractedError{
		Analysis: "unclassified analysis",
	}

	mockDB.On("GetUnclassifiedExtractedErrors", mock.Anything, getUnclassifiedExtractedErrorsBatchSize).Return(map[string]*types.ExtractedError{
		task.Id: extractedErr,
	}, nil).Once()
	mockTSDB.On("GetTaskById", mock.Anything, task.Id).Return(task, nil).Once()

	classifyCh := make(chan *types.TaskAndError)

	go i.periodicUnclassifiedExtractedErrorPollingFallback(ctx, classifyCh)

	item := <-classifyCh
	require.Equal(t, task.Id, item.Task.Id)
	require.Equal(t, extractedErr.Analysis, item.Error.Analysis)
}

func TestStartIngestingForRepo(t *testing.T) {
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

	extractedErr := &types.ExtractedError{
		Analysis:     "test analysis",
		ErrorMessage: "test error",
	}
	failureClass := &types.FailureClass{
		Id: "failure-class-1",
		Updates: []*types.FailureClassUpdate{
			{
				// Use an error message which doesn't exactly match, to ensure that we
				// call into Gemini.
				ErrorMessage: types.Ptr("task failed with: " + extractedErr.ErrorMessage),
			},
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mockDB := db_mocks.NewAutoGardenerDB(t)
	mockG := gemini_mocks.NewClient(t)
	mockTSDB := ts_mocks.NewRemoteDB(t)

	modCh := make(chan []*ts_types.Task)
	mockTSDB.On("ModifiedTasksCh", mock.Anything).Return((<-chan []*ts_types.Task)(modCh))
	mockTSDB.On("SearchTasks", mock.Anything, mock.Anything).Return([]*ts_types.Task{}, nil).Maybe()
	mockDB.On("GetUnclassifiedExtractedErrors", mock.Anything, getUnclassifiedExtractedErrorsBatchSize).Return(map[string]*types.ExtractedError{}, nil).Maybe()

	mockTDDB := td_mocks.NewDB(t)
	mockTDDB.On("GetTaskDriver", mock.Anything, task.Id).Return(nil, nil).Once()
	mockLogDog := task_details_mocks.NewLogDogClient(t)
	mockLogDog.On("GetBuildSteps", mock.Anything, task_details.LogDogProject, "").Return(nil, false, errors.New("coordinator: no access")).Once()

	i := &Ingester{
		db:         mockDB,
		gemini:     mockG,
		tsDB:       mockTSDB,
		extractor:  newErrorExtractor(mockDB, mockG, mockTDDB, mockLogDog),
		classifier: newErrorClassifier(mockDB, mockG),
	}

	// Stage 1: Extract error message
	mockDB.On("GetExtractedError", mock.Anything, task.Id).Return(nil, nil).Once()
	mockG.On("ExtractErrorMessage", mock.Anything, task).Return(extractedErr, nil).Once()
	mockDB.On("PutExtractedError", mock.Anything, task.Id, extractedErr).Return(nil).Once()

	// Stage 2: Classify extracted error
	mockDB.On("GetExtractedError", mock.Anything, task.Id).Return(extractedErr, nil).Once()
	mockDB.On("GetRecentFailureClasses", mock.Anything, task.Repo, mock.Anything, 0).Return([]*types.FailureClass{failureClass}, nil).Once()
	mockG.On("ClassifyFailure", mock.Anything, extractedErr, []*types.FailureClass{failureClass}, task.Repo).Return(failureClass.Id, nil).Once()
	mockDB.On("PutFailureClass", mock.Anything, mock.Anything).Return(nil).Once()

	doneCh := make(chan struct{})
	mockDB.On("PutExtractedError", mock.Anything, task.Id, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		close(doneCh)
	}).Once()

	i.StartIngestingForRepo(ctx, repoURL, 24*time.Hour)

	modCh <- []*ts_types.Task{task}

	<-doneCh
	mockDB.AssertExpectations(t)
	mockG.AssertExpectations(t)
	mockTSDB.AssertExpectations(t)
}
