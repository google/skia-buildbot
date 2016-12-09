package scheduling

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"
	"sort"
	"strings"
	"testing"
	"time"

	buildbucket_api "github.com/luci/luci-go/common/api/buildbucket/buildbucket/v1"
	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	exec_testutils "go.skia.org/infra/go/exec/testutils"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/blacklist"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/tryjobs"
	"go.skia.org/infra/task_scheduler/go/window"
)

const (
	// The test repo has two commits. The first commit adds a tasks.cfg file
	// with two task specs: a build task and a test task, the test task
	// depending on the build task. The second commit adds a perf task spec,
	// which also depends on the build task. Therefore, there are five total
	// possible tasks we could run:
	//
	// Build@c1, Test@c1, Build@c2, Test@c2, Perf@c2
	//
	c1        = "10ca3b86bac8991967ebe15cc89c22fd5396a77b"
	c2        = "d4fa60ab35c99c886220c4629c36b9785cc89c8b"
	buildTask = "Build-Ubuntu-GCC-Arm7-Release-Android"
	testTask  = "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
	perfTask  = "Perf-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
	repoName  = "skia.git"
)

var (
	rs1 = db.RepoState{
		Repo:     repoName,
		Revision: c1,
	}
	rs2 = db.RepoState{
		Repo:     repoName,
		Revision: c2,
	}

	projectRepoMapping = map[string]string{
		"skia": repoName,
	}

	androidTaskDims = map[string]string{
		"pool":        "Skia",
		"os":          "Android",
		"device_type": "grouper",
	}

	linuxTaskDims = map[string]string{
		"os":   "Ubuntu",
		"pool": "Skia",
	}

	androidBotDims = map[string][]string{
		"pool":        []string{"Skia"},
		"os":          []string{"Android"},
		"device_type": []string{"grouper"},
	}

	linuxBotDims = map[string][]string{
		"os":   []string{"Ubuntu"},
		"pool": []string{"Skia"},
	}
)

func makeTask(name, repo, revision string) *db.Task {
	return &db.Task{
		Commits: []string{revision},
		Created: time.Now(),
		TaskKey: db.TaskKey{
			RepoState: db.RepoState{
				Repo:     repo,
				Revision: revision,
			},
			Name: name,
		},
	}
}

func makeSwarmingRpcsTaskRequestMetadata(t *testing.T, task *db.Task, dims map[string]string) *swarming_api.SwarmingRpcsTaskRequestMetadata {
	tag := func(k, v string) string {
		return fmt.Sprintf("%s:%s", k, v)
	}
	ts := func(t time.Time) string {
		if util.TimeIsZero(t) {
			return ""
		}
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
	tags := []string{
		tag(db.SWARMING_TAG_ID, task.Id),
		tag(db.SWARMING_TAG_FORCED_JOB_ID, task.ForcedJobId),
		tag(db.SWARMING_TAG_NAME, task.Name),
		tag(swarming.DIMENSION_POOL_KEY, swarming.DIMENSION_POOL_VALUE_SKIA),
		tag(db.SWARMING_TAG_REPO, task.Repo),
		tag(db.SWARMING_TAG_REVISION, task.Revision),
	}
	for _, p := range task.ParentTaskIds {
		tags = append(tags, tag(db.SWARMING_TAG_PARENT_TASK_ID, p))
	}

	dimensions := make([]*swarming_api.SwarmingRpcsStringPair, 0, len(dims))
	for k, v := range dims {
		dimensions = append(dimensions, &swarming_api.SwarmingRpcsStringPair{
			Key:   k,
			Value: v,
		})
	}

	return &swarming_api.SwarmingRpcsTaskRequestMetadata{
		Request: &swarming_api.SwarmingRpcsTaskRequest{
			CreatedTs: ts(task.Created),
			Properties: &swarming_api.SwarmingRpcsTaskProperties{
				Dimensions: dimensions,
			},
			Tags: tags,
		},
		TaskId: task.SwarmingTaskId,
		TaskResult: &swarming_api.SwarmingRpcsTaskResult{
			AbandonedTs: abandoned,
			BotId:       task.SwarmingBotId,
			CreatedTs:   ts(task.Created),
			CompletedTs: ts(task.Finished),
			Failure:     failed,
			OutputsRef: &swarming_api.SwarmingRpcsFilesRef{
				Isolated: task.IsolatedOutput,
			},
			StartedTs: ts(task.Started),
			State:     state,
			Tags:      tags,
			TaskId:    task.SwarmingTaskId,
		},
	}
}

// Common setup for TaskScheduler tests.
func setup(t *testing.T) (*util.TempRepo, db.DB, *swarming.TestClient, *TaskScheduler, *mockhttpclient.URLMock) {
	testutils.LargeTest(t)
	testutils.SkipIfShort(t)
	tr := util.NewTempRepo()
	assert.NoError(t, os.Mkdir(path.Join(tr.Dir, TRIGGER_DIRNAME), os.ModePerm))
	d := db.NewInMemoryDB()
	isolateClient, err := isolate.NewClient(tr.Dir)
	assert.NoError(t, err)
	isolateClient.ServerUrl = isolate.FAKE_SERVER_URL
	swarmingClient := swarming.NewTestClient()
	urlMock := mockhttpclient.NewURLMock()
	repo, err := repograph.NewGraph(repoName, tr.Dir)
	assert.NoError(t, err)
	repos := repograph.Map{
		repoName: repo,
	}
	s, err := NewTaskScheduler(d, time.Duration(math.MaxInt64), 0, tr.Dir, repos, isolateClient, swarmingClient, urlMock.Client(), 1.0, tryjobs.API_URL_TESTING, tryjobs.BUCKET_TESTING, projectRepoMapping)
	assert.NoError(t, err)
	return tr, d, swarmingClient, s, urlMock
}

func TestGatherNewJobs(t *testing.T) {
	tr, _, _, s, _ := setup(t)
	defer tr.Cleanup()

	testGatherNewJobs := func(expectedJobs int) {
		assert.NoError(t, s.gatherNewJobs())
		jobs, err := s.jCache.UnfinishedJobs()
		assert.NoError(t, err)
		assert.Equal(t, expectedJobs, len(jobs))
	}

	// Ensure that the JobDB is empty.
	jobs, err := s.jCache.UnfinishedJobs()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(jobs))

	// Run gatherNewJobs, ensure that we added jobs for all commits in the
	// repo.
	testGatherNewJobs(5) // c1 has 2 jobs, c2 has 3 jobs.

	// Run gatherNewJobs again, ensure that we didn't add the same Jobs
	// again.
	testGatherNewJobs(5) // no new jobs == 5 total jobs.

	// Add a commit on master, run gatherNewJobs, ensure that we added the
	// new Jobs.
	d := path.Join(tr.Dir, "skia.git")
	exec_testutils.Run(t, d, "git", "checkout", "master")
	makeDummyCommits(t, d, 1, "master")
	assert.NoError(t, s.updateRepos())
	testGatherNewJobs(8) // we didn't add to the jobs spec, so 3 jobs/rev.

	// Add several commits on master, ensure that we added all of the Jobs.
	makeDummyCommits(t, d, 10, "master")
	assert.NoError(t, s.updateRepos())
	testGatherNewJobs(38) // 3 jobs/rev + 8 pre-existing jobs.

	// Add a commit on a branch other than master, run gatherNewJobs, ensure
	// that we added the new Jobs.
	branchName := "otherBranch"
	exec_testutils.Run(t, d, "git", "checkout", "-b", branchName)
	msg := "Branch commit"
	fileName := "some_other_file"
	assert.NoError(t, ioutil.WriteFile(path.Join(d, fileName), []byte(msg), os.ModePerm))
	exec_testutils.Run(t, d, "git", "add", fileName)
	exec_testutils.Run(t, d, "git", "commit", "-m", msg)
	exec_testutils.Run(t, d, "git", "push", "origin", branchName)
	assert.NoError(t, s.updateRepos())
	testGatherNewJobs(41) // 38 previous jobs + 3 new ones.

	// Add several commits in a row on different branches, ensure that we
	// added all of the Jobs for all of the new commits.
	makeDummyCommits(t, d, 5, branchName)
	exec_testutils.Run(t, d, "git", "checkout", "master")
	makeDummyCommits(t, d, 5, "master")
	assert.NoError(t, s.updateRepos())
	testGatherNewJobs(71) // 10 commits x 3 jobs/commit = 30, plus 41
}

func TestFindTaskCandidatesForJobs(t *testing.T) {
	tr, _, _, s, _ := setup(t)
	defer tr.Cleanup()

	test := func(jobs []*db.Job, expect map[db.TaskKey]*taskCandidate) {
		actual, err := s.findTaskCandidatesForJobs(jobs)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, actual, expect)
	}

	// Get all of the task specs, for future use.
	ts, err := s.taskCfgCache.GetTaskSpecsForRepoStates([]db.RepoState{rs1, rs2})
	assert.NoError(t, err)

	// Run on an empty job list, ensure empty list returned.
	test([]*db.Job{}, map[db.TaskKey]*taskCandidate{})

	now := time.Now().UTC()

	// Run for one job, ensure that we get the right set of task specs
	// returned (ie. all dependencies and their dependencies).
	j1 := &db.Job{
		Created:      now,
		Name:         "j1",
		Dependencies: map[string][]string{testTask: []string{buildTask}, buildTask: []string{}},
		Priority:     0.5,
		RepoState:    rs1.Copy(),
	}
	tc1 := &taskCandidate{
		JobCreated: now,
		TaskKey: db.TaskKey{
			RepoState: rs1.Copy(),
			Name:      buildTask,
		},
		TaskSpec: ts[rs1][buildTask].Copy(),
	}
	tc2 := &taskCandidate{
		JobCreated: now,
		TaskKey: db.TaskKey{
			RepoState: rs1.Copy(),
			Name:      testTask,
		},
		TaskSpec: ts[rs1][testTask].Copy(),
	}

	test([]*db.Job{j1}, map[db.TaskKey]*taskCandidate{
		tc1.TaskKey: tc1,
		tc2.TaskKey: tc2,
	})

	// Add a job, ensure that its dependencies are added and that the right
	// dependencies are de-duplicated.
	j2 := &db.Job{
		Created:      now,
		Name:         "j2",
		Dependencies: map[string][]string{testTask: []string{buildTask}, buildTask: []string{}},
		Priority:     0.6,
		RepoState:    rs2,
	}
	j3 := &db.Job{
		Created:      now,
		Name:         "j3",
		Dependencies: map[string][]string{perfTask: []string{buildTask}, buildTask: []string{}},
		Priority:     0.6,
		RepoState:    rs2,
	}
	tc3 := &taskCandidate{
		JobCreated: now,
		TaskKey: db.TaskKey{
			RepoState: rs2.Copy(),
			Name:      buildTask,
		},
		TaskSpec: ts[rs2][buildTask].Copy(),
	}
	tc4 := &taskCandidate{
		JobCreated: now,
		TaskKey: db.TaskKey{
			RepoState: rs2.Copy(),
			Name:      testTask,
		},
		TaskSpec: ts[rs2][testTask].Copy(),
	}
	tc5 := &taskCandidate{
		JobCreated: now,
		TaskKey: db.TaskKey{
			RepoState: rs2.Copy(),
			Name:      perfTask,
		},
		TaskSpec: ts[rs2][perfTask].Copy(),
	}
	allCandidates := map[db.TaskKey]*taskCandidate{
		tc1.TaskKey: tc1,
		tc2.TaskKey: tc2,
		tc3.TaskKey: tc3,
		tc4.TaskKey: tc4,
		tc5.TaskKey: tc5,
	}
	test([]*db.Job{j1, j2, j3}, allCandidates)

	// Finish j3, ensure that its task specs no longer show up.
	delete(allCandidates, j3.MakeTaskKey(perfTask))
	test([]*db.Job{j1, j2}, allCandidates)
}

