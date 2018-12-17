package modified

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
)

func TestMuxModifiedTasks(t *testing.T) {
	testutils.MediumTest(t)
	m := NewMuxModifiedTasks(&ModifiedTasksImpl{}, &ModifiedTasksImpl{})
	db.TestModifiedTasks(t, m)
}

func TestMuxMultipleTaskModifications(t *testing.T) {
	testutils.MediumTest(t)
	m := NewMuxModifiedTasks(&ModifiedTasksImpl{}, &ModifiedTasksImpl{})
	db.TestMultipleTaskModifications(t, m)
}

// Simple test to verify that we actually write to the write-only ModifiedTasks.
func TestMuxModifiedTasksWriteOnly(t *testing.T) {
	testutils.MediumTest(t)
	rw := &ModifiedTasksImpl{}
	w1 := &ModifiedTasksImpl{}
	w2 := &ModifiedTasksImpl{}
	w3 := &ModifiedTasksImpl{}
	m := NewMuxModifiedTasks(rw, w1, w2, w3)
	rwId, err := m.StartTrackingModifiedTasks()
	assert.NoError(t, err)
	wo := []db.ModifiedTasks{w1, w2, w3}
	ids := []string{}
	for _, w := range wo {
		id, err := w.StartTrackingModifiedTasks()
		assert.NoError(t, err)
		ids = append(ids, id)
	}

	check := func(expect ...*types.Task) {
		tasks, err := m.GetModifiedTasks(rwId)
		assert.NoError(t, err)
		deepequal.AssertDeepEqual(t, expect, tasks)
		for idx, w := range []db.ModifiedTasks{w1, w2, w3} {
			tasks, err := w.GetModifiedTasks(ids[idx])
			assert.NoError(t, err)
			deepequal.AssertDeepEqual(t, expect, tasks)
		}
	}
	check([]*types.Task{}...)
	t1 := types.MakeTestTask(time.Now(), []string{"a", "b"})
	t1.Id = "1"
	m.TrackModifiedTask(t1)
	check(t1)
}

func TestMuxModifiedJobs(t *testing.T) {
	testutils.MediumTest(t)
	m := NewMuxModifiedJobs(&ModifiedJobsImpl{}, &ModifiedJobsImpl{})
	db.TestModifiedJobs(t, m)
}

func TestMuxMultipleJobModifications(t *testing.T) {
	testutils.MediumTest(t)
	m := NewMuxModifiedJobs(&ModifiedJobsImpl{}, &ModifiedJobsImpl{})
	db.TestMultipleJobModifications(t, m)
}

// Simple test to verify that we actually write to the write-only ModifiedJobs.
func TestMuxModifiedJobsWriteOnly(t *testing.T) {
	testutils.MediumTest(t)
	rw := &ModifiedJobsImpl{}
	w1 := &ModifiedJobsImpl{}
	w2 := &ModifiedJobsImpl{}
	w3 := &ModifiedJobsImpl{}
	m := NewMuxModifiedJobs(rw, w1, w2, w3)
	rwId, err := m.StartTrackingModifiedJobs()
	assert.NoError(t, err)
	wo := []db.ModifiedJobs{w1, w2, w3}
	ids := []string{}
	for _, w := range wo {
		id, err := w.StartTrackingModifiedJobs()
		assert.NoError(t, err)
		ids = append(ids, id)
	}

	check := func(expect ...*types.Job) {
		jobs, err := m.GetModifiedJobs(rwId)
		assert.NoError(t, err)
		deepequal.AssertDeepEqual(t, expect, jobs)
		for idx, w := range []db.ModifiedJobs{w1, w2, w3} {
			jobs, err := w.GetModifiedJobs(ids[idx])
			assert.NoError(t, err)
			deepequal.AssertDeepEqual(t, expect, jobs)
		}
	}
	check([]*types.Job{}...)
	t1 := types.MakeTestJob(time.Now())
	t1.Id = "1"
	m.TrackModifiedJob(t1)
	check(t1)
}

func TestMuxModifiedComments(t *testing.T) {
	testutils.MediumTest(t)
	m := NewMuxModifiedComments(&ModifiedCommentsImpl{}, &ModifiedCommentsImpl{})
	db.TestModifiedComments(t, m)
}

func TestMuxMultipleCommentModifications(t *testing.T) {
	testutils.MediumTest(t)
	m := NewMuxModifiedComments(&ModifiedCommentsImpl{}, &ModifiedCommentsImpl{})
	db.TestMultipleCommentModifications(t, m)
}

// Simple test to verify that we actually write to the write-only ModifiedComments.
func TestMuxModifiedCommentsWriteOnly(t *testing.T) {
	testutils.MediumTest(t)
	rw := &ModifiedCommentsImpl{}
	w1 := &ModifiedCommentsImpl{}
	w2 := &ModifiedCommentsImpl{}
	w3 := &ModifiedCommentsImpl{}
	m := NewMuxModifiedComments(rw, w1, w2, w3)
	rwId, err := m.StartTrackingModifiedComments()
	assert.NoError(t, err)
	wo := []db.ModifiedComments{w1, w2, w3}
	ids := []string{}
	for _, w := range wo {
		id, err := w.StartTrackingModifiedComments()
		assert.NoError(t, err)
		ids = append(ids, id)
	}

	check := func(e1 []*types.TaskComment, e2 []*types.TaskSpecComment, e3 []*types.CommitComment) {
		a1, a2, a3, err := m.GetModifiedComments(rwId)
		assert.NoError(t, err)
		deepequal.AssertDeepEqual(t, e1, a1)
		deepequal.AssertDeepEqual(t, e2, a2)
		deepequal.AssertDeepEqual(t, e3, a3)
		for idx, w := range []db.ModifiedComments{w1, w2, w3} {
			a1, a2, a3, err := w.GetModifiedComments(ids[idx])
			assert.NoError(t, err)
			deepequal.AssertDeepEqual(t, e1, a1)
			deepequal.AssertDeepEqual(t, e2, a2)
			deepequal.AssertDeepEqual(t, e3, a3)
		}
	}
	check([]*types.TaskComment{}, []*types.TaskSpecComment{}, []*types.CommitComment{})
	c1 := types.MakeTaskComment(1, 1, 1, 1, time.Now())
	m.TrackModifiedTaskComment(c1)
	check([]*types.TaskComment{c1}, []*types.TaskSpecComment{}, []*types.CommitComment{})
}
