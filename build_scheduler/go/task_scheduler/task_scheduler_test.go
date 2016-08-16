package task_scheduler

import (
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/build_scheduler/go/db"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

func makeTask(name, revision string) *db.Task {
	return &db.Task{
		Commits:  []string{revision},
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

	repos := gitinfo.NewRepoMap(tr.Dir)
	_, err = repos.Repo(repo)
	assert.NoError(t, err)
	s := NewTaskScheduler(cache, time.Duration(math.MaxInt64), repos)

	// Check the initial set of task candidates. The two Build tasks
	// should be the only ones available.
	c, err := s.findTaskCandidates(commits)
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
	t1.Status = db.TASK_STATUS_PENDING
	assert.NoError(t, d.PutTask(t1))
	assert.NoError(t, cache.Update())

	c, err = s.findTaskCandidates(commits)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(c))
	for _, candidate := range c {
		assert.True(t, strings.HasPrefix(candidate.Name, "Build-"))
		assert.Equal(t, c2, candidate.Revision)
	}

	// The task failed. Ensure that its dependents are not candidates, but
	// the task itself is back in the list of candidates, in case we want
	// to retry.
	t1.Status = db.TASK_STATUS_FAILURE
	assert.NoError(t, d.PutTask(t1))
	assert.NoError(t, cache.Update())

	c, err = s.findTaskCandidates(commits)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(c))
	for _, candidate := range c {
		assert.True(t, strings.HasPrefix(candidate.Name, "Build-"))
	}

	// The task succeeded. Ensure that its dependents are candidates and
	// the task itself is not.
	t1.Status = db.TASK_STATUS_SUCCESS
	t1.IsolatedOutput = "fake isolated hash"
	assert.NoError(t, d.PutTask(t1))
	assert.NoError(t, cache.Update())

	c, err = s.findTaskCandidates(commits)
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
	t2.Status = db.TASK_STATUS_SUCCESS
	t2.IsolatedOutput = "fake isolated hash"
	assert.NoError(t, d.PutTask(t2))
	assert.NoError(t, cache.Update())

	// All test and perf tasks are now candidates, no build tasks.
	c, err = s.findTaskCandidates(commits)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(c))
	for _, candidate := range c {
		assert.True(t, !strings.HasPrefix(candidate.Name, "Build-"))
	}
}

func TestTestedness(t *testing.T) {
	tc := []struct {
		in  int
		out float64
	}{
		{
			in:  -1,
			out: -1.0,
		},
		{
			in:  0,
			out: 0.0,
		},
		{
			in:  1,
			out: 1.0,
		},
		{
			in:  2,
			out: 1.0 + 1.0/2.0,
		},
		{
			in:  3,
			out: 1.0 + float64(2.0)/float64(3.0),
		},
		{
			in:  4,
			out: 1.0 + 3.0/4.0,
		},
		{
			in:  4096,
			out: 1.0 + float64(4095)/float64(4096),
		},
	}
	for i, c := range tc {
		assert.Equal(t, c.out, testedness(c.in), fmt.Sprintf("test case #%d", i))
	}
}

func TestTestednessIncrease(t *testing.T) {
	tc := []struct {
		a   int
		b   int
		out float64
	}{
		// Invalid cases.
		{
			a:   -1,
			b:   10,
			out: -1.0,
		},
		{
			a:   10,
			b:   -1,
			out: -1.0,
		},
		{
			a:   0,
			b:   -1,
			out: -1.0,
		},
		{
			a:   0,
			b:   0,
			out: -1.0,
		},
		// Invalid because if we're re-running at already-tested commits
		// then we should have a blamelist which is at most the size of
		// the blamelist of the previous task. We naturally get negative
		// testedness increase in these cases.
		{
			a:   2,
			b:   1,
			out: -0.5,
		},
		// Testing only new commits.
		{
			a:   1,
			b:   0,
			out: 1.0 + 1.0,
		},
		{
			a:   2,
			b:   0,
			out: 2.0 + (1.0 + 1.0/2.0),
		},
		{
			a:   3,
			b:   0,
			out: 3.0 + (1.0 + float64(2.0)/float64(3.0)),
		},
		{
			a:   4096,
			b:   0,
			out: 4096.0 + (1.0 + float64(4095.0)/float64(4096.0)),
		},
		// Retries.
		{
			a:   1,
			b:   1,
			out: 0.0,
		},
		{
			a:   2,
			b:   2,
			out: 0.0,
		},
		{
			a:   3,
			b:   3,
			out: 0.0,
		},
		{
			a:   4096,
			b:   4096,
			out: 0.0,
		},
		// Bisect/backfills.
		{
			a:   1,
			b:   2,
			out: 0.5, // (1 + 1) - (1 + 1/2)
		},
		{
			a:   1,
			b:   3,
			out: float64(2.5) - (1.0 + float64(2.0)/float64(3.0)),
		},
		{
			a:   5,
			b:   10,
			out: 2.0*(1.0+float64(4.0)/float64(5.0)) - (1.0 + float64(9.0)/float64(10.0)),
		},
	}
	for i, c := range tc {
		assert.Equal(t, c.out, testednessIncrease(c.a, c.b), fmt.Sprintf("test case #%d", i))
	}
}

