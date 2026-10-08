package ingester

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.chromium.org/luci/logdog/api/logpb"
	annopb "go.chromium.org/luci/luciexe/legacy/annotee/proto"
	db_mocks "go.skia.org/infra/autogardener/go/db/mocks"
	gemini_mocks "go.skia.org/infra/autogardener/go/gemini/mocks"
	"go.skia.org/infra/autogardener/go/types"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/mcp/services/skia/task_details"
	task_details_mocks "go.skia.org/infra/mcp/services/skia/task_details/mocks"
	td_db "go.skia.org/infra/task_driver/go/db"
	td_mocks "go.skia.org/infra/task_driver/go/db/mocks"
	"go.skia.org/infra/task_driver/go/td"
	ts_types "go.skia.org/infra/task_scheduler/go/types"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func TestExtractErrorMessage(t *testing.T) {
	ctx := t.Context()
	mockDB := db_mocks.NewAutoGardenerDB(t)
	mockG := gemini_mocks.NewClient(t)
	mockTDDB := td_mocks.NewDB(t)
	mockLogDog := task_details_mocks.NewLogDogClient(t)
	e := newErrorExtractor(mockDB, mockG, mockTDDB, mockLogDog)

	task := &ts_types.Task{
		Id:             "task1",
		SwarmingTaskId: "7b0cc6e0f5226b10",
		Finished:       time.Now().Add(-5 * time.Minute),
	}

	// 1. Task already has an ExtractedError in the DB.
	t.Run("already extracted", func(t *testing.T) {
		e.processing = newTaskProcessingRegistry()
		existing := &types.ExtractedError{
			ErrorMessage: "error",
			Analysis:     "analysis",
		}
		mockDB.On("GetExtractedError", ctx, task.Id).Return(existing, nil).Once()
		extractedErr, err := e.extractErrorMessage(ctx, task, nil)
		require.NoError(t, err)
		require.Equal(t, existing, extractedErr)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
		mockTDDB.AssertExpectations(t)
	})

	// 2. Task needs error extraction.
	t.Run("not yet extracted", func(t *testing.T) {
		e.processing = newTaskProcessingRegistry()
		expected := &types.ExtractedError{
			Analysis:     "analysis",
			ErrorMessage: "error",
		}
		mockDB.On("GetExtractedError", ctx, task.Id).Return(nil, nil).Once()
		mockTDDB.On("GetTaskDriver", ctx, task.Id).Return(nil, nil).Once()
		mockLogDog.On("GetBuildSteps", ctx, task_details.LogDogProject, "7b0cc6e0f5226b11").Return(nil, false, errors.New("coordinator: no access")).Once()
		mockG.On("ExtractErrorMessage", ctx, task).Return(expected, nil).Once()
		mockDB.On("PutExtractedError", ctx, task.Id, expected).Return(nil).Once()

		extractedErr, err := e.extractErrorMessage(ctx, task, nil)
		require.NoError(t, err)
		require.Equal(t, expected, extractedErr)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
		mockTDDB.AssertExpectations(t)
	})

	// 3. Task is already being processed, return nil, nil.
	t.Run("already being processed", func(t *testing.T) {
		e.processing = newTaskProcessingRegistry()
		release, ok := e.processing.TryClaimTask(task.Id)
		require.True(t, ok)
		defer release()

		extractedErr, err := e.extractErrorMessage(ctx, task, nil)
		require.NoError(t, err)
		require.Nil(t, extractedErr)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
		mockTDDB.AssertExpectations(t)
	})
}

