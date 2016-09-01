package scheduling

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/gitrepo"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
)

func makeTask(name, repo, revision string) *db.Task {
	return &db.Task{
		Commits:  []string{revision},
		Created:  time.Now(),
		Name:     name,
		Repo:     repo,
		Revision: revision,
	}
}

func makeSwarmingRpcsTaskRequestMetadata(t *testing.T, task *db.Task) *swarming_api.SwarmingRpcsTaskRequestMetadata {
	tag := func(k, v string) string {
		return fmt.Sprintf("%s:%s", k, v)
	}
	ts := func(t time.Time) string {
		return t.Format(swarming.TIMESTAMP_FORMAT)
	}
	abandoned := ""
	state := db.SWARMING_STATE_PENDING
	failed := false
	switch task.Status {
	case db.TASK_STATUS_MISHAP:
		state = db.SWARMING_STATE_BOT_DIED
		abandoned = ts(task.Finished)
	case db.TASK_STATUS_RUNNING:
		state = db.SWARMING_STATE_RUNNING
	case db.TASK_STATUS_FAILURE:
		state = db.SWARMING_STATE_COMPLETED
		failed = true
	case db.TASK_STATUS_SUCCESS:
		state = db.SWARMING_STATE_COMPLETED
	case db.TASK_STATUS_PENDING:
		// noop
	default:
		assert.FailNow(t, "Unknown task status: %s", task.Status)
	}
	return &swarming_api.SwarmingRpcsTaskRequestMetadata{
		Request: &swarming_api.SwarmingRpcsTaskRequest{},
		TaskId:  task.SwarmingTaskId,
		TaskResult: &swarming_api.SwarmingRpcsTaskResult{
			AbandonedTs: abandoned,
			CreatedTs:   ts(task.Created),
			CompletedTs: ts(task.Finished),
			Failure:     failed,
			OutputsRef: &swarming_api.SwarmingRpcsFilesRef{
				Isolated: "???",
			},
			StartedTs: ts(task.Started),
			State:     state,
			Tags: []string{
				tag(db.SWARMING_TAG_ID, task.Id),
				tag(db.SWARMING_TAG_NAME, task.Name),
				tag(db.SWARMING_TAG_REPO, task.Repo),
				tag(db.SWARMING_TAG_REVISION, task.Revision),
			},
			TaskId: task.SwarmingTaskId,
		},
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
	c1 := "c06ac6093d3029dffe997e9d85e8e61fee5f87b9"
	c2 := "0f87799ac791b8d8573e93694d05b05a65e09668"
	buildTask := "Build-Ubuntu-GCC-Arm7-Release-Android"
	testTask := "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
	perfTask := "Perf-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
	repo := "skia.git"
	commits := map[string][]string{
		repo: []string{c1, c2},
	}

	assert.NoError(t, err)
	isolateClient, err := isolate.NewClient(tr.Dir)
	assert.NoError(t, err)
	isolateClient.ServerUrl = isolate.FAKE_SERVER_URL
	swarmingClient := swarming.NewTestClient()
	s, err := NewTaskScheduler(d, cache, time.Duration(math.MaxInt64), tr.Dir, []string{"skia.git"}, isolateClient, swarmingClient)
	assert.NoError(t, err)

	// Check the initial set of task candidates. The two Build tasks
	// should be the only ones available.
	c, err := s.findTaskCandidates(commits)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(c))
	key := fmt.Sprintf("%s|%s", repo, buildTask)
	assert.Equal(t, 2, len(c[key]))
	for _, candidate := range c[key] {
		assert.Equal(t, candidate.Name, buildTask)
	}

	// Insert a the Build task at c1 (1 dependent) into the database,
	// transition through various states.
	var t1 *db.Task
	for _, candidates := range c { // Order not guaranteed; find the right candidate.
		for _, candidate := range candidates {
			if candidate.Revision == c1 {
				t1 = makeTask(candidate.Name, candidate.Repo, candidate.Revision)
				break
			}
		}
	}
	assert.NotNil(t, t1)

	// We shouldn't duplicate pending or running tasks.
	for _, status := range []db.TaskStatus{db.TASK_STATUS_PENDING, db.TASK_STATUS_RUNNING} {
		t1.Status = status
		assert.NoError(t, d.PutTask(t1))
		assert.NoError(t, cache.Update())

		c, err = s.findTaskCandidates(commits)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(c))
		for _, candidates := range c {
			for _, candidate := range candidates {
				assert.Equal(t, candidate.Name, buildTask)
				assert.Equal(t, c2, candidate.Revision)
			}
		}
	}

	// The task failed. Ensure that its dependents are not candidates, but
	// the task itself is back in the list of candidates, in case we want
	// to retry.
	t1.Status = db.TASK_STATUS_FAILURE
	assert.NoError(t, d.PutTask(t1))
	assert.NoError(t, cache.Update())

	c, err = s.findTaskCandidates(commits)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(c))
	for _, candidates := range c {
		assert.Equal(t, 2, len(candidates))
		for _, candidate := range candidates {
			assert.Equal(t, candidate.Name, buildTask)
		}
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
	for _, candidates := range c {
		for _, candidate := range candidates {
			assert.False(t, t1.Name == candidate.Name && t1.Revision == candidate.Revision)
		}
	}

	// Create the other Build task.
	var t2 *db.Task
	for _, candidates := range c {
		for _, candidate := range candidates {
			if candidate.Revision == c2 && strings.HasPrefix(candidate.Name, "Build-") {
				t2 = makeTask(candidate.Name, candidate.Repo, candidate.Revision)
				break
			}
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
	assert.Equal(t, 2, len(c))
	assert.Equal(t, 2, len(c[fmt.Sprintf("%s|%s", repo, testTask)]))
	assert.Equal(t, 1, len(c[fmt.Sprintf("%s|%s", repo, perfTask)]))
	for _, candidates := range c {
		for _, candidate := range candidates {
			assert.NotEqual(t, candidate.Name, buildTask)
		}
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
	repoName := "skia.git"
	repo, err := gitrepo.NewRepo(repoName, path.Join(tr.Dir, repoName))
	assert.NoError(t, err)
	ids := make([]string, len(testCases))
	commitsBuf := make([]*gitrepo.Commit, 0, buildbot.MAX_BLAMELIST_COMMITS)
	for i, tc := range testCases {
		// Ensure that we get the expected blamelist.
		commits, stoleFrom, err := ComputeBlamelist(cache, repo, name, repoName, tc.Revision, commitsBuf)
		if tc.Revision == "" {
			assert.Error(t, err)
			continue
		} else {
			assert.NoError(t, err)
		}
		sort.Strings(commits)
		testutils.AssertDeepEqual(t, tc.Expected, commits)
		if tc.StoleFromIdx >= 0 {
			assert.NotNil(t, stoleFrom)
			assert.Equal(t, ids[tc.StoleFromIdx], stoleFrom.Id)
		} else {
			assert.Nil(t, stoleFrom)
		}

		// Insert the task into the DB.
		c := &taskCandidate{
			Name:     name,
			Repo:     repoName,
			Revision: tc.Revision,
		}
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
	c1 := "c06ac6093d3029dffe997e9d85e8e61fee5f87b9"
	c2 := "0f87799ac791b8d8573e93694d05b05a65e09668"
	buildTask := "Build-Ubuntu-GCC-Arm7-Release-Android"
	testTask := "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
	perfTask := "Perf-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
	repoName := "skia.git"

	assert.NoError(t, err)
	isolateClient, err := isolate.NewClient(tr.Dir)
	assert.NoError(t, err)
	isolateClient.ServerUrl = isolate.FAKE_SERVER_URL
	swarmingClient := swarming.NewTestClient()
	s, err := NewTaskScheduler(d, cache, time.Duration(math.MaxInt64), tr.Dir, []string{repoName}, isolateClient, swarmingClient)
	assert.NoError(t, err)

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
		assert.Equal(t, []string{c.Revision}, c.Commits)
		assert.Equal(t, 2.0, c.Score)
	}

	// Insert one of the tasks.
	var t1 *db.Task
	for _, c := range s.queue { // Order not guaranteed; find the right candidate.
		if c.Revision == c1 {
			t1 = makeTask(c.Name, c.Repo, c.Revision)
			break
		}
	}
	assert.NotNil(t, t1)
	t1.Status = db.TASK_STATUS_SUCCESS
	t1.IsolatedOutput = "fake isolated hash"
	assert.NoError(t, d.PutTask(t1))
	assert.NoError(t, cache.Update())

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
	t2 := makeTask(s.queue[buildIdx].Name, s.queue[buildIdx].Repo, s.queue[buildIdx].Revision)
	t2.Status = db.TASK_STATUS_SUCCESS
	t2.IsolatedOutput = "fake isolated hash"
	assert.NoError(t, d.PutTask(t2))
	assert.NoError(t, cache.Update())

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
	t3 := makeTask(testTask, repoName, c2)
	t3.Commits = append(t3.Commits, c1)
	t3.Status = db.TASK_STATUS_SUCCESS
	t3.IsolatedOutput = "fake isolated hash"
	assert.NoError(t, d.PutTask(t3))
	assert.NoError(t, cache.Update())

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

func makeTaskCandidate(name string, dims []string) *taskCandidate {
	return &taskCandidate{
		Name: name,
		TaskSpec: &TaskSpec{
			Dimensions: dims,
		},
	}
}

func makeSwarmingBot(id string, dims []string) *swarming_api.SwarmingRpcsBotInfo {
	d := make([]*swarming_api.SwarmingRpcsStringListPair, 0, len(dims))
	for _, s := range dims {
		split := strings.SplitN(s, ":", 2)
		d = append(d, &swarming_api.SwarmingRpcsStringListPair{
			Key:   split[0],
			Value: []string{split[1]},
		})
	}
	return &swarming_api.SwarmingRpcsBotInfo{
		BotId:      id,
		Dimensions: d,
	}
}

func TestGetCandidatesToSchedule(t *testing.T) {
	// Empty lists.
	rv := getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{}, []*taskCandidate{})
	assert.Equal(t, 0, len(rv))

	t1 := makeTaskCandidate("task1", []string{"k:v"})
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{}, []*taskCandidate{t1})
	assert.Equal(t, 0, len(rv))

	b1 := makeSwarmingBot("bot1", []string{"k:v"})
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{})
	assert.Equal(t, 0, len(rv))

	// Single match.
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{t1})
	testutils.AssertDeepEqual(t, []*taskCandidate{t1}, rv)

	// No match.
	t1.TaskSpec.Dimensions[0] = "k:v2"
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{t1})
	assert.Equal(t, 0, len(rv))

	// Add a task candidate to match b1.
	t1 = makeTaskCandidate("task1", []string{"k:v2"})
	t2 := makeTaskCandidate("task2", []string{"k:v"})
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{t1, t2})
	testutils.AssertDeepEqual(t, []*taskCandidate{t2}, rv)

	// Switch the task order.
	t1 = makeTaskCandidate("task1", []string{"k:v2"})
	t2 = makeTaskCandidate("task2", []string{"k:v"})
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{t2, t1})
	testutils.AssertDeepEqual(t, []*taskCandidate{t2}, rv)

	// Make both tasks match the bot, ensure that we pick the first one.
	t1 = makeTaskCandidate("task1", []string{"k:v"})
	t2 = makeTaskCandidate("task2", []string{"k:v"})
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{t1, t2})
	testutils.AssertDeepEqual(t, []*taskCandidate{t1}, rv)
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{t2, t1})
	testutils.AssertDeepEqual(t, []*taskCandidate{t2}, rv)

	// Multiple dimensions. Ensure that different permutations of the bots
	// and tasks lists give us the expected results.
	dims := []string{"k:v", "k2:v2", "k3:v3"}
	b1 = makeSwarmingBot("bot1", dims)
	b2 := makeSwarmingBot("bot2", t1.TaskSpec.Dimensions)
	t1 = makeTaskCandidate("task1", []string{"k:v"})
	t2 = makeTaskCandidate("task2", dims)
	// In the first two cases, the task with fewer dimensions has the
	// higher priority. It gets the bot with more dimensions because it
	// is first in sorted order. The second task does not get scheduled
	// because there is no bot available which can run it.
	// TODO(borenet): Use a more optimal solution to avoid this case.
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1, b2}, []*taskCandidate{t1, t2})
	testutils.AssertDeepEqual(t, []*taskCandidate{t1}, rv)
	t1 = makeTaskCandidate("task1", []string{"k:v"})
	t2 = makeTaskCandidate("task2", dims)
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b2, b1}, []*taskCandidate{t1, t2})
	testutils.AssertDeepEqual(t, []*taskCandidate{t1}, rv)
	// In these two cases, the task with more dimensions has the higher
	// priority. Both tasks get scheduled.
	t1 = makeTaskCandidate("task1", []string{"k:v"})
	t2 = makeTaskCandidate("task2", dims)
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1, b2}, []*taskCandidate{t2, t1})
	testutils.AssertDeepEqual(t, []*taskCandidate{t2, t1}, rv)
	t1 = makeTaskCandidate("task1", []string{"k:v"})
	t2 = makeTaskCandidate("task2", dims)
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b2, b1}, []*taskCandidate{t2, t1})
	testutils.AssertDeepEqual(t, []*taskCandidate{t2, t1}, rv)

	// Matching dimensions. More bots than tasks.
	b2 = makeSwarmingBot("bot2", dims)
	b3 := makeSwarmingBot("bot3", dims)
	t1 = makeTaskCandidate("task1", dims)
	t2 = makeTaskCandidate("task2", dims)
	t3 := makeTaskCandidate("task3", dims)
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1, b2, b3}, []*taskCandidate{t1, t2})
	testutils.AssertDeepEqual(t, []*taskCandidate{t1, t2}, rv)

	// More tasks than bots.
	t1 = makeTaskCandidate("task1", dims)
	t2 = makeTaskCandidate("task2", dims)
	t3 = makeTaskCandidate("task3", dims)
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1, b2}, []*taskCandidate{t1, t2, t3})
	testutils.AssertDeepEqual(t, []*taskCandidate{t1, t2}, rv)
}

