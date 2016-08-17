package db

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/luci/luci-go/common/api/swarming/swarming/v1"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
)

func TestUpdateFromSwarming(t *testing.T) {
	now := time.Now()
	task := Task{
		Id:       "A",
		Name:     "A",
		Repo:     "A",
		Revision: "A",
		Created:  now,
	}

	// Invalid input.
	assert.Error(t, task.UpdateFromSwarming(&swarming.SwarmingRpcsTaskRequestMetadata{}))
	assert.Error(t, task.UpdateFromSwarming(&swarming.SwarmingRpcsTaskRequestMetadata{
		TaskResult: &swarming.SwarmingRpcsTaskResult{
			Tags: []string{"invalid"},
		},
	}))
	// Unchanged.
	testutils.AssertDeepEqual(t, task, Task{
		Id:             "A",
		Name:           "A",
		Repo:           "A",
		Revision:       "A",
		Created:        now,
		Status:         TASK_STATUS_PENDING,
		IsolatedOutput: "",
		Swarming:       nil,
	})

	// Mismatched data.
	s := &swarming.SwarmingRpcsTaskRequestMetadata{
		TaskResult: &swarming.SwarmingRpcsTaskResult{
			CreatedTs: fmt.Sprintf("%d", now.UnixNano()),
			Failure:   false,
			State:     SWARMING_STATE_COMPLETED,
			Tags: []string{
				fmt.Sprintf("%s:B", SWARMING_TAG_ID),
				fmt.Sprintf("%s:A", SWARMING_TAG_NAME),
				fmt.Sprintf("%s:A", SWARMING_TAG_REPO),
				fmt.Sprintf("%s:A", SWARMING_TAG_REVISION),
			},
		},
	}
	assert.Error(t, task.UpdateFromSwarming(s))
	s.TaskResult.Tags[0] = fmt.Sprintf("%s:A", SWARMING_TAG_ID)
	s.TaskResult.Tags[1] = fmt.Sprintf("%s:B", SWARMING_TAG_NAME)
	assert.Error(t, task.UpdateFromSwarming(s))
	s.TaskResult.Tags[1] = fmt.Sprintf("%s:A", SWARMING_TAG_NAME)
	s.TaskResult.Tags[2] = fmt.Sprintf("%s:B", SWARMING_TAG_REPO)
	assert.Error(t, task.UpdateFromSwarming(s))
	s.TaskResult.Tags[0] = fmt.Sprintf("%s:A", SWARMING_TAG_REPO)
	s.TaskResult.Tags[1] = fmt.Sprintf("%s:B", SWARMING_TAG_REVISION)
	assert.Error(t, task.UpdateFromSwarming(s))
	s.TaskResult.Tags[1] = fmt.Sprintf("%s:A", SWARMING_TAG_REVISION)
	s.TaskResult.CreatedTs = fmt.Sprintf("%d", now.Add(time.Hour).UnixNano())
	assert.Error(t, task.UpdateFromSwarming(s))
	// Unchanged.
	testutils.AssertDeepEqual(t, task, Task{
		Id:             "A",
		Name:           "A",
		Repo:           "A",
		Revision:       "A",
		Created:        now,
		Status:         TASK_STATUS_PENDING,
		IsolatedOutput: "",
		Swarming:       nil,
	})

	// Basic update.
	task = Task{}
	s = &swarming.SwarmingRpcsTaskRequestMetadata{
		TaskResult: &swarming.SwarmingRpcsTaskResult{
			CreatedTs: fmt.Sprintf("%d", now.Add(2*time.Hour).UnixNano()),
			Failure:   false,
			State:     SWARMING_STATE_COMPLETED,
			Tags: []string{
				fmt.Sprintf("%s:C", SWARMING_TAG_ID),
				fmt.Sprintf("%s:C", SWARMING_TAG_NAME),
				fmt.Sprintf("%s:C", SWARMING_TAG_REPO),
				fmt.Sprintf("%s:C", SWARMING_TAG_REVISION),
			},
			OutputsRef: &swarming.SwarmingRpcsFilesRef{
				Isolated: "C",
			},
		},
	}
	assert.NoError(t, task.UpdateFromSwarming(s))
	testutils.AssertDeepEqual(t, task, Task{
		Id:             "C",
		Name:           "C",
		Repo:           "C",
		Revision:       "C",
		Created:        now.Add(2 * time.Hour),
		Status:         TASK_STATUS_SUCCESS,
		IsolatedOutput: "C",
		Swarming:       s,
	})

	// Status updates.

	s.TaskResult.OutputsRef = nil
	s.TaskResult.State = SWARMING_STATE_PENDING
	assert.NoError(t, task.UpdateFromSwarming(s))
	testutils.AssertDeepEqual(t, task, Task{
		Id:             "C",
		Name:           "C",
		Repo:           "C",
		Revision:       "C",
		Created:        now.Add(2 * time.Hour),
		Status:         TASK_STATUS_PENDING,
		IsolatedOutput: "",
		Swarming:       s,
	})

	s.TaskResult.State = SWARMING_STATE_RUNNING
	assert.NoError(t, task.UpdateFromSwarming(s))
	testutils.AssertDeepEqual(t, task, Task{
		Id:             "C",
		Name:           "C",
		Repo:           "C",
		Revision:       "C",
		Created:        now.Add(2 * time.Hour),
		Status:         TASK_STATUS_RUNNING,
		IsolatedOutput: "",
		Swarming:       s,
	})

	s.TaskResult.OutputsRef = &swarming.SwarmingRpcsFilesRef{
		Isolated: "",
	}
	for _, state := range []string{SWARMING_STATE_BOT_DIED, SWARMING_STATE_CANCELED, SWARMING_STATE_EXPIRED, SWARMING_STATE_TIMED_OUT} {
		s.TaskResult.State = state
		assert.NoError(t, task.UpdateFromSwarming(s))
		testutils.AssertDeepEqual(t, task, Task{
			Id:             "C",
			Name:           "C",
			Repo:           "C",
			Revision:       "C",
			Created:        now.Add(2 * time.Hour),
			Status:         TASK_STATUS_MISHAP,
			IsolatedOutput: "",
			Swarming:       s,
		})
	}

	s.TaskResult.OutputsRef.Isolated = "D"
	s.TaskResult.State = SWARMING_STATE_COMPLETED
	s.TaskResult.Failure = true
	assert.NoError(t, task.UpdateFromSwarming(s))
	testutils.AssertDeepEqual(t, task, Task{
		Id:             "C",
		Name:           "C",
		Repo:           "C",
		Revision:       "C",
		Created:        now.Add(2 * time.Hour),
		Status:         TASK_STATUS_FAILURE,
		IsolatedOutput: "D",
		Swarming:       s,
	})
}

