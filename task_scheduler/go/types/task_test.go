package types

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestCopyTaskKey(t *testing.T) {
	unittest.SmallTest(t)
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

// Test that Task.UpdateFromSwarming returns an error when the input data is
// invalid.
func TestUpdateFromSwarmingInvalid(t *testing.T) {
	unittest.SmallTest(t)
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

	testError := func(s *swarming_api.SwarmingRpcsTaskResult, msg string) {
		changed, err := task.UpdateFromSwarming(s)
		require.False(t, changed)
		require.Error(t, err)
		require.Contains(t, err.Error(), msg)
	}

	testError(nil, "Missing TaskResult")

	testError(&swarming_api.SwarmingRpcsTaskResult{
		CreatedTs: now.Format(swarming.TIMESTAMP_FORMAT),
		State:     swarming.TASK_STATE_COMPLETED,
		Tags:      []string{"invalid"},
	}, "key/value pairs must take the form \"key:value\"; \"invalid\" is invalid")

	testError(&swarming_api.SwarmingRpcsTaskResult{
		CreatedTs: "20160817T142302.543490",
		State:     swarming.TASK_STATE_COMPLETED,
	}, "Unable to parse task creation time")

	testError(&swarming_api.SwarmingRpcsTaskResult{
		CreatedTs: now.Format(swarming.TIMESTAMP_FORMAT),
		State:     swarming.TASK_STATE_COMPLETED,
		StartedTs: "20160817T142302.543490",
	}, "Unable to parse StartedTs")

	testError(&swarming_api.SwarmingRpcsTaskResult{
		CreatedTs:   now.Format(swarming.TIMESTAMP_FORMAT),
		State:       swarming.TASK_STATE_COMPLETED,
		CompletedTs: "20160817T142302.543490",
	}, "Unable to parse CompletedTs")

	testError(&swarming_api.SwarmingRpcsTaskResult{
		CreatedTs:   now.Format(swarming.TIMESTAMP_FORMAT),
		State:       swarming.TASK_STATE_EXPIRED,
		AbandonedTs: "20160817T142302.543490",
	}, "Unable to parse AbandonedTs")

	// Unchanged.
	assertdeep.Equal(t, task, copy)
}

// Test that Task.UpdateFromSwarming returns an error when the task "identity"
// fields do not match.
func TestUpdateFromSwarmingMismatched(t *testing.T) {
	unittest.SmallTest(t)
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

	testError := func(s *swarming_api.SwarmingRpcsTaskResult, msg string) {
		changed, err := task.UpdateFromSwarming(s)
		require.False(t, changed)
		require.Error(t, err)
		require.Contains(t, err.Error(), msg)
	}

	s := &swarming_api.SwarmingRpcsTaskResult{
		TaskId:    "A",
		CreatedTs: now.Format(swarming.TIMESTAMP_FORMAT),
		Failure:   false,
		State:     swarming.TASK_STATE_COMPLETED,
		Tags: []string{
			fmt.Sprintf("%s:B", SWARMING_TAG_ID),
			fmt.Sprintf("%s:A", SWARMING_TAG_NAME),
			fmt.Sprintf("%s:A", SWARMING_TAG_REPO),
			fmt.Sprintf("%s:A", SWARMING_TAG_REVISION),
		},
	}
	testError(s, "Id does not match")

	s.Tags[0] = fmt.Sprintf("%s:A", SWARMING_TAG_ID)
	s.Tags[1] = fmt.Sprintf("%s:B", SWARMING_TAG_NAME)
	testError(s, "Name does not match")

	s.Tags[1] = fmt.Sprintf("%s:A", SWARMING_TAG_NAME)
	s.Tags[2] = fmt.Sprintf("%s:B", SWARMING_TAG_REPO)
	testError(s, "Repo does not match")

	s.Tags[2] = fmt.Sprintf("%s:A", SWARMING_TAG_REPO)
	s.Tags[3] = fmt.Sprintf("%s:B", SWARMING_TAG_REVISION)
	testError(s, "Revision does not match")

	s.Tags[3] = fmt.Sprintf("%s:A", SWARMING_TAG_REVISION)
	s.CreatedTs = now.Add(time.Hour).Format(swarming.TIMESTAMP_FORMAT)
	testError(s, "Creation time has changed")

	s.CreatedTs = now.Format(swarming.TIMESTAMP_FORMAT)
	s.TaskId = "D"
	testError(s, ErrUnknownId.Error())

	// Unchanged.
	assertdeep.Equal(t, task, copy)
}

// Test that Task.UpdateFromSwarming sets the expected fields in an empty Task.
func TestUpdateFromSwarmingInit(t *testing.T) {
	unittest.SmallTest(t)
	now := time.Now().UTC().Round(time.Microsecond)
	task1 := &Task{
		SwarmingTaskId: "E",
	}
	s := &swarming_api.SwarmingRpcsTaskResult{
		TaskId: "E",
		// Include both AbandonedTs and CompletedTs to test that CompletedTs takes
		// precedence.
		AbandonedTs: now.Add(-1 * time.Minute).Format(swarming.TIMESTAMP_FORMAT),
		CreatedTs:   now.Add(-3 * time.Hour).Format(swarming.TIMESTAMP_FORMAT),
		CompletedTs: now.Add(-2 * time.Minute).Format(swarming.TIMESTAMP_FORMAT),
		Failure:     false,
		StartedTs:   now.Add(-time.Hour).Format(swarming.TIMESTAMP_FORMAT),
		State:       swarming.TASK_STATE_COMPLETED,
		Tags: []string{
			fmt.Sprintf("%s:A", SWARMING_TAG_ID),
			fmt.Sprintf("%s:B", SWARMING_TAG_NAME),
			fmt.Sprintf("%s:C", SWARMING_TAG_REPO),
			fmt.Sprintf("%s:D", SWARMING_TAG_REVISION),
			fmt.Sprintf("%s:E", SWARMING_TAG_PARENT_TASK_ID),
			fmt.Sprintf("%s:F", SWARMING_TAG_PARENT_TASK_ID),
			fmt.Sprintf("%s:G", SWARMING_TAG_FORCED_JOB_ID),
		},
		CasOutputRoot: &swarming_api.SwarmingRpcsCASReference{
			Digest: &swarming_api.SwarmingRpcsDigest{
				Hash:      "F",
				SizeBytes: 42,
			},
		},
		BotId: "G",
	}
	changed1, err1 := task1.UpdateFromSwarming(s)
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
		IsolatedOutput: "F/42",
		SwarmingBotId:  "G",
		ParentTaskIds:  []string{"E", "F"},
	})

	// Repeat to get Finished from AbandonedTs.
	task2 := &Task{
		SwarmingTaskId: "E",
	}
	s.CompletedTs = ""
	s.State = swarming.TASK_STATE_EXPIRED
	changed2, err2 := task2.UpdateFromSwarming(s)
	require.NoError(t, err2)
	require.True(t, changed2)
	assertdeep.Equal(t, task2, &Task{
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
		Finished:       now.Add(-time.Minute),
		Status:         TASK_STATUS_MISHAP,
		SwarmingTaskId: "E",
		IsolatedOutput: "F/42",
		SwarmingBotId:  "G",
		ParentTaskIds:  []string{"E", "F"},
	})
}

