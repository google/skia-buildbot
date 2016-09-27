package db

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sort"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
)

func TestJobCopy(t *testing.T) {
	now := time.Now()
	v := &Job{
		Created:      now.Add(time.Nanosecond),
		DbModified:   now.Add(time.Millisecond),
		Dependencies: []string{"A", "B"},
		Finished:     now.Add(time.Second),
		Id:           "abc123",
		IsForce:      true,
		Name:         "C",
		Priority:     1.2,
		RepoState: RepoState{
			Repo: DEFAULT_TEST_REPO,
		},
		Status: JOB_STATUS_SUCCESS,
	}
	testutils.AssertCopy(t, v, v.Copy())
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

	testutils.AssertDeepEqual(t, expected, jobs)
}

func TestJobEncoder(t *testing.T) {
	// TODO(benjaminwagner): Is there any way to cause an error?
	e := JobEncoder{}
	expectedJobs := map[*Job][]byte{}
	for i := 0; i < 25; i++ {
		job := &Job{}
		job.Id = fmt.Sprintf("Id-%d", i)
		job.Name = "Bingo-was-his-name-o"
		job.Dependencies = []string{fmt.Sprintf("a%d", i), fmt.Sprintf("b%d", i+1)}
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
	testutils.AssertDeepEqual(t, expectedJobs, actualJobs)
}

func TestJobEncoderNoJobs(t *testing.T) {
	e := JobEncoder{}
	job, serialized, err := e.Next()
	assert.NoError(t, err)
	assert.Nil(t, job)
	assert.Nil(t, serialized)
}

func TestJobDecoder(t *testing.T) {
	d := JobDecoder{}
	expectedJobs := map[string]*Job{}
	for i := 0; i < 250; i++ {
		job := &Job{}
		job.Id = fmt.Sprintf("Id-%d", i)
		job.Name = "Bingo-was-his-name-o"
		job.Dependencies = []string{fmt.Sprintf("a%d", i), fmt.Sprintf("b%d", i+1)}
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
	testutils.AssertDeepEqual(t, expectedJobs, actualJobs)
}

func TestJobDecoderNoJobs(t *testing.T) {
	d := JobDecoder{}
	result, err := d.Result()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result))
}

func TestJobDecoderError(t *testing.T) {
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