func TestExtractErrorMessages(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mockDB := db_mocks.NewAutoGardenerDB(t)
	mockG := gemini_mocks.NewClient(t)
	mockTDDB := td_mocks.NewDB(t)
	mockLogDog := task_details_mocks.NewLogDogClient(t)
	e := newErrorExtractor(mockDB, mockG, mockTDDB, mockLogDog)

	taskErr := &ts_types.Task{
		Id: "task-err",
	}
	someError := errors.New("uh oh")
	mockDB.On("GetExtractedError", mock.Anything, taskErr.Id).Return(nil, nil).Once()
	mockTDDB.On("GetTaskDriver", mock.Anything, taskErr.Id).Return(nil, nil).Once()
	mockLogDog.On("GetBuildSteps", mock.Anything, task_details.LogDogProject, "").Return(nil, false, errors.New("coordinator: no access")).Once()
	mockG.On("ExtractErrorMessage", mock.Anything, taskErr).Return(nil, someError).Once()

	taskOK := &ts_types.Task{
		Id: "task-ok",
	}
	expected := &types.ExtractedError{
		Analysis: "analysis",
	}
	mockDB.On("GetExtractedError", mock.Anything, taskOK.Id).Return(nil, nil).Once()
	mockTDDB.On("GetTaskDriver", mock.Anything, taskOK.Id).Return(nil, nil).Once()
	mockLogDog.On("GetBuildSteps", mock.Anything, task_details.LogDogProject, "").Return(nil, false, errors.New("coordinator: no access")).Once()
	mockG.On("ExtractErrorMessage", mock.Anything, taskOK).Return(expected, nil).Once()
	mockDB.On("PutExtractedError", mock.Anything, taskOK.Id, expected).Return(nil).Once()

	inputCh := make(chan *ts_types.Task)
	outputCh := make(chan *types.TaskAndError)

	go e.extractErrorMessages(ctx, inputCh, outputCh)

	inputCh <- taskErr
	inputCh <- taskOK
	result := <-outputCh
	require.Equal(t, taskOK.Id, result.Task.Id)
	require.Equal(t, expected.Analysis, result.Error.Analysis)
}

