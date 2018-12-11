package modified

import (
	"testing"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
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