// Test that Task.UpdateFromSwarming updates the expected fields in an existing
// Task.
func TestUpdateFromSwarmingUpdate(t *testing.T) {
	unittest.SmallTest(t)
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
		IsolatedOutput: "F",
		SwarmingBotId:  "H",
		ParentTaskIds:  []string{"E", "F"},
	}
	s := &swarming_api.SwarmingRpcsTaskResult{
		TaskId: "E",
		// Include both AbandonedTs and CompletedTs to test that CompletedTs takes
		// precedence.
		AbandonedTs: now.Add(-90 * time.Second).Format(swarming.TIMESTAMP_FORMAT),
		CreatedTs:   now.Add(-3 * time.Hour).Format(swarming.TIMESTAMP_FORMAT),
		CompletedTs: now.Add(-1 * time.Minute).Format(swarming.TIMESTAMP_FORMAT),
		Failure:     true,
		StartedTs:   now.Add(-2 * time.Minute).Format(swarming.TIMESTAMP_FORMAT),
		State:       swarming.TASK_STATE_COMPLETED,
		Tags: []string{
			fmt.Sprintf("%s:A", SWARMING_TAG_ID),
			fmt.Sprintf("%s:B", SWARMING_TAG_NAME),
			fmt.Sprintf("%s:C", SWARMING_TAG_REPO),
			fmt.Sprintf("%s:D", SWARMING_TAG_REVISION),
			fmt.Sprintf("%s:E", SWARMING_TAG_PARENT_TASK_ID),
			fmt.Sprintf("%s:F", SWARMING_TAG_PARENT_TASK_ID),
			fmt.Sprintf("%s:G", SWARMING_TAG_FORCED_JOB_ID),
		},
		CasOutputRoot: &swarming_api.SwarmingRpcsCASReference{
			Digest: &swarming_api.SwarmingRpcsDigest{
				Hash:      "H",
				SizeBytes: 42,
			},
		},
		BotId: "I",
	}
	changed, err := task.UpdateFromSwarming(s)
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
		IsolatedOutput: "H/42",
		SwarmingBotId:  "I",
		ParentTaskIds:  []string{"E", "F"},
	})

	// Make an unrelated change, no change to Task.
	s.ModifiedTs = now.Format(swarming.TIMESTAMP_FORMAT)
	changed, err = task.UpdateFromSwarming(s)
	require.NoError(t, err)
	require.False(t, changed)
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
		IsolatedOutput: "H/42",
		SwarmingBotId:  "I",
		ParentTaskIds:  []string{"E", "F"},
	})

	// Modify so that we get Finished from AbandonedTs.
	s.CompletedTs = ""
	s.State = swarming.TASK_STATE_EXPIRED
	changed, err = task.UpdateFromSwarming(s)
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
		Finished:       now.Add(-90 * time.Second),
		Status:         TASK_STATUS_MISHAP,
		SwarmingTaskId: "E",
		IsolatedOutput: "H/42",
		SwarmingBotId:  "I",
		ParentTaskIds:  []string{"E", "F"},
	})
}