func TestFilterTaskCandidates(t *testing.T) {
	tr, d, _, s, _ := setup(t)
	defer tr.Cleanup()

	// Fake out the initial candidates.
	k1 := db.TaskKey{
		RepoState: rs1,
		Name:      buildTask,
	}
	k2 := db.TaskKey{
		RepoState: rs1,
		Name:      testTask,
	}
	k3 := db.TaskKey{
		RepoState: rs2,
		Name:      buildTask,
	}
	k4 := db.TaskKey{
		RepoState: rs2,
		Name:      testTask,
	}
	k5 := db.TaskKey{
		RepoState: rs2,
		Name:      perfTask,
	}
	candidates := map[db.TaskKey]*taskCandidate{
		k1: &taskCandidate{
			TaskKey:  k1,
			TaskSpec: &specs.TaskSpec{},
		},
		k2: &taskCandidate{
			TaskKey: k2,
			TaskSpec: &specs.TaskSpec{
				Dependencies: []string{buildTask},
			},
		},
		k3: &taskCandidate{
			TaskKey:  k3,
			TaskSpec: &specs.TaskSpec{},
		},
		k4: &taskCandidate{
			TaskKey: k4,
			TaskSpec: &specs.TaskSpec{
				Dependencies: []string{buildTask},
			},
		},
		k5: &taskCandidate{
			TaskKey: k5,
			TaskSpec: &specs.TaskSpec{
				Dependencies: []string{buildTask},
			},
		},
	}

	// Check the initial set of task candidates. The two Build tasks
	// should be the only ones available.
	c, err := s.filterTaskCandidates(candidates)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(c))
	assert.Equal(t, 1, len(c[repoName]))
	assert.Equal(t, 2, len(c[repoName][buildTask]))
	for _, byRepo := range c {
		for _, byName := range byRepo {
			for _, candidate := range byName {
				assert.Equal(t, candidate.Name, buildTask)
			}
		}
	}

	// Insert a the Build task at c1 (1 dependent) into the database,
	// transition through various states.
	var t1 *db.Task
	for _, byRepo := range c { // Order not guaranteed, find the right candidate.
		for _, byName := range byRepo {
			for _, candidate := range byName {
				if candidate.Revision == c1 {
					t1 = makeTask(candidate.Name, candidate.Repo, candidate.Revision)
					break
				}
			}
		}
	}
	assert.NotNil(t, t1)

	// We shouldn't duplicate pending or running tasks.
	for _, status := range []db.TaskStatus{db.TASK_STATUS_PENDING, db.TASK_STATUS_RUNNING} {
		t1.Status = status
		assert.NoError(t, d.PutTask(t1))
		assert.NoError(t, s.tCache.Update())

		c, err = s.filterTaskCandidates(candidates)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(c))
		for _, byRepo := range c {
			for _, byName := range byRepo {
				for _, candidate := range byName {
					assert.Equal(t, candidate.Name, buildTask)
					assert.Equal(t, c2, candidate.Revision)
				}
			}
		}
	}

	// The task failed. Ensure that its dependents are not candidates, but
	// the task itself is back in the list of candidates, in case we want
	// to retry.
	t1.Status = db.TASK_STATUS_FAILURE
	assert.NoError(t, d.PutTask(t1))
	assert.NoError(t, s.tCache.Update())

	c, err = s.filterTaskCandidates(candidates)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(c))
	for _, byRepo := range c {
		assert.Equal(t, 1, len(byRepo))
		for _, byName := range byRepo {
			assert.Equal(t, 2, len(byName))
			for _, candidate := range byName {
				assert.Equal(t, candidate.Name, buildTask)
			}
		}
	}

	// The task succeeded. Ensure that its dependents are candidates and
	// the task itself is not.
	t1.Status = db.TASK_STATUS_SUCCESS
	t1.IsolatedOutput = "fake isolated hash"
	assert.NoError(t, d.PutTask(t1))
	assert.NoError(t, s.tCache.Update())

	c, err = s.filterTaskCandidates(candidates)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(c))
	for _, byRepo := range c {
		assert.Equal(t, 2, len(byRepo))
		for _, byName := range byRepo {
			for _, candidate := range byName {
				assert.False(t, t1.Name == candidate.Name && t1.Revision == candidate.Revision)
			}
		}
	}

	// Create the other Build task.
	var t2 *db.Task
	for _, byRepo := range c {
		for _, byName := range byRepo {
			for _, candidate := range byName {
				if candidate.Revision == c2 && strings.HasPrefix(candidate.Name, "Build-") {
					t2 = makeTask(candidate.Name, candidate.Repo, candidate.Revision)
					break
				}
			}
		}
	}
	assert.NotNil(t, t2)
	t2.Status = db.TASK_STATUS_SUCCESS
	t2.IsolatedOutput = "fake isolated hash"
	assert.NoError(t, d.PutTask(t2))
	assert.NoError(t, s.tCache.Update())

	// All test and perf tasks are now candidates, no build tasks.
	c, err = s.filterTaskCandidates(candidates)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(c))
	assert.Equal(t, 2, len(c[repoName][testTask]))
	assert.Equal(t, 1, len(c[repoName][perfTask]))
	for _, byRepo := range c {
		for _, byName := range byRepo {
			for _, candidate := range byName {
				assert.NotEqual(t, candidate.Name, buildTask)
			}
		}
	}

	// Add a try job. Ensure that no deps have been incorrectly satisfied.
	tryKey := k4.Copy()
	tryKey.Server = "dummy-server"
	tryKey.Issue = "dummy-issue"
	tryKey.Patchset = "dummy-patchset"
	candidates[tryKey] = &taskCandidate{
		TaskKey: tryKey,
		TaskSpec: &specs.TaskSpec{
			Dependencies: []string{buildTask},
		},
	}
	c, err = s.filterTaskCandidates(candidates)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(c))
	assert.Equal(t, 2, len(c[repoName][testTask]))
	assert.Equal(t, 1, len(c[repoName][perfTask]))
	for _, byRepo := range c {
		for _, byName := range byRepo {
			for _, candidate := range byName {
				assert.NotEqual(t, candidate.Name, buildTask)
				assert.False(t, candidate.IsTryJob())
			}
		}
	}
}

func TestProcessTaskCandidate(t *testing.T) {
	tr, _, _, s, _ := setup(t)
	defer tr.Cleanup()

	cache := newCacheWrapper(s.tCache)
	now := time.Unix(0, 1470674884000000)
	commitsBuf := make([]*repograph.Commit, 0, MAX_BLAMELIST_COMMITS)

	// Try job candidates have a specific score and no blamelist.
	c := &taskCandidate{
		JobCreated: now.Add(-1 * time.Hour),
		TaskKey: db.TaskKey{
			RepoState: db.RepoState{
				Patch: db.Patch{
					Server:   "my-server",
					Issue:    "my-issue",
					Patchset: "my-patchset",
				},
				Repo:     repoName,
				Revision: c1,
			},
		},
	}
	assert.NoError(t, s.processTaskCandidate(c, now, cache, commitsBuf))
	assert.Equal(t, CANDIDATE_SCORE_TRY_JOB+1.0, c.Score)
	assert.Nil(t, c.Commits)

	// Manually forced candidates have a blamelist and a specific score.
	c = &taskCandidate{
		JobCreated: now.Add(-2 * time.Hour),
		TaskKey: db.TaskKey{
			RepoState: db.RepoState{
				Repo:     repoName,
				Revision: c2,
			},
			ForcedJobId: "my-job",
		},
	}
	assert.NoError(t, s.processTaskCandidate(c, now, cache, commitsBuf))
	assert.Equal(t, CANDIDATE_SCORE_FORCE_RUN+2.0, c.Score)
	assert.Equal(t, 1, len(c.Commits))

	// All other candidates have a blamelist and a time-decayed score.
	c = &taskCandidate{
		TaskKey: db.TaskKey{
			RepoState: db.RepoState{
				Repo:     repoName,
				Revision: c2,
			},
		},
	}
	assert.NoError(t, s.processTaskCandidate(c, now, cache, commitsBuf))
	assert.True(t, c.Score > 0)
	assert.Equal(t, 1, len(c.Commits))

	// Now, replace the time window to ensure that this next candidate runs
	// at a commit outside the window. Ensure that it gets the correct
	// blamelist.
	var err error
	s.window, err = window.New(time.Nanosecond, 0, nil)
	assert.NoError(t, err)
	c = &taskCandidate{
		TaskKey: db.TaskKey{
			RepoState: db.RepoState{
				Repo:     repoName,
				Revision: c2,
			},
		},
	}
	assert.NoError(t, s.processTaskCandidate(c, now, cache, commitsBuf))
	assert.Equal(t, 0, len(c.Commits))
}

