package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	stdtesting "testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autogardener/go/db/mocks"
	autogardener_types "go.skia.org/infra/autogardener/go/types"
	"go.skia.org/infra/go/testutils"
)

func TestFailureClassesHandler(t *stdtesting.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockDB := mocks.NewAutoGardenerDB(t)
	modCh := make(chan []*autogardener_types.FailureClass, 1)
	defer close(modCh)
	mockDB.On("ModifiedFailureClassesCh", testutils.AnyContext, failureClassesWindow).Return((<-chan []*autogardener_types.FailureClass)(modCh)).Once()

	fc1 := &autogardener_types.FailureClass{
		Id:           "class-1",
		ErrorMessage: "Error 1",
		Analysis:     "Analysis 1",
	}
	fc2 := &autogardener_types.FailureClass{
		Id:           "class-2",
		ErrorMessage: "Error 2",
		Analysis:     "Analysis 2",
	}

	// Pre-populate class-1 via the initial snapshot channel.
	modCh <- []*autogardener_types.FailureClass{fc1}
	fcCache := newFailureClassesCache(ctx, mockDB)
	require.Eventually(t, func() bool {
		fcCache.mtx.RLock()
		defer fcCache.mtx.RUnlock()
		return fcCache.failureClasses["class-1"] != nil
	}, time.Second, 10*time.Millisecond)

	mockDB.On("GetTaskSummary", testutils.AnyContext, "task-a").Return(&autogardener_types.TaskSummary{
		FailureClassId: "class-1",
	}, nil).Once()
	mockDB.On("GetTaskSummary", testutils.AnyContext, "task-b").Return(&autogardener_types.TaskSummary{
		FailureClassId: "class-2",
	}, nil).Once()
	mockDB.On("GetTaskSummary", testutils.AnyContext, "task-c").Return(&autogardener_types.TaskSummary{
		FailureClassId: "class-2",
	}, nil).Once()
	// Unclassified task should not be cached and should not break the response.
	mockDB.On("GetTaskSummary", testutils.AnyContext, "task-unclassified").Return(&autogardener_types.TaskSummary{
		FailureClassId: "",
	}, nil).Twice()

	// class-1 is already in the cache from ModifiedFailureClassesCh; class-2 falls back to GetFailureClass.
	mockDB.On("GetFailureClass", testutils.AnyContext, "class-2").Return(fc2, nil).Once()

	reqBody, err := json.Marshal(failureClassesRequest{
		TaskIDs: []string{"task-a", "task-b", "task-c", "task-unclassified"},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/json/failure-classes", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()
	fcCache.Handler(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp []activeFailureClass
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Len(t, resp, 2)

	// class-2 has 2 tasks, so it should come first.
	require.Equal(t, "class-2", resp[0].FailureClass.Id)
	require.Equal(t, []string{"task-b", "task-c"}, resp[0].TaskIDs)

	// class-1 has 1 task.
	require.Equal(t, "class-1", resp[1].FailureClass.Id)
	require.Equal(t, "Analysis 1", resp[1].FailureClass.Analysis)
	require.Equal(t, []string{"task-a"}, resp[1].TaskIDs)

	// Push an updated FailureClass over ModifiedFailureClassesCh and verify the cache updates.
	fc1Updated := &autogardener_types.FailureClass{
		Id:           "class-1",
		ErrorMessage: "Error 1",
		Analysis:     "Updated Analysis 1",
	}
	modCh <- []*autogardener_types.FailureClass{fc1Updated}
	require.Eventually(t, func() bool {
		fcCache.mtx.RLock()
		defer fcCache.mtx.RUnlock()
		return fcCache.failureClasses["class-1"].Analysis == "Updated Analysis 1"
	}, time.Second, 10*time.Millisecond)

	// Second request should hit the in-memory cache for classified tasks and failure classes,
	// return the updated Analysis for class-1, and only query the DB for task-unclassified.
	req2 := httptest.NewRequest(http.MethodPost, "/json/failure-classes", bytes.NewReader(reqBody))
	rec2 := httptest.NewRecorder()
	fcCache.Handler(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	var resp2 []activeFailureClass
	require.NoError(t, json.NewDecoder(rec2.Body).Decode(&resp2))
	require.Len(t, resp2, 2)
	require.Equal(t, "Updated Analysis 1", resp2[1].FailureClass.Analysis)
}