func TestComputeBlamelist(t *testing.T) {
	testutils.SkipIfShort(t)

	// Setup.
	_, filename, _, _ := runtime.Caller(0)
	// Use the test repo from the buildbot package, since it's already set
	// up for this type of test.
	zipfile := filepath.Join(filepath.Dir(filename), "..", "..", "..", "go", "buildbot", "testdata", "testrepo.zip")
	tr := util.NewTempRepoFrom(zipfile)
	defer tr.Cleanup()
	d := db.NewInMemoryDB()
	cache, err := db.NewTaskCache(d, time.Hour)
	assert.NoError(t, err)

	repos := gitinfo.NewRepoMap(tr.Dir)

	// The test repo is laid out like this:
	//
	// *   06eb2a58139d3ff764f10232d5c8f9362d55e20f I (HEAD, master, Task #4)
	// *   ecb424466a4f3b040586a062c15ed58356f6590e F (Task #3)
	// |\
	// | * d30286d2254716d396073c177a754f9e152bbb52 H
	// | * 8d2d1247ef5d2b8a8d3394543df6c12a85881296 G (Task #2)
	// * | 67635e7015d74b06c00154f7061987f426349d9f E
	// * | 6d4811eddfa637fac0852c3a0801b773be1f260d D (Task #1)
	// * | d74dfd42a48325ab2f3d4a97278fc283036e0ea4 C (Task #6)
	// |/
	// *   4b822ebb7cedd90acbac6a45b897438746973a87 B (Task #0)
	// *   051955c355eb742550ddde4eccc3e90b6dc5b887 A
	//
	hashes := map[rune]string{
		'A': "051955c355eb742550ddde4eccc3e90b6dc5b887",
		'B': "4b822ebb7cedd90acbac6a45b897438746973a87",
		'C': "d74dfd42a48325ab2f3d4a97278fc283036e0ea4",
		'D': "6d4811eddfa637fac0852c3a0801b773be1f260d",
		'E': "67635e7015d74b06c00154f7061987f426349d9f",
		'F': "ecb424466a4f3b040586a062c15ed58356f6590e",
		'G': "8d2d1247ef5d2b8a8d3394543df6c12a85881296",
		'H': "d30286d2254716d396073c177a754f9e152bbb52",
		'I': "06eb2a58139d3ff764f10232d5c8f9362d55e20f",
	}

	// Test cases. Each test case builds on the previous cases.
	testCases := []struct {
		Revision     string
		Expected     []string
		StoleFromIdx int
	}{
		// 0. The first task.
		{
			Revision:     hashes['B'],
			Expected:     []string{hashes['B']}, // Task #0 is limited to a single commit.
			StoleFromIdx: -1,
		},
		// 1. On a linear set of commits, with at least one previous task.
		{
			Revision:     hashes['D'],
			Expected:     []string{hashes['D'], hashes['C']},
			StoleFromIdx: -1,
		},
		// 2. The first task on a new branch.
		{
			Revision:     hashes['G'],
			Expected:     []string{hashes['G']},
			StoleFromIdx: -1,
		},
		// 3. After a merge.
		{
			Revision:     hashes['F'],
			Expected:     []string{hashes['E'], hashes['H'], hashes['F']},
			StoleFromIdx: -1,
		},
		// 4. One last "normal" task.
		{
			Revision:     hashes['I'],
			Expected:     []string{hashes['I']},
			StoleFromIdx: -1,
		},
		// 5. No Revision.
		{
			Revision:     "",
			Expected:     []string{},
			StoleFromIdx: -1,
		},
		// 6. Steal commits from a previously-ingested task.
		{
			Revision:     hashes['C'],
			Expected:     []string{hashes['C']},
			StoleFromIdx: 1,
		},
	}
	name := "Test-Ubuntu12-ShuttleA-GTX660-x86-Release"
	repo := "skia.git"
	ids := make([]string, len(testCases))
	for i, tc := range testCases {
		// Ensure that we get the expected blamelist.
		c := &taskCandidate{
			Name:     name,
			Repo:     repo,
			Revision: tc.Revision,
		}
		commits, stoleFrom, err := ComputeBlamelist(cache, repos, c)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, tc.Expected, commits)
		if tc.StoleFromIdx >= 0 {
			assert.NotNil(t, stoleFrom)
			assert.Equal(t, ids[tc.StoleFromIdx], stoleFrom.Id)
		} else {
			assert.Nil(t, stoleFrom)
		}

		// Insert the task into the DB.
		task := c.MakeTask()
		task.Commits = commits
		task.Created = time.Now()
		if stoleFrom != nil {
			// Re-insert the stoleFrom task without the commits
			// which were stolen from it.
			stoleFromCommits := make([]string, 0, len(stoleFrom.Commits)-len(commits))
			for _, commit := range stoleFrom.Commits {
				if !util.In(commit, task.Commits) {
					stoleFromCommits = append(stoleFromCommits, commit)
				}
			}
			stoleFrom.Commits = stoleFromCommits
			assert.NoError(t, d.PutTasks([]*db.Task{task, stoleFrom}))
		} else {
			assert.NoError(t, d.PutTask(task))
		}
		ids[i] = task.Id
		assert.NoError(t, cache.Update())
	}

	// Extra: ensure that task #6 really stole the commit from #1.
	task, err := cache.GetTask(ids[1])
	assert.NoError(t, err)
	assert.False(t, util.In(hashes['C'], task.Commits), fmt.Sprintf("Expected not to find %s in %v", hashes['C'], task.Commits))
}