func TestProcessTaskCandidates(t *testing.T) {
	tr, _, _, s, _ := setup(t)
	defer tr.Cleanup()

	ts := time.Now()

	// Processing of individual candidates is already tested; just verify
	// that if we pass in a bunch of candidates they all get processed.
	assertProcessed := func(c *taskCandidate) {
		if c.IsTryJob() {
			assert.True(t, c.Score > CANDIDATE_SCORE_TRY_JOB)
			assert.Nil(t, c.Commits)
		} else if c.IsForceRun() {
			assert.True(t, c.Score > CANDIDATE_SCORE_FORCE_RUN)
			assert.Equal(t, 1, len(c.Commits))
		} else if c.Revision == rs2.Revision {
			assert.True(t, c.Score >= 0)
			assert.True(t, len(c.Commits) > 0)
		} else {
			assert.Equal(t, c.Score, -1.0)
			assert.Equal(t, len(c.Commits), 0)
		}
	}

	candidates := map[string]map[string][]*taskCandidate{
		repoName: map[string][]*taskCandidate{
			buildTask: []*taskCandidate{
				&taskCandidate{
					JobCreated: ts,
					TaskKey: db.TaskKey{
						RepoState: rs1,
						Name:      buildTask,
					},
					TaskSpec: &specs.TaskSpec{},
				},
				&taskCandidate{
					JobCreated: ts,
					TaskKey: db.TaskKey{
						RepoState: rs2,
						Name:      buildTask,
					},
					TaskSpec: &specs.TaskSpec{},
				},
				&taskCandidate{
					JobCreated: ts,
					TaskKey: db.TaskKey{
						RepoState:   rs2,
						Name:        buildTask,
						ForcedJobId: "my-job",
					},
					TaskSpec: &specs.TaskSpec{},
				},
			},
			testTask: []*taskCandidate{
				&taskCandidate{
					JobCreated: ts,
					TaskKey: db.TaskKey{
						RepoState: rs1,
						Name:      testTask,
					},
					TaskSpec: &specs.TaskSpec{},
				},
				&taskCandidate{
					JobCreated: ts,
					TaskKey: db.TaskKey{
						RepoState: rs2,
						Name:      testTask,
					},
					TaskSpec: &specs.TaskSpec{},
				},
			},
			perfTask: []*taskCandidate{
				&taskCandidate{
					JobCreated: ts,
					TaskKey: db.TaskKey{
						RepoState: rs2,
						Name:      perfTask,
					},
					TaskSpec: &specs.TaskSpec{},
				},
				&taskCandidate{
					JobCreated: ts,
					TaskKey: db.TaskKey{
						RepoState: db.RepoState{
							Patch: db.Patch{
								Server:   "my-server",
								Issue:    "my-issue",
								Patchset: "my-patchset",
							},
							Repo:     repoName,
							Revision: c1,
						},
						Name: perfTask,
					},
					TaskSpec: &specs.TaskSpec{},
				},
			},
		},
	}

	processed, err := s.processTaskCandidates(candidates, time.Now())
	assert.NoError(t, err)
	for _, c := range processed {
		assertProcessed(c)
	}
	assert.Equal(t, 7, len(processed))
}