func makeBot(id string, dims map[string]string) *swarming_api.SwarmingRpcsBotInfo {
	dimensions := make([]*swarming_api.SwarmingRpcsStringListPair, 0, len(dims))
	for k, v := range dims {
		dimensions = append(dimensions, &swarming_api.SwarmingRpcsStringListPair{
			Key:   k,
			Value: []string{v},
		})
	}
	return &swarming_api.SwarmingRpcsBotInfo{
		BotId:      id,
		Dimensions: dimensions,
	}
}

func TestSchedulingE2E(t *testing.T) {
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
	c1 := "c06ac6093d3029dffe997e9d85e8e61fee5f87b9"
	c2 := "0f87799ac791b8d8573e93694d05b05a65e09668"

	repoName := "skia.git"

	isolateClient, err := isolate.NewClient(tr.Dir)
	assert.NoError(t, err)
	isolateClient.ServerUrl = isolate.FAKE_SERVER_URL
	swarmingClient := swarming.NewTestClient()
	s, err := NewTaskScheduler(d, cache, time.Duration(math.MaxInt64), tr.Dir, []string{repoName}, isolateClient, swarmingClient)

	// Start testing. No free bots, so we get a full queue with nothing
	// scheduled.
	assert.NoError(t, s.MainLoop())
	tasks, err := cache.GetTasksForCommits(repoName, []string{c1, c2})
	assert.NoError(t, err)
	expect := map[string]map[string]*db.Task{
		c1: map[string]*db.Task{},
		c2: map[string]*db.Task{},
	}
	testutils.AssertDeepEqual(t, expect, tasks)

	// A bot is free but doesn't have all of the right dimensions to run a task.
	bot1 := makeBot("bot1", map[string]string{"pool": "Skia"})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	assert.NoError(t, s.MainLoop())
	tasks, err = cache.GetTasksForCommits(repoName, []string{c1, c2})
	assert.NoError(t, err)
	expect = map[string]map[string]*db.Task{
		c1: map[string]*db.Task{},
		c2: map[string]*db.Task{},
	}
	testutils.AssertDeepEqual(t, expect, tasks)
	assert.Equal(t, 2, len(s.queue))

	// One bot free, schedule a task, ensure it's not in the queue.
	bot1.Dimensions = append(bot1.Dimensions, &swarming_api.SwarmingRpcsStringListPair{
		Key:   "os",
		Value: []string{"Ubuntu"},
	})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	assert.NoError(t, s.MainLoop())
	tasks, err = cache.GetTasksForCommits(repoName, []string{c1, c2})
	assert.NoError(t, err)
	var t1 *db.Task
	for _, v := range tasks {
		for _, t := range v {
			t1 = t
			break
		}
	}
	assert.NotNil(t, t1)
	assert.Equal(t, 1, len(s.queue))

	// The task is complete.
	t1.Status = db.TASK_STATUS_SUCCESS
	t1.Finished = time.Now()
	t1.IsolatedOutput = "abc123"
	assert.NoError(t, d.PutTask(t1))
	swarmingClient.MockTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{
		makeSwarmingRpcsTaskRequestMetadata(t, t1),
	})

	// No bots free. Ensure that the queue is correct.
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{})
	assert.NoError(t, s.MainLoop())
	tasks, err = cache.GetTasksForCommits(repoName, []string{c1, c2})
	assert.NoError(t, err)
	// The tests don't use any time-based score scaling, because the commits
	// in the test repo have fixed timestamps and would eventually result in
	// zero scores. The side effect is that we don't know which of c1 or c2
	// will be chosen because they end up with the same score.
	expectLen := 2 // One remaining build task, plus one test task.
	if t1.Revision == c2 {
		expectLen = 3 // c2 adds a perf task.
	}
	assert.Equal(t, expectLen, len(s.queue))

	// More bots than tasks free, ensure the queue is correct.
	bot2 := makeBot("bot2", map[string]string{
		"pool":        "Skia",
		"os":          "Android",
		"device_type": "grouper",
	})
	bot3 := makeBot("bot3", map[string]string{
		"pool":        "Skia",
		"os":          "Android",
		"device_type": "grouper",
	})
	bot4 := makeBot("bot4", map[string]string{
		"pool": "Skia",
		"os":   "Ubuntu",
	})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2, bot3, bot4})
	assert.NoError(t, s.MainLoop())
	_, err = cache.GetTasksForCommits(repoName, []string{c1, c2})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(s.queue))

	// Second compile task finished.
	var t2 *db.Task
	var t3 *db.Task
	var t4 *db.Task
	tasks, err = cache.GetTasksForCommits(repoName, []string{c1, c2})
	assert.NoError(t, err)
	for _, v := range tasks {
		for _, task := range v {
			if task.Name == t1.Name {
				if (t1.Revision == c1 && task.Revision == c2) || (t1.Revision == c2 && task.Revision == c1) {
					t2 = task
				}
			} else {
				if t3 == nil {
					t3 = task
				} else {
					t4 = task
				}
			}
		}
	}
	assert.NotNil(t, t2)
	assert.NotNil(t, t3)
	t2.Status = db.TASK_STATUS_SUCCESS
	t2.Finished = time.Now()
	t2.IsolatedOutput = "abc123"

	// No new bots free; ensure that the newly-available tasks are in the queue.
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{})
	mockTasks := []*swarming_api.SwarmingRpcsTaskRequestMetadata{
		makeSwarmingRpcsTaskRequestMetadata(t, t2),
		makeSwarmingRpcsTaskRequestMetadata(t, t3),
	}
	if t4 != nil {
		mockTasks = append(mockTasks, makeSwarmingRpcsTaskRequestMetadata(t, t4))
	}
	swarmingClient.MockTasks(mockTasks)
	assert.NoError(t, s.MainLoop())
	expectLen = 1 // Test task from c1
	if t2.Revision == c2 {
		expectLen = 2 // Test and perf tasks from c2
	}
	assert.Equal(t, expectLen, len(s.queue))

	// Finish the other tasks.
	t3, err = cache.GetTask(t3.Id)
	assert.NoError(t, err)
	t3.Status = db.TASK_STATUS_SUCCESS
	t3.Finished = time.Now()
	t3.IsolatedOutput = "abc123"
	if t4 != nil {
		t4, err = cache.GetTask(t4.Id)
		assert.NoError(t, err)
		t4.Status = db.TASK_STATUS_SUCCESS
		t4.Finished = time.Now()
		t4.IsolatedOutput = "abc123"
	}

	// Ensure that we finally run all of the tasks and insert into the DB.
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2, bot3, bot4})
	mockTasks = []*swarming_api.SwarmingRpcsTaskRequestMetadata{
		makeSwarmingRpcsTaskRequestMetadata(t, t3),
	}
	if t4 != nil {
		mockTasks = append(mockTasks, makeSwarmingRpcsTaskRequestMetadata(t, t4))
	}
	assert.NoError(t, s.MainLoop())
	tasks, err = cache.GetTasksForCommits(repoName, []string{c1, c2})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(tasks[c1]))
	assert.Equal(t, 3, len(tasks[c2]))
	assert.Equal(t, 0, len(s.queue))

	// Mark everything as finished. Ensure that the queue still ends up empty.
	tasksList := []*db.Task{}
	for _, v := range tasks {
		for _, task := range v {
			if task.Status != db.TASK_STATUS_SUCCESS {
				task.Status = db.TASK_STATUS_SUCCESS
				task.Finished = time.Now()
				task.IsolatedOutput = "abc123"
				tasksList = append(tasksList, task)
			}
		}
	}
	mockTasks = make([]*swarming_api.SwarmingRpcsTaskRequestMetadata, 0, len(tasksList))
	for _, task := range tasksList {
		mockTasks = append(mockTasks, makeSwarmingRpcsTaskRequestMetadata(t, task))
	}
	swarmingClient.MockTasks(mockTasks)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2, bot3, bot4})
	assert.NoError(t, s.MainLoop())
	assert.Equal(t, 0, len(s.queue))
}

