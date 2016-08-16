package db

import (
	"fmt"
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
