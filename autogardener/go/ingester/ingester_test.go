package ingester

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	db_mocks "go.skia.org/infra/autogardener/go/db/mocks"
	gemini_mocks "go.skia.org/infra/autogardener/go/gemini/mocks"
	"go.skia.org/infra/autogardener/go/types"
	ts_types "go.skia.org/infra/task_scheduler/go/types"
)

func TestIngestionQueue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := newIngestionQueue(ctx)

	task1 := &ts_types.Task{
		Id: "task1",
	}
	task2 := &ts_types.Task{
		Id: "task2",
	}

	// Push task1 twice.
	q.Push(task1)
	q.Push(task1)

	// We should only get one task1.
	select {
	case popped := <-q.Pop():
		require.Equal(t, task1.Id, popped.Id)
	case <-time.After(time.Second):
		t.Fatal("Timed out waiting for task1")
	}

	// Ensure no second task1 is coming.
	select {
	case <-q.Pop():
		t.Fatal("Received duplicate task1")
	case <-time.After(100 * time.Millisecond):
		// Expected
	}

	// Push task1 again. Since it was popped, it should be re-accepted.
	q.Push(task1)
	select {
	case popped := <-q.Pop():
		require.Equal(t, task1.Id, popped.Id)
	case <-time.After(time.Second):
		t.Fatal("Timed out waiting for task1")
	}

	// Push task1 then task2.
	q.Push(task1)
	q.Push(task2)

	// Pop task1 and task2. Ensure FIFO order.
	select {
	case popped := <-q.Pop():
		require.Equal(t, "task1", popped.Id)
	case <-time.After(time.Second):
		t.Fatal("Timed out waiting for task")
	}
	select {
	case popped := <-q.Pop():
		require.Equal(t, "task2", popped.Id)
	case <-time.After(time.Second):
		t.Fatal("Timed out waiting for task")
	}

	// Concurrent test.
	const numTasks = 100
	const numPushers = 10
	var wg sync.WaitGroup
	startCh := make(chan struct{})
	for j := 0; j < numPushers; j++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			for k := 0; k < numTasks; k++ {
				q.Push(&ts_types.Task{
					Id: fmt.Sprintf("task-%d", k),
				})
			}
		}()
	}

	// Start pushers and consume tasks.
	close(startCh)
	received := map[string]bool{}
	for len(received) < numTasks {
		select {
		case popped := <-q.Pop():
			received[popped.Id] = true
		case <-time.After(5 * time.Second):
			t.Fatalf("Timed out waiting for tasks; received %d/%d", len(received), numTasks)
		}
	}

	// Wait for all pushers to finish.
	wg.Wait()

	// Drain any remaining tasks. Since pushers might have pushed more if they
	// finished after a task was popped and removed from inQueue, we just
	// make sure we don't hang.
	for {
		select {
		case <-q.Pop():
		case <-time.After(100 * time.Millisecond):
			return
		}
	}
}

func TestIngestTask(t *testing.T) {
	ctx := context.Background()
	mockDB := db_mocks.NewAutoGardenerDB(t)
	mockG := gemini_mocks.NewClient(t)
	i := &Ingester{
		db:     mockDB,
		gemini: mockG,
	}

	task := &ts_types.Task{
		Id: "task1",
	}

	// 1. Task already has a summary in the DB.
	mockDB.On("GetTaskSummary", ctx, task.Id).Return(&types.TaskSummary{}, nil).Once()
	err := i.ingestTask(ctx, task)
	require.NoError(t, err)

	// 2. Task needs to be summarized.
	summary := &types.TaskSummary{
		Analysis:     "analysis",
		ErrorMessage: "error",
	}
	mockDB.On("GetTaskSummary", ctx, task.Id).Return(nil, nil).Once()
	mockG.On("GetTaskSummary", ctx, task).Return(summary, nil).Once()
	mockDB.On("PutTaskSummary", ctx, task.Id, summary).Return(nil).Once()

	err = i.ingestTask(ctx, task)
	require.NoError(t, err)
}