func makeDummyCommits(t *testing.T, repoDir string, numCommits int) {
	_, err := exec.RunCwd(repoDir, "git", "config", "user.email", "test@skia.org")
	assert.NoError(t, err)
	_, err = exec.RunCwd(repoDir, "git", "config", "user.name", "Skia Tester")
	assert.NoError(t, err)
	_, err = exec.RunCwd(repoDir, "git", "checkout", "master")
	assert.NoError(t, err)
	dummyFile := path.Join(repoDir, "dummyfile.txt")
	for i := 0; i < numCommits; i++ {
		title := fmt.Sprintf("Dummy #%d", i)
		assert.NoError(t, ioutil.WriteFile(dummyFile, []byte(title), os.ModePerm))
		_, err = exec.RunCwd(repoDir, "git", "add", dummyFile)
		assert.NoError(t, err)
		_, err = exec.RunCwd(repoDir, "git", "commit", "-m", title)
		assert.NoError(t, err)
		_, err = exec.RunCwd(repoDir, "git", "push", "origin", "master")
		assert.NoError(t, err)
	}
}

func TestSchedulerStealingFrom(t *testing.T) {
	testutils.SkipIfShort(t)

	// Setup.
	tr := util.NewTempRepo()
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
	c1 := "c06ac6093d3029dffe997e9d85e8e61fee5f87b9"
	c2 := "0f87799ac791b8d8573e93694d05b05a65e09668"
	buildTask := "Build-Ubuntu-GCC-Arm7-Release-Android"
	repoName := "skia.git"
	repoDir := path.Join(tr.Dir, repoName)

	repos := gitinfo.NewRepoMap(tr.Dir)
	repo, err := repos.Repo(repoName)
	assert.NoError(t, err)
	isolateClient, err := isolate.NewClient(tr.Dir)
	assert.NoError(t, err)
	isolateClient.ServerUrl = isolate.FAKE_SERVER_URL
	swarmingClient := swarming.NewTestClient()
	s, err := NewTaskScheduler(d, cache, time.Duration(math.MaxInt64), tr.Dir, []string{"skia.git"}, isolateClient, swarmingClient)
	assert.NoError(t, err)

	// Run both available compile tasks.
	bot1 := makeBot("bot1", map[string]string{"pool": "Skia", "os": "Ubuntu"})
	bot2 := makeBot("bot2", map[string]string{"pool": "Skia", "os": "Ubuntu"})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2})
	assert.NoError(t, s.MainLoop())
	tasks, err := cache.GetTasksForCommits(repoName, []string{c1, c2})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks[c1]))
	assert.Equal(t, 1, len(tasks[c2]))
	tasksList := []*db.Task{}
	for _, v := range tasks {
		for _, task := range v {
			if task.Status != db.TASK_STATUS_SUCCESS {
				task.Status = db.TASK_STATUS_SUCCESS
				task.Finished = time.Now()
				task.IsolatedOutput = "abc123"
				tasksList = append(tasksList, task)
			}
		}
	}
	assert.NoError(t, d.PutTasks(tasksList))
	assert.NoError(t, cache.Update())

	// Add some commits.
	makeDummyCommits(t, repoDir, 10)
	commits, err := repo.RevList("HEAD")
	assert.NoError(t, err)

	// Run one task. Ensure that it's at tip-of-tree.
	head, err := repo.FullHash("HEAD")
	assert.NoError(t, err)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	assert.NoError(t, s.MainLoop())
	tasks, err = cache.GetTasksForCommits(repoName, commits)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks[head]))
	task := tasks[head][buildTask]
	assert.Equal(t, head, task.Revision)
	expect := commits[:len(commits)-2]
	sort.Strings(expect)
	sort.Strings(task.Commits)
	testutils.AssertDeepEqual(t, expect, task.Commits)

	task.Status = db.TASK_STATUS_SUCCESS
	task.Finished = time.Now()
	task.IsolatedOutput = "abc123"
	assert.NoError(t, d.PutTask(task))
	assert.NoError(t, cache.Update())

	oldTasksByCommit := tasks

	// Run backfills, ensuring that each one steals the right set of commits
	// from previous builds, until all of the build task candidates have run.
	for i := 0; i < 9; i++ {
		// Now, run another task. The new task should bisect the old one.
		swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
		assert.NoError(t, s.MainLoop())
		tasks, err = cache.GetTasksForCommits(repoName, commits)
		assert.NoError(t, err)
		var newTask *db.Task
		for _, v := range tasks {
			for _, task := range v {
				if task.Status == db.TASK_STATUS_PENDING {
					assert.True(t, newTask == nil || task.Id == newTask.Id)
					newTask = task
				}
			}
		}
		assert.NotNil(t, newTask)

		oldTask := oldTasksByCommit[newTask.Revision][newTask.Name]
		assert.NotNil(t, oldTask)
		assert.True(t, util.In(newTask.Revision, oldTask.Commits))

		// Find the updated old task.
		updatedOldTask, err := cache.GetTask(oldTask.Id)
		assert.NoError(t, err)
		assert.NotNil(t, updatedOldTask)

		// Ensure that the blamelists are correct.
		old := util.NewStringSet(oldTask.Commits)
		new := util.NewStringSet(newTask.Commits)
		updatedOld := util.NewStringSet(updatedOldTask.Commits)

		testutils.AssertDeepEqual(t, old, new.Union(updatedOld))
		assert.Equal(t, 0, len(new.Intersect(updatedOld)))
		// Finish the new task.
		newTask.Status = db.TASK_STATUS_SUCCESS
		newTask.Finished = time.Now()
		newTask.IsolatedOutput = "abc123"
		assert.NoError(t, d.PutTask(newTask))
		assert.NoError(t, cache.Update())
		oldTasksByCommit = tasks

	}

	// Ensure that we're really done.
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	assert.NoError(t, s.MainLoop())
	tasks, err = cache.GetTasksForCommits(repoName, commits)
	assert.NoError(t, err)
	var newTask *db.Task
	for _, v := range tasks {
		for _, task := range v {
			if task.Status == db.TASK_STATUS_PENDING {
				assert.True(t, newTask == nil || task.Id == newTask.Id)
				newTask = task
			}
		}
	}
	assert.Nil(t, newTask)
}