func TestTestedness(t *testing.T) {
	testutils.SmallTest(t)
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
	testutils.SmallTest(t)
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
	testutils.LargeTest(t)
	testutils.SkipIfShort(t)

	// Setup.
	tr := git_testutils.GitInit(t)
	defer tr.Cleanup()

	d := db.NewInMemoryTaskDB()
	w, err := window.New(time.Hour, 0, nil)
	cache, err := db.NewTaskCache(d, w)
	assert.NoError(t, err)

	// The test repo is laid out like this:
	//
	// *   O (HEAD, master, Case #10)
	// *   N
	// *   M (Case #11)
	// *   L
	// *   K (Case #7)
	// *   J (Case #6)
	// |\
	// | * I
	// | * H (Case #5)
	// * | G
	// * | F (Case #4)
	// * | E (Case #9)
	// |/
	// *   D (Case #3)
	// *   C (Case #2)
	// ...
	// *   B (Case #1)
	// *   A (Case #0)
	//
	hashes := map[string]string{}
	commit := func(file, name string) {
		hashes[name] = tr.CommitGenMsg(file, name)
	}

	// Initial commit.
	f := "somefile"
	f2 := "file2"
	commit(f, "A")

	type testCase struct {
		Revision     string
		Expected     []string
		StoleFromIdx int
	}

	name := "Test-Ubuntu12-ShuttleA-GTX660-x86-Release"
	repoDir, repoName := path.Split(tr.Dir())
	repo, err := repograph.NewGraph(repoName, repoDir)
	assert.NoError(t, err)
	ids := []string{}
	commitsBuf := make([]*repograph.Commit, 0, MAX_BLAMELIST_COMMITS)
	test := func(tc *testCase) {
		// Update the repo.
		assert.NoError(t, repo.Update())
		// Self-check: make sure we don't pass in empty commit hashes.
		for _, h := range tc.Expected {
			assert.NotEqual(t, h, "")
		}

		// Ensure that we get the expected blamelist.
		revision := repo.Get(tc.Revision)
		assert.NotNil(t, revision)
		commits, stoleFrom, err := ComputeBlamelist(cache, repo, name, repoName, revision, commitsBuf)
		if tc.Revision == "" {
			assert.Error(t, err)
			return
		} else {
			assert.NoError(t, err)
		}
		sort.Strings(commits)
		sort.Strings(tc.Expected)
		testutils.AssertDeepEqual(t, tc.Expected, commits)
		if tc.StoleFromIdx >= 0 {
			assert.NotNil(t, stoleFrom)
			assert.Equal(t, ids[tc.StoleFromIdx], stoleFrom.Id)
		} else {
			assert.Nil(t, stoleFrom)
		}

		// Insert the task into the DB.
		c := &taskCandidate{
			TaskKey: db.TaskKey{
				RepoState: db.RepoState{
					Repo:     repoName,
					Revision: tc.Revision,
				},
				Name: name,
			},
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
		ids = append(ids, task.Id)
		assert.NoError(t, cache.Update())
	}

	// Commit B.
	commit(f, "B")

	// Test cases. Each test case builds on the previous cases.

	// 0. First task of this spec, not at a branch head. Blamelist should be
	// empty.
	test(&testCase{
		Revision:     hashes["A"],
		Expected:     []string{},
		StoleFromIdx: -1,
	})

	// 1. The first task, at HEAD.
	test(&testCase{
		Revision:     hashes["B"],
		Expected:     []string{hashes["B"]}, // Task #1 is limited to a single commit.
		StoleFromIdx: -1,
	})

	// The above used a special case of "commit has no parents". Test the
	// other (blamelist too long) case by creating a bunch of commits.
	for i := 0; i < MAX_BLAMELIST_COMMITS+1; i++ {
		commit(f, "C")
	}
	commit(f, "D")

	// 2. Blamelist too long, not a branch head.
	test(&testCase{
		Revision:     hashes["C"],
		Expected:     []string{},
		StoleFromIdx: -1,
	})

	// 3. Blamelist too long, is a branch head.
	test(&testCase{
		Revision:     hashes["D"],
		Expected:     []string{hashes["D"]},
		StoleFromIdx: -1,
	})

	// Create the remaining commits.
	tr.CreateBranchTrackBranch("otherbranch", "master")
	tr.CheckoutBranch("master")
	commit(f, "E")
	commit(f, "F")
	commit(f, "G")
	tr.CheckoutBranch("otherbranch")
	commit(f2, "H")
	commit(f2, "I")
	tr.CheckoutBranch("master")
	hashes["J"] = tr.MergeBranch("otherbranch")
	commit(f, "K")

	// 4. On a linear set of commits, with at least one previous task.
	test(&testCase{
		Revision:     hashes["F"],
		Expected:     []string{hashes["E"], hashes["F"]},
		StoleFromIdx: -1,
	})
	// 5. The first task on a new branch.
	test(&testCase{
		Revision:     hashes["H"],
		Expected:     []string{hashes["H"]},
		StoleFromIdx: -1,
	})
	// 6. After a merge.
	test(&testCase{
		Revision:     hashes["J"],
		Expected:     []string{hashes["G"], hashes["I"], hashes["J"]},
		StoleFromIdx: -1,
	})
	// 7. One last "normal" task.
	test(&testCase{
		Revision:     hashes["K"],
		Expected:     []string{hashes["K"]},
		StoleFromIdx: -1,
	})
	// 8. Steal commits from a previously-ingested task.
	test(&testCase{
		Revision:     hashes["E"],
		Expected:     []string{hashes["E"]},
		StoleFromIdx: 4,
	})

	// Ensure that task #8 really stole the commit from #4.
	task, err := cache.GetTask(ids[4])
	assert.NoError(t, err)
	assert.False(t, util.In(hashes["E"], task.Commits), fmt.Sprintf("Expected not to find %s in %v", hashes["E"], task.Commits))

	// 9. Retry #8.
	test(&testCase{
		Revision:     hashes["E"],
		Expected:     []string{hashes["E"]},
		StoleFromIdx: 8,
	})

	// Ensure that task #9 really stole the commit from #8.
	task, err = cache.GetTask(ids[8])
	assert.NoError(t, err)
	assert.Equal(t, 0, len(task.Commits))

	// Four more commits.
	commit(f, "L")
	commit(f, "M")
	commit(f, "N")
	commit(f, "O")

	// 10. Not really a test case, but setting up for #11.
	test(&testCase{
		Revision:     hashes["O"],
		Expected:     []string{hashes["L"], hashes["M"], hashes["N"], hashes["O"]},
		StoleFromIdx: -1,
	})

	// 11. Steal *two* commits from #10.
	test(&testCase{
		Revision:     hashes["M"],
		Expected:     []string{hashes["L"], hashes["M"]},
		StoleFromIdx: 10,
	})
}

func TestTimeDecay24Hr(t *testing.T) {
	testutils.SmallTest(t)
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
	tr, d, _, s, _ := setup(t)
	defer tr.Cleanup()

	// Ensure that the queue is initially empty.
	assert.Equal(t, 0, len(s.queue))

	// Our test repo has a job pointing to every task.
	now := time.Now()
	j1 := &db.Job{
		Created:      now,
		Name:         "j1",
		Dependencies: map[string][]string{buildTask: []string{}},
		Priority:     0.5,
		RepoState:    rs1.Copy(),
	}
	j2 := &db.Job{
		Created:      now,
		Name:         "j2",
		Dependencies: map[string][]string{testTask: []string{buildTask}, buildTask: []string{}},
		Priority:     0.5,
		RepoState:    rs1.Copy(),
	}
	j3 := &db.Job{
		Created:      now,
		Name:         "j3",
		Dependencies: map[string][]string{buildTask: []string{}},
		Priority:     0.5,
		RepoState:    rs2.Copy(),
	}
	j4 := &db.Job{
		Created:      now,
		Name:         "j4",
		Dependencies: map[string][]string{testTask: []string{buildTask}, buildTask: []string{}},
		Priority:     0.5,
		RepoState:    rs2.Copy(),
	}
	j5 := &db.Job{
		Created:      now,
		Name:         "j5",
		Dependencies: map[string][]string{perfTask: []string{buildTask}, buildTask: []string{}},
		Priority:     0.5,
		RepoState:    rs2.Copy(),
	}
	assert.NoError(t, d.PutJobs([]*db.Job{j1, j2, j3, j4, j5}))
	assert.NoError(t, s.tCache.Update())
	assert.NoError(t, s.jCache.Update())

	// Regenerate the task queue.
	queue, err := s.regenerateTaskQueue(time.Now())
	assert.NoError(t, err)
	assert.Equal(t, 2, len(queue)) // Two Build tasks.

	testSort := func() {
		// Ensure that we sorted correctly.
		if len(queue) == 0 {
			return
		}
		highScore := queue[0].Score
		for _, c := range queue {
			assert.True(t, highScore >= c.Score)
			highScore = c.Score
		}
	}
	testSort()

	// Since we haven't run any task yet, we should have the two Build
	// tasks. The one at HEAD should have a single-commit blamelist and a
	// score of 2.0. The other should have no commits in its blamelist and
	// a score of -1.0.
	for _, c := range queue {
		assert.Equal(t, buildTask, c.Name)
		if c.Revision == c1 {
			assert.Equal(t, 0, len(c.Commits))
			assert.Equal(t, -1.0, c.Score)
		} else {
			assert.Equal(t, []string{c.Revision}, c.Commits)
			assert.Equal(t, 2.0, c.Score)
		}
	}

	// Insert the first task, even though it scored lower.
	var t1 *db.Task
	for _, c := range queue { // Order not guaranteed; find the right candidate.
		if c.Revision == c1 {
			t1 = makeTask(c.Name, c.Repo, c.Revision)
			break
		}
	}
	assert.NotNil(t, t1)
	t1.Status = db.TASK_STATUS_SUCCESS
	t1.IsolatedOutput = "fake isolated hash"
	assert.NoError(t, d.PutTask(t1))
	assert.NoError(t, s.tCache.Update())

	// Regenerate the task queue.
	queue, err = s.regenerateTaskQueue(time.Now())
	assert.NoError(t, err)

	// Now we expect the queue to contain the other Build task and the one
	// Test task we unblocked by running the first Build task.
	assert.Equal(t, 2, len(queue))
	testSort()
	for _, c := range queue {
		if c.Name == testTask {
			assert.Equal(t, -1.0, c.Score)
			assert.Equal(t, 0, len(c.Commits))
		} else {
			assert.Equal(t, c.Name, buildTask)
			assert.Equal(t, 2.0, c.Score)
			assert.Equal(t, []string{c.Revision}, c.Commits)
		}
	}
	buildIdx := 0
	testIdx := 1
	if queue[1].Name == buildTask {
		buildIdx = 1
		testIdx = 0
	}
	assert.Equal(t, buildTask, queue[buildIdx].Name)
	assert.Equal(t, c2, queue[buildIdx].Revision)

	assert.Equal(t, testTask, queue[testIdx].Name)
	assert.Equal(t, c1, queue[testIdx].Revision)

	// Run the other Build task.
	t2 := makeTask(queue[buildIdx].Name, queue[buildIdx].Repo, queue[buildIdx].Revision)
	t2.Status = db.TASK_STATUS_SUCCESS
	t2.IsolatedOutput = "fake isolated hash"
	assert.NoError(t, d.PutTask(t2))
	assert.NoError(t, s.tCache.Update())

	// Regenerate the task queue.
	queue, err = s.regenerateTaskQueue(time.Now())
	assert.NoError(t, err)
	assert.Equal(t, 3, len(queue))
	testSort()
	perfIdx := -1
	for i, c := range queue {
		if c.Name == perfTask {
			perfIdx = i
		}
		if c.Revision == c2 {
			assert.Equal(t, 2.0, c.Score)
			assert.Equal(t, []string{c.Revision}, c.Commits)
		} else {
			assert.Equal(t, c.Name, testTask)
			assert.Equal(t, -1.0, c.Score)
			assert.Equal(t, 0, len(c.Commits))
		}
	}
	assert.True(t, perfIdx > -1)

	// Run the Test task at tip of tree, but make its blamelist cover both
	// commits.
	t3 := makeTask(testTask, repoName, c2)
	t3.Commits = append(t3.Commits, c1)
	t3.Status = db.TASK_STATUS_SUCCESS
	t3.IsolatedOutput = "fake isolated hash"
	assert.NoError(t, d.PutTask(t3))
	assert.NoError(t, s.tCache.Update())

	// Regenerate the task queue.
	queue, err = s.regenerateTaskQueue(time.Now())
	assert.NoError(t, err)

	// Now we expect the queue to contain one Test and one Perf task. The
	// Test task is a backfill, and should have a score of 0.5.
	assert.Equal(t, 2, len(queue))
	testSort()
	// First candidate should be the perf task.
	assert.Equal(t, perfTask, queue[0].Name)
	assert.Equal(t, 2.0, queue[0].Score)
	// The test task is next, a backfill.
	assert.Equal(t, testTask, queue[1].Name)
	assert.Equal(t, 0.5, queue[1].Score)
}

func makeTaskCandidate(name string, dims []string) *taskCandidate {
	return &taskCandidate{
		Score: 1.0,
		TaskKey: db.TaskKey{
			Name: name,
		},
		TaskSpec: &specs.TaskSpec{
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
	testutils.MediumTest(t)
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
	tr, d, swarmingClient, s, _ := setup(t)
	defer tr.Cleanup()

	// Start testing. No free bots, so we get a full queue with nothing
	// scheduled.
	assert.NoError(t, s.MainLoop())
	tasks, err := s.tCache.GetTasksForCommits(repoName, []string{c1, c2})
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
	tasks, err = s.tCache.GetTasksForCommits(repoName, []string{c1, c2})
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
	assert.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.GetTasksForCommits(repoName, []string{c1, c2})
	assert.NoError(t, err)
	var t1 *db.Task
	for _, v := range tasks {
		for _, t := range v {
			t1 = t
			break
		}
	}
	assert.NotNil(t, t1)
	assert.Equal(t, c2, t1.Revision)
	assert.Equal(t, buildTask, t1.Name)
	assert.Equal(t, []string{c2}, t1.Commits)
	assert.Equal(t, 1, len(s.queue))

	// The task is complete.
	t1.Status = db.TASK_STATUS_SUCCESS
	t1.Finished = time.Now()
	t1.IsolatedOutput = "abc123"
	assert.NoError(t, d.PutTask(t1))
	swarmingClient.MockTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{
		makeSwarmingRpcsTaskRequestMetadata(t, t1, linuxTaskDims),
	})

	// No bots free. Ensure that the queue is correct.
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{})
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.GetTasksForCommits(repoName, []string{c1, c2})
	assert.NoError(t, err)
	expectLen := 3 // One remaining build task, plus one test task and one perf task.
	assert.Equal(t, expectLen, len(s.queue))

	// More bots than tasks free, ensure the queue is correct.
	bot2 := makeBot("bot2", androidTaskDims)
	bot3 := makeBot("bot3", androidTaskDims)
	bot4 := makeBot("bot4", linuxTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2, bot3, bot4})
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	_, err = s.tCache.GetTasksForCommits(repoName, []string{c1, c2})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(s.queue)) // One remaining build task.

	// The test task and perf task should have triggered.
	var t2 *db.Task
	var t3 *db.Task
	tasks, err = s.tCache.GetTasksForCommits(repoName, []string{c1, c2})
	assert.NoError(t, err)
	for _, v := range tasks {
		for _, task := range v {
			assert.Equal(t, c2, task.Revision)
			assert.Equal(t, []string{c2}, task.Commits)
			if task.Name == testTask {
				assert.Nil(t, t2)
				t2 = task
			} else if task.Name == perfTask {
				assert.Nil(t, t3)
				t3 = task
			} else {
				// The previously-finished build task.
				assert.Equal(t, buildTask, task.Name)
			}
		}
	}
	assert.NotNil(t, t2)
	assert.NotNil(t, t3)
	t2.Status = db.TASK_STATUS_SUCCESS
	t2.Finished = time.Now()
	t2.IsolatedOutput = "abc123"

	// No new bots free; only the remaining build task should be in the queue.
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{})
	mockTasks := []*swarming_api.SwarmingRpcsTaskRequestMetadata{
		makeSwarmingRpcsTaskRequestMetadata(t, t2, linuxTaskDims),
		makeSwarmingRpcsTaskRequestMetadata(t, t3, linuxTaskDims),
	}
	swarmingClient.MockTasks(mockTasks)
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	expectLen = 1 // Build task from c1
	assert.Equal(t, expectLen, len(s.queue))

	// Finish the other task.
	t3, err = s.tCache.GetTask(t3.Id)
	assert.NoError(t, err)
	t3.Status = db.TASK_STATUS_SUCCESS
	t3.Finished = time.Now()
	t3.IsolatedOutput = "abc123"

	// Ensure that we finalize all of the tasks and insert into the DB.
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2, bot3, bot4})
	mockTasks = []*swarming_api.SwarmingRpcsTaskRequestMetadata{
		makeSwarmingRpcsTaskRequestMetadata(t, t3, linuxTaskDims),
	}
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.GetTasksForCommits(repoName, []string{c1, c2})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks[c1]))
	assert.Equal(t, 3, len(tasks[c2]))
	assert.Equal(t, 1, len(s.queue)) // build task from c1.

	// Mark everything as finished. Ensure that the queue still ends up (almost) empty.
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
		mockTasks = append(mockTasks, makeSwarmingRpcsTaskRequestMetadata(t, task, linuxTaskDims))
	}
	swarmingClient.MockTasks(mockTasks)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2, bot3, bot4})
	assert.NoError(t, s.MainLoop())
	assert.Equal(t, 1, len(s.queue))
}

func makeDummyCommits(t *testing.T, repoDir string, numCommits int, branch string) {
	exec_testutils.Run(t, repoDir, "git", "config", "user.email", "test@skia.org")
	exec_testutils.Run(t, repoDir, "git", "config", "user.name", "Skia Tester")
	dummyFile := path.Join(repoDir, "dummyfile.txt")
	for i := 0; i < numCommits; i++ {
		title := fmt.Sprintf("Dummy #%d/%d", i, numCommits)
		assert.NoError(t, ioutil.WriteFile(dummyFile, []byte(title), os.ModePerm))
		exec_testutils.Run(t, repoDir, "git", "add", dummyFile)
		exec_testutils.Run(t, repoDir, "git", "commit", "-m", title)
		exec_testutils.Run(t, repoDir, "git", "push", "origin", branch)
	}
}