// Test that Task.UpdateFromSwarming updates the Status field correctly.
func TestUpdateFromSwarmingUpdateStatus(t *testing.T) {
	unittest.SmallTest(t)
	now := time.Now().UTC().Round(time.Microsecond)

	testUpdateStatus := func(s *swarming_api.SwarmingRpcsTaskResult, newStatus TaskStatus) {
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
		changed, err := task.UpdateFromSwarming(s)
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

	s := &swarming_api.SwarmingRpcsTaskResult{
		TaskId:    "E",
		CreatedTs: now.Add(-3 * time.Hour).Format(swarming.TIMESTAMP_FORMAT),
		Failure:   false,
		State:     swarming.TASK_STATE_PENDING,
		Tags: []string{
			fmt.Sprintf("%s:A", SWARMING_TAG_ID),
			fmt.Sprintf("%s:B", SWARMING_TAG_NAME),
			fmt.Sprintf("%s:C", SWARMING_TAG_REPO),
			fmt.Sprintf("%s:D", SWARMING_TAG_REVISION),
			fmt.Sprintf("%s:E", SWARMING_TAG_PARENT_TASK_ID),
			fmt.Sprintf("%s:F", SWARMING_TAG_PARENT_TASK_ID),
			fmt.Sprintf("%s:G", SWARMING_TAG_FORCED_JOB_ID),
		},
	}

	testUpdateStatus(s, TASK_STATUS_PENDING)

	s.State = swarming.TASK_STATE_RUNNING
	testUpdateStatus(s, TASK_STATUS_RUNNING)

	for _, state := range []string{swarming.TASK_STATE_BOT_DIED, swarming.TASK_STATE_CANCELED, swarming.TASK_STATE_EXPIRED, swarming.TASK_STATE_TIMED_OUT} {
		s.State = state
		testUpdateStatus(s, TASK_STATUS_MISHAP)
	}

	s.State = swarming.TASK_STATE_COMPLETED
	s.Failure = true
	testUpdateStatus(s, TASK_STATUS_FAILURE)
}

func TestCopyTask(t *testing.T) {
	unittest.SmallTest(t)
	now := time.Now()
	v := &Task{
		Attempt:        3,
		Commits:        []string{"a", "b"},
		Created:        now.Add(time.Nanosecond),
		DbModified:     now.Add(time.Millisecond),
		Finished:       now.Add(time.Second),
		Id:             "42",
		IsolatedOutput: "lonely-result",
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
	unittest.SmallTest(t)

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
	unittest.SmallTest(t)
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
	unittest.SmallTest(t)
	v := &TaskSummary{
		Attempt:        1,
		Id:             "123",
		MaxAttempts:    2,
		Status:         TASK_STATUS_FAILURE,
		SwarmingTaskId: "abc123",
	}
	assertdeep.Copy(t, v, v.Copy())
}
