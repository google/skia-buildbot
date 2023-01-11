package types

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
)

func TestCopyTaskKey(t *testing.T) {
	v := TaskKey{
		RepoState: RepoState{
			Repo:     "nou.git",
			Revision: "1",
		},
		Name:        "Build",
		ForcedJobId: "123",
	}
	assertdeep.Copy(t, v, v.Copy())
}

// Test that Task.UpdateFromTaskResult returns an error when the input data is
// invalid.
func TestUpdateFromTaskResultInvalid(t *testing.T) {
	now := time.Now().UTC().Round(time.Microsecond)
	task := &Task{
		Id: "A",
		TaskKey: TaskKey{
			RepoState: RepoState{
				Repo:     "A",
				Revision: "A",
			},
			Name:        "A",
			ForcedJobId: "A",
		},
		Created: now,
		Commits: []string{"A", "B"},
	}
	copy := task.Copy()

	testError := func(s *TaskResult, msg string) {
		changed, err := task.UpdateFromTaskResult(s)
		require.False(t, changed)
		require.Error(t, err)
		require.Contains(t, err.Error(), msg)
	}

	testError(nil, "Missing TaskResult")

	testError(&TaskResult{
		Created: now,
		Status:  TASK_STATUS_SUCCESS,
		Tags: map[string][]string{
			SWARMING_TAG_NAME: {"too", "many", "values"},
		},
	}, fmt.Sprintf("Expected a single value for tag key %q", SWARMING_TAG_NAME))

	// Unchanged.
	assertdeep.Equal(t, task, copy)
}

// Test that Task.UpdateFromTaskResult returns an error when the task "identity"
// fields do not match.
func TestUpdateFromTaskResultMismatched(t *testing.T) {
	now := time.Now().UTC().Round(time.Microsecond)
	task := &Task{
		Id: "A",
		TaskKey: TaskKey{
			RepoState: RepoState{
				Repo:     "A",
				Revision: "A",
			},
			Name:        "A",
			ForcedJobId: "A",
		},
		Created:        now,
		Commits:        []string{"A", "B"},
		SwarmingTaskId: "A",
	}
	copy := task.Copy()

	testError := func(s *TaskResult, msg string) {
		changed, err := task.UpdateFromTaskResult(s)
		require.False(t, changed)
		require.Error(t, err)
		require.Contains(t, err.Error(), msg)
	}

	s := &TaskResult{
		ID:      "A",
		Created: now,
		Status:  TASK_STATUS_FAILURE,
		Tags: map[string][]string{
			SWARMING_TAG_ID:       {"B"},
			SWARMING_TAG_NAME:     {"A"},
			SWARMING_TAG_REPO:     {"A"},
			SWARMING_TAG_REVISION: {"A"},
		},
	}
	testError(s, "Id does not match")

	s.Tags[SWARMING_TAG_ID] = []string{"A"}
	s.Tags[SWARMING_TAG_NAME] = []string{"B"}
	testError(s, "Name does not match")

	s.Tags[SWARMING_TAG_NAME] = []string{"A"}
	s.Tags[SWARMING_TAG_REPO] = []string{"B"}
	testError(s, "Repo does not match")

	s.Tags[SWARMING_TAG_REPO] = []string{"A"}
	s.Tags[SWARMING_TAG_REVISION] = []string{"B"}
	testError(s, "Revision does not match")

	s.Tags[SWARMING_TAG_REVISION] = []string{"A"}
	s.Created = now.Add(time.Hour)
	testError(s, "Creation time has changed")

	s.Created = now
	s.ID = "D"
	testError(s, ErrUnknownId.Error())

	// Unchanged.
	assertdeep.Equal(t, task, copy)
}

// Test that Task.UpdateFromTaskResult sets the expected fields in an empty Task.
func TestUpdateFromTaskResultInit(t *testing.T) {
	now := time.Now().UTC().Round(time.Microsecond)
	task1 := &Task{
		SwarmingTaskId: "E",
	}
	s := &TaskResult{
		ID:       "E",
		Created:  now.Add(-3 * time.Hour),
		Finished: now.Add(-2 * time.Minute),
		Started:  now.Add(-time.Hour),
		Status:   TASK_STATUS_SUCCESS,
		Tags: map[string][]string{
			SWARMING_TAG_ID:             {"A"},
			SWARMING_TAG_NAME:           {"B"},
			SWARMING_TAG_REPO:           {"C"},
			SWARMING_TAG_REVISION:       {"D"},
			SWARMING_TAG_PARENT_TASK_ID: {"E", "F"},
			SWARMING_TAG_FORCED_JOB_ID:  {"G"},
		},
		CasOutput: "aaaabbbbccccddddaaaabbbbccccddddaaaabbbbccccddddaaaabbbbccccdddd/32",
		MachineID: "G",
	}
	changed1, err1 := task1.UpdateFromTaskResult(s)
	require.NoError(t, err1)
	require.True(t, changed1)
	assertdeep.Equal(t, task1, &Task{
		Id: "A",
		TaskKey: TaskKey{
			RepoState: RepoState{
				Repo:     "C",
				Revision: "D",
			},
			Name:        "B",
			ForcedJobId: "G",
		},
		Created:        now.Add(-3 * time.Hour),
		Commits:        nil,
		Started:        now.Add(-time.Hour),
		Finished:       now.Add(-2 * time.Minute),
		Status:         TASK_STATUS_SUCCESS,
		SwarmingTaskId: "E",
		IsolatedOutput: "aaaabbbbccccddddaaaabbbbccccddddaaaabbbbccccddddaaaabbbbccccdddd/32",
		SwarmingBotId:  "G",
		ParentTaskIds:  []string{"E", "F"},
	})
}