func TestSchedulerStealingFrom(t *testing.T) {
	tr, d, swarmingClient, s, _ := setup(t)
	defer tr.Cleanup()

	// Run the available compile task at c2.
	bot1 := makeBot("bot1", linuxTaskDims)
	bot2 := makeBot("bot2", linuxTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2})
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	tasks, err := s.tCache.GetTasksForCommits(repoName, []string{c1, c2})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks[c1]))
	assert.Equal(t, 1, len(tasks[c2]))

	// Finish the one task.
	tasksList := []*db.Task{}
	t1 := tasks[c2][buildTask]
	t1.Status = db.TASK_STATUS_SUCCESS
	t1.Finished = time.Now()
	t1.IsolatedOutput = "abc123"
	tasksList = append(tasksList, t1)

	// Forcibly create and insert a second task at c1.
	t2 := t1.Copy()
	t2.Id = "t2id"
	t2.Revision = c1
	t2.Commits = []string{c1}
	tasksList = append(tasksList, t2)

	assert.NoError(t, d.PutTasks(tasksList))
	assert.NoError(t, s.tCache.Update())

	// Add some commits.
	repoDir := path.Join(tr.Dir, repoName)
	exec_testutils.Run(t, repoDir, "git", "checkout", "master")
	makeDummyCommits(t, repoDir, 10, "master")
	assert.NoError(t, s.repos[repoName].Repo().Update())
	commits, err := s.repos[repoName].Repo().RevList("HEAD")
	assert.NoError(t, err)

	// Run one task. Ensure that it's at tip-of-tree.
	head, err := s.repos[repoName].Repo().RevParse("HEAD")
	assert.NoError(t, err)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.GetTasksForCommits(repoName, commits)
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
	assert.NoError(t, s.tCache.Update())

	oldTasksByCommit := tasks

	// Run backfills, ensuring that each one steals the right set of commits
	// from previous builds, until all of the build task candidates have run.
	for i := 0; i < 9; i++ {
		// Now, run another task. The new task should bisect the old one.
		swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
		assert.NoError(t, s.MainLoop())
		assert.NoError(t, s.tCache.Update())
		tasks, err = s.tCache.GetTasksForCommits(repoName, commits)
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
		updatedOldTask, err := s.tCache.GetTask(oldTask.Id)
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
		assert.NoError(t, s.tCache.Update())
		oldTasksByCommit = tasks

	}

	// Ensure that we're really done.
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.GetTasksForCommits(repoName, commits)
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
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)

	workdir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, workdir)
	assert.NoError(t, os.Mkdir(path.Join(workdir, TRIGGER_DIRNAME), os.ModePerm))

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
	cfg := &specs.TasksCfg{
		Tasks: map[string]*specs.TaskSpec{
			taskName: &specs.TaskSpec{
				CipdPackages: []*specs.CipdPackage{},
				Dependencies: []string{},
				Dimensions:   []string{"pool:Skia"},
				Isolate:      "dummy.isolate",
				Priority:     1.0,
			},
		},
		Jobs: map[string]*specs.JobSpec{
			"j1": &specs.JobSpec{
				TaskSpecs: []string{taskName},
			},
		},
	}
	f, err := os.Create(path.Join(repoDir, specs.TASKS_CFG_FILE))
	assert.NoError(t, err)
	assert.NoError(t, json.NewEncoder(f).Encode(&cfg))
	assert.NoError(t, f.Close())
	run(repoDir, "git", "add", specs.TASKS_CFG_FILE)
	run(repoDir, "git", "commit", "-m", "Add more tasks!")
	run(repoDir, "git", "push", "origin", "master")
	run(repoDir, "git", "branch", "-u", "origin/master")

	// Setup the scheduler.
	d := db.NewInMemoryDB()
	isolateClient, err := isolate.NewClient(workdir)
	assert.NoError(t, err)
	isolateClient.ServerUrl = isolate.FAKE_SERVER_URL
	swarmingClient := swarming.NewTestClient()
	repo, err := repograph.NewGraph(repoName, workdir)
	assert.NoError(t, err)
	repos := repograph.Map{
		repoName: repo,
	}
	s, err := NewTaskScheduler(d, time.Duration(math.MaxInt64), 0, workdir, repos, isolateClient, swarmingClient, mockhttpclient.NewURLMock().Client(), 1.0, tryjobs.API_URL_TESTING, tryjobs.BUCKET_TESTING, projectRepoMapping)
	assert.NoError(t, err)

	mockTasks := []*swarming_api.SwarmingRpcsTaskRequestMetadata{}
	mock := func(task *db.Task) {
		task.Status = db.TASK_STATUS_SUCCESS
		task.Finished = time.Now()
		task.IsolatedOutput = "abc123"
		mockTasks = append(mockTasks, makeSwarmingRpcsTaskRequestMetadata(t, task, linuxTaskDims))
		swarmingClient.MockTasks(mockTasks)
	}

	// Cycle once.
	bot1 := makeBot("bot1", map[string]string{"pool": "Skia"})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	assert.Equal(t, 0, len(s.queue))
	head, err := s.repos[repoName].Repo().RevParse("HEAD")
	assert.NoError(t, err)
	tasks, err := s.tCache.GetTasksForCommits(repoName, []string{head})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks[head]))
	mock(tasks[head][taskName])

	// Add some commits to the repo.
	exec_testutils.Run(t, repoDir, "git", "checkout", "master")
	makeDummyCommits(t, repoDir, 8, "master")
	assert.NoError(t, s.repos[repoName].Repo().Update())
	commits, err := s.repos[repoName].Repo().RevList(fmt.Sprintf("%s..HEAD", head))
	assert.Nil(t, err)
	assert.Equal(t, 8, len(commits))

	// Trigger builds simultaneously.
	bot2 := makeBot("bot2", map[string]string{"pool": "Skia"})
	bot3 := makeBot("bot3", map[string]string{"pool": "Skia"})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2, bot3})
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	assert.Equal(t, 5, len(s.queue))
	tasks, err = s.tCache.GetTasksForCommits(repoName, commits)
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
	assert.NoError(t, s.tCache.Update())
	assert.Equal(t, 0, len(s.queue))
	tasks, err = s.tCache.GetTasksForCommits(repoName, commits)
	assert.NoError(t, err)
	for _, byName := range tasks {
		for _, task := range byName {
			assert.Equal(t, 1, len(task.Commits))
		}
	}
}

func TestSchedulingRetry(t *testing.T) {
	tr, d, swarmingClient, s, _ := setup(t)
	defer tr.Cleanup()

	// Run the available compile task at c2.
	bot1 := makeBot("bot1", linuxTaskDims)
	bot2 := makeBot("bot2", linuxTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2})
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	tasks, err := s.tCache.UnfinishedTasks()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks))
	t1 := tasks[0]
	assert.NotNil(t, t1)
	// Forcibly add a second build task at c1.
	t2 := t1.Copy()
	t2.Id = "t2Id"
	t2.Revision = c1
	t2.Commits = []string{c1}

	// One task successful, the other not.
	t1.Status = db.TASK_STATUS_FAILURE
	t1.Finished = time.Now()
	t2.Status = db.TASK_STATUS_SUCCESS
	t2.Finished = time.Now()
	t2.IsolatedOutput = "abc123"

	assert.NoError(t, d.PutTasks([]*db.Task{t1, t2}))
	assert.NoError(t, s.tCache.Update())

	// Cycle. Ensure that we schedule a retry of t1.
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.UnfinishedTasks()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks))
	t3 := tasks[0]
	assert.NotNil(t, t3)
	assert.Equal(t, t1.Id, t3.RetryOf)

	// The retry failed. Ensure that we don't schedule another.
	t3.Status = db.TASK_STATUS_FAILURE
	t3.Finished = time.Now()
	assert.NoError(t, d.PutTask(t3))
	assert.NoError(t, s.tCache.Update())
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.UnfinishedTasks()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))
}

func TestParentTaskId(t *testing.T) {
	tr, d, swarmingClient, s, _ := setup(t)
	defer tr.Cleanup()

	// Run the available compile task at c2.
	bot1 := makeBot("bot1", linuxTaskDims)
	bot2 := makeBot("bot2", linuxTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2})
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	tasks, err := s.tCache.UnfinishedTasks()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks))
	t1 := tasks[0]
	t1.Status = db.TASK_STATUS_SUCCESS
	t1.Finished = time.Now()
	t1.IsolatedOutput = "abc123"
	assert.Equal(t, 0, len(t1.ParentTaskIds))
	assert.NoError(t, d.PutTasks([]*db.Task{t1}))
	assert.NoError(t, s.tCache.Update())

	// Run the dependent tasks. Ensure that their parent IDs are correct.
	bot3 := makeBot("bot3", androidTaskDims)
	bot4 := makeBot("bot4", androidTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot3, bot4})
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.UnfinishedTasks()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(tasks))
	for _, task := range tasks {
		assert.Equal(t, 1, len(task.ParentTaskIds))
		p := task.ParentTaskIds[0]
		assert.Equal(t, p, t1.Id)

		updated, err := task.UpdateFromSwarming(makeSwarmingRpcsTaskRequestMetadata(t, task, linuxTaskDims).TaskResult)
		assert.NoError(t, err)
		assert.False(t, updated)
	}
}

func TestBlacklist(t *testing.T) {
	// The blacklist has its own tests, so this test just verifies that it's
	// actually integrated into the scheduler.
	tr, _, swarmingClient, s, _ := setup(t)
	defer tr.Cleanup()

	// Mock some bots, add one of the build tasks to the blacklist.
	bot1 := makeBot("bot1", linuxTaskDims)
	bot2 := makeBot("bot2", linuxTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2})
	assert.NoError(t, s.GetBlacklist().AddRule(&blacklist.Rule{
		AddedBy:          "Tests",
		TaskSpecPatterns: []string{".*"},
		Commits:          []string{c1},
		Description:      "desc",
		Name:             "My-Rule",
	}, s.repos))
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	tasks, err := s.tCache.UnfinishedTasks()
	assert.NoError(t, err)
	// The blacklisted commit should not have been triggered.
	assert.Equal(t, 1, len(tasks))
	assert.NotEqual(t, c1, tasks[0].Revision)
}