func TestTimeDecay24Hr(t *testing.T) {
	tc := []struct {
		decayAmt24Hr float64
		elapsed      time.Duration
		out          float64
	}{
		{
			decayAmt24Hr: 1.0,
			elapsed:      10 * time.Hour,
			out:          1.0,
		},
		{
			decayAmt24Hr: 0.5,
			elapsed:      0 * time.Hour,
			out:          1.0,
		},
		{
			decayAmt24Hr: 0.5,
			elapsed:      24 * time.Hour,
			out:          0.5,
		},
		{
			decayAmt24Hr: 0.5,
			elapsed:      12 * time.Hour,
			out:          0.75,
		},
		{
			decayAmt24Hr: 0.5,
			elapsed:      36 * time.Hour,
			out:          0.25,
		},
		{
			decayAmt24Hr: 0.5,
			elapsed:      48 * time.Hour,
			out:          0.0,
		},
		{
			decayAmt24Hr: 0.5,
			elapsed:      72 * time.Hour,
			out:          0.0,
		},
	}
	for i, c := range tc {
		assert.Equal(t, c.out, timeDecay24Hr(c.decayAmt24Hr, c.elapsed), fmt.Sprintf("test case #%d", i))
	}
}

func TestRegenerateTaskQueue(t *testing.T) {
	testutils.SkipIfShort(t)

	// Setup.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	d := db.NewInMemoryDB()
	cache, err := db.NewTaskCache(d, time.Hour)
	assert.NoError(t, err)

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
	buildTask := "Build-Ubuntu-GCC-Arm7-Release-Android"
	testTask := "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
	perfTask := "Perf-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"

	// Pre-load the git repo.
	repos := gitinfo.NewRepoMap(tr.Dir)
	_, err = repos.Repo(repo)
	assert.NoError(t, err)
	s := NewTaskScheduler(cache, time.Duration(math.MaxInt64), repos)

	// Ensure that the queue is initially empty.
	assert.Equal(t, 0, len(s.queue))

	// Regenerate the task queue.
	assert.NoError(t, s.regenerateTaskQueue())
	assert.Equal(t, 2, len(s.queue)) // Two Build tasks.

	testSort := func() {
		// Ensure that we sorted correctly.
		if len(s.queue) == 0 {
			return
		}
		highScore := s.queue[0].Score
		for _, c := range s.queue {
			assert.True(t, highScore >= c.Score)
			highScore = c.Score
		}
	}
	testSort()

	// Since we haven't run any task yet, we should have the two Build
	// tasks, each with a blamelist of 1 commit (since we don't go past
	// taskCandidate.Revision when computing blamelists when we haven't run
	// a given task spec before), and a score of 2.0.
	for _, c := range s.queue {
		assert.Equal(t, buildTask, c.Name)
		assert.Equal(t, 2.0, c.Score)
		assert.Equal(t, []string{c.Revision}, c.Commits)
	}

	// Insert one of the tasks.
	var t1 *db.Task
	for _, c := range s.queue { // Order not guaranteed; find the right candidate.
		if c.Revision == c1 {
			t1 = makeTask(c.Name, c.Revision)
			break
		}
	}
	assert.NotNil(t, t1)
	t1.Status = db.TASK_STATUS_SUCCESS
	t1.IsolatedOutput = "fake isolated hash"
	assert.NoError(t, d.PutTask(t1))

	// Regenerate the task queue.
	assert.NoError(t, s.regenerateTaskQueue())

	// Now we expect the queue to contain the other Build task and the one
	// Test task we unblocked by running the first Build task.
	assert.Equal(t, 2, len(s.queue))
	testSort()
	for _, c := range s.queue {
		assert.Equal(t, 2.0, c.Score)
		assert.Equal(t, []string{c.Revision}, c.Commits)
	}
	buildIdx := 0
	testIdx := 1
	if s.queue[1].Name == buildTask {
		buildIdx = 1
		testIdx = 0
	}
	assert.Equal(t, buildTask, s.queue[buildIdx].Name)
	assert.Equal(t, c2, s.queue[buildIdx].Revision)

	assert.Equal(t, testTask, s.queue[testIdx].Name)
	assert.Equal(t, c1, s.queue[testIdx].Revision)

	// Run the other Build task.
	t2 := makeTask(s.queue[buildIdx].Name, s.queue[buildIdx].Revision)
	t2.Status = db.TASK_STATUS_SUCCESS
	t2.IsolatedOutput = "fake isolated hash"
	assert.NoError(t, d.PutTask(t2))

	// Regenerate the task queue.
	assert.NoError(t, s.regenerateTaskQueue())
	assert.Equal(t, 3, len(s.queue))
	testSort()
	perfIdx := -1
	for i, c := range s.queue {
		if c.Name == perfTask {
			perfIdx = i
		} else {
			assert.Equal(t, c.Name, testTask)
		}
		assert.Equal(t, 2.0, c.Score)
		assert.Equal(t, []string{c.Revision}, c.Commits)
	}
	assert.True(t, perfIdx > -1)

	// Run the Test task at tip of tree, but make its blamelist cover both
	// commits.
	t3 := makeTask(testTask, c2)
	t3.Commits = append(t3.Commits, c1)
	t3.Status = db.TASK_STATUS_SUCCESS
	t3.IsolatedOutput = "fake isolated hash"
	assert.NoError(t, d.PutTask(t3))

	// Regenerate the task queue.
	assert.NoError(t, s.regenerateTaskQueue())

	// Now we expect the queue to contain one Test and one Perf task. The
	// Test task is a backfill, and should have a score of 0.5.
	assert.Equal(t, 2, len(s.queue))
	testSort()
	// First candidate should be the perf task.
	assert.Equal(t, perfTask, s.queue[0].Name)
	assert.Equal(t, 2.0, s.queue[0].Score)
	// The test task is next, a backfill.
	assert.Equal(t, testTask, s.queue[1].Name)
	assert.Equal(t, 0.5, s.queue[1].Score)
}