// Test that Task.UpdateFromTaskResult updates the expected fields in an existing
// Task.
func TestUpdateFromTaskResultUpdate(t *testing.T) {
	now := time.Now().UTC().Round(time.Microsecond)
	task := &Task{
		Id: "A",
		TaskKey: TaskKey{
			RepoState: RepoState{
				Repo:     "C",
				Revision: "D",
			},
			Name:        "B",
			ForcedJobId: "G",
		},
		Created:        now.Add(-3 * time.Hour),
		Commits:        []string{"D", "Z"},
		Started:        now.Add(-2 * time.Hour),
		Finished:       now.Add(-1 * time.Hour),
		Status:         TASK_STATUS_SUCCESS,
		SwarmingTaskId: "E",
		IsolatedOutput: "aaaabbbbccccddddaaaabbbbccccddddaaaabbbbccccddddaaaabbbbccccdddd/32",
		SwarmingBotId:  "H",
		ParentTaskIds:  []string{"E", "F"},
	}
	s := &TaskResult{
		ID: "E",
		// Include both AbandonedTs and CompletedTs to test that CompletedTs takes
		// precedence.
		Created:  now.Add(-3 * time.Hour),
		Finished: now.Add(-1 * time.Minute),
		Started:  now.Add(-2 * time.Minute),
		Status:   TASK_STATUS_FAILURE,
		Tags: map[string][]string{
			SWARMING_TAG_ID:             {"A"},
			SWARMING_TAG_NAME:           {"B"},
			SWARMING_TAG_REPO:           {"C"},
			SWARMING_TAG_REVISION:       {"D"},
			SWARMING_TAG_PARENT_TASK_ID: {"E", "F"},
			SWARMING_TAG_FORCED_JOB_ID:  {"G"},
		},
		CasOutput: "aaaabbbbccccddddaaaabbbbccccddddaaaabbbbccccddddaaaabbbbccccdddd/32",
		MachineID: "I",
	}
	changed, err := task.UpdateFromTaskResult(s)
	require.NoError(t, err)
	require.True(t, changed)
	assertdeep.Equal(t, task, &Task{
		Id: "A",
		TaskKey: TaskKey{
			RepoState: RepoState{
				Repo:     "C",
				Revision: "D",
			},
			Name:        "B",
			ForcedJobId: "G",
		},
		Created:        now.Add(-3 * time.Hour),
		Commits:        []string{"D", "Z"},
		Started:        now.Add(-2 * time.Minute),
		Finished:       now.Add(-1 * time.Minute),
		Status:         TASK_STATUS_FAILURE,
		SwarmingTaskId: "E",
		IsolatedOutput: "aaaabbbbccccddddaaaabbbbccccddddaaaabbbbccccddddaaaabbbbccccdddd/32",
		SwarmingBotId:  "I",
		ParentTaskIds:  []string{"E", "F"},
	})
}

// Test that Task.UpdateFromTaskResult updates the Status field correctly.
func TestUpdateFromTaskResultUpdateStatus(t *testing.T) {
	now := time.Now().UTC().Round(time.Microsecond)

	testUpdateStatus := func(s *TaskResult, newStatus TaskStatus) {
		task := &Task{
			Id: "A",
			TaskKey: TaskKey{
				RepoState: RepoState{
					Repo:     "C",
					Revision: "D",
				},
				Name:        "B",
				ForcedJobId: "G",
			},
			Created:        now.Add(-3 * time.Hour),
			Commits:        []string{"D", "Z"},
			Status:         TASK_STATUS_SUCCESS,
			SwarmingTaskId: "E",
			ParentTaskIds:  []string{"E", "F"},
		}
		changed, err := task.UpdateFromTaskResult(s)
		require.NoError(t, err)
		require.True(t, changed)
		assertdeep.Equal(t, task, &Task{
			Id: "A",
			TaskKey: TaskKey{
				RepoState: RepoState{
					Repo:     "C",
					Revision: "D",
				},
				Name:        "B",
				ForcedJobId: "G",
			},
			Created:        now.Add(-3 * time.Hour),
			Commits:        []string{"D", "Z"},
			Status:         newStatus,
			SwarmingTaskId: "E",
			ParentTaskIds:  []string{"E", "F"},
		})
	}

	s := &TaskResult{
		ID:      "E",
		Created: now.Add(-3 * time.Hour),
		Status:  TASK_STATUS_PENDING,
		Tags: map[string][]string{
			SWARMING_TAG_ID:             {"A"},
			SWARMING_TAG_NAME:           {"B"},
			SWARMING_TAG_REPO:           {"C"},
			SWARMING_TAG_REVISION:       {"D"},
			SWARMING_TAG_PARENT_TASK_ID: {"E", "F"},
			SWARMING_TAG_FORCED_JOB_ID:  {"G"},
		},
	}

	testUpdateStatus(s, TASK_STATUS_PENDING)

	s.Status = TASK_STATUS_RUNNING
	testUpdateStatus(s, TASK_STATUS_RUNNING)

	s.Status = TASK_STATUS_MISHAP
	testUpdateStatus(s, TASK_STATUS_MISHAP)

	s.Status = TASK_STATUS_FAILURE
	testUpdateStatus(s, TASK_STATUS_FAILURE)
}

