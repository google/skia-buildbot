package task_scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/task_scheduler/go/types"
)

func TestTaskList_String(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	tasks := TaskList{
		{
			Id: "123",
			TaskKey: types.TaskKey{
				Name: "Task 1",
				RepoState: types.RepoState{
					Revision: "abcdef1234567890",
				},
			},
			Status:   types.TASK_STATUS_SUCCESS,
			Created:  now,
			Finished: now.Add(time.Minute),
		},
		{
			Id: "456",
			TaskKey: types.TaskKey{
				Name: "Task 2",
				RepoState: types.RepoState{
					Revision: "fedcba0987654321",
				},
			},
			Status:   types.TASK_STATUS_FAILURE,
			Created:  now.Add(2 * time.Minute),
			Finished: now.Add(3 * time.Minute),
		},
	}

	expected := `| ID | Name | Status | Revision | Created |
|----|------|--------|----------|---------|
| 123 | Task 1 | SUCCESS | abcdef1234567890 | 2025-01-01T12:00:00Z |
| 456 | Task 2 | FAILURE | fedcba0987654321 | 2025-01-01T12:02:00Z |
`
	assert.Equal(t, expected, tasks.String())
}

func TestTaskWrapper_String(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	task := &TaskWrapper{
		Task: &types.Task{
			Id: "123",
			TaskKey: types.TaskKey{
				Name: "Task 1",
				RepoState: types.RepoState{
					Revision: "abcdef1234567890",
				},
			},
			Status:         types.TASK_STATUS_SUCCESS,
			Created:        now,
			Started:        now.Add(10 * time.Second),
			Finished:       now.Add(time.Minute),
			SwarmingTaskId: "swarm-123",
			SwarmingBotId:  "some-bot",
		},
	}

	expected := `# Task Details

**ID:** 123
**Name:** Task 1
**Status:** SUCCESS
**Revision:** abcdef1234567890
**Created:** 2025-01-01T12:00:00Z
**Started:** 2025-01-01T12:00:10Z
**Finished:** 2025-01-01T12:01:00Z
**Swarming Task ID:** swarm-123
**Swarming Bot ID:** some-bot
`
	assert.Equal(t, expected, task.String())
}

func TestTaskHealthReport_String(t *testing.T) {
	report := &TaskHealthReport{
		Commits: []*vcsinfo.ShortCommit{
			{Hash: "0000000000000000", Subject: "Commit 2"},
			{Hash: "abcdef1234567890", Subject: "Commit 1"},
			{Hash: "1234567890abcdef", Subject: "Commit 0"},
		},
		Tasks: map[string]map[string]*types.Task{
			"Task A": {
				"abcdef1234567890": {Status: types.TASK_STATUS_SUCCESS, Id: "a1"},
				"1234567890abcdef": {Status: types.TASK_STATUS_FAILURE, Id: "a2"},
			},
			"Task B": {
				"abcdef1234567890": {Status: types.TASK_STATUS_SUCCESS, Id: "b1"},
			},
		},
	}

	expected := `| Commit  | Subject |
|---------|---------|
| 0000000 | Commit 2 |
| abcdef1 | Commit 1 |
| 1234567 | Commit 0 |


# Task Results

## Task A

| Commit  | Result  | Task ID |
|---------|---------|---------|
| 0000000 |         |         |
| abcdef1 | SUCCESS | a1 |
| 1234567 | FAILURE | a2 |

## Task B

| Commit  | Result  | Task ID |
|---------|---------|---------|
| 0000000 |         |         |
| abcdef1 | SUCCESS | b1 |
| 1234567 |         |    |

`
	assert.Equal(t, expected, report.String())
}
