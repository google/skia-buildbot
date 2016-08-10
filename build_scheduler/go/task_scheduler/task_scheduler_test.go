package task_scheduler

import (
	"strings"
	"testing"
	"time"

	"github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/satori/go.uuid"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/build_scheduler/go/db"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

func makeTask(name, revision string) *db.Task {
	return &db.Task{
		SwarmingRpcsTaskRequestMetadata: &swarming.SwarmingRpcsTaskRequestMetadata{
			TaskResult: &swarming.SwarmingRpcsTaskResult{},
		},
		Commits:  []string{revision},
		Id:       uuid.NewV5(uuid.NewV1(), uuid.NewV4().String()).String(),
		Name:     name,
		Revision: revision,
	}
}

func TestFindTaskCandidates(t *testing.T) {
	testutils.SkipIfShort(t)

	// Setup.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	d := db.NewInMemoryDB()
	cache, err := db.NewTaskCache(d, time.Hour)
	assert.NoError(t, err)
	s := NewTaskScheduler(cache, tr.Dir)

	// The test repo has two commits. The first commit adds a tasks.cfg file
	// with two task specs: a build task and a test task, the test task
	// depending on the build task. The second commit adds a perf task spec,
	// which also depends on the build task. Therefore, there are five total
	// possible tasks we could run:
	//
	// Build@c1, Test@c1, Build@c2, Test@c2, Perf@c2
	//
	c1 := "60f5df31760312423e635a342ab122e8117d363e"
	c2 := "71f2d15f79b7807a4d510b7b8e7c5633daae6859"
	repo := "skia.git"
	commits := map[string][]string{
		repo: []string{c1, c2},
	}

	// Check the initial set of task candidates. The two Build tasks
	// should be the only ones available.
	c, err := s.FindTaskCandidates(commits)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(c))
	for _, candidate := range c {
		assert.True(t, strings.HasPrefix(candidate.Name, "Build-"))
	}

	// Insert a the Build task at c1 (1 dependent) into the database,
	// transition through various states.
	var t1 *db.Task
	for _, candidate := range c { // Order not guaranteed; find the right candidate.
		if candidate.Revision == c1 {
			t1 = makeTask(candidate.Name, candidate.Revision)
			break
		}
	}
	assert.NotNil(t, t1)

	// We shouldn't duplicate pending tasks.
	t1.TaskResult.State = db.TASK_STATE_PENDING
	assert.NoError(t, d.PutTask(t1))
	assert.NoError(t, cache.Update())

	c, err = s.FindTaskCandidates(commits)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(c))
	for _, candidate := range c {
		assert.True(t, strings.HasPrefix(candidate.Name, "Build-"))
		assert.Equal(t, c2, candidate.Revision)
	}

	// We shouldn't duplicate running tasks.
	t1.TaskResult.State = db.TASK_STATE_RUNNING
	assert.NoError(t, d.PutTask(t1))
	assert.NoError(t, cache.Update())

	c, err = s.FindTaskCandidates(commits)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(c))
	for _, candidate := range c {
		assert.True(t, strings.HasPrefix(candidate.Name, "Build-"))
	}

	// The task failed. Ensure that its dependents are not candidates, but
	// the task itself is back in the list of candidates, in case we want
	// to retry.
	t1.TaskResult.State = db.TASK_STATE_COMPLETED
	t1.TaskResult.Failure = true
	assert.NoError(t, d.PutTask(t1))
	assert.NoError(t, cache.Update())

	c, err = s.FindTaskCandidates(commits)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(c))
	for _, candidate := range c {
		assert.True(t, strings.HasPrefix(candidate.Name, "Build-"))
	}

	// The task succeeded. Ensure that its dependents are candidates and
	// the task itself is not.
	t1.TaskResult.Failure = false
	t1.IsolatedOutput = "fake isolated hash"
	assert.NoError(t, d.PutTask(t1))
	assert.NoError(t, cache.Update())

	c, err = s.FindTaskCandidates(commits)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(c))
	for _, candidate := range c {
		assert.False(t, t1.Name == candidate.Name && t1.Revision == candidate.Revision)
	}

	// Create the other Build task.
	var t2 *db.Task
	for _, candidate := range c {
		if candidate.Revision == c2 && strings.HasPrefix(candidate.Name, "Build-") {
			t2 = makeTask(candidate.Name, candidate.Revision)
			break
		}
	}
	assert.NotNil(t, t2)
	t2.TaskResult.State = db.TASK_STATE_COMPLETED
	t2.TaskResult.Failure = false
	t2.IsolatedOutput = "fake isolated hash"
	assert.NoError(t, d.PutTask(t2))
	assert.NoError(t, cache.Update())

	// All test and perf tasks are now candidates, no build tasks.
	c, err = s.FindTaskCandidates(commits)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(c))
	for _, candidate := range c {
		assert.True(t, !strings.HasPrefix(candidate.Name, "Build-"))
	}
}
