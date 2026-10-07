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
		Id: "class-1",
		Updates: []*autogardener_types.FailureClassUpdate{
			{
				ErrorMessage: autogardener_types.Ptr("Error 1"),
				Analysis:     autogardener_types.Ptr("Analysis 1"),
			},
		},
	}
	fc2 := &autogardener_types.FailureClass{
		Id: "class-2",
		Updates: []*autogardener_types.FailureClassUpdate{
			{
				ErrorMessage: autogardener_types.Ptr("Error 2"),
				Analysis:     autogardener_types.Ptr("Analysis 2"),
			},
		},
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
	require.Equal(t, "Analysis 1", resp[1].FailureClass.Analysis())
	require.Equal(t, []string{"task-a"}, resp[1].TaskIDs)

	// Push an updated FailureClass over ModifiedFailureClassesCh and verify the cache updates.
	fc1Updated := &autogardener_types.FailureClass{
		Id: "class-1",
		Updates: []*autogardener_types.FailureClassUpdate{
			{
				ErrorMessage: autogardener_types.Ptr("Error 1"),
				Analysis:     autogardener_types.Ptr("Updated Analysis 1"),
			},
		},
	}
	modCh <- []*autogardener_types.FailureClass{fc1Updated}
	require.Eventually(t, func() bool {
		fcCache.mtx.RLock()
		defer fcCache.mtx.RUnlock()
		return fcCache.failureClasses["class-1"].Analysis() == "Updated Analysis 1"
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
	require.Equal(t, "Updated Analysis 1", resp2[1].FailureClass.Analysis())

	// Link class-1 as DuplicateOf class-3 (which has no direct tasks in the request)
	// and verify both are returned to the client so the client can resolve or un-merge them.
	fc3 := &autogardener_types.FailureClass{
		Id: "class-3",
		Updates: []*autogardener_types.FailureClassUpdate{
			{
				ErrorMessage: autogardener_types.Ptr("Error 3"),
				Analysis:     autogardener_types.Ptr("Analysis 3"),
				Duplicates:   autogardener_types.Ptr([]string{"class-1"}),
			},
		},
	}
	fc1Merged := &autogardener_types.FailureClass{
		Id: "class-1",
		Updates: []*autogardener_types.FailureClassUpdate{
			{
				ErrorMessage: autogardener_types.Ptr("Error 1"),
				Analysis:     autogardener_types.Ptr("Updated Analysis 1"),
				DuplicateOf:  autogardener_types.Ptr("class-3"),
			},
		},
	}
	modCh <- []*autogardener_types.FailureClass{fc1Merged}
	require.Eventually(t, func() bool {
		fcCache.mtx.RLock()
		defer fcCache.mtx.RUnlock()
		return fcCache.failureClasses["class-1"].DuplicateOf() == "class-3"
	}, time.Second, 10*time.Millisecond)

	mockDB.On("GetTaskSummary", testutils.AnyContext, "task-unclassified").Return(&autogardener_types.TaskSummary{
		FailureClassId: "",
	}, nil).Once()
	mockDB.On("GetFailureClass", testutils.AnyContext, "class-3").Return(fc3, nil).Once()

	req3 := httptest.NewRequest(http.MethodPost, "/json/failure-classes", bytes.NewReader(reqBody))
	rec3 := httptest.NewRecorder()
	fcCache.Handler(rec3, req3)
	require.Equal(t, http.StatusOK, rec3.Code)

	var resp3 []activeFailureClass
	require.NoError(t, json.NewDecoder(rec3.Body).Decode(&resp3))
	require.Len(t, resp3, 3)
	require.Equal(t, "class-2", resp3[0].FailureClass.Id)
	require.Equal(t, []string{"task-b", "task-c"}, resp3[0].TaskIDs)
	require.Equal(t, "class-1", resp3[1].FailureClass.Id)
	require.Equal(t, []string{"task-a"}, resp3[1].TaskIDs)
	require.Equal(t, "class-3", resp3[2].FailureClass.Id)
	require.Empty(t, resp3[2].TaskIDs)
}