func TestExtractErrorMessage_TaskDriverStepsCheck(t *testing.T) {
	mockTime := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	ctx := context.WithValue(t.Context(), now.ContextKey, mockTime)

	// Mock sleepFn to no-op.
	oldSleep := sleepFn
	sleepFn = func(d time.Duration) {}
	defer func() {
		sleepFn = oldSleep
	}()

	t.Run("all steps finished", func(t *testing.T) {
		mockDB := db_mocks.NewAutoGardenerDB(t)
		mockG := gemini_mocks.NewClient(t)
		mockTDDB := td_mocks.NewDB(t)

		taskID := "task-all-finished"
		mockTDDB.On("GetTaskDriver", ctx, taskID).Return(&td_db.TaskDriverRun{
			TaskId: taskID,
			Steps: map[string]*td_db.Step{
				"step-1": {
					Properties: &td.StepProperties{
						Id:   "step-1",
						Name: "Step 1",
					},
					Finished: mockTime.Add(-5 * time.Minute),
				},
			},
		}, nil).Once()

		e := newErrorExtractor(mockDB, mockG, mockTDDB, nil)

		task := &ts_types.Task{
			Id:       taskID,
			Finished: mockTime.Add(-5 * time.Minute),
		}

		expected := &types.ExtractedError{
			Analysis:     "analysis",
			ErrorMessage: "error",
		}

		mockDB.On("GetExtractedError", ctx, task.Id).Return(nil, nil).Once()
		mockG.On("ExtractErrorMessage", ctx, task).Return(expected, nil).Once()
		mockDB.On("PutExtractedError", ctx, task.Id, expected).Return(nil).Once()

		extractedErr, err := e.extractErrorMessage(ctx, task, nil)
		require.NoError(t, err)
		require.Equal(t, expected, extractedErr)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
		mockTDDB.AssertExpectations(t)
	})

	t.Run("unfinished steps re-enqueue", func(t *testing.T) {
		mockDB := db_mocks.NewAutoGardenerDB(t)
		mockG := gemini_mocks.NewClient(t)
		mockTDDB := td_mocks.NewDB(t)

		taskID := "task-unfinished"
		mockTDDB.On("GetTaskDriver", ctx, taskID).Return(&td_db.TaskDriverRun{
			TaskId: taskID,
			Steps: map[string]*td_db.Step{
				"step-1": {
					Properties: &td.StepProperties{
						Id:   "step-1",
						Name: "Step 1",
					},
				},
			},
		}, nil).Once()

		e := newErrorExtractor(mockDB, mockG, mockTDDB, nil)

		task := &ts_types.Task{
			Id:       taskID,
			Finished: mockTime.Add(-1 * time.Minute), // well within the 5 min timeout
		}

		mockDB.On("GetExtractedError", ctx, task.Id).Return(nil, nil).Once()

		taskCh := make(chan *ts_types.Task, 1)

		extractedErr, err := e.extractErrorMessage(ctx, task, taskCh)
		require.NoError(t, err)
		require.Nil(t, extractedErr)

		// Verify task was re-enqueued after sleep
		select {
		case popped := <-taskCh:
			require.Equal(t, task.Id, popped.Id)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timed out waiting for task to be re-enqueued")
		}

		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
		mockTDDB.AssertExpectations(t)
	})

	t.Run("unfinished steps exceeded timeout", func(t *testing.T) {
		mockDB := db_mocks.NewAutoGardenerDB(t)
		mockG := gemini_mocks.NewClient(t)
		mockTDDB := td_mocks.NewDB(t)

		taskID := "task-unfinished-exceeded"
		mockTDDB.On("GetTaskDriver", ctx, taskID).Return(&td_db.TaskDriverRun{
			TaskId: taskID,
			Steps: map[string]*td_db.Step{
				"step-1": {
					Properties: &td.StepProperties{
						Id:   "step-1",
						Name: "Step 1",
					},
				},
			},
		}, nil).Once()

		e := newErrorExtractor(mockDB, mockG, mockTDDB, nil)

		task := &ts_types.Task{
			Id:       taskID,
			Finished: mockTime.Add(-15 * time.Minute), // exceeded the 5 min timeout
		}

		expected := &types.ExtractedError{
			Analysis:     "analysis",
			ErrorMessage: "error",
		}

		mockDB.On("GetExtractedError", ctx, task.Id).Return(nil, nil).Once()
		mockG.On("ExtractErrorMessage", ctx, task).Return(expected, nil).Once()
		mockDB.On("PutExtractedError", ctx, task.Id, expected).Return(nil).Once()

		taskCh := make(chan *ts_types.Task, 1)

		extractedErr, err := e.extractErrorMessage(ctx, task, taskCh)
		require.NoError(t, err)
		require.Equal(t, expected, extractedErr)

		// Verify no task was re-enqueued
		select {
		case <-taskCh:
			t.Fatal("task was unexpectedly re-enqueued")
		default:
		}

		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
		mockTDDB.AssertExpectations(t)
	})

	t.Run("recipe all steps and logs finished", func(t *testing.T) {
		mockDB := db_mocks.NewAutoGardenerDB(t)
		mockG := gemini_mocks.NewClient(t)
		mockTDDB := td_mocks.NewDB(t)
		mockLogDog := task_details_mocks.NewLogDogClient(t)

		taskID := "task-recipe-finished"
		swarmingTaskID := "7b0cc6e0f5226b10"
		fixedSwarmingID := "7b0cc6e0f5226b11"
		mockTDDB.On("GetTaskDriver", ctx, taskID).Return(nil, nil).Once()

		nowTs := timestamppb.New(mockTime)
		stepProto := &annopb.Step{
			Name:   "",
			Status: annopb.Status_FAILURE,
			Ended:  nowTs,
			Substep: []*annopb.Step_Substep{
				{
					Substep: &annopb.Step_Substep_Step{
						Step: &annopb.Step{
							Name:         "symbolized nanobench",
							Status:       annopb.Status_FAILURE,
							Ended:        nowTs,
							StdoutStream: &annopb.LogdogStream{Name: "steps/symbolized_nanobench/0/stdout"},
						},
					},
				},
			},
		}
		mockLogDog.On("GetBuildSteps", ctx, task_details.LogDogProject, fixedSwarmingID).Return(stepProto, true, nil).Once()
		mockLogDog.On("GetLastEntry", ctx, task_details.LogDogProject, fixedSwarmingID+"/+/steps/symbolized_nanobench/0/stdout").Return(&logpb.LogEntry{StreamIndex: 0}, true, nil).Once()

		e := newErrorExtractor(mockDB, mockG, mockTDDB, mockLogDog)

		task := &ts_types.Task{
			Id:             taskID,
			SwarmingTaskId: swarmingTaskID,
			Finished:       mockTime.Add(-1 * time.Minute),
		}
		expected := &types.ExtractedError{
			Analysis:     "analysis",
			ErrorMessage: "error",
		}

		mockDB.On("GetExtractedError", ctx, task.Id).Return(nil, nil).Once()
		mockG.On("ExtractErrorMessage", ctx, task).Return(expected, nil).Once()
		mockDB.On("PutExtractedError", ctx, task.Id, expected).Return(nil).Once()

		extractedErr, err := e.extractErrorMessage(ctx, task, nil)
		require.NoError(t, err)
		require.Equal(t, expected, extractedErr)
	})

	t.Run("unfinished recipe step logs re-enqueue", func(t *testing.T) {
		mockDB := db_mocks.NewAutoGardenerDB(t)
		mockG := gemini_mocks.NewClient(t)
		mockTDDB := td_mocks.NewDB(t)
		mockLogDog := task_details_mocks.NewLogDogClient(t)

		taskID := "task-unfinished-recipe"
		swarmingTaskID := "7b0cc6e0f5226b10"
		fixedSwarmingID := "7b0cc6e0f5226b11"
		mockTDDB.On("GetTaskDriver", ctx, taskID).Return(nil, nil).Once()

		nowTs := timestamppb.New(mockTime)
		stepProto := &annopb.Step{
			Name:   "",
			Status: annopb.Status_FAILURE,
			Ended:  nowTs,
			Substep: []*annopb.Step_Substep{
				{
					Substep: &annopb.Step_Substep_Step{
						Step: &annopb.Step{
							Name:         "symbolized nanobench",
							Status:       annopb.Status_FAILURE,
							Ended:        nowTs,
							StdoutStream: &annopb.LogdogStream{Name: "steps/symbolized_nanobench/0/stdout"},
						},
					},
				},
			},
		}
		// Step annotations are finished, but stdout stream tail is not yet finished.
		mockLogDog.On("GetBuildSteps", ctx, task_details.LogDogProject, fixedSwarmingID).Return(stepProto, true, nil).Once()
		mockLogDog.On("GetLastEntry", ctx, task_details.LogDogProject, fixedSwarmingID+"/+/steps/symbolized_nanobench/0/stdout").Return(&logpb.LogEntry{StreamIndex: 0}, false, nil).Once()

		e := newErrorExtractor(mockDB, mockG, mockTDDB, mockLogDog)

		task := &ts_types.Task{
			Id:             taskID,
			SwarmingTaskId: swarmingTaskID,
			Finished:       mockTime.Add(-1 * time.Minute),
		}

		mockDB.On("GetExtractedError", ctx, task.Id).Return(nil, nil).Once()

		taskCh := make(chan *ts_types.Task, 1)

		extractedErr, err := e.extractErrorMessage(ctx, task, taskCh)
		require.NoError(t, err)
		require.Nil(t, extractedErr)

		select {
		case popped := <-taskCh:
			require.Equal(t, task.Id, popped.Id)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timed out waiting for task to be re-enqueued")
		}
	})

	t.Run("unfinished recipe step logs exceeded timeout", func(t *testing.T) {
		mockDB := db_mocks.NewAutoGardenerDB(t)
		mockG := gemini_mocks.NewClient(t)
		mockTDDB := td_mocks.NewDB(t)
		mockLogDog := task_details_mocks.NewLogDogClient(t)

		taskID := "task-unfinished-recipe-exceeded"
		swarmingTaskID := "7b0cc6e0f5226b10"
		fixedSwarmingID := "7b0cc6e0f5226b11"
		mockTDDB.On("GetTaskDriver", ctx, taskID).Return(nil, nil).Once()

		nowTs := timestamppb.New(mockTime)
		stepProto := &annopb.Step{
			Name:   "",
			Status: annopb.Status_FAILURE,
			Ended:  nowTs,
			Substep: []*annopb.Step_Substep{
				{
					Substep: &annopb.Step_Substep_Step{
						Step: &annopb.Step{
							Name:         "symbolized nanobench",
							Status:       annopb.Status_FAILURE,
							Ended:        nowTs,
							StdoutStream: &annopb.LogdogStream{Name: "steps/symbolized_nanobench/0/stdout"},
						},
					},
				},
			},
		}
		mockLogDog.On("GetBuildSteps", ctx, task_details.LogDogProject, fixedSwarmingID).Return(stepProto, true, nil).Once()
		mockLogDog.On("GetLastEntry", ctx, task_details.LogDogProject, fixedSwarmingID+"/+/steps/symbolized_nanobench/0/stdout").Return((*logpb.LogEntry)(nil), false, nil).Once()

		e := newErrorExtractor(mockDB, mockG, mockTDDB, mockLogDog)

		task := &ts_types.Task{
			Id:             taskID,
			SwarmingTaskId: swarmingTaskID,
			Finished:       mockTime.Add(-15 * time.Minute),
		}
		expected := &types.ExtractedError{
			Analysis:     "analysis",
			ErrorMessage: "error",
		}

		mockDB.On("GetExtractedError", ctx, task.Id).Return(nil, nil).Once()
		mockG.On("ExtractErrorMessage", ctx, task).Return(expected, nil).Once()
		mockDB.On("PutExtractedError", ctx, task.Id, expected).Return(nil).Once()

		taskCh := make(chan *ts_types.Task, 1)

		extractedErr, err := e.extractErrorMessage(ctx, task, taskCh)
		require.NoError(t, err)
		require.Equal(t, expected, extractedErr)

		select {
		case <-taskCh:
			t.Fatal("task was unexpectedly re-enqueued")
		default:
		}
	})
}