func TestMultipleCandidatesBackfillingEachOther(t *testing.T) {
	testutils.SkipIfShort(t)

	workdir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, workdir)

	run := func(dir string, cmd ...string) {
		_, err := exec.RunCwd(dir, cmd...)
		assert.NoError(t, err)
	}

	addFile := func(repoDir, subPath, contents string) {
		assert.NoError(t, ioutil.WriteFile(path.Join(repoDir, subPath), []byte(contents), os.ModePerm))
		run(repoDir, "git", "add", subPath)
	}

	repoName := "skia.git"
	repoDir := path.Join(workdir, repoName)

	assert.NoError(t, ioutil.WriteFile(path.Join(workdir, ".gclient"), []byte("dummy"), os.ModePerm))

	assert.NoError(t, os.Mkdir(path.Join(workdir, repoName), os.ModePerm))
	run(repoDir, "git", "init")
	run(repoDir, "git", "remote", "add", "origin", ".")

	infraBotsSubDir := path.Join("infra", "bots")
	infraBotsDir := path.Join(repoDir, infraBotsSubDir)
	assert.NoError(t, os.MkdirAll(infraBotsDir, os.ModePerm))

	addFile(repoDir, "somefile.txt", "dummy3")
	addFile(repoDir, path.Join(infraBotsSubDir, "dummy.isolate"), `{
  'variables': {
    'command': [
      'python', 'recipes.py', 'run',
    ],
    'files': [
      '../../somefile.txt',
    ],
  },
}`)

	// Create a single task in the config.
	taskName := "dummytask"
	cfg := &TasksCfg{
		Tasks: map[string]*TaskSpec{
			taskName: &TaskSpec{
				CipdPackages: []*CipdPackage{},
				Dependencies: []string{},
				Dimensions:   []string{"pool:Skia"},
				Isolate:      "dummy.isolate",
				Priority:     1.0,
			},
		},
	}
	f, err := os.Create(path.Join(repoDir, TASKS_CFG_FILE))
	assert.NoError(t, err)
	assert.NoError(t, json.NewEncoder(f).Encode(&cfg))
	assert.NoError(t, f.Close())
	run(repoDir, "git", "add", TASKS_CFG_FILE)
	run(repoDir, "git", "commit", "-m", "Add more tasks!")
	run(repoDir, "git", "push", "origin", "master")
	run(repoDir, "git", "branch", "-u", "origin/master")

	// Setup the scheduler.
	repos := gitinfo.NewRepoMap(workdir)
	repo, err := repos.Repo(repoName)
	assert.NoError(t, err)
	d := db.NewInMemoryDB()
	cache, err := db.NewTaskCache(d, time.Hour)
	assert.NoError(t, err)
	isolateClient, err := isolate.NewClient(workdir)
	assert.NoError(t, err)
	isolateClient.ServerUrl = isolate.FAKE_SERVER_URL
	swarmingClient := swarming.NewTestClient()
	s, err := NewTaskScheduler(d, cache, time.Duration(math.MaxInt64), workdir, []string{repoName}, isolateClient, swarmingClient)
	assert.NoError(t, err)

	mockTasks := []*swarming_api.SwarmingRpcsTaskRequestMetadata{}
	mock := func(task *db.Task) {
		mockTasks = append(mockTasks, makeSwarmingRpcsTaskRequestMetadata(t, task))
		swarmingClient.MockTasks(mockTasks)
	}

	// Cycle once.
	bot1 := makeBot("bot1", map[string]string{"pool": "Skia"})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	assert.NoError(t, s.MainLoop())
	assert.Equal(t, 0, len(s.queue))
	head, err := repo.FullHash("HEAD")
	assert.NoError(t, err)
	tasks, err := cache.GetTasksForCommits(repoName, []string{head})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks[head]))
	mock(tasks[head][taskName])

	// Add some commits to the repo.
	makeDummyCommits(t, repoDir, 8)
	commits, err := repo.RevList(fmt.Sprintf("%s..HEAD", head))
	assert.Nil(t, err)
	assert.Equal(t, 8, len(commits))

	// Trigger builds simultaneously.
	bot2 := makeBot("bot2", map[string]string{"pool": "Skia"})
	bot3 := makeBot("bot3", map[string]string{"pool": "Skia"})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2, bot3})
	assert.NoError(t, s.MainLoop())
	assert.Equal(t, 5, len(s.queue))
	tasks, err = cache.GetTasksForCommits(repoName, commits)
	assert.NoError(t, err)

	// If we're queueing correctly, we should've triggered tasks at
	// commits[0], commits[4], and either commits[2] or commits[6].
	var t1, t2, t3 *db.Task
	for _, byName := range tasks {
		for _, task := range byName {
			if task.Revision == commits[0] {
				t1 = task
			} else if task.Revision == commits[4] {
				t2 = task
			} else if task.Revision == commits[2] || task.Revision == commits[6] {
				t3 = task
			} else {
				assert.FailNow(t, fmt.Sprintf("Task has unknown revision: %v", task))
			}
		}
	}
	assert.NotNil(t, t1)
	assert.NotNil(t, t2)
	assert.NotNil(t, t3)
	mock(t1)
	mock(t2)
	mock(t3)

	// Ensure that we got the blamelists right.
	mkCopy := func(orig []string) []string {
		rv := make([]string, len(orig))
		copy(rv, orig)
		return rv
	}
	var expect1, expect2, expect3 []string
	if t3.Revision == commits[2] {
		expect1 = mkCopy(commits[:2])
		expect2 = mkCopy(commits[4:])
		expect3 = mkCopy(commits[2:4])
	} else {
		expect1 = mkCopy(commits[:4])
		expect2 = mkCopy(commits[4:6])
		expect3 = mkCopy(commits[6:])
	}
	sort.Strings(expect1)
	sort.Strings(expect2)
	sort.Strings(expect3)
	sort.Strings(t1.Commits)
	sort.Strings(t2.Commits)
	sort.Strings(t3.Commits)
	testutils.AssertDeepEqual(t, expect1, t1.Commits)
	testutils.AssertDeepEqual(t, expect2, t2.Commits)
	testutils.AssertDeepEqual(t, expect3, t3.Commits)

	// Just for good measure, check the task at the head of the queue.
	expectIdx := 2
	if t3.Revision == commits[expectIdx] {
		expectIdx = 6
	}
	assert.Equal(t, commits[expectIdx], s.queue[0].Revision)

	// Run again with 5 bots to check the case where we bisect the same
	// task twice.
	bot4 := makeBot("bot4", map[string]string{"pool": "Skia"})
	bot5 := makeBot("bot5", map[string]string{"pool": "Skia"})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2, bot3, bot4, bot5})
	assert.NoError(t, s.MainLoop())
	assert.Equal(t, 0, len(s.queue))
	tasks, err = cache.GetTasksForCommits(repoName, commits)
	assert.NoError(t, err)
	for _, byName := range tasks {
		for _, task := range byName {
			assert.Equal(t, 1, len(task.Commits))
		}
	}
}
