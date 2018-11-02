package bigtable

import (
	"context"
	"fmt"
	"testing"

	"github.com/pborman/uuid"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
)

func setup(t *testing.T) (*DB, func()) {
	testutils.LargeTest(t)
	project := "test-project"
	// The BigTable emulator persists across tests, so use a different
	// instance for each test to ensure that we start from a clean slate.
	instance := fmt.Sprintf("ts-bigtable-test-%s", uuid.New())
	assert.NoError(t, bt.InitBigtable(project, instance, TABLE_CONFIG))
	d, err := NewBigTableDB(context.Background(), project, instance, nil)
	assert.NoError(t, err)
	return d, func() {
		testutils.AssertCloses(t, d)
	}
}

func makeTestTask() *db.Task {
	return &db.Task{
		TaskKey: db.TaskKey{
			RepoState: db.RepoState{
				Repo:     common.REPO_SKIA,
				Revision: "13853a120d5a4110d98b9d81b4daae92ba6119e2",
			},
		},
	}
}

func makeTestJob() *db.Job {
	return &db.Job{
		RepoState: db.RepoState{
			Repo:     common.REPO_SKIA,
			Revision: "13853a120d5a4110d98b9d81b4daae92ba6119e2",
		},
	}
}

func TestBigTableDBAssignIDs(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()

	ctx := context.Background()

	// Tasks.
	knownTaskIds := util.StringSet{}

	// Helper function to test task ID.
	checkTask := func(expectNum int) {
		task := makeTestTask()
		expect := makeRowKeyTask(task, fmt.Sprintf(local_db.SEQUENCE_NUMBER_FORMAT, expectNum))
		assert.NoError(t, d.AssignTaskId(ctx, task))
		assert.Equal(t, expect, task.Id)
		assert.False(t, knownTaskIds[task.Id])
		knownTaskIds[task.Id] = true
	}

	// There is currently no data in the ID table, so we should end up with
	// a UUID of one.
	checkTask(1)

	// Now we'll increment the ID.
	checkTask(2)
	checkTask(3)

	// Helper function to test multiple task IDs.
	checkTasks := func(expectNums []int) {
		tasks := make([]*db.Task, 0, len(expectNums))
		expect := make([]string, 0, len(expectNums))
		for _, e := range expectNums {
			task := makeTestTask()
			tasks = append(tasks, task)
			expect = append(expect, makeRowKeyTask(task, fmt.Sprintf(local_db.SEQUENCE_NUMBER_FORMAT, e)))
		}
		assert.NoError(t, d.AssignTaskIds(ctx, tasks))
		for i, task := range tasks {
			assert.Equal(t, expect[i], task.Id)
			assert.False(t, knownTaskIds[task.Id])
			knownTaskIds[task.Id] = true
		}
	}

	// Now assign multiple IDs at a time.
	checkTasks([]int{4, 5, 6, 7, 8})
	checkTasks([]int{9, 10, 11})

	// Jobs.
	knownJobIds := util.StringSet{}

	// Helper function to test job ID.
	checkJob := func(expectNum int) {
		job := makeTestJob()
		expect := makeRowKeyJob(job, fmt.Sprintf(local_db.SEQUENCE_NUMBER_FORMAT, expectNum))
		assert.NoError(t, d.AssignJobId(ctx, job))
		assert.Equal(t, expect, job.Id)
		assert.False(t, knownJobIds[job.Id])
		knownJobIds[job.Id] = true
	}

	// There is currently no row for job IDs, so we should end up with a
	// UUID of one.
	checkJob(1)

	// Now we'll increment the ID.
	checkJob(2)
	checkJob(3)

	// Helper function to test multiple job IDs.
	checkJobs := func(expectNums []int) {
		jobs := make([]*db.Job, 0, len(expectNums))
		expect := make([]string, 0, len(expectNums))
		for _, e := range expectNums {
			job := makeTestJob()
			jobs = append(jobs, job)
			expect = append(expect, makeRowKeyJob(job, fmt.Sprintf(local_db.SEQUENCE_NUMBER_FORMAT, e)))
		}
		assert.NoError(t, d.AssignJobIds(ctx, jobs))
		for i, job := range jobs {
			assert.Equal(t, expect[i], job.Id)
			assert.False(t, knownJobIds[job.Id])
			knownJobIds[job.Id] = true
		}
	}

	// Now assign multiple IDs at a time.
	checkJobs([]int{4, 5, 6, 7, 8})
	checkJobs([]int{9, 10, 11})
}
