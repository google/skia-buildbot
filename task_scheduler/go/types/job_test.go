package types

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
)

func TestJobCopy(t *testing.T) {
	v := MakeFullJob(time.Now())
	assertdeep.Copy(t, v, v.Copy())
}

// Test that sort.Sort(JobSlice(...)) works correctly.
func TestJobSort(t *testing.T) {
	jobs := []*Job{}
	addJob := func(ts time.Time) {
		job := &Job{
			Created: ts,
		}
		jobs = append(jobs, job)
	}

	// Add jobs with various creation timestamps.
	addJob(time.Date(2008, time.August, 8, 8, 8, 8, 8, time.UTC))               // 0
	addJob(time.Date(1776, time.July, 4, 13, 0, 0, 0, time.UTC))                // 1
	addJob(time.Date(2016, time.December, 31, 23, 59, 59, 999999999, time.UTC)) // 2
	addJob(time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC))              // 3

	// Manually sort.
	expected := []*Job{jobs[1], jobs[3], jobs[0], jobs[2]}

	sort.Sort(JobSlice(jobs))

	assertdeep.Equal(t, expected, jobs)
}

func TestJobDeriveStatus(t *testing.T) {
	// No tasks for the Job: in progress.
	j1 := &Job{
		Dependencies: map[string][]string{"test": {"build"}, "build": {}},
		Name:         "j1",
		RepoState: RepoState{
			Repo:     "my-repo",
			Revision: "my-revision",
		},
	}
	require.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// Test empty vs nil j1.Tasks.
	j1.Tasks = map[string][]*TaskSummary{}
	require.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// Add a task for the job. It's still in progress.
	t1 := &TaskSummary{Status: TASK_STATUS_RUNNING}
	j1.Tasks = map[string][]*TaskSummary{"build": {t1}}
	require.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// Okay, it succeeded.
	t1.Status = TASK_STATUS_SUCCESS
	require.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// Or, maybe the first task failed, but we still have a retry.
	t1.Status = TASK_STATUS_FAILURE
	t1.MaxAttempts = 2
	require.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// Or maybe it was a mishap, but we still have a retry.
	t1.Status = TASK_STATUS_MISHAP
	require.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// Now a retry has been triggered.
	t2 := &TaskSummary{Status: TASK_STATUS_PENDING}
	j1.Tasks["build"] = append(j1.Tasks["build"], t2)
	require.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// Now it's running.
	t2.Status = TASK_STATUS_RUNNING
	require.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// It failed, and there aren't any retries left!
	t2.Status = TASK_STATUS_FAILURE
	require.Equal(t, j1.DeriveStatus(), JOB_STATUS_FAILURE)

	// Or it was a mishap.
	t2.Status = TASK_STATUS_MISHAP
	require.Equal(t, j1.DeriveStatus(), JOB_STATUS_MISHAP)

	// No, it succeeded.
	t2.Status = TASK_STATUS_SUCCESS
	require.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// Add the test task.
	t3 := &TaskSummary{Status: TASK_STATUS_RUNNING}
	j1.Tasks["test"] = []*TaskSummary{t3}
	require.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// It succeeded!
	t3.Status = TASK_STATUS_SUCCESS
	require.Equal(t, j1.DeriveStatus(), JOB_STATUS_SUCCESS)
}