func TestTrybots(t *testing.T) {
	tr, d, swarmingClient, s, mock := setup(t)
	defer tr.Cleanup()

	// The trybot integrator has its own tests, so just verify that we can
	// receive a try request, execute the necessary tasks, and report its
	// results back.

	// Run ourselves out of tasks.
	bot1 := makeBot("bot1", linuxTaskDims)
	bot2 := makeBot("bot2", androidTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2})
	now := time.Now()

	n := 0
	for i := 0; i < 10; i++ {
		assert.NoError(t, s.MainLoop())
		assert.NoError(t, s.tCache.Update())
		tasks, err := s.tCache.UnfinishedTasks()
		assert.NoError(t, err)
		if len(tasks) == 0 {
			break
		}
		for _, t := range tasks {
			t.Status = db.TASK_STATUS_SUCCESS
			t.Finished = now
			t.IsolatedOutput = "abc123"
			n++
		}
		assert.NoError(t, d.PutTasks(tasks))
		assert.NoError(t, s.tCache.Update())
	}
	assert.Equal(t, 3, n)
	jobs, err := s.jCache.UnfinishedJobs()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(jobs)) // Jobs at c1 never finish.

	// Create a try job.
	tryjobs.MockOutExec()
	defer exec.SetRunForTesting(exec.DefaultRun)

	b := tryjobs.Build(t, now)
	rs := db.RepoState{
		Patch: db.Patch{
			Server:   "https://codereview.chromium.org/",
			Issue:    "10001",
			Patchset: "20002",
		},
		Repo:     rs1.Repo,
		Revision: rs1.Revision,
	}
	b.ParametersJson = testutils.MarshalJSON(t, tryjobs.Params(t, testTask, "skia", rs.Revision, rs.Server, rs.Issue, rs.Patchset))
	tryjobs.MockPeek(mock, []*buildbucket_api.ApiBuildMessage{b}, now, "", "", nil)
	tryjobs.MockTryLeaseBuild(mock, b.Id, now, nil)
	tryjobs.MockJobStarted(mock, b.Id, now, nil)
	assert.NoError(t, s.tryjobs.Poll(now))
	assert.True(t, mock.Empty())

	// Ensure that we added a Job.
	assert.NoError(t, s.jCache.Update())
	jobs, err = s.jCache.UnfinishedJobs()
	assert.NoError(t, err)
	assert.Equal(t, 3, len(jobs))
	var tryJob *db.Job
	for _, j := range jobs {
		if j.IsTryJob() {
			tryJob = j
			break
		}
	}
	assert.NotNil(t, tryJob)
	assert.False(t, tryJob.Done())

	// Run through the try job's tasks.
	for i := 0; i < 10; i++ {
		assert.NoError(t, s.MainLoop())
		assert.NoError(t, s.tCache.Update())
		tasks, err := s.tCache.UnfinishedTasks()
		assert.NoError(t, err)
		if len(tasks) == 0 {
			break
		}
		for _, task := range tasks {
			assert.Equal(t, rs, task.RepoState)
			task.Status = db.TASK_STATUS_SUCCESS
			task.Finished = now
			task.IsolatedOutput = "abc123"
			n++
		}
		assert.NoError(t, d.PutTasks(tasks))
		assert.NoError(t, s.tCache.Update())
	}
	assert.True(t, mock.Empty())

	// Some final checks.
	assert.NoError(t, s.jCache.Update())
	assert.Equal(t, 5, n)
	tryJob, err = s.jCache.GetJob(tryJob.Id)
	assert.NoError(t, err)
	assert.True(t, tryJob.IsTryJob())
	assert.True(t, tryJob.Done())
	assert.True(t, tryJob.Finished.After(tryJob.Created))
	jobs, err = s.jCache.UnfinishedJobs()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(jobs)) // Jobs at c1 never finish.
}

func TestGetTasksForJob(t *testing.T) {
	tr, d, swarmingClient, s, _ := setup(t)
	defer tr.Cleanup()

	// Cycle once, check that we have empty sets for all Jobs.
	assert.NoError(t, s.MainLoop())
	jobs, err := s.jCache.UnfinishedJobs()
	assert.NoError(t, err)
	assert.Equal(t, 5, len(jobs))
	var j1, j2, j3, j4, j5 *db.Job
	for _, j := range jobs {
		if j.Revision == c1 {
			if j.Name == buildTask {
				j1 = j
			} else {
				j2 = j
			}
		} else {
			if j.Name == buildTask {
				j3 = j
			} else if j.Name == testTask {
				j4 = j
			} else {
				j5 = j
			}
		}
		tasksByName, err := s.getTasksForJob(j)
		assert.NoError(t, err)
		for _, tasks := range tasksByName {
			assert.Equal(t, 0, len(tasks))
		}
	}
	assert.NotNil(t, j1)
	assert.NotNil(t, j2)
	assert.NotNil(t, j3)
	assert.NotNil(t, j4)
	assert.NotNil(t, j5)

	// Run the available compile task at c2.
	bot1 := makeBot("bot1", linuxTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	tasks, err := s.tCache.UnfinishedTasks()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks))
	t1 := tasks[0]
	assert.NotNil(t, t1)
	assert.Equal(t, t1.Revision, c2)

	// Test that we get the new tasks where applicable.
	expect := map[string]map[string][]*db.Task{
		j1.Id: map[string][]*db.Task{
			buildTask: {},
		},
		j2.Id: map[string][]*db.Task{
			buildTask: {},
			testTask:  {},
		},
		j3.Id: map[string][]*db.Task{
			buildTask: {t1},
		},
		j4.Id: map[string][]*db.Task{
			buildTask: {t1},
			testTask:  {},
		},
		j5.Id: map[string][]*db.Task{
			buildTask: {t1},
			perfTask:  {},
		},
	}
	for _, j := range jobs {
		tasksByName, err := s.getTasksForJob(j)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, expect[j.Id], tasksByName)
	}

	// Mark the task successful.
	t1.Status = db.TASK_STATUS_FAILURE
	t1.Finished = time.Now()
	assert.NoError(t, d.PutTasks([]*db.Task{t1}))
	assert.NoError(t, s.tCache.Update())

	// Test that the results propagated through.
	for _, j := range jobs {
		tasksByName, err := s.getTasksForJob(j)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, expect[j.Id], tasksByName)
	}

	// Cycle. Ensure that we schedule a retry of t1.
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.UnfinishedTasks()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tasks))
	t2 := tasks[0]
	assert.NotNil(t, t2)
	assert.Equal(t, t1.Id, t2.RetryOf)

	// Verify that both the original t1 and its retry show up.
	t1, err = s.tCache.GetTask(t1.Id) // t1 was updated.
	assert.NoError(t, err)
	expect[j3.Id][buildTask] = []*db.Task{t1, t2}
	expect[j4.Id][buildTask] = []*db.Task{t1, t2}
	expect[j5.Id][buildTask] = []*db.Task{t1, t2}
	for _, j := range jobs {
		tasksByName, err := s.getTasksForJob(j)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, expect[j.Id], tasksByName)
	}

	// The retry succeeded.
	t2.Status = db.TASK_STATUS_SUCCESS
	t2.Finished = time.Now()
	t2.IsolatedOutput = "abc"
	assert.NoError(t, d.PutTask(t2))
	assert.NoError(t, s.tCache.Update())
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.UnfinishedTasks()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(tasks))

	// Test that the results propagated through.
	for _, j := range jobs {
		tasksByName, err := s.getTasksForJob(j)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, expect[j.Id], tasksByName)
	}

	// Schedule the remaining tasks.
	bot3 := makeBot("bot3", androidTaskDims)
	bot4 := makeBot("bot4", androidTaskDims)
	bot5 := makeBot("bot5", androidTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot3, bot4, bot5})
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())

	// Verify that the new tasks show up.
	tasks, err = s.tCache.UnfinishedTasks()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(tasks)) // Test and perf at c2.
	var t4, t5 *db.Task
	for _, task := range tasks {
		if task.Name == testTask {
			t4 = task
		} else {
			t5 = task
		}
	}
	expect[j4.Id][testTask] = []*db.Task{t4}
	expect[j5.Id][perfTask] = []*db.Task{t5}
	for _, j := range jobs {
		tasksByName, err := s.getTasksForJob(j)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, expect[j.Id], tasksByName)
	}
}

func TestTaskTimeouts(t *testing.T) {
	tr, _, swarmingClient, s, _ := setup(t)
	defer tr.Cleanup()

	// The test repo does not set any timeouts. Ensure that we get
	// reasonable default values.
	bot1 := makeBot("bot1", map[string]string{"pool": "Skia", "os": "Ubuntu", "gpu": "none"})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	unfinished, err := s.tCache.UnfinishedTasks()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(unfinished))
	task := unfinished[0]
	swarmingTask, err := swarmingClient.GetTaskMetadata(task.SwarmingTaskId)
	assert.NoError(t, err)
	// These are the defaults in go/swarming/swarming.go.
	assert.Equal(t, int64(60*60), swarmingTask.Request.Properties.ExecutionTimeoutSecs)
	assert.Equal(t, int64(20*60), swarmingTask.Request.Properties.IoTimeoutSecs)
	assert.Equal(t, int64(4*60*60), swarmingTask.Request.ExpirationSecs)
	// Fail the task to get it out of the unfinished list.
	task.Status = db.TASK_STATUS_FAILURE
	assert.NoError(t, s.db.PutTask(task))

	// Rewrite tasks.json with some timeouts.
	name := "Timeout-Task"
	cfg := &specs.TasksCfg{
		Jobs: map[string]*specs.JobSpec{
			"Timeout-Job": &specs.JobSpec{
				Priority:  1.0,
				TaskSpecs: []string{name},
			},
		},
		Tasks: map[string]*specs.TaskSpec{
			name: &specs.TaskSpec{
				CipdPackages: []*specs.CipdPackage{},
				Dependencies: []string{},
				Dimensions: []string{
					"pool:Skia",
					"os:Mac",
					"gpu:my-gpu",
				},
				ExecutionTimeout: 40 * time.Minute,
				Expiration:       2 * time.Hour,
				IoTimeout:        3 * time.Minute,
				Isolate:          "compile_skia.isolate",
				Priority:         1.0,
			},
		},
	}
	repoDir := path.Join(tr.Dir, "skia.git")
	tasksJson := path.Join(repoDir, specs.TASKS_CFG_FILE)
	testutils.WriteFile(t, tasksJson, testutils.MarshalJSON(t, &cfg))
	exec_testutils.Run(t, repoDir, "git", "add", specs.TASKS_CFG_FILE)
	exec_testutils.Run(t, repoDir, "git", "commit", "-m", "timeouts")

	// Cycle, ensure that we get the expected timeouts.
	bot2 := makeBot("bot2", map[string]string{"pool": "Skia", "os": "Mac", "gpu": "my-gpu"})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot2})
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.tCache.Update())
	unfinished, err = s.tCache.UnfinishedTasks()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(unfinished))
	task = unfinished[0]
	assert.Equal(t, name, task.Name)
	swarmingTask, err = swarmingClient.GetTaskMetadata(task.SwarmingTaskId)
	assert.NoError(t, err)
	assert.Equal(t, int64(40*60), swarmingTask.Request.Properties.ExecutionTimeoutSecs)
	assert.Equal(t, int64(3*60), swarmingTask.Request.Properties.IoTimeoutSecs)
	assert.Equal(t, int64(2*60*60), swarmingTask.Request.ExpirationSecs)
}