// Test that sort.Sort(TaskSlice(...)) works correctly.
func TestSort(t *testing.T) {
	tasks := []*Task{}
	addTask := func(ts time.Time) {
		task := &Task{
			Created: ts,
		}
		tasks = append(tasks, task)
	}

	// Add tasks with various creation timestamps.
	addTask(time.Date(2008, time.August, 8, 8, 8, 8, 8, time.UTC))               // 0
	addTask(time.Date(1776, time.July, 4, 13, 0, 0, 0, time.UTC))                // 1
	addTask(time.Date(2016, time.December, 31, 23, 59, 59, 999999999, time.UTC)) // 2
	addTask(time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC))              // 3

	// Manually sort.
	expected := []*Task{tasks[1], tasks[3], tasks[0], tasks[2]}

	sort.Sort(TaskSlice(tasks))

	testutils.AssertDeepEqual(t, expected, tasks)
}

func TestTaskEncoder(t *testing.T) {
	// TODO(benjaminwagner): Is there any way to cause an error?
	e := TaskEncoder{}
	expectedTasks := map[*Task][]byte{}
	for i := 0; i < 25; i++ {
		task := &Task{}
		task.Id = fmt.Sprintf("Id-%d", i)
		task.Name = "Bingo-was-his-name-o"
		task.Commits = []string{fmt.Sprintf("a%d", i), fmt.Sprintf("b%d", i+1)}
		var buf bytes.Buffer
		err := gob.NewEncoder(&buf).Encode(task)
		assert.NoError(t, err)
		expectedTasks[task] = buf.Bytes()
		assert.True(t, e.Process(task))
	}

	actualTasks := map[*Task][]byte{}
	for task, serialized, err := e.Next(); task != nil; task, serialized, err = e.Next() {
		assert.NoError(t, err)
		actualTasks[task] = serialized
	}
	testutils.AssertDeepEqual(t, expectedTasks, actualTasks)
}

func TestTaskEncoderNoTasks(t *testing.T) {
	e := TaskEncoder{}
	task, serialized, err := e.Next()
	assert.NoError(t, err)
	assert.Nil(t, task)
	assert.Nil(t, serialized)
}

func TestTaskDecoder(t *testing.T) {
	d := TaskDecoder{}
	expectedTasks := map[string]*Task{}
	for i := 0; i < 25; i++ {
		task := &Task{}
		task.Id = fmt.Sprintf("Id-%d", i)
		task.Name = "Bingo-was-his-name-o"
		task.Commits = []string{fmt.Sprintf("a%d", i), fmt.Sprintf("b%d", i+1)}
		var buf bytes.Buffer
		err := gob.NewEncoder(&buf).Encode(task)
		assert.NoError(t, err)
		expectedTasks[task.Id] = task
		assert.True(t, d.Process(buf.Bytes()))
	}

	actualTasks := map[string]*Task{}
	result, err := d.Result()
	assert.NoError(t, err)
	assert.Equal(t, len(expectedTasks), len(result))
	for _, task := range result {
		actualTasks[task.Id] = task
	}
	testutils.AssertDeepEqual(t, expectedTasks, actualTasks)
}

func TestTaskDecoderNoTasks(t *testing.T) {
	d := TaskDecoder{}
	result, err := d.Result()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result))
}

func TestTaskDecoderError(t *testing.T) {
	task := &Task{}
	task.Id = "Id"
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(task)
	assert.NoError(t, err)
	serialized := buf.Bytes()
	invalid := append([]byte("Hi Mom!"), serialized...)

	d := TaskDecoder{}
	// Process should return true before it encounters an invalid result.
	assert.True(t, d.Process(serialized))
	assert.True(t, d.Process(serialized))
	// Process may return true or false after encountering an invalid value.
	_ = d.Process(invalid)
	_ = d.Process(serialized)

	// Result should return error.
	result, err := d.Result()
	assert.Error(t, err)
	assert.Equal(t, 0, len(result))
}
