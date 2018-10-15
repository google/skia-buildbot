package db

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sort"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
)

func makeFullJob(now time.Time) *Job {
	return &Job{
		BuildbucketBuildId:  12345,
		BuildbucketLeaseKey: 987,
		Created:             now.Add(time.Nanosecond),
		DbModified:          now.Add(time.Millisecond),
		Dependencies:        map[string][]string{"A": {"B"}, "B": {}},
		Finished:            now.Add(time.Second),
		Id:                  "abc123",
		IsForce:             true,
		Name:                "C",
		Priority:            1.2,
		RepoState: RepoState{
			Repo: DEFAULT_TEST_REPO,
		},
		Status: JOB_STATUS_SUCCESS,
		Tasks: map[string][]*TaskSummary{
			"task-name": {&TaskSummary{
				Id:             "12345",
				Status:         TASK_STATUS_FAILURE,
				SwarmingTaskId: "abc123",
			}},
		},
	}
}

func TestJobCopy(t *testing.T) {
	testutils.SmallTest(t)
	v := makeFullJob(time.Now())
	deepequal.AssertCopy(t, v, v.Copy())
}

// Test that sort.Sort(JobSlice(...)) works correctly.
func TestJobSort(t *testing.T) {
	testutils.SmallTest(t)
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

	deepequal.AssertDeepEqual(t, expected, jobs)
}

func TestJobEncoder(t *testing.T) {
	testutils.SmallTest(t)
	// TODO(benjaminwagner): Is there any way to cause an error?
	e := JobEncoder{}
	expectedJobs := map[*Job][]byte{}
	for i := 0; i < 25; i++ {
		job := &Job{}
		job.Id = fmt.Sprintf("Id-%d", i)
		job.Name = "Bingo-was-his-name-o"
		job.Dependencies = map[string][]string{}
		job.Tasks = map[string][]*TaskSummary{}
		var buf bytes.Buffer
		err := gob.NewEncoder(&buf).Encode(job)
		assert.NoError(t, err)
		expectedJobs[job] = buf.Bytes()
		assert.True(t, e.Process(job))
	}

	actualJobs := map[*Job][]byte{}
	for job, serialized, err := e.Next(); job != nil; job, serialized, err = e.Next() {
		assert.NoError(t, err)
		actualJobs[job] = serialized
	}

	deepequal.AssertDeepEqual(t, expectedJobs, actualJobs)
}

func TestJobEncoderNoJobs(t *testing.T) {
	testutils.SmallTest(t)
	e := JobEncoder{}
	job, serialized, err := e.Next()
	assert.NoError(t, err)
	assert.Nil(t, job)
	assert.Nil(t, serialized)
}

func TestJobDecoder(t *testing.T) {
	testutils.SmallTest(t)
	d := JobDecoder{}
	expectedJobs := map[string]*Job{}
	for i := 0; i < 250; i++ {
		job := &Job{}
		job.Id = fmt.Sprintf("Id-%d", i)
		job.Name = "Bingo-was-his-name-o"
		job.Dependencies = map[string][]string{}
		job.Tasks = map[string][]*TaskSummary{}
		var buf bytes.Buffer
		err := gob.NewEncoder(&buf).Encode(job)
		assert.NoError(t, err)
		expectedJobs[job.Id] = job
		assert.True(t, d.Process(buf.Bytes()))
	}

	actualJobs := map[string]*Job{}
	result, err := d.Result()
	assert.NoError(t, err)
	assert.Equal(t, len(expectedJobs), len(result))
	for _, job := range result {
		actualJobs[job.Id] = job
	}
	deepequal.AssertDeepEqual(t, expectedJobs, actualJobs)
}

func TestJobDecoderNoJobs(t *testing.T) {
	testutils.SmallTest(t)
	d := JobDecoder{}
	result, err := d.Result()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result))
}

func TestJobDecoderError(t *testing.T) {
	testutils.SmallTest(t)
	job := &Job{}
	job.Id = "Id"
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(job)
	assert.NoError(t, err)
	serialized := buf.Bytes()
	invalid := append([]byte("Hi Mom!"), serialized...)

	d := JobDecoder{}
	// Process should return true before it encounters an invalid result.
	assert.True(t, d.Process(serialized))
	assert.True(t, d.Process(serialized))
	// Process may return true or false after encountering an invalid value.
	_ = d.Process(invalid)
	for i := 0; i < 250; i++ {
		_ = d.Process(serialized)
	}

	// Result should return error.
	result, err := d.Result()
	assert.Error(t, err)
	assert.Equal(t, 0, len(result))
}

func TestJobDeriveStatus(t *testing.T) {
	testutils.SmallTest(t)
	// No tasks for the Job: in progress.
	j1 := &Job{
		Dependencies: map[string][]string{"test": {"build"}, "build": {}},
		Name:         "j1",
		RepoState: RepoState{
			Repo:     "my-repo",
			Revision: "my-revision",
		},
	}
	assert.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// Test empty vs nil j1.Tasks.
	j1.Tasks = map[string][]*TaskSummary{}
	assert.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// Add a task for the job. It's still in progress.
	t1 := &TaskSummary{Status: TASK_STATUS_RUNNING}
	j1.Tasks = map[string][]*TaskSummary{"build": {t1}}
	assert.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// Okay, it succeeded.
	t1.Status = TASK_STATUS_SUCCESS
	assert.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// Or, maybe the first task failed, but we still have a retry.
	t1.Status = TASK_STATUS_FAILURE
	t1.MaxAttempts = 2
	assert.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// Or maybe it was a mishap, but we still have a retry.
	t1.Status = TASK_STATUS_MISHAP
	assert.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// Now a retry has been triggered.
	t2 := &TaskSummary{Status: TASK_STATUS_PENDING}
	j1.Tasks["build"] = append(j1.Tasks["build"], t2)
	assert.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// Now it's running.
	t2.Status = TASK_STATUS_RUNNING
	assert.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// It failed, and there aren't any retries left!
	t2.Status = TASK_STATUS_FAILURE
	assert.Equal(t, j1.DeriveStatus(), JOB_STATUS_FAILURE)

	// Or it was a mishap.
	t2.Status = TASK_STATUS_MISHAP
	assert.Equal(t, j1.DeriveStatus(), JOB_STATUS_MISHAP)

	// No, it succeeded.
	t2.Status = TASK_STATUS_SUCCESS
	assert.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// Add the test task.
	t3 := &TaskSummary{Status: TASK_STATUS_RUNNING}
	j1.Tasks["test"] = []*TaskSummary{t3}
	assert.Equal(t, j1.DeriveStatus(), JOB_STATUS_IN_PROGRESS)

	// It succeeded!
	t3.Status = TASK_STATUS_SUCCESS
	assert.Equal(t, j1.DeriveStatus(), JOB_STATUS_SUCCESS)
}