func TestPeriodicJobs(t *testing.T) {
	tr, _, _, s, _ := setup(t)
	defer tr.Cleanup()

	// Rewrite tasks.json with a periodic job.
	name := "Periodic-Task"
	cfg := &specs.TasksCfg{
		Jobs: map[string]*specs.JobSpec{
			"Periodic-Job": &specs.JobSpec{
				Priority:  1.0,
				TaskSpecs: []string{name},
				Trigger:   "nightly",
			},
		},
		Tasks: map[string]*specs.TaskSpec{
			name: &specs.TaskSpec{
				CipdPackages: []*specs.CipdPackage{},
				Dependencies: []string{},
				Dimensions: []string{
					"pool:Skia",
					"os:Mac",
					"gpu:my-gpu",
				},
				ExecutionTimeout: 40 * time.Minute,
				Expiration:       2 * time.Hour,
				IoTimeout:        3 * time.Minute,
				Isolate:          "compile_skia.isolate",
				Priority:         1.0,
			},
		},
	}
	repoDir := path.Join(tr.Dir, "skia.git")
	tasksJson := path.Join(repoDir, specs.TASKS_CFG_FILE)
	testutils.WriteFile(t, tasksJson, testutils.MarshalJSON(t, &cfg))
	exec_testutils.Run(t, repoDir, "git", "add", specs.TASKS_CFG_FILE)
	exec_testutils.Run(t, repoDir, "git", "commit", "-m", "nightly-job")

	// Cycle, ensure that the periodic task is not added.
	assert.NoError(t, s.MainLoop())
	assert.NoError(t, s.jCache.Update())
	unfinished, err := s.jCache.UnfinishedJobs()
	assert.NoError(t, err)
	assert.Equal(t, 5, len(unfinished)) // Existing per-commit jobs.
	assert.Equal(t, 0, len(s.triggerMetrics.LastTriggered))
	assert.Equal(t, 0, len(s.triggerMetrics.metrics))

	// Write the trigger file. Cycle, ensure that the trigger file was
	// removed and the periodic task was added.
	triggerFile := path.Join(tr.Dir, TRIGGER_DIRNAME, "nightly")
	assert.NoError(t, ioutil.WriteFile(triggerFile, []byte{}, os.ModePerm))
	assert.NoError(t, s.MainLoop())
	_, err = os.Stat(triggerFile)
	assert.True(t, os.IsNotExist(err))
	assert.NoError(t, s.jCache.Update())
	unfinished, err = s.jCache.UnfinishedJobs()
	assert.NoError(t, err)
	assert.Equal(t, 6, len(unfinished))
	assert.Equal(t, 1, len(s.triggerMetrics.LastTriggered))
	assert.Equal(t, 1, len(s.triggerMetrics.metrics))

	// Test the metrics by creating a new periodicTriggerMetrics and
	// verifying that we get the same last-triggered time.
	metrics, err := newPeriodicTriggerMetrics(s.workdir)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(metrics.LastTriggered))
	assert.Equal(t, 1, len(metrics.metrics))
	assert.Equal(t, s.triggerMetrics.LastTriggered["nightly"].Unix(), metrics.LastTriggered["nightly"].Unix())
}

func TestUpdateUnfinishedTasks(t *testing.T) {
	tr, _, swarmingClient, s, _ := setup(t)
	defer tr.Cleanup()

	// Create a few tasks.
	now := time.Unix(1480683321, 0).UTC()
	t1 := &db.Task{
		Id:             "t1",
		Created:        now.Add(-time.Minute),
		Status:         db.TASK_STATUS_RUNNING,
		SwarmingTaskId: "swarmt1",
	}
	t2 := &db.Task{
		Id:             "t2",
		Created:        now.Add(-10 * time.Minute),
		Status:         db.TASK_STATUS_PENDING,
		SwarmingTaskId: "swarmt2",
	}
	t3 := &db.Task{
		Id:             "t3",
		Created:        now.Add(-5 * time.Hour), // Outside the 4-hour window.
		Status:         db.TASK_STATUS_PENDING,
		SwarmingTaskId: "swarmt3",
	}

	// Insert the tasks into the DB.
	tasks := []*db.Task{t1, t2, t3}
	assert.NoError(t, s.db.PutTasks(tasks))
	assert.NoError(t, s.tCache.Update())

	// Update the tasks, mock in Swarming.
	t1.Status = db.TASK_STATUS_SUCCESS
	t2.Status = db.TASK_STATUS_FAILURE
	t3.Status = db.TASK_STATUS_SUCCESS

	m1 := makeSwarmingRpcsTaskRequestMetadata(t, t1, linuxTaskDims)
	m2 := makeSwarmingRpcsTaskRequestMetadata(t, t2, linuxTaskDims)
	m3 := makeSwarmingRpcsTaskRequestMetadata(t, t3, linuxTaskDims)
	swarmingClient.MockTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{m1, m2, m3})

	// Assert that the third task doesn't show up in the time range query.
	got, err := swarmingClient.ListTasks(now.Add(-4*time.Hour), now, []string{"pool:Skia"}, "")
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*swarming_api.SwarmingRpcsTaskRequestMetadata{m1, m2}, got)

	// Ensure that we update the tasks as expected.
	assert.NoError(t, s.updateUnfinishedTasks(now))
	for _, task := range tasks {
		got, err := s.db.GetTaskById(task.Id)
		assert.NoError(t, err)
		// Ignore DbModified when comparing.
		task.DbModified = got.DbModified
		testutils.AssertDeepEqual(t, task, got)
	}
}

// setupAddTasksTest calls setup then adds 7 commits to the repo and returns
// their hashes.
func setupAddTasksTest(t *testing.T) ([]string, db.DB, *TaskScheduler, func()) {
	tr, d, _, s, _ := setup(t)

	// Add some commits to test blamelist calculation.
	dir := path.Join(tr.Dir, "skia.git")
	exec_testutils.Run(t, dir, "git", "checkout", "master")
	makeDummyCommits(t, dir, 7, "master")
	assert.NoError(t, s.updateRepos())
	hashes, err := s.repos[repoName].Repo().RevList("HEAD")
	assert.NoError(t, err)

	return hashes, d, s, func() {
		tr.Cleanup()
	}
}

// assertBlamelist asserts task.Commits contains exactly the hashes at the given
// indexes of hashes, in any order.
func assertBlamelist(t *testing.T, hashes []string, task *db.Task, indexes []int) {
	expected := util.NewStringSet()
	for _, idx := range indexes {
		expected[hashes[idx]] = true
	}
	assert.Equal(t, expected, util.NewStringSet(task.Commits))
}

// assertModifiedTasks asserts that the result of GetModifiedTasks is deep-equal
// to expected, in any order.
func assertModifiedTasks(t *testing.T, d db.TaskReader, id string, expected []*db.Task) {
	tasksById := map[string]*db.Task{}
	modTasks, err := d.GetModifiedTasks(id)
	assert.NoError(t, err)
	assert.Equal(t, len(expected), len(modTasks))
	for _, task := range modTasks {
		tasksById[task.Id] = task
	}

	assert.Equal(t, len(expected), len(tasksById))
	for i, expectedTask := range expected {
		actualTask, ok := tasksById[expectedTask.Id]
		assert.True(t, ok, "Missing task; idx %d; id %s", i, expectedTask.Id)
		testutils.AssertDeepEqual(t, expectedTask, actualTask)
	}
}

// addTasksSingleTaskSpec should add tasks and compute simple blamelists.
func TestAddTasksSingleTaskSpecSimple(t *testing.T) {
	hashes, d, s, cleanup := setupAddTasksTest(t)
	defer cleanup()

	t1 := makeTask("toil", repoName, hashes[6])
	assert.NoError(t, d.PutTask(t1))
	assert.NoError(t, s.tCache.Update())

	trackId, err := d.StartTrackingModifiedTasks()
	assert.NoError(t, err)

	t2 := makeTask("toil", repoName, hashes[5]) // Commits should be {5}
	t3 := makeTask("toil", repoName, hashes[3]) // Commits should be {3, 4}
	t4 := makeTask("toil", repoName, hashes[2]) // Commits should be {2}
	t5 := makeTask("toil", repoName, hashes[0]) // Commits should be {0, 1}

	// Clear Commits on some tasks, set incorrect Commits on others to
	// ensure it's ignored.
	t3.Commits = nil
	t4.Commits = []string{hashes[5], hashes[4], hashes[3], hashes[2]}
	sort.Strings(t4.Commits)

	// Specify tasks in wrong order to ensure results are deterministic.
	assert.NoError(t, s.addTasksSingleTaskSpec([]*db.Task{t5, t2, t3, t4}))

	assertBlamelist(t, hashes, t2, []int{5})
	assertBlamelist(t, hashes, t3, []int{3, 4})
	assertBlamelist(t, hashes, t4, []int{2})
	assertBlamelist(t, hashes, t5, []int{0, 1})

	// Check that the tasks were inserted into the DB.
	assertModifiedTasks(t, d, trackId, []*db.Task{t2, t3, t4, t5})
}

// addTasksSingleTaskSpec should compute blamelists when new tasks bisect each
// other.
func TestAddTasksSingleTaskSpecBisectNew(t *testing.T) {
	hashes, d, s, cleanup := setupAddTasksTest(t)
	defer cleanup()

	t1 := makeTask("toil", repoName, hashes[6])
	assert.NoError(t, d.PutTask(t1))
	assert.NoError(t, s.tCache.Update())

	trackId, err := d.StartTrackingModifiedTasks()
	assert.NoError(t, err)

	// t2.Commits = {1, 2, 3, 4, 5}
	t2 := makeTask("toil", repoName, hashes[1])
	// t3.Commits = {3, 4, 5}
	// t2.Commits = {1, 2}
	t3 := makeTask("toil", repoName, hashes[3])
	// t4.Commits = {4, 5}
	// t3.Commits = {3}
	t4 := makeTask("toil", repoName, hashes[4])
	// t5.Commits = {0}
	t5 := makeTask("toil", repoName, hashes[0])
	// t6.Commits = {2}
	// t2.Commits = {1}
	t6 := makeTask("toil", repoName, hashes[2])
	// t7.Commits = {1}
	// t2.Commits = {}
	t7 := makeTask("toil", repoName, hashes[1])

	// Specify tasks in wrong order to ensure results are deterministic.
	tasks := []*db.Task{t5, t2, t7, t3, t6, t4}

	// Assign Ids.
	for _, task := range tasks {
		assert.NoError(t, d.AssignId(task))
	}

	assert.NoError(t, s.addTasksSingleTaskSpec(tasks))

	assertBlamelist(t, hashes, t2, []int{})
	assertBlamelist(t, hashes, t3, []int{3})
	assertBlamelist(t, hashes, t4, []int{4, 5})
	assertBlamelist(t, hashes, t5, []int{0})
	assertBlamelist(t, hashes, t6, []int{2})
	assertBlamelist(t, hashes, t7, []int{1})

	// Check that the tasks were inserted into the DB.
	assertModifiedTasks(t, d, trackId, tasks)
}