func TestCopyTask(t *testing.T) {
	now := time.Now()
	v := &Task{
		Attempt:        3,
		Commits:        []string{"a", "b"},
		Created:        now.Add(time.Nanosecond),
		DbModified:     now.Add(time.Millisecond),
		Finished:       now.Add(time.Second),
		Id:             "42",
		IsolatedOutput: "aaaabbbbccccddddaaaabbbbccccddddaaaabbbbccccddddaaaabbbbccccdddd/32",
		Jobs:           []string{"123abc", "456def"},
		MaxAttempts:    2,
		ParentTaskIds:  []string{"38", "39", "40"},
		Properties: map[string]string{
			"color":   "blue",
			"awesome": "true",
		},
		RetryOf:        "41",
		Started:        now.Add(time.Minute),
		Status:         TASK_STATUS_MISHAP,
		SwarmingBotId:  "ENIAC",
		SwarmingTaskId: "abc123",
		TaskExecutor:   TaskExecutor_Swarming,
		TaskKey: TaskKey{
			RepoState: RepoState{
				Repo:     "nou.git",
				Revision: "1",
			},
			Name: "Build",
		},
	}
	assertdeep.Copy(t, v, v.Copy())
}

func TestValidateTask(t *testing.T) {

	test := func(task *Task, msg string) {
		err := task.Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), msg)
		require.False(t, task.Valid())
	}

	tmpl := MakeTestTask(time.Now(), []string{"a"})
	tmpl.SwarmingTaskId = ""
	tmpl.Properties = map[string]string{
		"barnDoor": "open",
	}

	// Verify success.
	err := tmpl.Validate()
	require.NoError(t, err)
	require.True(t, tmpl.Valid())
	{
		task := tmpl.Copy()
		task.MaxAttempts = 0
		err := tmpl.Validate()
		require.NoError(t, err)
		require.True(t, tmpl.Valid())
	}
	// Test invalid cases.
	{
		task := tmpl.Copy()
		task.Name = ""
		test(task, "TaskKey is not valid")
	}
	{
		task := tmpl.Copy()
		task.IsolatedOutput = "loneliness"
		test(task, "Can not specify Swarming info")
	}
	{
		task := tmpl.Copy()
		task.SwarmingBotId = "skynet"
		test(task, "Can not specify Swarming info")
	}
	{
		task := tmpl.Copy()
		task.Properties = map[string]string{
			"\xc3\x28Door": "open",
		}
		test(task, "Invalid property key")
	}
	{
		task := tmpl.Copy()
		task.Properties = map[string]string{
			"barnDoor": "\xc3\x28",
		}
		test(task, "Invalid property value")
	}
	{
		task := tmpl.Copy()
		task.MaxAttempts = 0
		task.Attempt = 1
		test(task, "Task MaxAttempts is not initialized")
	}
	{
		task := tmpl.Copy()
		task.MaxAttempts = 0
		task.Attempt = 1
		test(task, "Task MaxAttempts is not initialized")
	}
	{
		task := tmpl.Copy()
		task.Attempt = 2
		test(task, "Task Attempt 2 not less than MaxAttempts")
	}
	{
		task := tmpl.Copy()
		task.MaxAttempts = -1
		task.Attempt = -5
		test(task, "Task Attempt is negative")
	}
}

// Test that sort.Sort(TaskSlice(...)) works correctly.
func TestTaskSort(t *testing.T) {
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

	assertdeep.Equal(t, expected, tasks)
}

func TestCopyTaskSummary(t *testing.T) {
	v := &TaskSummary{
		Attempt:        1,
		Id:             "123",
		MaxAttempts:    2,
		Status:         TASK_STATUS_FAILURE,
		SwarmingTaskId: "abc123",
	}
	assertdeep.Copy(t, v, v.Copy())
}