// addTasksSingleTaskSpec should compute blamelists when new tasks bisect old
// tasks.
func TestAddTasksSingleTaskSpecBisectOld(t *testing.T) {
	hashes, d, s, cleanup := setupAddTasksTest(t)
	defer cleanup()

	t1 := makeTask("toil", repoName, hashes[6])
	t2 := makeTask("toil", repoName, hashes[1])
	t2.Commits = []string{hashes[1], hashes[2], hashes[3], hashes[4], hashes[5]}
	sort.Strings(t2.Commits)
	assert.NoError(t, d.PutTasks([]*db.Task{t1, t2}))
	assert.NoError(t, s.tCache.Update())

	trackId, err := d.StartTrackingModifiedTasks()
	assert.NoError(t, err)

	// t3.Commits = {3, 4, 5}
	// t2.Commits = {1, 2}
	t3 := makeTask("toil", repoName, hashes[3])
	// t4.Commits = {4, 5}
	// t3.Commits = {3}
	t4 := makeTask("toil", repoName, hashes[4])
	// t5.Commits = {2}
	// t2.Commits = {1}
	t5 := makeTask("toil", repoName, hashes[2])
	// t6.Commits = {4, 5}
	// t4.Commits = {}
	t6 := makeTask("toil", repoName, hashes[4])

	// Specify tasks in wrong order to ensure results are deterministic.
	assert.NoError(t, s.addTasksSingleTaskSpec([]*db.Task{t5, t3, t6, t4}))

	t2Updated, err := d.GetTaskById(t2.Id)
	assert.NoError(t, err)
	assertBlamelist(t, hashes, t2Updated, []int{1})
	assertBlamelist(t, hashes, t3, []int{3})
	assertBlamelist(t, hashes, t4, []int{})
	assertBlamelist(t, hashes, t5, []int{2})
	assertBlamelist(t, hashes, t6, []int{4, 5})

	// Check that the tasks were inserted into the DB.
	t2.Commits = t2Updated.Commits
	t2.DbModified = t2Updated.DbModified
	assertModifiedTasks(t, d, trackId, []*db.Task{t2, t3, t4, t5, t6})
}

// addTasksSingleTaskSpec should update existing tasks, keeping the correct
// blamelist.
func TestAddTasksSingleTaskSpecUpdate(t *testing.T) {
	hashes, d, s, cleanup := setupAddTasksTest(t)
	defer cleanup()

	t1 := makeTask("toil", repoName, hashes[6])
	t2 := makeTask("toil", repoName, hashes[3])
	t3 := makeTask("toil", repoName, hashes[4])
	t3.Commits = nil // Stolen by t5
	t4 := makeTask("toil", repoName, hashes[0])
	t4.Commits = []string{hashes[0], hashes[1], hashes[2]}
	sort.Strings(t4.Commits)
	t5 := makeTask("toil", repoName, hashes[4])
	t5.Commits = []string{hashes[4], hashes[5]}
	sort.Strings(t5.Commits)

	tasks := []*db.Task{t1, t2, t3, t4, t5}
	assert.NoError(t, d.PutTasks(tasks))
	assert.NoError(t, s.tCache.Update())

	trackId, err := d.StartTrackingModifiedTasks()
	assert.NoError(t, err)

	// Make an update.
	for _, task := range tasks {
		task.Status = db.TASK_STATUS_MISHAP
	}

	// Specify tasks in wrong order to ensure results are deterministic.
	assert.NoError(t, s.addTasksSingleTaskSpec([]*db.Task{t5, t3, t1, t4, t2}))

	// Check that blamelists did not change.
	assertBlamelist(t, hashes, t1, []int{6})
	assertBlamelist(t, hashes, t2, []int{3})
	assertBlamelist(t, hashes, t3, []int{})
	assertBlamelist(t, hashes, t4, []int{0, 1, 2})
	assertBlamelist(t, hashes, t5, []int{4, 5})

	// Check that the tasks were inserted into the DB.
	assertModifiedTasks(t, d, trackId, []*db.Task{t1, t2, t3, t4, t5})
}

// AddTasks should call addTasksSingleTaskSpec for each group of tasks.
func TestAddTasks(t *testing.T) {
	hashes, d, s, cleanup := setupAddTasksTest(t)
	defer cleanup()

	toil1 := makeTask("toil", repoName, hashes[6])
	duty1 := makeTask("duty", repoName, hashes[6])
	work1 := makeTask("work", repoName, hashes[6])
	work2 := makeTask("work", repoName, hashes[1])
	work2.Commits = []string{hashes[1], hashes[2], hashes[3], hashes[4], hashes[5]}
	sort.Strings(work2.Commits)
	onus1 := makeTask("onus", repoName, hashes[6])
	onus2 := makeTask("onus", repoName, hashes[3])
	onus2.Commits = []string{hashes[3], hashes[4], hashes[5]}
	sort.Strings(onus2.Commits)
	assert.NoError(t, d.PutTasks([]*db.Task{toil1, duty1, work1, work2, onus1, onus2}))

	trackId, err := d.StartTrackingModifiedTasks()
	assert.NoError(t, err)

	// toil2.Commits = {5}
	toil2 := makeTask("toil", repoName, hashes[5])
	// toil3.Commits = {3, 4}
	toil3 := makeTask("toil", repoName, hashes[3])

	// duty2.Commits = {1, 2, 3, 4, 5}
	duty2 := makeTask("duty", repoName, hashes[1])
	// duty3.Commits = {3, 4, 5}
	// duty2.Commits = {1, 2}
	duty3 := makeTask("duty", repoName, hashes[3])

	// work3.Commits = {3, 4, 5}
	// work2.Commits = {1, 2}
	work3 := makeTask("work", repoName, hashes[3])
	// work4.Commits = {2}
	// work2.Commits = {1}
	work4 := makeTask("work", repoName, hashes[2])

	onus2.Status = db.TASK_STATUS_MISHAP
	// onus3 steals all commits from onus2
	onus3 := makeTask("onus", repoName, hashes[3])
	// onus4 steals all commits from onus3
	onus4 := makeTask("onus", repoName, hashes[3])

	tasks := map[string]map[string][]*db.Task{
		repoName: map[string][]*db.Task{
			"toil": []*db.Task{toil2, toil3},
			"duty": []*db.Task{duty2, duty3},
			"work": []*db.Task{work3, work4},
			"onus": []*db.Task{onus2, onus3, onus4},
		},
	}

	assert.NoError(t, s.AddTasks(tasks))

	assertBlamelist(t, hashes, toil2, []int{5})
	assertBlamelist(t, hashes, toil3, []int{3, 4})

	assertBlamelist(t, hashes, duty2, []int{1, 2})
	assertBlamelist(t, hashes, duty3, []int{3, 4, 5})

	work2Updated, err := d.GetTaskById(work2.Id)
	assert.NoError(t, err)
	assertBlamelist(t, hashes, work2Updated, []int{1})
	assertBlamelist(t, hashes, work3, []int{3, 4, 5})
	assertBlamelist(t, hashes, work4, []int{2})

	assertBlamelist(t, hashes, onus2, []int{})
	assertBlamelist(t, hashes, onus3, []int{})
	assertBlamelist(t, hashes, onus4, []int{3, 4, 5})

	// Check that the tasks were inserted into the DB.
	work2.Commits = work2Updated.Commits
	work2.DbModified = work2Updated.DbModified
	assertModifiedTasks(t, d, trackId, []*db.Task{toil2, toil3, duty2, duty3, work2, work3, work4, onus2, onus3, onus4})
}

// AddTasks should not leave DB in an inconsistent state if there is a partial error.
func TestAddTasksFailure(t *testing.T) {
	hashes, d, s, cleanup := setupAddTasksTest(t)
	defer cleanup()

	toil1 := makeTask("toil", repoName, hashes[6])
	duty1 := makeTask("duty", repoName, hashes[6])
	duty2 := makeTask("duty", repoName, hashes[5])
	assert.NoError(t, d.PutTasks([]*db.Task{toil1, duty1, duty2}))

	// Cause ErrConcurrentUpdate in AddTasks.
	cachedDuty2 := duty2.Copy()
	duty2.Status = db.TASK_STATUS_MISHAP
	assert.NoError(t, d.PutTask(duty2))

	trackId, err := d.StartTrackingModifiedTasks()
	assert.NoError(t, err)

	// toil2.Commits = {3, 4, 5}
	toil2 := makeTask("toil", repoName, hashes[3])

	cachedDuty2.Status = db.TASK_STATUS_FAILURE
	// duty3.Commits = {3, 4}
	duty3 := makeTask("duty", repoName, hashes[3])

	tasks := map[string]map[string][]*db.Task{
		repoName: map[string][]*db.Task{
			"toil": []*db.Task{toil2},
			"duty": []*db.Task{cachedDuty2, duty3},
		},
	}

	// Try multiple times to reduce chance of test passing flakily.
	for i := 0; i < 3; i++ {
		err := s.AddTasks(tasks)
		assert.Error(t, err)
		assert.True(t, db.IsConcurrentUpdate(err))
		modTasks, err := d.GetModifiedTasks(trackId)
		assert.NoError(t, err)
		// "duty" tasks should never be updated.
		for _, task := range modTasks {
			assert.Equal(t, "toil", task.Name)
			testutils.AssertDeepEqual(t, toil2, task)
			assertBlamelist(t, hashes, toil2, []int{3, 4, 5})
		}
	}

	duty2.Status = db.TASK_STATUS_FAILURE
	tasks[repoName]["duty"] = []*db.Task{duty2, duty3}
	assert.NoError(t, s.AddTasks(tasks))

	assertBlamelist(t, hashes, toil2, []int{3, 4, 5})

	assertBlamelist(t, hashes, duty2, []int{5})
	assertBlamelist(t, hashes, duty3, []int{3, 4})

	// Check that the tasks were inserted into the DB.
	assertModifiedTasks(t, d, trackId, []*db.Task{toil2, duty2, duty3})
}
