package scheduling

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/require"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/deepequal/assertdeep"
	depot_tools_testutils "go.skia.org/infra/go/depot_tools/testutils"
	skfs "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gcs/mem_gcsclient"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/blacklist"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/db/memory"
	"go.skia.org/infra/task_scheduler/go/isolate_cache"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	tcc_testutils "go.skia.org/infra/task_scheduler/go/task_cfg_cache/testutils"
	swarming_testutils "go.skia.org/infra/task_scheduler/go/testutils"
	"go.skia.org/infra/task_scheduler/go/tryjobs"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

const (
	fakeGerritUrl = "https://fake-skia-review.googlesource.com"

	scoreDelta = 0.000001
)

var (
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
		"pool":        {"Skia"},
		"os":          {"Android"},
		"device_type": {"grouper"},
	}

	linuxBotDims = map[string][]string{
		"os":   {"Ubuntu"},
		"pool": {"Skia"},
	}
)

func getCommit(t *testing.T, ctx context.Context, gb *git_testutils.GitBuilder, commit string) string {
	co := git.Checkout{
		GitDir: git.GitDir(gb.Dir()),
	}
	rv, err := co.RevParse(ctx, commit)
	require.NoError(t, err)
	return rv
}

func getRS1(t *testing.T, ctx context.Context, gb *git_testutils.GitBuilder) types.RepoState {
	return types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: getCommit(t, ctx, gb, "HEAD^"),
	}
}

func getRS2(t *testing.T, ctx context.Context, gb *git_testutils.GitBuilder) types.RepoState {
	return types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: getCommit(t, ctx, gb, "HEAD"),
	}
}

func makeTask(name, repo, revision string) *types.Task {
	return &types.Task{
		Commits: []string{revision},
		Created: time.Now(),
		TaskKey: types.TaskKey{
			RepoState: types.RepoState{
				Repo:     repo,
				Revision: revision,
			},
			Name: name,
		},
		MaxAttempts:    types.DEFAULT_MAX_TASK_ATTEMPTS,
		SwarmingTaskId: "swarmid",
	}
}

func makeSwarmingRpcsTaskRequestMetadata(t *testing.T, task *types.Task, dims map[string]string) *swarming_api.SwarmingRpcsTaskRequestMetadata {
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
	state := swarming.TASK_STATE_PENDING
	failed := false
	switch task.Status {
	case types.TASK_STATUS_MISHAP:
		state = swarming.TASK_STATE_BOT_DIED
		abandoned = ts(task.Finished)
	case types.TASK_STATUS_RUNNING:
		state = swarming.TASK_STATE_RUNNING
	case types.TASK_STATUS_FAILURE:
		state = swarming.TASK_STATE_COMPLETED
		failed = true
	case types.TASK_STATUS_SUCCESS:
		state = swarming.TASK_STATE_COMPLETED
	case types.TASK_STATUS_PENDING:
		// noop
	default:
		require.FailNow(t, "Unknown task status: %s", task.Status)
	}
	tags := []string{
		tag(types.SWARMING_TAG_ID, task.Id),
		tag(types.SWARMING_TAG_FORCED_JOB_ID, task.ForcedJobId),
		tag(types.SWARMING_TAG_NAME, task.Name),
		tag(swarming.DIMENSION_POOL_KEY, swarming.DIMENSION_POOL_VALUE_SKIA),
		tag(types.SWARMING_TAG_REPO, task.Repo),
		tag(types.SWARMING_TAG_REVISION, task.Revision),
	}
	for _, p := range task.ParentTaskIds {
		tags = append(tags, tag(types.SWARMING_TAG_PARENT_TASK_ID, p))
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
func setup(t *testing.T) (context.Context, *git_testutils.GitBuilder, *memory.InMemoryDB, *swarming_testutils.TestClient, *TaskScheduler, *mockhttpclient.URLMock, func()) {
	unittest.LargeTest(t)

	ctx, gb, _, _ := tcc_testutils.SetupTestRepo(t)
	ctx, cancel := context.WithCancel(ctx)

	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	d := memory.NewInMemoryDB()
	isolateClient, err := isolate.NewClient(tmp, isolate.ISOLATE_SERVER_URL_FAKE)
	require.NoError(t, err)
	swarmingClient := swarming_testutils.NewTestClient()
	urlMock := mockhttpclient.NewURLMock()
	repos, err := repograph.NewLocalMap(ctx, []string{gb.RepoUrl()}, tmp)
	require.NoError(t, err)
	require.NoError(t, repos.Update(ctx))
	projectRepoMapping := map[string]string{
		"skia": gb.RepoUrl(),
	}
	depotTools := depot_tools_testutils.GetDepotTools(t, ctx)
	gitcookies := path.Join(tmp, "gitcookies_fake")
	require.NoError(t, ioutil.WriteFile(gitcookies, []byte(".googlesource.com\tTRUE\t/\tTRUE\t123\to\tgit-user.google.com=abc123"), os.ModePerm))
	g, err := gerrit.NewGerrit(fakeGerritUrl, gitcookies, urlMock.Client())
	require.NoError(t, err)
	btProject, btInstance, btCleanup := tcc_testutils.SetupBigTable(t)
	btCleanupIsolate := isolate_cache.SetupSharedBigTable(t, btProject, btInstance)
	s, err := NewTaskScheduler(ctx, d, nil, time.Duration(math.MaxInt64), 0, tmp, "fake.server", repos, isolateClient, swarmingClient, urlMock.Client(), 1.0, tryjobs.API_URL_TESTING, tryjobs.BUCKET_TESTING, projectRepoMapping, swarming.POOLS_PUBLIC, "", depotTools, g, btProject, btInstance, nil, mem_gcsclient.New("diag_unit_tests"), btInstance)
	require.NoError(t, err)
	return ctx, gb, d, swarmingClient, s, urlMock, func() {
		testutils.AssertCloses(t, s)
		testutils.RemoveAll(t, tmp)
		gb.Cleanup()
		btCleanupIsolate()
		btCleanup()
		cancel()
	}
}

// runMainLoop calls s.MainLoop, asserts there was no error, and waits for
// background goroutines to finish.
func runMainLoop(t *testing.T, s *TaskScheduler, ctx context.Context) {
	require.NoError(t, s.MainLoop(ctx))
	s.testWaitGroup.Wait()
}

func lastDiagnostics(t *testing.T, s *TaskScheduler) taskSchedulerMainLoopDiagnostics {
	ctx := context.Background()
	lastname := ""
	require.NoError(t, s.diagClient.AllFilesInDirectory(ctx, path.Join(s.diagInstance, GCS_MAIN_LOOP_DIAGNOSTICS_DIR), func(item *storage.ObjectAttrs) {
		if lastname == "" || item.Name > lastname {
			lastname = item.Name
		}
	}))
	require.NotEqual(t, lastname, "")
	reader, err := s.diagClient.FileReader(ctx, lastname)
	require.NoError(t, err)
	defer testutils.AssertCloses(t, reader)
	rv := taskSchedulerMainLoopDiagnostics{}
	require.NoError(t, json.NewDecoder(reader).Decode(&rv))
	return rv
}

func updateRepos(t *testing.T, ctx context.Context, s *TaskScheduler) {
	acked := false
	ack := func() {
		acked = true
	}
	nack := func() {
		require.FailNow(t, "Should not have called nack()")
	}
	err := s.repos.UpdateWithCallback(ctx, func(repoUrl string, g *repograph.Graph) error {
		return s.HandleRepoUpdate(ctx, repoUrl, g, ack, nack)
	})
	require.NoError(t, err)
	require.True(t, acked)
}

func TestGatherNewJobs(t *testing.T) {
	ctx, gb, _, _, s, _, cleanup := setup(t)
	defer cleanup()

	testGatherNewJobs := func(expectedJobs int) {
		updateRepos(t, ctx, s)
		jobs, err := s.jCache.UnfinishedJobs()
		require.NoError(t, err)
		require.Equal(t, expectedJobs, len(jobs))
	}

	// Ensure that the JobDB is empty.
	jobs, err := s.jCache.UnfinishedJobs()
	require.NoError(t, err)
	require.Equal(t, 0, len(jobs))

	// Run gatherNewJobs, ensure that we added jobs for all commits in the
	// repo.
	testGatherNewJobs(5) // c1 has 2 jobs, c2 has 3 jobs.

	// Run gatherNewJobs again, ensure that we didn't add the same Jobs
	// again.
	testGatherNewJobs(5) // no new jobs == 5 total jobs.

	// Add a commit on master, run gatherNewJobs, ensure that we added the
	// new Jobs.
	makeDummyCommits(ctx, gb, 1)
	updateRepos(t, ctx, s)
	testGatherNewJobs(8) // we didn't add to the jobs spec, so 3 jobs/rev.

	// Add several commits on master, ensure that we added all of the Jobs.
	makeDummyCommits(ctx, gb, 10)
	updateRepos(t, ctx, s)
	testGatherNewJobs(38) // 3 jobs/rev + 8 pre-existing jobs.

	// Add a commit on a branch other than master, run gatherNewJobs, ensure
	// that we added the new Jobs.
	branchName := "otherBranch"
	gb.CreateBranchTrackBranch(ctx, branchName, "master")
	msg := "Branch commit"
	fileName := "some_other_file"
	gb.Add(ctx, fileName, msg)
	gb.Commit(ctx)
	updateRepos(t, ctx, s)
	testGatherNewJobs(41) // 38 previous jobs + 3 new ones.

	// Add several commits in a row on different branches, ensure that we
	// added all of the Jobs for all of the new commits.
	makeDummyCommits(ctx, gb, 5)
	gb.CheckoutBranch(ctx, "master")
	makeDummyCommits(ctx, gb, 5)
	updateRepos(t, ctx, s)
	testGatherNewJobs(71) // 10 commits x 3 jobs/commit = 30, plus 41

	// Add one more commit on the non-master branch which marks all but one
	// job to only run on master. Ensure that we don't pick them up.
	gb.CheckoutBranch(ctx, branchName)
	cfg, err := specs.ReadTasksCfg(gb.Dir())
	require.NoError(t, err)
	for name, jobSpec := range cfg.Jobs {
		if name != tcc_testutils.BuildTaskName {
			jobSpec.Trigger = specs.TRIGGER_MASTER_ONLY
		}
	}
	cfgBytes, err := specs.EncodeTasksCfg(cfg)
	require.NoError(t, err)
	gb.Add(ctx, "infra/bots/tasks.json", string(cfgBytes))
	gb.CommitMsgAt(ctx, "abcd", time.Now())
	updateRepos(t, ctx, s)
	testGatherNewJobs(72)
}

func TestFindTaskCandidatesForJobs(t *testing.T) {
	ctx, gb, _, _, s, _, cleanup := setup(t)
	defer cleanup()

	test := func(jobs []*types.Job, expect map[types.TaskKey]*taskCandidate) {
		actual, err := s.findTaskCandidatesForJobs(ctx, jobs)
		require.NoError(t, err)
		assertdeep.Equal(t, actual, expect)
	}

	rs1 := getRS1(t, ctx, gb)
	rs2 := getRS2(t, ctx, gb)

	// Get all of the task specs, for future use.
	_, err := s.cacher.GetOrCacheRepoState(ctx, rs1)
	require.NoError(t, err)
	_, err = s.cacher.GetOrCacheRepoState(ctx, rs2)
	require.NoError(t, err)
	cfg1, err := s.taskCfgCache.Get(ctx, rs1)
	require.NoError(t, err)
	cfg2, err := s.taskCfgCache.Get(ctx, rs2)
	require.NoError(t, err)

	// Run on an empty job list, ensure empty list returned.
	test([]*types.Job{}, map[types.TaskKey]*taskCandidate{})

	now := time.Now().UTC()

	// Run for one job, ensure that we get the right set of task specs
	// returned (ie. all dependencies and their dependencies).
	j1 := &types.Job{
		Created:      now,
		Id:           "job1id",
		Name:         "j1",
		Dependencies: map[string][]string{tcc_testutils.TestTaskName: {tcc_testutils.BuildTaskName}, tcc_testutils.BuildTaskName: {}},
		Priority:     0.5,
		RepoState:    rs1.Copy(),
	}
	tc1 := &taskCandidate{
		Jobs: []*types.Job{j1},
		TaskKey: types.TaskKey{
			RepoState: rs1.Copy(),
			Name:      tcc_testutils.BuildTaskName,
		},
		TaskSpec: cfg1.Tasks[tcc_testutils.BuildTaskName].Copy(),
	}
	tc2 := &taskCandidate{
		Jobs: []*types.Job{j1},
		TaskKey: types.TaskKey{
			RepoState: rs1.Copy(),
			Name:      tcc_testutils.TestTaskName,
		},
		TaskSpec: cfg1.Tasks[tcc_testutils.TestTaskName].Copy(),
	}

	test([]*types.Job{j1}, map[types.TaskKey]*taskCandidate{
		tc1.TaskKey: tc1,
		tc2.TaskKey: tc2,
	})

	// Add a job, ensure that its dependencies are added and that the right
	// dependencies are de-duplicated.
	j2 := &types.Job{
		Created:      now,
		Id:           "job2id",
		Name:         "j2",
		Dependencies: map[string][]string{tcc_testutils.TestTaskName: {tcc_testutils.BuildTaskName}, tcc_testutils.BuildTaskName: {}},
		Priority:     0.6,
		RepoState:    rs2,
	}
	j3 := &types.Job{
		Created:      now,
		Id:           "job3id",
		Name:         "j3",
		Dependencies: map[string][]string{tcc_testutils.PerfTaskName: {tcc_testutils.BuildTaskName}, tcc_testutils.BuildTaskName: {}},
		Priority:     0.6,
		RepoState:    rs2,
	}
	tc3 := &taskCandidate{
		Jobs: []*types.Job{j2, j3},
		TaskKey: types.TaskKey{
			RepoState: rs2.Copy(),
			Name:      tcc_testutils.BuildTaskName,
		},
		TaskSpec: cfg2.Tasks[tcc_testutils.BuildTaskName].Copy(),
	}
	tc4 := &taskCandidate{
		Jobs: []*types.Job{j2},
		TaskKey: types.TaskKey{
			RepoState: rs2.Copy(),
			Name:      tcc_testutils.TestTaskName,
		},
		TaskSpec: cfg2.Tasks[tcc_testutils.TestTaskName].Copy(),
	}
	tc5 := &taskCandidate{
		Jobs: []*types.Job{j3},
		TaskKey: types.TaskKey{
			RepoState: rs2.Copy(),
			Name:      tcc_testutils.PerfTaskName,
		},
		TaskSpec: cfg2.Tasks[tcc_testutils.PerfTaskName].Copy(),
	}
	allCandidates := map[types.TaskKey]*taskCandidate{
		tc1.TaskKey: tc1,
		tc2.TaskKey: tc2,
		tc3.TaskKey: tc3,
		tc4.TaskKey: tc4,
		tc5.TaskKey: tc5,
	}
	test([]*types.Job{j1, j2, j3}, allCandidates)

	// Finish j3, ensure that its task specs no longer show up.
	delete(allCandidates, j3.MakeTaskKey(tcc_testutils.PerfTaskName))
	// This is hacky, but findTaskCandidatesForJobs accepts an already-
	// filtered list of jobs, so we have to pretend it never existed.
	tc3.Jobs = tc3.Jobs[:1]
	test([]*types.Job{j1, j2}, allCandidates)

	// Ensure that we don't generate candidates for jobs at nonexistent commits.
	j4 := &types.Job{
		Created:      now,
		Id:           "job4id",
		Name:         "j4",
		Dependencies: map[string][]string{tcc_testutils.PerfTaskName: {tcc_testutils.BuildTaskName}, tcc_testutils.BuildTaskName: {}},
		Priority:     0.6,
		RepoState: types.RepoState{
			Repo:     rs2.Repo,
			Revision: "aaaaabbbbbcccccdddddeeeeefffff1111122222",
		},
	}
	test([]*types.Job{j4}, map[types.TaskKey]*taskCandidate{})
}

func TestFilterTaskCandidates(t *testing.T) {
	ctx, gb, _, _, s, _, cleanup := setup(t)
	defer cleanup()

	rs1 := getRS1(t, ctx, gb)
	rs2 := getRS2(t, ctx, gb)
	c1 := rs1.Revision
	c2 := rs2.Revision

	// Fake out the initial candidates.
	k1 := types.TaskKey{
		RepoState: rs1,
		Name:      tcc_testutils.BuildTaskName,
	}
	k2 := types.TaskKey{
		RepoState: rs1,
		Name:      tcc_testutils.TestTaskName,
	}
	k3 := types.TaskKey{
		RepoState: rs2,
		Name:      tcc_testutils.BuildTaskName,
	}
	k4 := types.TaskKey{
		RepoState: rs2,
		Name:      tcc_testutils.TestTaskName,
	}
	k5 := types.TaskKey{
		RepoState: rs2,
		Name:      tcc_testutils.PerfTaskName,
	}
	candidates := map[types.TaskKey]*taskCandidate{
		k1: {
			TaskKey:  k1,
			TaskSpec: &specs.TaskSpec{},
		},
		k2: {
			TaskKey: k2,
			TaskSpec: &specs.TaskSpec{
				Dependencies: []string{tcc_testutils.BuildTaskName},
			},
		},
		k3: {
			TaskKey:  k3,
			TaskSpec: &specs.TaskSpec{},
		},
		k4: {
			TaskKey: k4,
			TaskSpec: &specs.TaskSpec{
				Dependencies: []string{tcc_testutils.BuildTaskName},
			},
		},
		k5: {
			TaskKey: k5,
			TaskSpec: &specs.TaskSpec{
				Dependencies: []string{tcc_testutils.BuildTaskName},
			},
		},
	}

	clearDiagnostics := func(candidates map[types.TaskKey]*taskCandidate) {
		for _, c := range candidates {
			c.Diagnostics = nil
		}
	}

	// Check the initial set of task candidates. The two Build tasks
	// should be the only ones available.
	c, err := s.filterTaskCandidates(candidates)
	require.NoError(t, err)
	require.Equal(t, 1, len(c))
	require.Equal(t, 1, len(c[gb.RepoUrl()]))
	require.Equal(t, 2, len(c[gb.RepoUrl()][tcc_testutils.BuildTaskName]))
	for _, byRepo := range c {
		for _, byName := range byRepo {
			for _, candidate := range byName {
				require.Equal(t, candidate.Name, tcc_testutils.BuildTaskName)
			}
		}
	}
	// Check filtering diagnostics. Non-Build tasks have unmet dependencies.
	for _, candidate := range candidates {
		if candidate.Name != tcc_testutils.BuildTaskName {
			require.Equal(t, candidate.Diagnostics.Filtering.UnmetDependencies, []string{tcc_testutils.BuildTaskName})
		}
	}

	// Insert a the Build task at c1 (1 dependent) into the database,
	// transition through various states.
	var t1 *types.Task
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
	require.NotNil(t, t1)

	// We shouldn't duplicate pending or running tasks.
	for _, status := range []types.TaskStatus{types.TASK_STATUS_PENDING, types.TASK_STATUS_RUNNING} {
		clearDiagnostics(candidates)

		t1.Status = status
		require.NoError(t, s.putTask(t1))

		c, err = s.filterTaskCandidates(candidates)
		require.NoError(t, err)
		require.Equal(t, 1, len(c))
		for _, byRepo := range c {
			for _, byName := range byRepo {
				for _, candidate := range byName {
					require.Equal(t, candidate.Name, tcc_testutils.BuildTaskName)
					require.Equal(t, c2, candidate.Revision)
				}
			}
		}
		// Check filtering diagnostics.
		for _, candidate := range candidates {
			if candidate.Name != tcc_testutils.BuildTaskName {
				// Non-Build tasks have unmet dependencies.
				require.Equal(t, candidate.Diagnostics.Filtering.UnmetDependencies, []string{tcc_testutils.BuildTaskName})
			} else if candidate.Revision == c1 {
				// Blocked by t1
				require.Equal(t, candidate.Diagnostics.Filtering.SupersededByTask, t1.Id)
			}
		}
	}

	clearDiagnostics(candidates)

	// The task failed. Ensure that its dependents are not candidates, but
	// the task itself is back in the list of candidates, in case we want
	// to retry.
	t1.Status = types.TASK_STATUS_FAILURE
	require.NoError(t, s.putTask(t1))

	c, err = s.filterTaskCandidates(candidates)
	require.NoError(t, err)
	require.Equal(t, 1, len(c))
	for _, byRepo := range c {
		require.Equal(t, 1, len(byRepo))
		for _, byName := range byRepo {
			require.Equal(t, 2, len(byName))
			for _, candidate := range byName {
				require.Equal(t, candidate.Name, tcc_testutils.BuildTaskName)
			}
		}
	}
	// Check filtering diagnostics.
	for _, candidate := range candidates {
		if candidate.Name != tcc_testutils.BuildTaskName {
			// Non-Build tasks have unmet dependencies.
			require.Equal(t, candidate.Diagnostics.Filtering.UnmetDependencies, []string{tcc_testutils.BuildTaskName})
		}
	}

	clearDiagnostics(candidates)

	// The task succeeded. Ensure that its dependents are candidates and
	// the task itself is not.
	t1.Status = types.TASK_STATUS_SUCCESS
	t1.IsolatedOutput = "fake isolated hash"
	require.NoError(t, s.putTask(t1))

	c, err = s.filterTaskCandidates(candidates)
	require.NoError(t, err)
	require.Equal(t, 1, len(c))
	for _, byRepo := range c {
		require.Equal(t, 2, len(byRepo))
		for _, byName := range byRepo {
			for _, candidate := range byName {
				require.False(t, t1.Name == candidate.Name && t1.Revision == candidate.Revision)
			}
		}
	}
	// Candidate with k1 is blocked by t1.
	require.Equal(t, candidates[k1].Diagnostics.Filtering.SupersededByTask, t1.Id)

	clearDiagnostics(candidates)

	// Create the other Build task.
	var t2 *types.Task
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
	require.NotNil(t, t2)
	t2.Status = types.TASK_STATUS_SUCCESS
	t2.IsolatedOutput = "fake isolated hash"
	require.NoError(t, s.putTask(t2))

	// All test and perf tasks are now candidates, no build tasks.
	c, err = s.filterTaskCandidates(candidates)
	require.NoError(t, err)
	require.Equal(t, 1, len(c))
	require.Equal(t, 2, len(c[gb.RepoUrl()][tcc_testutils.TestTaskName]))
	require.Equal(t, 1, len(c[gb.RepoUrl()][tcc_testutils.PerfTaskName]))
	for _, byRepo := range c {
		for _, byName := range byRepo {
			for _, candidate := range byName {
				require.NotEqual(t, candidate.Name, tcc_testutils.BuildTaskName)
			}
		}
	}
	// Build candidates are blocked by completed tasks.
	require.Equal(t, candidates[k1].Diagnostics.Filtering.SupersededByTask, t1.Id)
	require.Equal(t, candidates[k3].Diagnostics.Filtering.SupersededByTask, t2.Id)

	clearDiagnostics(candidates)

	// Add a try job. Ensure that no deps have been incorrectly satisfied.
	tryKey := k4.Copy()
	tryKey.Server = "dummy-server"
	tryKey.Issue = "dummy-issue"
	tryKey.Patchset = "dummy-patchset"
	candidates[tryKey] = &taskCandidate{
		TaskKey: tryKey,
		TaskSpec: &specs.TaskSpec{
			Dependencies: []string{tcc_testutils.BuildTaskName},
		},
	}
	c, err = s.filterTaskCandidates(candidates)
	require.NoError(t, err)
	require.Equal(t, 1, len(c))
	require.Equal(t, 2, len(c[gb.RepoUrl()][tcc_testutils.TestTaskName]))
	require.Equal(t, 1, len(c[gb.RepoUrl()][tcc_testutils.PerfTaskName]))
	for _, byRepo := range c {
		for _, byName := range byRepo {
			for _, candidate := range byName {
				require.NotEqual(t, candidate.Name, tcc_testutils.BuildTaskName)
				require.False(t, candidate.IsTryJob())
			}
		}
	}
	// Check diagnostics for tryKey
	require.Equal(t, candidates[tryKey].Diagnostics.Filtering.UnmetDependencies, []string{tcc_testutils.BuildTaskName})
}

func TestProcessTaskCandidate(t *testing.T) {
	ctx, gb, _, _, s, _, cleanup := setup(t)
	defer cleanup()

	c1 := getRS1(t, ctx, gb).Revision
	c2 := getRS2(t, ctx, gb).Revision

	cache := newCacheWrapper(s.tCache)
	now := time.Unix(0, 1470674884000000)
	commitsBuf := make([]*repograph.Commit, 0, MAX_BLAMELIST_COMMITS)

	checkDiagTryForced := func(c *taskCandidate, diag *taskCandidateScoringDiagnostics) {
		require.Equal(t, c.Jobs[0].Priority, diag.Priority)
		require.Equal(t, now.Sub(c.Jobs[0].Created).Hours(), diag.JobCreatedHours)
		// The remaining fields should always be 0 for try/forced jobs.
		require.Equal(t, 0, diag.StoleFromCommits)
		require.Equal(t, 0.0, diag.TestednessIncrease)
		require.Equal(t, 0.0, diag.TimeDecay)
	}

	tryjobRs := types.RepoState{
		Patch: types.Patch{
			Server:   "my-server",
			Issue:    "my-issue",
			Patchset: "my-patchset",
		},
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}
	tryjob := &types.Job{
		Id:        "tryjobId",
		Created:   now.Add(-1 * time.Hour),
		Name:      "job",
		Priority:  0.5,
		RepoState: tryjobRs,
	}
	c := &taskCandidate{
		Jobs: []*types.Job{tryjob},
		TaskKey: types.TaskKey{
			Name:      tcc_testutils.BuildTaskName,
			RepoState: tryjobRs,
		},
	}
	diag := &taskCandidateScoringDiagnostics{}
	require.NoError(t, s.processTaskCandidate(ctx, c, now, cache, commitsBuf, diag))
	// Try job candidates have a specific score and no blamelist.
	require.InDelta(t, (CANDIDATE_SCORE_TRY_JOB+1.0)*0.5, c.Score, scoreDelta)
	require.Nil(t, c.Commits)
	checkDiagTryForced(c, diag)

	// Retries are scored lower.
	c = &taskCandidate{
		Attempt: 1,
		Jobs:    []*types.Job{tryjob},
		TaskKey: types.TaskKey{
			Name:      tcc_testutils.BuildTaskName,
			RepoState: tryjobRs,
		},
	}
	diag = &taskCandidateScoringDiagnostics{}
	require.NoError(t, s.processTaskCandidate(ctx, c, now, cache, commitsBuf, diag))
	require.InDelta(t, (CANDIDATE_SCORE_TRY_JOB+1.0)*0.5*CANDIDATE_SCORE_TRY_JOB_RETRY_MULTIPLIER, c.Score, scoreDelta)
	require.Nil(t, c.Commits)
	checkDiagTryForced(c, diag)

	rs2 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c2,
	}
	forcedJob := &types.Job{
		Id:        "forcedJobId",
		Created:   now.Add(-2 * time.Hour),
		Name:      tcc_testutils.BuildTaskName,
		Priority:  0.5,
		RepoState: rs2,
	}
	// Manually forced candidates have a blamelist and a specific score.
	c = &taskCandidate{
		Jobs: []*types.Job{forcedJob},
		TaskKey: types.TaskKey{
			Name:        tcc_testutils.BuildTaskName,
			RepoState:   rs2,
			ForcedJobId: forcedJob.Id,
		},
	}
	diag = &taskCandidateScoringDiagnostics{}
	require.NoError(t, s.processTaskCandidate(ctx, c, now, cache, commitsBuf, diag))
	require.InDelta(t, (CANDIDATE_SCORE_FORCE_RUN+2.0)*0.5, c.Score, scoreDelta)
	require.Equal(t, 2, len(c.Commits))
	checkDiagTryForced(c, diag)

	// All other candidates have a blamelist and a time-decayed score.
	regularJob := &types.Job{
		Id:        "regularJobId",
		Created:   now.Add(-1 * time.Hour),
		Name:      tcc_testutils.BuildTaskName,
		Priority:  0.5,
		RepoState: rs2,
	}
	c = &taskCandidate{
		Jobs: []*types.Job{regularJob},
		TaskKey: types.TaskKey{
			Name:      tcc_testutils.BuildTaskName,
			RepoState: rs2,
		},
	}
	diag = &taskCandidateScoringDiagnostics{}
	require.NoError(t, s.processTaskCandidate(ctx, c, now, cache, commitsBuf, diag))
	require.True(t, c.Score > 0)
	require.Equal(t, 2, len(c.Commits))
	require.Equal(t, 0.5, diag.Priority)
	require.Equal(t, 1.0, diag.JobCreatedHours)
	require.Equal(t, 0, diag.StoleFromCommits)
	require.Equal(t, 3.5, diag.TestednessIncrease)
	require.InDelta(t, 1.0, diag.TimeDecay, scoreDelta)

	// Now, replace the time window to ensure that this next candidate runs
	// at a commit outside the window. Ensure that it gets the correct
	// blamelist.
	var err error
	s.window, err = window.New(time.Nanosecond, 0, nil)
	require.NoError(t, err)
	c = &taskCandidate{
		Jobs: []*types.Job{regularJob},
		TaskKey: types.TaskKey{
			Name:      tcc_testutils.BuildTaskName,
			RepoState: rs2,
		},
	}
	diag = &taskCandidateScoringDiagnostics{}
	require.NoError(t, s.processTaskCandidate(ctx, c, now, cache, commitsBuf, diag))
	require.Equal(t, 0, len(c.Commits))
}

func TestRegularJobRetryScoring(t *testing.T) {
	ctx, gb, _, _, s, _, cleanup := setup(t)
	defer cleanup()

	rs1 := getRS1(t, ctx, gb)
	rs2 := getRS2(t, ctx, gb)

	cache := newCacheWrapper(s.tCache)
	now := time.Now()
	commitsBuf := make([]*repograph.Commit, 0, MAX_BLAMELIST_COMMITS)

	checkDiag := func(c *taskCandidate, diag *taskCandidateScoringDiagnostics) {
		// All candidates in this test have a single Job.
		require.Equal(t, c.Jobs[0].Priority, diag.Priority)
		require.Equal(t, now.Sub(c.Jobs[0].Created).Hours(), diag.JobCreatedHours)
		// The commits are added close enough to "now" that there is no time decay.
		require.Equal(t, 1.0, diag.TimeDecay)
	}

	j1 := &types.Job{
		Id:        "regularJobId1",
		Created:   now.Add(-1 * time.Hour),
		Name:      tcc_testutils.BuildTaskName,
		Priority:  0.5,
		RepoState: rs1,
	}
	j2 := &types.Job{
		Id:        "regularJobId2",
		Created:   now.Add(-1 * time.Hour),
		Name:      tcc_testutils.BuildTaskName,
		Priority:  0.5,
		RepoState: rs2,
	}
	// Candidates at rs1 and rs2
	c1 := &taskCandidate{
		Jobs: []*types.Job{j1},
		TaskKey: types.TaskKey{
			Name:      tcc_testutils.BuildTaskName,
			RepoState: rs1,
		},
	}
	c2 := &taskCandidate{
		Jobs: []*types.Job{j2},
		TaskKey: types.TaskKey{
			Name:      tcc_testutils.BuildTaskName,
			RepoState: rs2,
		},
	}
	// Regular task at HEAD with 2 commits has score 3.5 scaled by priority 0.5.
	diag := &taskCandidateScoringDiagnostics{}
	require.NoError(t, s.processTaskCandidate(ctx, c2, now, cache, commitsBuf, diag))
	require.InDelta(t, 3.5*0.5, c2.Score, scoreDelta)
	require.Equal(t, 2, len(c2.Commits))
	require.Equal(t, 0, diag.StoleFromCommits)
	require.Equal(t, 3.5, diag.TestednessIncrease)
	checkDiag(c2, diag)
	// Regular task at HEAD^ (no backfill) with 1 commit has score 2 scaled by
	// priority 0.5.
	diag = &taskCandidateScoringDiagnostics{}
	require.NoError(t, s.processTaskCandidate(ctx, c1, now, cache, commitsBuf, diag))
	require.InDelta(t, 2*0.5, c1.Score, scoreDelta)
	require.Equal(t, 1, len(c1.Commits))
	require.Equal(t, 0, diag.StoleFromCommits)
	require.Equal(t, 2.0, diag.TestednessIncrease)
	checkDiag(c1, diag)

	// Add a task at rs2 that failed.
	t2 := makeTask(c2.Name, c2.Repo, c2.Revision)
	t2.Status = types.TASK_STATUS_FAILURE
	t2.Commits = util.CopyStringSlice(c2.Commits)
	require.NoError(t, s.putTask(t2))

	// Update Attempt and RetryOf before calling processTaskCandidate.
	c2.Attempt = 1
	c2.RetryOf = t2.Id

	// Retry task at rs2 with 2 commits for 2nd of 2 attempts has score 0.75
	// scaled by priority 0.5.
	diag = &taskCandidateScoringDiagnostics{}
	require.NoError(t, s.processTaskCandidate(ctx, c2, now, cache, commitsBuf, diag))
	require.InDelta(t, 0.75*0.5, c2.Score, scoreDelta)
	require.Equal(t, 2, len(c2.Commits))
	require.Equal(t, 2, diag.StoleFromCommits)
	require.Equal(t, 0.0, diag.TestednessIncrease)
	checkDiag(c2, diag)
	// Regular task at rs1 (backfilling failed task) with 1 commit has score 1.25
	// scaled by priority 0.5.
	diag = &taskCandidateScoringDiagnostics{}
	require.NoError(t, s.processTaskCandidate(ctx, c1, now, cache, commitsBuf, diag))
	require.InDelta(t, 1.25*0.5, c1.Score, scoreDelta)
	require.Equal(t, 1, len(c1.Commits))
	require.Equal(t, 2, diag.StoleFromCommits)
	require.Equal(t, 0.5, diag.TestednessIncrease)
	checkDiag(c1, diag)

	// Actually, the task at rs2 had a mishap.
	t2.Status = types.TASK_STATUS_MISHAP
	require.NoError(t, s.putTask(t2))

	// Scores should be same as for FAILURE.
	diag = &taskCandidateScoringDiagnostics{}
	require.NoError(t, s.processTaskCandidate(ctx, c2, now, cache, commitsBuf, diag))
	require.InDelta(t, 0.75*0.5, c2.Score, scoreDelta)
	require.Equal(t, 2, len(c2.Commits))
	require.Equal(t, 2, diag.StoleFromCommits)
	require.Equal(t, 0.0, diag.TestednessIncrease)
	checkDiag(c2, diag)
	diag = &taskCandidateScoringDiagnostics{}
	require.NoError(t, s.processTaskCandidate(ctx, c1, now, cache, commitsBuf, diag))
	require.InDelta(t, 1.25*0.5, c1.Score, scoreDelta)
	require.Equal(t, 1, len(c1.Commits))
	require.Equal(t, 2, diag.StoleFromCommits)
	require.Equal(t, 0.5, diag.TestednessIncrease)
	checkDiag(c1, diag)
}

func TestProcessTaskCandidates(t *testing.T) {
	ctx, gb, _, _, s, _, cleanup := setup(t)
	defer cleanup()

	ts := time.Now()

	rs1 := getRS1(t, ctx, gb)
	rs2 := getRS2(t, ctx, gb)
	_, err := s.cacher.GetOrCacheRepoState(ctx, rs1)
	require.NoError(t, err)
	_, err = s.cacher.GetOrCacheRepoState(ctx, rs2)
	require.NoError(t, err)

	// Processing of individual candidates is already tested; just verify
	// that if we pass in a bunch of candidates they all get processed.
	// The JobSpecs do not specify priority, so they use the default of 0.5.
	assertProcessed := func(c *taskCandidate) {
		if c.IsTryJob() {
			require.True(t, c.Score > CANDIDATE_SCORE_TRY_JOB*0.5)
			require.Nil(t, c.Commits)
		} else if c.IsForceRun() {
			require.True(t, c.Score > CANDIDATE_SCORE_FORCE_RUN*0.5)
			require.Equal(t, 2, len(c.Commits))
		} else if c.Revision == rs2.Revision {
			if c.Name == tcc_testutils.PerfTaskName {
				require.InDelta(t, 2.0*0.5, c.Score, scoreDelta)
				require.Equal(t, 1, len(c.Commits))
			} else if c.Name == tcc_testutils.BuildTaskName {
				// Already covered by the forced job, so zero score.
				require.InDelta(t, 0, c.Score, scoreDelta)
				// Scores below the BuildTask at rs1, so it has a blamelist of 1 commit.
				require.Equal(t, 1, len(c.Commits))
			} else {
				require.InDelta(t, 3.5*0.5, c.Score, scoreDelta)
				require.Equal(t, 2, len(c.Commits))
			}
		} else {
			require.InDelta(t, 0.5*0.5, c.Score, scoreDelta) // These will be backfills.
			require.Equal(t, 1, len(c.Commits))
		}
	}

	testJob1 := &types.Job{
		Id:        "testJob1",
		Created:   ts,
		Name:      tcc_testutils.TestTaskName,
		Priority:  0.5,
		RepoState: rs1,
	}
	testJob2 := &types.Job{
		Id:        "testJob2",
		Created:   ts,
		Name:      tcc_testutils.TestTaskName,
		Priority:  0.5,
		RepoState: rs2,
	}
	perfJob2 := &types.Job{
		Id:        "perfJob2",
		Created:   ts,
		Name:      tcc_testutils.PerfTaskName,
		Priority:  0.5,
		RepoState: rs2,
	}
	forcedBuildJob2 := &types.Job{
		Id:        "forcedBuildJob2",
		Created:   ts,
		Name:      tcc_testutils.BuildTaskName,
		Priority:  0.5,
		RepoState: rs2,
	}
	tryjobRs := types.RepoState{
		Patch: types.Patch{
			Server:   "my-server",
			Issue:    "my-issue",
			Patchset: "my-patchset",
		},
		Repo:     gb.RepoUrl(),
		Revision: rs1.Revision,
	}
	perfTryjob2 := &types.Job{
		Id:        "perfJob2",
		Created:   ts,
		Name:      tcc_testutils.PerfTaskName,
		Priority:  0.5,
		RepoState: tryjobRs,
	}

	candidates := map[string]map[string][]*taskCandidate{
		gb.RepoUrl(): {
			tcc_testutils.BuildTaskName: {
				{
					Jobs: []*types.Job{testJob1},
					TaskKey: types.TaskKey{
						RepoState: rs1,
						Name:      tcc_testutils.BuildTaskName,
					},
					TaskSpec: &specs.TaskSpec{},
				},
				{
					Jobs: []*types.Job{testJob2, perfJob2},
					TaskKey: types.TaskKey{
						RepoState: rs2,
						Name:      tcc_testutils.BuildTaskName,
					},
					TaskSpec: &specs.TaskSpec{},
				},
				{
					Jobs: []*types.Job{forcedBuildJob2},
					TaskKey: types.TaskKey{
						RepoState:   rs2,
						Name:        tcc_testutils.BuildTaskName,
						ForcedJobId: forcedBuildJob2.Id,
					},
					TaskSpec: &specs.TaskSpec{},
				},
			},
			tcc_testutils.TestTaskName: {
				{
					Jobs: []*types.Job{testJob1},
					TaskKey: types.TaskKey{
						RepoState: rs1,
						Name:      tcc_testutils.TestTaskName,
					},
					TaskSpec: &specs.TaskSpec{},
				},
				{
					Jobs: []*types.Job{testJob2},
					TaskKey: types.TaskKey{
						RepoState: rs2,
						Name:      tcc_testutils.TestTaskName,
					},
					TaskSpec: &specs.TaskSpec{},
				},
			},
			tcc_testutils.PerfTaskName: {
				{
					Jobs: []*types.Job{perfJob2},
					TaskKey: types.TaskKey{
						RepoState: rs2,
						Name:      tcc_testutils.PerfTaskName,
					},
					TaskSpec: &specs.TaskSpec{},
				},
				{
					Jobs: []*types.Job{perfTryjob2},
					TaskKey: types.TaskKey{
						RepoState: tryjobRs,
						Name:      tcc_testutils.PerfTaskName,
					},
					TaskSpec: &specs.TaskSpec{},
				},
			},
		},
	}

	processed, err := s.processTaskCandidates(ctx, candidates, time.Now())
	require.NoError(t, err)
	for _, c := range processed {
		assertProcessed(c)
	}
	require.Equal(t, 7, len(processed))
}

func TestTestedness(t *testing.T) {
	unittest.SmallTest(t)
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
		require.Equal(t, c.out, testedness(c.in), fmt.Sprintf("test case #%d", i))
	}
}

func TestTestednessIncrease(t *testing.T) {
	unittest.SmallTest(t)
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
		require.Equal(t, c.out, testednessIncrease(c.a, c.b), fmt.Sprintf("test case #%d", i))
	}
}

func TestComputeBlamelist(t *testing.T) {
	unittest.LargeTest(t)

	// Setup.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gb := git_testutils.GitInit(t, ctx)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	d := memory.NewInMemoryTaskDB()
	w, err := window.New(time.Hour, 0, nil)
	cache, err := cache.NewTaskCache(ctx, d, w, nil)
	require.NoError(t, err)

	repos, err := repograph.NewLocalMap(ctx, []string{gb.RepoUrl()}, tmp)
	require.NoError(t, err)
	require.NoError(t, repos.Update(ctx))
	repo := repos[gb.RepoUrl()]
	btProject, btInstance, btCleanup := tcc_testutils.SetupBigTable(t)
	defer btCleanup()
	btCleanupIsolate := isolate_cache.SetupSharedBigTable(t, btProject, btInstance)
	defer btCleanupIsolate()
	tcc, err := task_cfg_cache.NewTaskCfgCache(ctx, repos, btProject, btInstance, nil)
	require.NoError(t, err)

	// The test repo is laid out like this:
	// *   T (HEAD, master, Case #12)
	// *   S (Time travel commit; before the start of the window)
	// *   R (Case #11)
	// |\
	// * | Q
	// | * P
	// |/
	// *   O (Case #9)
	// *   N
	// *   M (Case #10)
	// *   L
	// *   K (Case #6)
	// *   J (Case #5)
	// |\
	// | * I
	// | * H (Case #4)
	// * | G
	// * | F (Case #3)
	// * | E (Case #8, previously #7)
	// |/
	// *   D (Case #2)
	// *   C (Case #1)
	// ...
	// *   B (Case #0)
	// *   A
	// *   _ (No TasksCfg; blamelists shouldn't include this)
	//
	hashes := map[string]string{}
	name := "Test-Ubuntu12-ShuttleA-GTX660-x86-Release"
	taskCfg := &specs.TasksCfg{
		Tasks: map[string]*specs.TaskSpec{
			name: {},
		},
	}
	commit := func(file, name string) {
		hashes[name] = gb.CommitGenMsg(ctx, file, name)
		require.NoError(t, tcc.Set(ctx, types.RepoState{
			Repo:     gb.RepoUrl(),
			Revision: hashes[name],
		}, taskCfg, nil))
	}

	f := "somefile"
	f2 := "file2"

	// Initial commit.
	hashes["_"] = gb.CommitGenMsg(ctx, f, "_")

	// First commit containing TasksCfg.
	commit(f, "A")

	type testCase struct {
		Revision     string
		Expected     []string
		StoleFromIdx int
		TaskName     string
	}

	ids := []string{}
	commitsBuf := make([]*repograph.Commit, 0, MAX_BLAMELIST_COMMITS)
	test := func(tc *testCase) {
		// Update the repo.
		require.NoError(t, repo.Update(ctx))
		// Self-check: make sure we don't pass in empty commit hashes.
		for _, h := range tc.Expected {
			require.NotEqual(t, h, "")
		}

		// Ensure that we get the expected blamelist.
		revision := repo.Get(tc.Revision)
		require.NotNil(t, revision)
		taskName := tc.TaskName
		if taskName == "" {
			taskName = name
		}
		commits, stoleFrom, err := ComputeBlamelist(ctx, cache, repo, taskName, gb.RepoUrl(), revision, commitsBuf, tcc, w)
		if tc.Revision == "" {
			require.Error(t, err)
			return
		} else {
			require.NoError(t, err)
		}
		sort.Strings(commits)
		sort.Strings(tc.Expected)
		assertdeep.Equal(t, tc.Expected, commits)
		if tc.StoleFromIdx >= 0 {
			require.NotNil(t, stoleFrom)
			require.Equal(t, ids[tc.StoleFromIdx], stoleFrom.Id)
		} else {
			require.Nil(t, stoleFrom)
		}

		// Insert the task into the DB.
		c := &taskCandidate{
			TaskKey: types.TaskKey{
				RepoState: types.RepoState{
					Repo:     gb.RepoUrl(),
					Revision: tc.Revision,
				},
				Name: taskName,
			},
			TaskSpec: &specs.TaskSpec{},
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
			require.NoError(t, d.PutTasks([]*types.Task{task, stoleFrom}))
			cache.AddTasks([]*types.Task{task, stoleFrom})
		} else {
			require.NoError(t, d.PutTask(task))
			cache.AddTasks([]*types.Task{task})
		}
		ids = append(ids, task.Id)
		require.NoError(t, cache.Update())
	}

	// Commit B.
	commit(f, "B")

	// Test cases. Each test case builds on the previous cases.

	// 0. The first task, at HEAD.
	test(&testCase{
		Revision:     hashes["B"],
		Expected:     []string{hashes["B"], hashes["A"]},
		StoleFromIdx: -1,
	})

	// Test the blamelist too long case by creating a bunch of commits.
	for i := 0; i < MAX_BLAMELIST_COMMITS+1; i++ {
		commit(f, "C")
	}
	commit(f, "D")

	// 1. Blamelist too long, not a branch head.
	test(&testCase{
		Revision:     hashes["C"],
		Expected:     []string{hashes["C"]},
		StoleFromIdx: -1,
	})

	// 2. Blamelist too long, is a branch head.
	test(&testCase{
		Revision:     hashes["D"],
		Expected:     []string{hashes["D"]},
		StoleFromIdx: -1,
	})

	// Create the remaining commits.
	gb.CreateBranchTrackBranch(ctx, "otherbranch", "master")
	gb.CheckoutBranch(ctx, "master")
	commit(f, "E")
	commit(f, "F")
	commit(f, "G")
	gb.CheckoutBranch(ctx, "otherbranch")
	commit(f2, "H")
	commit(f2, "I")
	gb.CheckoutBranch(ctx, "master")

	hashes["J"] = gb.MergeBranch(ctx, "otherbranch")
	require.NoError(t, tcc.Set(ctx, types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: hashes["J"],
	}, taskCfg, nil))

	commit(f, "K")

	// 3. On a linear set of commits, with at least one previous task.
	test(&testCase{
		Revision:     hashes["F"],
		Expected:     []string{hashes["E"], hashes["F"]},
		StoleFromIdx: -1,
	})
	// 4. The first task on a new branch.
	test(&testCase{
		Revision:     hashes["H"],
		Expected:     []string{hashes["H"]},
		StoleFromIdx: -1,
	})
	// 5. After a merge.
	test(&testCase{
		Revision:     hashes["J"],
		Expected:     []string{hashes["G"], hashes["I"], hashes["J"]},
		StoleFromIdx: -1,
	})
	// 6. One last "normal" task.
	test(&testCase{
		Revision:     hashes["K"],
		Expected:     []string{hashes["K"]},
		StoleFromIdx: -1,
	})
	// 7. Steal commits from a previously-ingested task.
	test(&testCase{
		Revision:     hashes["E"],
		Expected:     []string{hashes["E"]},
		StoleFromIdx: 3,
	})

	// Ensure that task #8 really stole the commit from #3.
	task, err := cache.GetTask(ids[3])
	require.NoError(t, err)
	require.False(t, util.In(hashes["E"], task.Commits), fmt.Sprintf("Expected not to find %s in %v", hashes["E"], task.Commits))

	// 8. Retry #7.
	test(&testCase{
		Revision:     hashes["E"],
		Expected:     []string{hashes["E"]},
		StoleFromIdx: 7,
	})

	// Ensure that task #8 really stole the commit from #7.
	task, err = cache.GetTask(ids[7])
	require.NoError(t, err)
	require.Equal(t, 0, len(task.Commits))

	// Four more commits.
	commit(f, "L")
	commit(f, "M")
	commit(f, "N")
	commit(f, "O")

	// 9. Not really a test case, but setting up for #10.
	test(&testCase{
		Revision:     hashes["O"],
		Expected:     []string{hashes["L"], hashes["M"], hashes["N"], hashes["O"]},
		StoleFromIdx: -1,
	})

	// 10. Steal *two* commits from #9.
	test(&testCase{
		Revision:     hashes["M"],
		Expected:     []string{hashes["L"], hashes["M"]},
		StoleFromIdx: 9,
	})

	// 11. Verify that we correctly track when task specs were added.
	gb.CreateBranchTrackBranch(ctx, "otherbranch2", "master")
	commit("asjkffda", "P")
	gb.CheckoutBranch(ctx, "master")
	commit(f, "Q")
	newTaskCfg := taskCfg.Copy()
	newTaskCfg.Tasks["added-task"] = &specs.TaskSpec{}
	require.NoError(t, tcc.Set(ctx, types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: hashes["Q"],
	}, newTaskCfg, nil))
	hashes["R"] = gb.MergeBranch(ctx, "otherbranch2")
	require.NoError(t, tcc.Set(ctx, types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: hashes["R"],
	}, newTaskCfg, nil))
	// Existing task should get a normal blamelist of all three new commits.
	test(&testCase{
		Revision:     hashes["R"],
		Expected:     []string{hashes["P"], hashes["Q"], hashes["R"]},
		StoleFromIdx: -1,
	})
	// The added task's blamelist should only include commits at which the
	// task was defined, ie. not P.
	test(&testCase{
		Revision:     hashes["R"],
		Expected:     []string{hashes["Q"], hashes["R"]},
		StoleFromIdx: -1,
		TaskName:     "added-task",
	})

	// 12. Stop computing blamelists when we reach a commit outside of the
	// scheduling window.
	gb.AddGen(ctx, f)
	hashes["S"] = gb.CommitMsgAt(ctx, "S", w.EarliestStart().Add(-time.Hour))
	require.NoError(t, tcc.Set(ctx, types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: hashes["S"],
	}, taskCfg, nil))
	commit(f, "T")
	test(&testCase{
		Revision:     hashes["T"],
		Expected:     []string{hashes["T"]},
		StoleFromIdx: -1,
	})
}

func TestTimeDecay24Hr(t *testing.T) {
	unittest.SmallTest(t)
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
		require.Equal(t, c.out, timeDecay24Hr(c.decayAmt24Hr, c.elapsed), fmt.Sprintf("test case #%d", i))
	}
}

func TestRegenerateTaskQueue(t *testing.T) {
	ctx, gb, d, _, s, _, cleanup := setup(t)
	defer cleanup()

	// Ensure that the queue is initially empty.
	require.Equal(t, 0, len(s.queue))

	rs1 := getRS1(t, ctx, gb)
	rs2 := getRS2(t, ctx, gb)
	_, err := s.cacher.GetOrCacheRepoState(ctx, rs1)
	require.NoError(t, err)
	_, err = s.cacher.GetOrCacheRepoState(ctx, rs2)
	require.NoError(t, err)
	c1 := rs1.Revision
	c2 := rs2.Revision

	// Our test repo has a job pointing to every task.
	now := time.Now()
	j1 := &types.Job{
		Created:      now,
		Name:         "j1",
		Dependencies: map[string][]string{tcc_testutils.BuildTaskName: {}},
		Priority:     0.5,
		RepoState:    rs1.Copy(),
	}
	j2 := &types.Job{
		Created:      now,
		Name:         "j2",
		Dependencies: map[string][]string{tcc_testutils.TestTaskName: {tcc_testutils.BuildTaskName}, tcc_testutils.BuildTaskName: {}},
		Priority:     0.5,
		RepoState:    rs1.Copy(),
	}
	j3 := &types.Job{
		Created:      now,
		Name:         "j3",
		Dependencies: map[string][]string{tcc_testutils.BuildTaskName: {}},
		Priority:     0.5,
		RepoState:    rs2.Copy(),
	}
	j4 := &types.Job{
		Created:      now,
		Name:         "j4",
		Dependencies: map[string][]string{tcc_testutils.TestTaskName: {tcc_testutils.BuildTaskName}, tcc_testutils.BuildTaskName: {}},
		Priority:     0.5,
		RepoState:    rs2.Copy(),
	}
	j5 := &types.Job{
		Created:      now,
		Name:         "j5",
		Dependencies: map[string][]string{tcc_testutils.PerfTaskName: {tcc_testutils.BuildTaskName}, tcc_testutils.BuildTaskName: {}},
		Priority:     0.5,
		RepoState:    rs2.Copy(),
	}
	require.NoError(t, d.PutJobs([]*types.Job{j1, j2, j3, j4, j5}))
	require.NoError(t, s.tCache.Update())
	s.jCache.AddJobs([]*types.Job{j1, j2, j3, j4, j5})
	require.NoError(t, s.jCache.Update())

	// Regenerate the task queue.
	queue, _, err := s.regenerateTaskQueue(ctx, time.Now())
	require.NoError(t, err)
	require.Equal(t, 2, len(queue)) // Two Build tasks.

	testSort := func() {
		// Ensure that we sorted correctly.
		if len(queue) == 0 {
			return
		}
		highScore := queue[0].Score
		for _, c := range queue {
			require.True(t, highScore >= c.Score)
			highScore = c.Score
		}
	}
	testSort()

	// Since we haven't run any task yet, we should have the two Build
	// tasks.
	// The one at HEAD should have a two-commit blamelist and a
	// score of 3.5, scaled by a priority of 0.875 due to three jobs
	// depending on it (1 - 0.5^3).
	require.Equal(t, tcc_testutils.BuildTaskName, queue[0].Name)
	require.Equal(t, []string{c2, c1}, queue[0].Commits)
	require.InDelta(t, 3.5*0.875, queue[0].Score, scoreDelta)
	// The other should have one commit in its blamelist and
	// a score of 0.5, scaled by a priority of 0.75 due to two jobs.
	require.Equal(t, tcc_testutils.BuildTaskName, queue[1].Name)
	require.Equal(t, []string{c1}, queue[1].Commits)
	require.InDelta(t, 0.5*0.75, queue[1].Score, scoreDelta)

	// Insert the task at c1, even though it scored lower.
	t1 := makeTask(queue[1].Name, queue[1].Repo, queue[1].Revision)
	require.NotNil(t, t1)
	t1.Status = types.TASK_STATUS_SUCCESS
	t1.IsolatedOutput = "fake isolated hash"
	require.NoError(t, s.putTask(t1))

	// Regenerate the task queue.
	queue, _, err = s.regenerateTaskQueue(ctx, time.Now())
	require.NoError(t, err)

	// Now we expect the queue to contain the other Build task and the one
	// Test task we unblocked by running the first Build task.
	require.Equal(t, 2, len(queue))
	testSort()
	for _, c := range queue {
		if c.Name == tcc_testutils.TestTaskName {
			require.InDelta(t, 2.0*0.5, c.Score, scoreDelta)
			require.Equal(t, 1, len(c.Commits))
		} else {
			require.Equal(t, c.Name, tcc_testutils.BuildTaskName)
			require.InDelta(t, 2.0*0.875, c.Score, scoreDelta)
			require.Equal(t, []string{c.Revision}, c.Commits)
		}
	}
	buildIdx := 0
	testIdx := 1
	if queue[1].Name == tcc_testutils.BuildTaskName {
		buildIdx = 1
		testIdx = 0
	}
	require.Equal(t, tcc_testutils.BuildTaskName, queue[buildIdx].Name)
	require.Equal(t, c2, queue[buildIdx].Revision)

	require.Equal(t, tcc_testutils.TestTaskName, queue[testIdx].Name)
	require.Equal(t, c1, queue[testIdx].Revision)

	// Run the other Build task.
	t2 := makeTask(queue[buildIdx].Name, queue[buildIdx].Repo, queue[buildIdx].Revision)
	t2.Status = types.TASK_STATUS_SUCCESS
	t2.IsolatedOutput = "fake isolated hash"
	require.NoError(t, s.putTask(t2))

	// Regenerate the task queue.
	queue, _, err = s.regenerateTaskQueue(ctx, time.Now())
	require.NoError(t, err)
	require.Equal(t, 3, len(queue))
	testSort()
	perfIdx := -1
	for i, c := range queue {
		if c.Name == tcc_testutils.PerfTaskName {
			perfIdx = i
			require.Equal(t, c2, c.Revision)
			require.InDelta(t, 2.0*0.5, c.Score, scoreDelta)
			require.Equal(t, []string{c.Revision}, c.Commits)
		} else {
			require.Equal(t, c.Name, tcc_testutils.TestTaskName)
			if c.Revision == c2 {
				require.InDelta(t, 3.5*0.5, c.Score, scoreDelta)
				require.Equal(t, []string{c2, c1}, c.Commits)
			} else {
				require.InDelta(t, 0.5*0.5, c.Score, scoreDelta)
				require.Equal(t, []string{c.Revision}, c.Commits)
			}
		}
	}
	require.True(t, perfIdx > -1)

	// Run the Test task at tip of tree; its blamelist covers both commits.
	t3 := makeTask(tcc_testutils.TestTaskName, gb.RepoUrl(), c2)
	t3.Commits = []string{c2, c1}
	t3.Status = types.TASK_STATUS_SUCCESS
	t3.IsolatedOutput = "fake isolated hash"
	require.NoError(t, s.putTask(t3))

	// Regenerate the task queue.
	queue, _, err = s.regenerateTaskQueue(ctx, time.Now())
	require.NoError(t, err)

	// Now we expect the queue to contain one Test and one Perf task. The
	// Test task is a backfill, and should have a score of 0.5, scaled by
	// the priority of 0.5.
	require.Equal(t, 2, len(queue))
	testSort()
	// First candidate should be the perf task.
	require.Equal(t, tcc_testutils.PerfTaskName, queue[0].Name)
	require.InDelta(t, 2.0*0.5, queue[0].Score, scoreDelta)
	// The test task is next, a backfill.
	require.Equal(t, tcc_testutils.TestTaskName, queue[1].Name)
	require.InDelta(t, 0.5*0.5, queue[1].Score, scoreDelta)
}

func makeTaskCandidate(name string, dims []string) *taskCandidate {
	return &taskCandidate{
		Score: 1.0,
		TaskKey: types.TaskKey{
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
	unittest.MediumTest(t)
	// Empty lists.
	rv := getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{}, []*taskCandidate{})
	require.Equal(t, 0, len(rv))

	// checkDiags takes a list of bots with the same dimensions and a list of
	// ordered candidates that match those bots and checks the Diagnostics for
	// candidates.
	checkDiags := func(bots []*swarming_api.SwarmingRpcsBotInfo, candidates []*taskCandidate) {
		var expectedBots []string
		if len(bots) > 0 {
			expectedBots = make([]string, len(bots), len(bots))
			for i, b := range bots {
				expectedBots[i] = b.BotId
			}
		}
		for i, c := range candidates {
			// These conditions are not tested.
			require.False(t, c.Diagnostics.Scheduling.OverSchedulingLimitPerTaskSpec)
			require.False(t, c.Diagnostics.Scheduling.ScoreBelowThreshold)
			// NoBotsAvailable and MatchingBots will be the same for all candidates.
			require.Equal(t, len(expectedBots) == 0, c.Diagnostics.Scheduling.NoBotsAvailable)
			require.Equal(t, expectedBots, c.Diagnostics.Scheduling.MatchingBots)
			require.Equal(t, i, c.Diagnostics.Scheduling.NumHigherScoreSimilarCandidates)
			if i == 0 {
				// First candidate.
				require.Nil(t, c.Diagnostics.Scheduling.LastSimilarCandidate)
			} else {
				last := candidates[i-1]
				require.Equal(t, &last.TaskKey, c.Diagnostics.Scheduling.LastSimilarCandidate)
			}
			require.Equal(t, i < len(bots), c.Diagnostics.Scheduling.Selected)
		}
		// Clear diagnostics for next test.
		for _, c := range candidates {
			c.Diagnostics = nil
		}
	}

	t1 := makeTaskCandidate("task1", []string{"k:v"})
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{}, []*taskCandidate{t1})
	require.Equal(t, 0, len(rv))
	checkDiags([]*swarming_api.SwarmingRpcsBotInfo{}, []*taskCandidate{t1})

	b1 := makeSwarmingBot("bot1", []string{"k:v"})
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{})
	require.Equal(t, 0, len(rv))

	// Single match.
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{t1})
	assertdeep.Equal(t, []*taskCandidate{t1}, rv)
	checkDiags([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{t1})

	// No match.
	t1.TaskSpec.Dimensions[0] = "k:v2"
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{t1})
	require.Equal(t, 0, len(rv))
	checkDiags([]*swarming_api.SwarmingRpcsBotInfo{}, []*taskCandidate{t1})

	// Add a task candidate to match b1.
	t1 = makeTaskCandidate("task1", []string{"k:v2"})
	t2 := makeTaskCandidate("task2", []string{"k:v"})
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{t1, t2})
	assertdeep.Equal(t, []*taskCandidate{t2}, rv)
	checkDiags([]*swarming_api.SwarmingRpcsBotInfo{}, []*taskCandidate{t1})
	checkDiags([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{t2})

	// Switch the task order.
	t1 = makeTaskCandidate("task1", []string{"k:v2"})
	t2 = makeTaskCandidate("task2", []string{"k:v"})
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{t2, t1})
	assertdeep.Equal(t, []*taskCandidate{t2}, rv)
	checkDiags([]*swarming_api.SwarmingRpcsBotInfo{}, []*taskCandidate{t1})
	checkDiags([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{t2})

	// Make both tasks match the bot, ensure that we pick the first one.
	t1 = makeTaskCandidate("task1", []string{"k:v"})
	t2 = makeTaskCandidate("task2", []string{"k:v"})
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{t1, t2})
	assertdeep.Equal(t, []*taskCandidate{t1}, rv)
	checkDiags([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{t1, t2})
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{t2, t1})
	assertdeep.Equal(t, []*taskCandidate{t2}, rv)
	checkDiags([]*swarming_api.SwarmingRpcsBotInfo{b1}, []*taskCandidate{t2, t1})

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
	assertdeep.Equal(t, []*taskCandidate{t1}, rv)
	// Can't use checkDiags for these cases.
	require.Equal(t, []string{b1.BotId, b2.BotId}, t1.Diagnostics.Scheduling.MatchingBots)
	require.Equal(t, 0, t1.Diagnostics.Scheduling.NumHigherScoreSimilarCandidates)
	require.Nil(t, t1.Diagnostics.Scheduling.LastSimilarCandidate)
	require.True(t, t1.Diagnostics.Scheduling.Selected)
	t1.Diagnostics = nil
	require.Equal(t, []string{b1.BotId}, t2.Diagnostics.Scheduling.MatchingBots)
	require.Equal(t, 1, t2.Diagnostics.Scheduling.NumHigherScoreSimilarCandidates)
	require.Equal(t, &t1.TaskKey, t2.Diagnostics.Scheduling.LastSimilarCandidate)
	require.False(t, t2.Diagnostics.Scheduling.Selected)
	t2.Diagnostics = nil

	t1 = makeTaskCandidate("task1", []string{"k:v"})
	t2 = makeTaskCandidate("task2", dims)
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b2, b1}, []*taskCandidate{t1, t2})
	assertdeep.Equal(t, []*taskCandidate{t1}, rv)
	require.Equal(t, []string{b1.BotId, b2.BotId}, t1.Diagnostics.Scheduling.MatchingBots)
	require.Equal(t, 0, t1.Diagnostics.Scheduling.NumHigherScoreSimilarCandidates)
	require.Nil(t, t1.Diagnostics.Scheduling.LastSimilarCandidate)
	require.True(t, t1.Diagnostics.Scheduling.Selected)
	t1.Diagnostics = nil
	require.Equal(t, []string{b1.BotId}, t2.Diagnostics.Scheduling.MatchingBots)
	require.Equal(t, 1, t2.Diagnostics.Scheduling.NumHigherScoreSimilarCandidates)
	require.Equal(t, &t1.TaskKey, t2.Diagnostics.Scheduling.LastSimilarCandidate)
	require.False(t, t2.Diagnostics.Scheduling.Selected)
	t2.Diagnostics = nil

	// In these two cases, the task with more dimensions has the higher
	// priority. Both tasks get scheduled.
	t1 = makeTaskCandidate("task1", []string{"k:v"})
	t2 = makeTaskCandidate("task2", dims)
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1, b2}, []*taskCandidate{t2, t1})
	assertdeep.Equal(t, []*taskCandidate{t2, t1}, rv)
	require.Equal(t, []string{b1.BotId, b2.BotId}, t1.Diagnostics.Scheduling.MatchingBots)
	require.Equal(t, 1, t1.Diagnostics.Scheduling.NumHigherScoreSimilarCandidates)
	require.Equal(t, &t2.TaskKey, t1.Diagnostics.Scheduling.LastSimilarCandidate)
	require.True(t, t1.Diagnostics.Scheduling.Selected)
	t1.Diagnostics = nil
	require.Equal(t, []string{b1.BotId}, t2.Diagnostics.Scheduling.MatchingBots)
	require.Equal(t, 0, t2.Diagnostics.Scheduling.NumHigherScoreSimilarCandidates)
	require.Nil(t, t2.Diagnostics.Scheduling.LastSimilarCandidate)
	require.True(t, t2.Diagnostics.Scheduling.Selected)
	t2.Diagnostics = nil

	t1 = makeTaskCandidate("task1", []string{"k:v"})
	t2 = makeTaskCandidate("task2", dims)
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b2, b1}, []*taskCandidate{t2, t1})
	assertdeep.Equal(t, []*taskCandidate{t2, t1}, rv)
	require.Equal(t, []string{b1.BotId, b2.BotId}, t1.Diagnostics.Scheduling.MatchingBots)
	require.Equal(t, 1, t1.Diagnostics.Scheduling.NumHigherScoreSimilarCandidates)
	require.Equal(t, &t2.TaskKey, t1.Diagnostics.Scheduling.LastSimilarCandidate)
	require.True(t, t1.Diagnostics.Scheduling.Selected)
	t1.Diagnostics = nil
	require.Equal(t, []string{b1.BotId}, t2.Diagnostics.Scheduling.MatchingBots)
	require.Equal(t, 0, t2.Diagnostics.Scheduling.NumHigherScoreSimilarCandidates)
	require.Nil(t, t2.Diagnostics.Scheduling.LastSimilarCandidate)
	require.True(t, t2.Diagnostics.Scheduling.Selected)
	t2.Diagnostics = nil

	// Matching dimensions. More bots than tasks.
	b2 = makeSwarmingBot("bot2", dims)
	b3 := makeSwarmingBot("bot3", dims)
	t1 = makeTaskCandidate("task1", dims)
	t2 = makeTaskCandidate("task2", dims)
	t3 := makeTaskCandidate("task3", dims)
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1, b2, b3}, []*taskCandidate{t1, t2})
	assertdeep.Equal(t, []*taskCandidate{t1, t2}, rv)
	checkDiags([]*swarming_api.SwarmingRpcsBotInfo{b1, b2, b3}, []*taskCandidate{t1, t2})

	// More tasks than bots.
	t1 = makeTaskCandidate("task1", dims)
	t2 = makeTaskCandidate("task2", dims)
	t3 = makeTaskCandidate("task3", dims)
	rv = getCandidatesToSchedule([]*swarming_api.SwarmingRpcsBotInfo{b1, b2}, []*taskCandidate{t1, t2, t3})
	assertdeep.Equal(t, []*taskCandidate{t1, t2}, rv)
	checkDiags([]*swarming_api.SwarmingRpcsBotInfo{b1, b2}, []*taskCandidate{t1, t2, t3})
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
	ctx, gb, _, swarmingClient, s, _, cleanup := setup(t)
	defer cleanup()

	c1 := getRS1(t, ctx, gb).Revision
	c2 := getRS2(t, ctx, gb).Revision

	// Start testing. No free bots, so we get a full queue with nothing
	// scheduled.
	updateRepos(t, ctx, s)
	runMainLoop(t, s, ctx)
	tasks, err := s.tCache.GetTasksForCommits(gb.RepoUrl(), []string{c1, c2})
	require.NoError(t, err)
	expect := map[string]map[string]*types.Task{
		c1: {},
		c2: {},
	}
	assertdeep.Equal(t, expect, tasks)
	require.Equal(t, 2, len(s.queue)) // Two compile tasks.

	// A bot is free but doesn't have all of the right dimensions to run a task.
	bot1 := makeBot("bot1", map[string]string{"pool": "Skia"})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	runMainLoop(t, s, ctx)
	tasks, err = s.tCache.GetTasksForCommits(gb.RepoUrl(), []string{c1, c2})
	require.NoError(t, err)
	expect = map[string]map[string]*types.Task{
		c1: {},
		c2: {},
	}
	assertdeep.Equal(t, expect, tasks)
	require.Equal(t, 2, len(s.queue)) // Still two compile tasks.

	// One bot free, schedule a task, ensure it's not in the queue.
	bot1.Dimensions = append(bot1.Dimensions, &swarming_api.SwarmingRpcsStringListPair{
		Key:   "os",
		Value: []string{"Ubuntu"},
	})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.GetTasksForCommits(gb.RepoUrl(), []string{c1, c2})
	require.NoError(t, err)
	t1 := tasks[c2][tcc_testutils.BuildTaskName]
	require.NotNil(t, t1)
	require.Equal(t, c2, t1.Revision)
	require.Equal(t, tcc_testutils.BuildTaskName, t1.Name)
	require.Equal(t, []string{c2, c1}, t1.Commits)
	require.Equal(t, 1, len(s.queue))

	// The task is complete.
	t1.Status = types.TASK_STATUS_SUCCESS
	t1.Finished = time.Now()
	t1.IsolatedOutput = "abc123"
	require.NoError(t, s.putTask(t1))
	swarmingClient.MockTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{
		makeSwarmingRpcsTaskRequestMetadata(t, t1, linuxTaskDims),
	})

	// No bots free. Ensure that the queue is correct.
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{})
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.GetTasksForCommits(gb.RepoUrl(), []string{c1, c2})
	require.NoError(t, err)
	for _, c := range t1.Commits {
		expect[c][t1.Name] = t1
	}
	assertdeep.Equal(t, expect, tasks)
	expectLen := 3 // One remaining build task, plus one test task and one perf task.
	require.Equal(t, expectLen, len(s.queue))

	// More bots than tasks free, ensure the queue is correct.
	bot2 := makeBot("bot2", androidTaskDims)
	bot3 := makeBot("bot3", androidTaskDims)
	bot4 := makeBot("bot4", linuxTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2, bot3, bot4})
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	_, err = s.tCache.GetTasksForCommits(gb.RepoUrl(), []string{c1, c2})
	require.NoError(t, err)
	require.Equal(t, 0, len(s.queue))

	// The build, test, and perf tasks should have triggered.
	var t2 *types.Task
	var t3 *types.Task
	var t4 *types.Task
	tasks, err = s.tCache.GetTasksForCommits(gb.RepoUrl(), []string{c1, c2})
	require.NoError(t, err)
	require.Equal(t, 2, len(tasks))
	for commit, v := range tasks {
		if commit == c1 {
			// Build task at c1 and test task at c2 whose blamelist also has c1.
			require.Equal(t, 2, len(v))
			for _, task := range v {
				if task.Revision != commit {
					continue
				}
				require.Equal(t, tcc_testutils.BuildTaskName, task.Name)
				require.Nil(t, t4)
				t4 = task
				require.Equal(t, c1, task.Revision)
				require.Equal(t, []string{c1}, task.Commits)
			}
		} else {
			require.Equal(t, 3, len(v))
			for _, task := range v {
				if task.Name == tcc_testutils.TestTaskName {
					require.Nil(t, t2)
					t2 = task
					require.Equal(t, c2, task.Revision)
					require.Equal(t, []string{c2, c1}, task.Commits)
				} else if task.Name == tcc_testutils.PerfTaskName {
					require.Nil(t, t3)
					t3 = task
					require.Equal(t, c2, task.Revision)
					require.Equal(t, []string{c2}, task.Commits)
				} else {
					// This is the first task we triggered.
					require.Equal(t, tcc_testutils.BuildTaskName, task.Name)
				}
			}
		}
	}
	require.NotNil(t, t2)
	require.NotNil(t, t3)
	require.NotNil(t, t4)
	t4.Status = types.TASK_STATUS_SUCCESS
	t4.Finished = time.Now()
	t4.IsolatedOutput = "abc123"
	require.NoError(t, s.putTask(t4))

	// No new bots free; only the remaining test task should be in the queue.
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{})
	mockTasks := []*swarming_api.SwarmingRpcsTaskRequestMetadata{
		makeSwarmingRpcsTaskRequestMetadata(t, t2, linuxTaskDims),
		makeSwarmingRpcsTaskRequestMetadata(t, t3, linuxTaskDims),
		makeSwarmingRpcsTaskRequestMetadata(t, t4, linuxTaskDims),
	}
	swarmingClient.MockTasks(mockTasks)
	require.NoError(t, s.updateUnfinishedTasks())
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	expectLen = 1 // Test task from c1
	require.Equal(t, expectLen, len(s.queue))

	// Finish the other task.
	t3, err = s.tCache.GetTask(t3.Id)
	require.NoError(t, err)
	t3.Status = types.TASK_STATUS_SUCCESS
	t3.Finished = time.Now()
	t3.IsolatedOutput = "abc123"

	// Ensure that we finalize all of the tasks and insert into the DB.
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2, bot3, bot4})
	mockTasks = []*swarming_api.SwarmingRpcsTaskRequestMetadata{
		makeSwarmingRpcsTaskRequestMetadata(t, t3, linuxTaskDims),
	}
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.GetTasksForCommits(gb.RepoUrl(), []string{c1, c2})
	require.NoError(t, err)
	require.Equal(t, 2, len(tasks[c1]))
	require.Equal(t, 3, len(tasks[c2]))
	require.Equal(t, 0, len(s.queue))

	// Mark everything as finished. Ensure that the queue still ends up empty.
	tasksList := []*types.Task{}
	for _, v := range tasks {
		for _, task := range v {
			if task.Status != types.TASK_STATUS_SUCCESS {
				task.Status = types.TASK_STATUS_SUCCESS
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
	require.NoError(t, s.updateUnfinishedTasks())
	runMainLoop(t, s, ctx)
	require.Equal(t, 0, len(s.queue))
}

func makeDummyCommits(ctx context.Context, gb *git_testutils.GitBuilder, numCommits int) {
	for i := 0; i < numCommits; i++ {
		gb.AddGen(ctx, "dummyfile.txt")
		gb.CommitMsg(ctx, fmt.Sprintf("Dummy #%d/%d", i, numCommits))
	}
}

func TestSchedulerStealingFrom(t *testing.T) {
	ctx, gb, _, swarmingClient, s, _, cleanup := setup(t)
	defer cleanup()

	c1 := getRS1(t, ctx, gb).Revision
	c2 := getRS2(t, ctx, gb).Revision

	// Run the available compile task at c2.
	bot1 := makeBot("bot1", linuxTaskDims)
	bot2 := makeBot("bot2", linuxTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2})
	updateRepos(t, ctx, s)
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	tasks, err := s.tCache.GetTasksForCommits(gb.RepoUrl(), []string{c1, c2})
	require.NoError(t, err)
	require.Equal(t, 1, len(tasks[c1]))
	require.Equal(t, 1, len(tasks[c2]))

	// Finish the one task.
	tasksList := []*types.Task{}
	t1 := tasks[c2][tcc_testutils.BuildTaskName]
	t1.Status = types.TASK_STATUS_SUCCESS
	t1.Finished = time.Now()
	t1.IsolatedOutput = "abc123"
	tasksList = append(tasksList, t1)

	// Forcibly create and insert a second task at c1.
	t2 := t1.Copy()
	t2.Id = "t2id"
	t2.Revision = c1
	t2.Commits = []string{c1}
	tasksList = append(tasksList, t2)

	require.NoError(t, s.putTasks(tasksList))

	// Add some commits.
	makeDummyCommits(ctx, gb, 10)
	require.NoError(t, s.repos[gb.RepoUrl()].Update(ctx))
	commits, err := s.repos[gb.RepoUrl()].Get("master").AllCommits()
	require.NoError(t, err)

	// Run one task. Ensure that it's at tip-of-tree.
	head := s.repos[gb.RepoUrl()].Get("master").Hash
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	updateRepos(t, ctx, s)
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.GetTasksForCommits(gb.RepoUrl(), commits)
	require.NoError(t, err)
	require.Equal(t, 1, len(tasks[head]))
	task := tasks[head][tcc_testutils.BuildTaskName]
	require.Equal(t, head, task.Revision)
	expect := commits[:len(commits)-2]
	sort.Strings(expect)
	sort.Strings(task.Commits)
	assertdeep.Equal(t, expect, task.Commits)

	task.Status = types.TASK_STATUS_SUCCESS
	task.Finished = time.Now()
	task.IsolatedOutput = "abc123"
	require.NoError(t, s.putTask(task))

	oldTasksByCommit := tasks

	// Run backfills, ensuring that each one steals the right set of commits
	// from previous builds, until all of the build task candidates have run.
	for i := 0; i < 9; i++ {
		// Now, run another task. The new task should bisect the old one.
		swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
		runMainLoop(t, s, ctx)
		require.NoError(t, s.tCache.Update())
		tasks, err = s.tCache.GetTasksForCommits(gb.RepoUrl(), commits)
		require.NoError(t, err)
		var newTask *types.Task
		for _, v := range tasks {
			for _, task := range v {
				if task.Status == types.TASK_STATUS_PENDING {
					require.True(t, newTask == nil || task.Id == newTask.Id)
					newTask = task
				}
			}
		}
		require.NotNil(t, newTask)

		oldTask := oldTasksByCommit[newTask.Revision][newTask.Name]
		require.NotNil(t, oldTask)
		require.True(t, util.In(newTask.Revision, oldTask.Commits))

		// Find the updated old task.
		updatedOldTask, err := s.tCache.GetTask(oldTask.Id)
		require.NoError(t, err)
		require.NotNil(t, updatedOldTask)

		// Ensure that the blamelists are correct.
		old := util.NewStringSet(oldTask.Commits)
		new := util.NewStringSet(newTask.Commits)
		updatedOld := util.NewStringSet(updatedOldTask.Commits)

		assertdeep.Equal(t, old, new.Union(updatedOld))
		require.Equal(t, 0, len(new.Intersect(updatedOld)))
		// Finish the new task.
		newTask.Status = types.TASK_STATUS_SUCCESS
		newTask.Finished = time.Now()
		newTask.IsolatedOutput = "abc123"
		require.NoError(t, s.putTask(newTask))
		oldTasksByCommit = tasks

	}

	// Ensure that we're really done.
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.GetTasksForCommits(gb.RepoUrl(), commits)
	require.NoError(t, err)
	var newTask *types.Task
	for _, v := range tasks {
		for _, task := range v {
			if task.Status == types.TASK_STATUS_PENDING {
				require.True(t, newTask == nil || task.Id == newTask.Id)
				newTask = task
			}
		}
	}
	require.Nil(t, newTask)
}

// spyDB calls onPutTasks before delegating PutTask(s) to DB.
type spyDB struct {
	db.DB
	onPutTasks func([]*types.Task)
}

func (s *spyDB) PutTask(task *types.Task) error {
	s.onPutTasks([]*types.Task{task})
	return s.DB.PutTask(task)
}

func (s *spyDB) PutTasks(tasks []*types.Task) error {
	s.onPutTasks(tasks)
	return s.DB.PutTasks(tasks)
}

func testMultipleCandidatesBackfillingEachOtherSetup(t *testing.T) (context.Context, *git_testutils.GitBuilder, db.DB, *TaskScheduler, *swarming_testutils.TestClient, []string, func(*types.Task), func()) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	gb := git_testutils.GitInit(t, ctx)
	workdir, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	require.NoError(t, ioutil.WriteFile(path.Join(workdir, ".gclient"), []byte("dummy"), os.ModePerm))
	infraBotsSubDir := path.Join("infra", "bots")

	gb.Add(ctx, "somefile.txt", "dummy3")
	gb.Add(ctx, path.Join(infraBotsSubDir, "dummy.isolate"), `{
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
			taskName: {
				CipdPackages: []*specs.CipdPackage{},
				Dependencies: []string{},
				Dimensions:   []string{"pool:Skia"},
				Isolate:      "dummy.isolate",
				Priority:     1.0,
			},
		},
		Jobs: map[string]*specs.JobSpec{
			"j1": {
				TaskSpecs: []string{taskName},
			},
		},
	}
	cfgStr := testutils.MarshalJSON(t, cfg)
	gb.Add(ctx, specs.TASKS_CFG_FILE, cfgStr)
	gb.Commit(ctx)

	// Setup the scheduler.
	d := memory.NewInMemoryDB()
	isolateClient, err := isolate.NewClient(workdir, isolate.ISOLATE_SERVER_URL_FAKE)
	require.NoError(t, err)
	swarmingClient := swarming_testutils.NewTestClient()
	repos, err := repograph.NewLocalMap(ctx, []string{gb.RepoUrl()}, workdir)
	require.NoError(t, err)
	require.NoError(t, repos.Update(ctx))
	projectRepoMapping := map[string]string{
		"skia": gb.RepoUrl(),
	}
	urlMock := mockhttpclient.NewURLMock()
	depotTools := depot_tools_testutils.GetDepotTools(t, ctx)
	gitcookies := path.Join(workdir, "gitcookies_fake")
	require.NoError(t, ioutil.WriteFile(gitcookies, []byte(".googlesource.com\tTRUE\t/\tTRUE\t123\to\tgit-user.google.com=abc123"), os.ModePerm))
	g, err := gerrit.NewGerrit(fakeGerritUrl, gitcookies, urlMock.Client())
	require.NoError(t, err)

	btProject, btInstance, btCleanup := tcc_testutils.SetupBigTable(t)
	btCleanupIsolate := isolate_cache.SetupSharedBigTable(t, btProject, btInstance)
	s, err := NewTaskScheduler(ctx, d, nil, time.Duration(math.MaxInt64), 0, workdir, "fake.server", repos, isolateClient, swarmingClient, mockhttpclient.NewURLMock().Client(), 1.0, tryjobs.API_URL_TESTING, tryjobs.BUCKET_TESTING, projectRepoMapping, swarming.POOLS_PUBLIC, "", depotTools, g, btProject, btInstance, nil, mem_gcsclient.New("diag_unit_tests"), btInstance)
	require.NoError(t, err)

	mockTasks := []*swarming_api.SwarmingRpcsTaskRequestMetadata{}
	mock := func(task *types.Task) {
		task.Status = types.TASK_STATUS_SUCCESS
		task.Finished = time.Now()
		task.IsolatedOutput = "abc123"
		mockTasks = append(mockTasks, makeSwarmingRpcsTaskRequestMetadata(t, task, linuxTaskDims))
		swarmingClient.MockTasks(mockTasks)
	}

	// Cycle once.
	bot1 := makeBot("bot1", map[string]string{"pool": "Skia"})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	updateRepos(t, ctx, s)
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	require.Equal(t, 0, len(s.queue))
	head := s.repos[gb.RepoUrl()].Get("master").Hash
	tasks, err := s.tCache.GetTasksForCommits(gb.RepoUrl(), []string{head})
	require.NoError(t, err)
	require.Equal(t, 1, len(tasks[head]))
	mock(tasks[head][taskName])

	// Add some commits to the repo.
	gb.CheckoutBranch(ctx, "master")
	makeDummyCommits(ctx, gb, 8)
	require.NoError(t, s.repos[gb.RepoUrl()].Update(ctx))
	commits, err := s.repos[gb.RepoUrl()].RevList(head, "master")
	require.Nil(t, err)
	require.Equal(t, 8, len(commits))
	updateRepos(t, ctx, s) // Most tests want this.
	return ctx, gb, d, s, swarmingClient, commits, mock, func() {
		testutils.AssertCloses(t, s)
		gb.Cleanup()
		testutils.RemoveAll(t, workdir)
		btCleanupIsolate()
		btCleanup()
		cancel()
	}
}

func TestMultipleCandidatesBackfillingEachOther(t *testing.T) {
	ctx, gb, d, s, swarmingClient, commits, mock, cleanup := testMultipleCandidatesBackfillingEachOtherSetup(t)
	defer cleanup()

	// Trigger builds simultaneously.
	bot1 := makeBot("bot1", map[string]string{"pool": "Skia"})
	bot2 := makeBot("bot2", map[string]string{"pool": "Skia"})
	bot3 := makeBot("bot3", map[string]string{"pool": "Skia"})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2, bot3})
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	require.Equal(t, 5, len(s.queue))
	tasks, err := s.tCache.GetTasksForCommits(gb.RepoUrl(), commits)
	require.NoError(t, err)

	// If we're queueing correctly, we should've triggered tasks at
	// commits[0], commits[4], and either commits[2] or commits[6].
	var t1, t2, t3 *types.Task
	for _, byName := range tasks {
		for _, task := range byName {
			if task.Revision == commits[0] {
				t1 = task
			} else if task.Revision == commits[4] {
				t2 = task
			} else if task.Revision == commits[2] || task.Revision == commits[6] {
				t3 = task
			} else {
				require.FailNow(t, fmt.Sprintf("Task has unknown revision: %v", task))
			}
		}
	}
	require.NotNil(t, t1)
	require.NotNil(t, t2)
	require.NotNil(t, t3)
	mock(t1)
	mock(t2)
	mock(t3)

	// Ensure that we got the blamelists right.
	var expect1, expect2, expect3 []string
	if t3.Revision == commits[2] {
		expect1 = util.CopyStringSlice(commits[:2])
		expect2 = util.CopyStringSlice(commits[4:])
		expect3 = util.CopyStringSlice(commits[2:4])
	} else {
		expect1 = util.CopyStringSlice(commits[:4])
		expect2 = util.CopyStringSlice(commits[4:6])
		expect3 = util.CopyStringSlice(commits[6:])
	}
	sort.Strings(expect1)
	sort.Strings(expect2)
	sort.Strings(expect3)
	sort.Strings(t1.Commits)
	sort.Strings(t2.Commits)
	sort.Strings(t3.Commits)
	assertdeep.Equal(t, expect1, t1.Commits)
	assertdeep.Equal(t, expect2, t2.Commits)
	assertdeep.Equal(t, expect3, t3.Commits)

	// Just for good measure, check the task at the head of the queue.
	expectIdx := 2
	if t3.Revision == commits[expectIdx] {
		expectIdx = 6
	}
	require.Equal(t, commits[expectIdx], s.queue[0].Revision)

	retryCount := 0
	causeConcurrentUpdate := func(tasks []*types.Task) {
		// HACK(benjaminwagner): Filter out PutTask calls from
		// updateUnfinishedTasks by looking for new tasks.
		anyNew := false
		for _, task := range tasks {
			if util.TimeIsZero(task.DbModified) {
				anyNew = true
				break
			}
		}
		if !anyNew {
			return
		}
		if retryCount < 3 {
			taskToUpdate := []*types.Task{t1, t2, t3}[retryCount]
			retryCount++
			taskInDb, err := d.GetTaskById(taskToUpdate.Id)
			require.NoError(t, err)
			taskInDb.Status = types.TASK_STATUS_SUCCESS
			require.NoError(t, d.PutTask(taskInDb))
			s.tCache.AddTasks([]*types.Task{taskInDb})
		}
	}
	s.db = &spyDB{
		DB:         d,
		onPutTasks: causeConcurrentUpdate,
	}

	// Run again with 5 bots to check the case where we bisect the same
	// task twice.
	bot4 := makeBot("bot4", map[string]string{"pool": "Skia"})
	bot5 := makeBot("bot5", map[string]string{"pool": "Skia"})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2, bot3, bot4, bot5})
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	require.Equal(t, 0, len(s.queue))
	require.Equal(t, 3, retryCount)
	tasks, err = s.tCache.GetTasksForCommits(gb.RepoUrl(), commits)
	require.NoError(t, err)
	for _, byName := range tasks {
		for _, task := range byName {
			require.Equal(t, 1, len(task.Commits))
			require.Equal(t, task.Revision, task.Commits[0])
			if util.In(task.Id, []string{t1.Id, t2.Id, t3.Id}) {
				require.Equal(t, types.TASK_STATUS_SUCCESS, task.Status)
			} else {
				require.Equal(t, types.TASK_STATUS_PENDING, task.Status)
			}
		}
	}
}

func TestSchedulingRetry(t *testing.T) {
	ctx, gb, _, swarmingClient, s, _, cleanup := setup(t)
	defer cleanup()

	c1 := getRS1(t, ctx, gb).Revision

	// Run the available compile task at c2.
	bot1 := makeBot("bot1", linuxTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	updateRepos(t, ctx, s)
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	tasks, err := s.tCache.UnfinishedTasks()
	require.NoError(t, err)
	require.Equal(t, 1, len(tasks))
	t1 := tasks[0]
	require.NotNil(t, t1)
	// Ensure c2, not c1.
	require.NotEqual(t, c1, t1.Revision)
	c2 := t1.Revision

	// Forcibly add a second build task at c1.
	t2 := t1.Copy()
	t2.Id = "t2Id"
	t2.Revision = c1
	t2.Commits = []string{c1}
	t1.Commits = []string{c2}

	// One task successful, the other not.
	t1.Status = types.TASK_STATUS_FAILURE
	t1.Finished = time.Now()
	t2.Status = types.TASK_STATUS_SUCCESS
	t2.Finished = time.Now()
	t2.IsolatedOutput = "abc123"

	require.NoError(t, s.putTasks([]*types.Task{t1, t2}))

	// Cycle. Ensure that we schedule a retry of t1.
	prev := t1
	i := 1
	for {
		runMainLoop(t, s, ctx)
		require.NoError(t, s.tCache.Update())
		tasks, err = s.tCache.UnfinishedTasks()
		require.NoError(t, err)
		if len(tasks) == 0 {
			break
		}
		require.Equal(t, 1, len(tasks))
		retry := tasks[0]
		require.NotNil(t, retry)
		require.Equal(t, prev.Id, retry.RetryOf)
		require.Equal(t, i, retry.Attempt)
		require.Equal(t, c2, retry.Revision)
		retry.Status = types.TASK_STATUS_FAILURE
		retry.Finished = time.Now()
		require.NoError(t, s.putTask(retry))

		prev = retry
		i++
	}
	require.Equal(t, 5, i)
}

func TestParentTaskId(t *testing.T) {
	ctx, _, _, swarmingClient, s, _, cleanup := setup(t)
	defer cleanup()

	// Run the available compile task at c2.
	bot1 := makeBot("bot1", linuxTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	updateRepos(t, ctx, s)
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	tasks, err := s.tCache.UnfinishedTasks()
	require.NoError(t, err)
	require.Equal(t, 1, len(tasks))
	t1 := tasks[0]
	t1.Status = types.TASK_STATUS_SUCCESS
	t1.Finished = time.Now()
	t1.IsolatedOutput = "abc123"
	require.Equal(t, 0, len(t1.ParentTaskIds))
	require.NoError(t, s.putTasks([]*types.Task{t1}))

	// Run the dependent tasks. Ensure that their parent IDs are correct.
	bot3 := makeBot("bot3", androidTaskDims)
	bot4 := makeBot("bot4", androidTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot3, bot4})
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.UnfinishedTasks()
	require.NoError(t, err)
	require.Equal(t, 2, len(tasks))
	for _, task := range tasks {
		require.Equal(t, 1, len(task.ParentTaskIds))
		p := task.ParentTaskIds[0]
		require.Equal(t, p, t1.Id)

		updated, err := task.UpdateFromSwarming(makeSwarmingRpcsTaskRequestMetadata(t, task, linuxTaskDims).TaskResult)
		require.NoError(t, err)
		require.False(t, updated)
	}
}

func TestBlacklist(t *testing.T) {
	// The blacklist has its own tests, so this test just verifies that it's
	// actually integrated into the scheduler.
	ctx, gb, _, swarmingClient, s, _, cleanup := setup(t)
	defer cleanup()
	c, cleanupfs := skfs.NewClientForTesting(t)
	defer cleanupfs()
	bl, err := blacklist.New(context.Background(), c)
	require.NoError(t, err)
	s.bl = bl

	c1 := getRS1(t, ctx, gb).Revision

	// Mock some bots, add one of the build tasks to the blacklist.
	bot1 := makeBot("bot1", linuxTaskDims)
	bot2 := makeBot("bot2", linuxTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2})
	require.NoError(t, s.GetBlacklist().AddRule(&blacklist.Rule{
		AddedBy:          "Tests",
		TaskSpecPatterns: []string{".*"},
		Commits:          []string{c1},
		Description:      "desc",
		Name:             "My-Rule",
	}, s.repos))
	updateRepos(t, ctx, s)
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	tasks, err := s.tCache.UnfinishedTasks()
	require.NoError(t, err)
	// The blacklisted commit should not have been triggered.
	require.Equal(t, 1, len(tasks))
	require.NotEqual(t, c1, tasks[0].Revision)
	// Candidate diagnostics should indicate the blacklist rule.
	diag := lastDiagnostics(t, s)
	foundBlacklisted := 0
	for _, c := range diag.Candidates {
		if c.Revision == c1 {
			foundBlacklisted++
			require.Equal(t, "My-Rule", c.Diagnostics.Filtering.BlacklistedByRule)
		} else if c.TaskKey == tasks[0].TaskKey {
			require.Nil(t, c.Diagnostics.Filtering)
		} else {
			require.Equal(t, "", c.Diagnostics.Filtering.BlacklistedByRule)
			require.True(t, len(c.Diagnostics.Filtering.UnmetDependencies) > 0)
		}
	}
	// Should be one Build task and one Test task blacklisted.
	require.Equal(t, 2, foundBlacklisted)
}

func TestGetTasksForJob(t *testing.T) {
	ctx, gb, _, swarmingClient, s, _, cleanup := setup(t)
	defer cleanup()

	c1 := getRS1(t, ctx, gb).Revision
	c2 := getRS2(t, ctx, gb).Revision

	// Cycle once, check that we have empty sets for all Jobs.
	updateRepos(t, ctx, s)
	runMainLoop(t, s, ctx)
	jobs, err := s.jCache.UnfinishedJobs()
	require.NoError(t, err)
	require.Equal(t, 5, len(jobs))
	var j1, j2, j3, j4, j5 *types.Job
	for _, j := range jobs {
		if j.Revision == c1 {
			if j.Name == tcc_testutils.BuildTaskName {
				j1 = j
			} else {
				j2 = j
			}
		} else {
			if j.Name == tcc_testutils.BuildTaskName {
				j3 = j
			} else if j.Name == tcc_testutils.TestTaskName {
				j4 = j
			} else {
				j5 = j
			}
		}
		tasksByName, err := s.getTasksForJob(j)
		require.NoError(t, err)
		for _, tasks := range tasksByName {
			require.Equal(t, 0, len(tasks))
		}
	}
	require.NotNil(t, j1)
	require.NotNil(t, j2)
	require.NotNil(t, j3)
	require.NotNil(t, j4)
	require.NotNil(t, j5)

	// Run the available compile task at c2.
	bot1 := makeBot("bot1", linuxTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	tasks, err := s.tCache.UnfinishedTasks()
	require.NoError(t, err)
	require.Equal(t, 1, len(tasks))
	t1 := tasks[0]
	require.NotNil(t, t1)
	require.Equal(t, t1.Revision, c2)

	// Test that we get the new tasks where applicable.
	expect := map[string]map[string][]*types.Task{
		j1.Id: {
			tcc_testutils.BuildTaskName: {},
		},
		j2.Id: {
			tcc_testutils.BuildTaskName: {},
			tcc_testutils.TestTaskName:  {},
		},
		j3.Id: {
			tcc_testutils.BuildTaskName: {t1},
		},
		j4.Id: {
			tcc_testutils.BuildTaskName: {t1},
			tcc_testutils.TestTaskName:  {},
		},
		j5.Id: {
			tcc_testutils.BuildTaskName: {t1},
			tcc_testutils.PerfTaskName:  {},
		},
	}
	for _, j := range jobs {
		tasksByName, err := s.getTasksForJob(j)
		require.NoError(t, err)
		assertdeep.Equal(t, expect[j.Id], tasksByName)
	}

	// Mark the task as failed.
	t1.Status = types.TASK_STATUS_FAILURE
	t1.Finished = time.Now()
	require.NoError(t, s.putTasks([]*types.Task{t1}))

	// Test that the results propagated through.
	for _, j := range jobs {
		tasksByName, err := s.getTasksForJob(j)
		require.NoError(t, err)
		assertdeep.Equal(t, expect[j.Id], tasksByName)
	}

	// Cycle. Ensure that we schedule a retry of t1.
	// Need two bots, since the retry will score lower than the Build task at c1.
	bot2 := makeBot("bot2", linuxTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2})
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.UnfinishedTasks()
	require.NoError(t, err)
	require.Equal(t, 2, len(tasks))
	var t2, t3 *types.Task
	for _, task := range tasks {
		if task.TaskKey == t1.TaskKey {
			t2 = task
		} else {
			t3 = task
		}
	}
	require.NotNil(t, t2)
	require.Equal(t, t1.Id, t2.RetryOf)

	// Verify that both the original t1 and its retry show up.
	t1, err = s.tCache.GetTask(t1.Id) // t1 was updated.
	require.NoError(t, err)
	expect[j1.Id][tcc_testutils.BuildTaskName] = []*types.Task{t3}
	expect[j2.Id][tcc_testutils.BuildTaskName] = []*types.Task{t3}
	expect[j3.Id][tcc_testutils.BuildTaskName] = []*types.Task{t1, t2}
	expect[j4.Id][tcc_testutils.BuildTaskName] = []*types.Task{t1, t2}
	expect[j5.Id][tcc_testutils.BuildTaskName] = []*types.Task{t1, t2}
	for _, j := range jobs {
		tasksByName, err := s.getTasksForJob(j)
		require.NoError(t, err)
		assertdeep.Equal(t, expect[j.Id], tasksByName)
	}

	// The retry succeeded.
	t2.Status = types.TASK_STATUS_SUCCESS
	t2.Finished = time.Now()
	t2.IsolatedOutput = "abc"
	// The Build at c1 failed.
	t3.Status = types.TASK_STATUS_FAILURE
	t3.Finished = time.Now()
	require.NoError(t, s.putTasks([]*types.Task{t2, t3}))
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{})
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	tasks, err = s.tCache.UnfinishedTasks()
	require.NoError(t, err)
	require.Equal(t, 0, len(tasks))

	// Test that the results propagated through.
	for _, j := range jobs {
		tasksByName, err := s.getTasksForJob(j)
		require.NoError(t, err)
		assertdeep.Equal(t, expect[j.Id], tasksByName)
	}

	// Schedule the remaining tasks.
	bot3 := makeBot("bot3", androidTaskDims)
	bot4 := makeBot("bot4", androidTaskDims)
	bot5 := makeBot("bot5", androidTaskDims)
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot3, bot4, bot5})
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())

	// Verify that the new tasks show up.
	tasks, err = s.tCache.UnfinishedTasks()
	require.NoError(t, err)
	require.Equal(t, 2, len(tasks)) // Test and perf at c2.
	var t4, t5 *types.Task
	for _, task := range tasks {
		if task.Name == tcc_testutils.TestTaskName {
			t4 = task
		} else {
			t5 = task
		}
	}
	expect[j4.Id][tcc_testutils.TestTaskName] = []*types.Task{t4}
	expect[j5.Id][tcc_testutils.PerfTaskName] = []*types.Task{t5}
	for _, j := range jobs {
		tasksByName, err := s.getTasksForJob(j)
		require.NoError(t, err)
		assertdeep.Equal(t, expect[j.Id], tasksByName)
	}
}

func TestTaskTimeouts(t *testing.T) {
	ctx, gb, _, swarmingClient, s, _, cleanup := setup(t)
	defer cleanup()

	// The test repo does not set any timeouts. Ensure that we get
	// reasonable default values.
	bot1 := makeBot("bot1", map[string]string{"pool": "Skia", "os": "Ubuntu", "gpu": "none"})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1})
	updateRepos(t, ctx, s)
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	unfinished, err := s.tCache.UnfinishedTasks()
	require.NoError(t, err)
	require.Equal(t, 1, len(unfinished))
	task := unfinished[0]
	swarmingTask, err := swarmingClient.GetTaskMetadata(task.SwarmingTaskId)
	require.NoError(t, err)
	// These are the defaults in go/swarming/swarming.go.
	require.Equal(t, int64(60*60), swarmingTask.Request.Properties.ExecutionTimeoutSecs)
	require.Equal(t, int64(20*60), swarmingTask.Request.Properties.IoTimeoutSecs)
	require.Equal(t, int64(4*60*60), swarmingTask.Request.ExpirationSecs)
	// Fail the task to get it out of the unfinished list.
	task.Status = types.TASK_STATUS_FAILURE
	require.NoError(t, s.putTask(task))

	// Rewrite tasks.json with some timeouts.
	name := "Timeout-Task"
	cfg := &specs.TasksCfg{
		Jobs: map[string]*specs.JobSpec{
			"Timeout-Job": {
				Priority:  1.0,
				TaskSpecs: []string{name},
			},
		},
		Tasks: map[string]*specs.TaskSpec{
			name: {
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
	gb.Add(ctx, specs.TASKS_CFG_FILE, testutils.MarshalJSON(t, &cfg))
	gb.Commit(ctx)

	// Cycle, ensure that we get the expected timeouts.
	bot2 := makeBot("bot2", map[string]string{"pool": "Skia", "os": "Mac", "gpu": "my-gpu"})
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot2})
	updateRepos(t, ctx, s)
	runMainLoop(t, s, ctx)
	require.NoError(t, s.tCache.Update())
	unfinished, err = s.tCache.UnfinishedTasks()
	require.NoError(t, err)
	require.Equal(t, 1, len(unfinished))
	task = unfinished[0]
	require.Equal(t, name, task.Name)
	swarmingTask, err = swarmingClient.GetTaskMetadata(task.SwarmingTaskId)
	require.NoError(t, err)
	require.Equal(t, int64(40*60), swarmingTask.Request.Properties.ExecutionTimeoutSecs)
	require.Equal(t, int64(3*60), swarmingTask.Request.Properties.IoTimeoutSecs)
	require.Equal(t, int64(2*60*60), swarmingTask.Request.ExpirationSecs)
}

func TestPeriodicJobs(t *testing.T) {
	ctx, gb, _, _, s, _, cleanup := setup(t)
	defer cleanup()

	// Rewrite tasks.json with a periodic job.
	nightlyName := "Nightly-Job"
	weeklyName := "Weekly-Job"
	names := []string{nightlyName, weeklyName}
	taskName := "Periodic-Task"
	cfg := &specs.TasksCfg{
		Jobs: map[string]*specs.JobSpec{
			nightlyName: {
				Priority:  1.0,
				TaskSpecs: []string{taskName},
				Trigger:   specs.TRIGGER_NIGHTLY,
			},
			weeklyName: {
				Priority:  1.0,
				TaskSpecs: []string{taskName},
				Trigger:   specs.TRIGGER_WEEKLY,
			},
		},
		Tasks: map[string]*specs.TaskSpec{
			taskName: {
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
	gb.Add(ctx, specs.TASKS_CFG_FILE, testutils.MarshalJSON(t, &cfg))
	gb.Commit(ctx)
	updateRepos(t, ctx, s)
	runMainLoop(t, s, ctx)

	// Trigger the periodic jobs. Make sure that we inserted the new Job.
	require.NoError(t, s.MaybeTriggerPeriodicJobs(ctx, specs.TRIGGER_NIGHTLY))
	require.NoError(t, s.jCache.Update())
	start := time.Now().Add(-10 * time.Minute)
	end := time.Now().Add(10 * time.Minute)
	jobs, err := s.jCache.GetMatchingJobsFromDateRange(names, start, end)
	require.NoError(t, err)
	require.Equal(t, 1, len(jobs[nightlyName]))
	require.Equal(t, nightlyName, jobs[nightlyName][0].Name)
	require.Equal(t, 0, len(jobs[weeklyName]))

	// Ensure that we don't trigger another.
	require.NoError(t, s.MaybeTriggerPeriodicJobs(ctx, specs.TRIGGER_NIGHTLY))
	require.NoError(t, s.jCache.Update())
	jobs, err = s.jCache.GetMatchingJobsFromDateRange(names, start, end)
	require.NoError(t, err)
	require.Equal(t, 1, len(jobs[nightlyName]))
	require.Equal(t, 0, len(jobs[weeklyName]))

	// Hack the old Job's created time to simulate it scrolling out of the
	// window.
	oldJob := jobs[nightlyName][0]
	oldJob.Created = start.Add(-23 * time.Hour)
	require.NoError(t, s.db.PutJob(oldJob))
	s.jCache.AddJobs([]*types.Job{oldJob})
	require.NoError(t, s.jCache.Update())
	jobs, err = s.jCache.GetMatchingJobsFromDateRange(names, start, end)
	require.NoError(t, err)
	require.Equal(t, 0, len(jobs[nightlyName]))
	require.Equal(t, 0, len(jobs[weeklyName]))
	require.NoError(t, s.MaybeTriggerPeriodicJobs(ctx, specs.TRIGGER_NIGHTLY))
	require.NoError(t, s.jCache.Update())
	jobs, err = s.jCache.GetMatchingJobsFromDateRange(names, start, end)
	require.NoError(t, err)
	require.Equal(t, 1, len(jobs[nightlyName]))
	require.Equal(t, nightlyName, jobs[nightlyName][0].Name)
	require.Equal(t, 0, len(jobs[weeklyName]))

	// Make sure we don't confuse different triggers.
	require.NoError(t, s.MaybeTriggerPeriodicJobs(ctx, specs.TRIGGER_WEEKLY))
	require.NoError(t, s.jCache.Update())
	jobs, err = s.jCache.GetMatchingJobsFromDateRange(names, start, end)
	require.NoError(t, err)
	require.Equal(t, 1, len(jobs[nightlyName]))
	require.Equal(t, nightlyName, jobs[nightlyName][0].Name)
	require.Equal(t, 1, len(jobs[weeklyName]))
	require.Equal(t, weeklyName, jobs[weeklyName][0].Name)
}

func TestUpdateUnfinishedTasks(t *testing.T) {
	_, _, _, swarmingClient, s, _, cleanup := setup(t)
	defer cleanup()

	// Create a few tasks.
	now := time.Unix(1480683321, 0).UTC()
	t1 := &types.Task{
		Id:             "t1",
		Created:        now.Add(-time.Minute),
		Status:         types.TASK_STATUS_RUNNING,
		SwarmingTaskId: "swarmt1",
	}
	t2 := &types.Task{
		Id:             "t2",
		Created:        now.Add(-10 * time.Minute),
		Status:         types.TASK_STATUS_PENDING,
		SwarmingTaskId: "swarmt2",
	}
	t3 := &types.Task{
		Id:             "t3",
		Created:        now.Add(-5 * time.Hour), // Outside the 4-hour window.
		Status:         types.TASK_STATUS_PENDING,
		SwarmingTaskId: "swarmt3",
	}
	// Include a fake task to ensure it's ignored.
	t4 := &types.Task{
		Id:      "t4",
		Created: now.Add(-time.Minute),
		Status:  types.TASK_STATUS_PENDING,
	}

	// Insert the tasks into the DB.
	tasks := []*types.Task{t1, t2, t3, t4}
	require.NoError(t, s.putTasks(tasks))

	// Update the tasks, mock in Swarming.
	t1.Status = types.TASK_STATUS_SUCCESS
	t2.Status = types.TASK_STATUS_FAILURE
	t3.Status = types.TASK_STATUS_SUCCESS

	m1 := makeSwarmingRpcsTaskRequestMetadata(t, t1, linuxTaskDims)
	m2 := makeSwarmingRpcsTaskRequestMetadata(t, t2, linuxTaskDims)
	m3 := makeSwarmingRpcsTaskRequestMetadata(t, t3, linuxTaskDims)
	swarmingClient.MockTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{m1, m2, m3})

	// Assert that the third task doesn't show up in the time range query.
	got, err := swarmingClient.ListTasks(now.Add(-4*time.Hour), now, []string{"pool:Skia"}, "")
	require.NoError(t, err)
	assertdeep.Equal(t, []*swarming_api.SwarmingRpcsTaskRequestMetadata{m1, m2}, got)

	// Ensure that we update the tasks as expected.
	require.NoError(t, s.updateUnfinishedTasks())
	for _, task := range tasks {
		got, err := s.db.GetTaskById(task.Id)
		require.NoError(t, err)
		// Ignore DbModified when comparing.
		task.DbModified = got.DbModified
		assertdeep.Equal(t, task, got)
	}
}

// setupAddTasksTest calls setup then adds 7 commits to the repo and returns
// their hashes.
func setupAddTasksTest(t *testing.T) (context.Context, *git_testutils.GitBuilder, []string, *memory.InMemoryDB, *TaskScheduler, func()) {
	ctx, gb, d, _, s, _, cleanup := setup(t)

	// Add some commits to test blamelist calculation.
	makeDummyCommits(ctx, gb, 7)
	updateRepos(t, ctx, s)
	hashes, err := s.repos[gb.RepoUrl()].Get("master").AllCommits()
	require.NoError(t, err)
	for _, hash := range hashes {
		require.NoError(t, s.taskCfgCache.Set(ctx, types.RepoState{
			Repo:     gb.RepoUrl(),
			Revision: hash,
		}, &specs.TasksCfg{
			Tasks: map[string]*specs.TaskSpec{
				"duty": {},
				"onus": {},
				"toil": {},
				"work": {},
			},
		}, nil))
	}

	return ctx, gb, hashes, d, s, func() {
		cleanup()
	}
}

// assertBlamelist asserts task.Commits contains exactly the hashes at the given
// indexes of hashes, in any order.
func assertBlamelist(t *testing.T, hashes []string, task *types.Task, indexes []int) {
	expected := util.NewStringSet()
	for _, idx := range indexes {
		expected[hashes[idx]] = true
	}
	require.Equal(t, expected, util.NewStringSet(task.Commits))
}

// assertModifiedTasks asserts that the result of GetModifiedTasks is deep-equal
// to expected, in any order.
func assertModifiedTasks(t *testing.T, d db.TaskReader, mod <-chan []*types.Task, expected []*types.Task) {
	tasksById := map[string]*types.Task{}
	require.NoError(t, testutils.EventuallyConsistent(10*time.Second, func() error {
		// Use a select so that the test will fail after 10 seconds
		// rather than time out after 10 minutes (or whatever the
		// overall timeout is set to).
		select {
		case modTasks := <-mod:
			for _, task := range modTasks {
				tasksById[task.Id] = task
			}
			for _, expectedTask := range expected {
				actualTask, ok := tasksById[expectedTask.Id]
				if !ok {
					time.Sleep(50 * time.Millisecond)
					return testutils.TryAgainErr
				}
				if !deepequal.DeepEqual(expectedTask, actualTask) {
					time.Sleep(50 * time.Millisecond)
					return testutils.TryAgainErr
				}
			}
			return nil
		default:
			// Nothing to do.
		}
		time.Sleep(50 * time.Millisecond)
		return testutils.TryAgainErr
	}))
}

// addTasksSingleTaskSpec should add tasks and compute simple blamelists.
func TestAddTasksSingleTaskSpecSimple(t *testing.T) {
	ctx, gb, hashes, d, s, cleanup := setupAddTasksTest(t)
	defer cleanup()

	t1 := makeTask("toil", gb.RepoUrl(), hashes[6])
	require.NoError(t, s.putTask(t1))

	mod := d.ModifiedTasksCh(ctx)
	<-mod // The first batch is unused.

	t2 := makeTask("toil", gb.RepoUrl(), hashes[5]) // Commits should be {5}
	t3 := makeTask("toil", gb.RepoUrl(), hashes[3]) // Commits should be {3, 4}
	t4 := makeTask("toil", gb.RepoUrl(), hashes[2]) // Commits should be {2}
	t5 := makeTask("toil", gb.RepoUrl(), hashes[0]) // Commits should be {0, 1}

	// Clear Commits on some tasks, set incorrect Commits on others to
	// ensure it's ignored.
	t3.Commits = nil
	t4.Commits = []string{hashes[5], hashes[4], hashes[3], hashes[2]}
	sort.Strings(t4.Commits)

	// Specify tasks in wrong order to ensure results are deterministic.
	require.NoError(t, s.addTasksSingleTaskSpec(ctx, []*types.Task{t5, t2, t3, t4}))

	assertBlamelist(t, hashes, t2, []int{5})
	assertBlamelist(t, hashes, t3, []int{3, 4})
	assertBlamelist(t, hashes, t4, []int{2})
	assertBlamelist(t, hashes, t5, []int{0, 1})

	// Check that the tasks were inserted into the DB.
	assertModifiedTasks(t, d, mod, []*types.Task{t2, t3, t4, t5})
}

// addTasksSingleTaskSpec should compute blamelists when new tasks bisect each
// other.
func TestAddTasksSingleTaskSpecBisectNew(t *testing.T) {
	ctx, gb, hashes, d, s, cleanup := setupAddTasksTest(t)
	defer cleanup()

	t1 := makeTask("toil", gb.RepoUrl(), hashes[6])
	require.NoError(t, s.putTask(t1))

	mod := d.ModifiedTasksCh(ctx)
	<-mod // The first batch is unused.

	// t2.Commits = {1, 2, 3, 4, 5}
	t2 := makeTask("toil", gb.RepoUrl(), hashes[1])
	// t3.Commits = {3, 4, 5}
	// t2.Commits = {1, 2}
	t3 := makeTask("toil", gb.RepoUrl(), hashes[3])
	// t4.Commits = {4, 5}
	// t3.Commits = {3}
	t4 := makeTask("toil", gb.RepoUrl(), hashes[4])
	// t5.Commits = {0}
	t5 := makeTask("toil", gb.RepoUrl(), hashes[0])
	// t6.Commits = {2}
	// t2.Commits = {1}
	t6 := makeTask("toil", gb.RepoUrl(), hashes[2])
	// t7.Commits = {1}
	// t2.Commits = {}
	t7 := makeTask("toil", gb.RepoUrl(), hashes[1])

	// Specify tasks in wrong order to ensure results are deterministic.
	tasks := []*types.Task{t5, t2, t7, t3, t6, t4}

	// Assign Ids.
	for _, task := range tasks {
		require.NoError(t, d.AssignId(task))
	}

	require.NoError(t, s.addTasksSingleTaskSpec(ctx, tasks))

	assertBlamelist(t, hashes, t2, []int{})
	assertBlamelist(t, hashes, t3, []int{3})
	assertBlamelist(t, hashes, t4, []int{4, 5})
	assertBlamelist(t, hashes, t5, []int{0})
	assertBlamelist(t, hashes, t6, []int{2})
	assertBlamelist(t, hashes, t7, []int{1})

	// Check that the tasks were inserted into the DB.
	assertModifiedTasks(t, d, mod, tasks)
}

// addTasksSingleTaskSpec should compute blamelists when new tasks bisect old
// tasks.
func TestAddTasksSingleTaskSpecBisectOld(t *testing.T) {
	ctx, gb, hashes, d, s, cleanup := setupAddTasksTest(t)
	defer cleanup()

	t1 := makeTask("toil", gb.RepoUrl(), hashes[6])
	t2 := makeTask("toil", gb.RepoUrl(), hashes[1])
	t2.Commits = []string{hashes[1], hashes[2], hashes[3], hashes[4], hashes[5]}
	sort.Strings(t2.Commits)
	require.NoError(t, s.putTasks([]*types.Task{t1, t2}))

	mod := d.ModifiedTasksCh(ctx)
	<-mod // The first batch is unused.

	// t3.Commits = {3, 4, 5}
	// t2.Commits = {1, 2}
	t3 := makeTask("toil", gb.RepoUrl(), hashes[3])
	// t4.Commits = {4, 5}
	// t3.Commits = {3}
	t4 := makeTask("toil", gb.RepoUrl(), hashes[4])
	// t5.Commits = {2}
	// t2.Commits = {1}
	t5 := makeTask("toil", gb.RepoUrl(), hashes[2])
	// t6.Commits = {4, 5}
	// t4.Commits = {}
	t6 := makeTask("toil", gb.RepoUrl(), hashes[4])

	// Specify tasks in wrong order to ensure results are deterministic.
	require.NoError(t, s.addTasksSingleTaskSpec(ctx, []*types.Task{t5, t3, t6, t4}))

	t2Updated, err := d.GetTaskById(t2.Id)
	require.NoError(t, err)
	assertBlamelist(t, hashes, t2Updated, []int{1})
	assertBlamelist(t, hashes, t3, []int{3})
	assertBlamelist(t, hashes, t4, []int{})
	assertBlamelist(t, hashes, t5, []int{2})
	assertBlamelist(t, hashes, t6, []int{4, 5})

	// Check that the tasks were inserted into the DB.
	t2.Commits = t2Updated.Commits
	t2.DbModified = t2Updated.DbModified
	assertModifiedTasks(t, d, mod, []*types.Task{t2, t3, t4, t5, t6})
}

// addTasksSingleTaskSpec should update existing tasks, keeping the correct
// blamelist.
func TestAddTasksSingleTaskSpecUpdate(t *testing.T) {
	ctx, gb, hashes, d, s, cleanup := setupAddTasksTest(t)
	defer cleanup()

	t1 := makeTask("toil", gb.RepoUrl(), hashes[6])
	t2 := makeTask("toil", gb.RepoUrl(), hashes[3])
	t3 := makeTask("toil", gb.RepoUrl(), hashes[4])
	t3.Commits = nil // Stolen by t5
	t4 := makeTask("toil", gb.RepoUrl(), hashes[0])
	t4.Commits = []string{hashes[0], hashes[1], hashes[2]}
	sort.Strings(t4.Commits)
	t5 := makeTask("toil", gb.RepoUrl(), hashes[4])
	t5.Commits = []string{hashes[4], hashes[5]}
	sort.Strings(t5.Commits)

	tasks := []*types.Task{t1, t2, t3, t4, t5}
	require.NoError(t, s.putTasks(tasks))

	mod := d.ModifiedTasksCh(ctx)
	<-mod // The first batch is unused.

	// Make an update.
	for _, task := range tasks {
		task.Status = types.TASK_STATUS_MISHAP
	}

	// Specify tasks in wrong order to ensure results are deterministic.
	require.NoError(t, s.addTasksSingleTaskSpec(ctx, []*types.Task{t5, t3, t1, t4, t2}))

	// Check that blamelists did not change.
	assertBlamelist(t, hashes, t1, []int{6})
	assertBlamelist(t, hashes, t2, []int{3})
	assertBlamelist(t, hashes, t3, []int{})
	assertBlamelist(t, hashes, t4, []int{0, 1, 2})
	assertBlamelist(t, hashes, t5, []int{4, 5})

	// Check that the tasks were inserted into the DB.
	assertModifiedTasks(t, d, mod, []*types.Task{t1, t2, t3, t4, t5})
}

// AddTasks should call addTasksSingleTaskSpec for each group of tasks.
func TestAddTasks(t *testing.T) {
	ctx, gb, hashes, d, s, cleanup := setupAddTasksTest(t)
	defer cleanup()

	toil1 := makeTask("toil", gb.RepoUrl(), hashes[6])
	duty1 := makeTask("duty", gb.RepoUrl(), hashes[6])
	work1 := makeTask("work", gb.RepoUrl(), hashes[6])
	work2 := makeTask("work", gb.RepoUrl(), hashes[1])
	work2.Commits = []string{hashes[1], hashes[2], hashes[3], hashes[4], hashes[5]}
	sort.Strings(work2.Commits)
	onus1 := makeTask("onus", gb.RepoUrl(), hashes[6])
	onus2 := makeTask("onus", gb.RepoUrl(), hashes[3])
	onus2.Commits = []string{hashes[3], hashes[4], hashes[5]}
	sort.Strings(onus2.Commits)
	require.NoError(t, s.putTasks([]*types.Task{toil1, duty1, work1, work2, onus1, onus2}))

	mod := d.ModifiedTasksCh(ctx)
	<-mod // The first batch is unused.

	// toil2.Commits = {5}
	toil2 := makeTask("toil", gb.RepoUrl(), hashes[5])
	// toil3.Commits = {3, 4}
	toil3 := makeTask("toil", gb.RepoUrl(), hashes[3])

	// duty2.Commits = {1, 2, 3, 4, 5}
	duty2 := makeTask("duty", gb.RepoUrl(), hashes[1])
	// duty3.Commits = {3, 4, 5}
	// duty2.Commits = {1, 2}
	duty3 := makeTask("duty", gb.RepoUrl(), hashes[3])

	// work3.Commits = {3, 4, 5}
	// work2.Commits = {1, 2}
	work3 := makeTask("work", gb.RepoUrl(), hashes[3])
	// work4.Commits = {2}
	// work2.Commits = {1}
	work4 := makeTask("work", gb.RepoUrl(), hashes[2])

	onus2.Status = types.TASK_STATUS_MISHAP
	// onus3 steals all commits from onus2
	onus3 := makeTask("onus", gb.RepoUrl(), hashes[3])
	// onus4 steals all commits from onus3
	onus4 := makeTask("onus", gb.RepoUrl(), hashes[3])

	tasks := map[string]map[string][]*types.Task{
		gb.RepoUrl(): {
			"toil": {toil2, toil3},
			"duty": {duty2, duty3},
			"work": {work3, work4},
			"onus": {onus2, onus3, onus4},
		},
	}

	require.NoError(t, s.addTasks(ctx, tasks))

	assertBlamelist(t, hashes, toil2, []int{5})
	assertBlamelist(t, hashes, toil3, []int{3, 4})

	assertBlamelist(t, hashes, duty2, []int{1, 2})
	assertBlamelist(t, hashes, duty3, []int{3, 4, 5})

	work2Updated, err := d.GetTaskById(work2.Id)
	require.NoError(t, err)
	assertBlamelist(t, hashes, work2Updated, []int{1})
	assertBlamelist(t, hashes, work3, []int{3, 4, 5})
	assertBlamelist(t, hashes, work4, []int{2})

	assertBlamelist(t, hashes, onus2, []int{})
	assertBlamelist(t, hashes, onus3, []int{})
	assertBlamelist(t, hashes, onus4, []int{3, 4, 5})

	// Check that the tasks were inserted into the DB.
	work2.Commits = work2Updated.Commits
	work2.DbModified = work2Updated.DbModified
	assertModifiedTasks(t, d, mod, []*types.Task{toil2, toil3, duty2, duty3, work2, work3, work4, onus2, onus3, onus4})
}

// AddTasks should not leave DB in an inconsistent state if there is a partial error.
func TestAddTasksFailure(t *testing.T) {
	ctx, gb, hashes, d, s, cleanup := setupAddTasksTest(t)
	defer cleanup()

	toil1 := makeTask("toil", gb.RepoUrl(), hashes[6])
	duty1 := makeTask("duty", gb.RepoUrl(), hashes[6])
	duty2 := makeTask("duty", gb.RepoUrl(), hashes[5])
	require.NoError(t, s.putTasks([]*types.Task{toil1, duty1, duty2}))
	d.Wait()

	// Cause ErrConcurrentUpdate in AddTasks.
	cachedDuty2 := duty2.Copy()
	duty2.Status = types.TASK_STATUS_MISHAP
	require.NoError(t, d.PutTask(duty2))
	d.Wait()

	mod := d.ModifiedTasksCh(ctx)
	<-mod // The first batch is unused.

	// toil2.Commits = {3, 4, 5}
	toil2 := makeTask("toil", gb.RepoUrl(), hashes[3])

	cachedDuty2.Status = types.TASK_STATUS_FAILURE
	// duty3.Commits = {3, 4}
	duty3 := makeTask("duty", gb.RepoUrl(), hashes[3])

	tasks := map[string]map[string][]*types.Task{
		gb.RepoUrl(): {
			"toil": {toil2},
			"duty": {cachedDuty2, duty3},
		},
	}

	// Try multiple times to reduce chance of test passing flakily.
	for i := 0; i < 3; i++ {
		err := s.addTasks(ctx, tasks)
		require.Error(t, err)
		modTasks := <-mod
		// "duty" tasks should never be updated.
		for _, task := range modTasks {
			require.Equal(t, "toil", task.Name)
			assertdeep.Equal(t, toil2, task)
			assertBlamelist(t, hashes, toil2, []int{3, 4, 5})
		}
	}

	duty2.Status = types.TASK_STATUS_FAILURE
	tasks[gb.RepoUrl()]["duty"] = []*types.Task{duty2, duty3}
	require.NoError(t, s.addTasks(ctx, tasks))

	assertBlamelist(t, hashes, toil2, []int{3, 4, 5})

	assertBlamelist(t, hashes, duty2, []int{5})
	assertBlamelist(t, hashes, duty3, []int{3, 4})

	// Check that the tasks were inserted into the DB.
	assertModifiedTasks(t, d, mod, []*types.Task{toil2, duty2, duty3})
}

// AddTasks should retry on ErrConcurrentUpdate.
func TestAddTasksRetries(t *testing.T) {
	ctx, gb, hashes, d, s, cleanup := setupAddTasksTest(t)
	defer cleanup()

	toil1 := makeTask("toil", gb.RepoUrl(), hashes[6])
	duty1 := makeTask("duty", gb.RepoUrl(), hashes[6])
	work1 := makeTask("work", gb.RepoUrl(), hashes[6])
	toil2 := makeTask("toil", gb.RepoUrl(), hashes[1])
	toil2.Commits = []string{hashes[1], hashes[2], hashes[3], hashes[4], hashes[5]}
	sort.Strings(toil2.Commits)
	duty2 := makeTask("duty", gb.RepoUrl(), hashes[1])
	duty2.Commits = util.CopyStringSlice(toil2.Commits)
	work2 := makeTask("work", gb.RepoUrl(), hashes[1])
	work2.Commits = util.CopyStringSlice(toil2.Commits)
	require.NoError(t, s.putTasks([]*types.Task{toil1, toil2, duty1, duty2, work1, work2}))

	mod := d.ModifiedTasksCh(ctx)
	<-mod // The first batch is unused.

	// *3.Commits = {3, 4, 5}
	// *2.Commits = {1, 2}
	toil3 := makeTask("toil", gb.RepoUrl(), hashes[3])
	duty3 := makeTask("duty", gb.RepoUrl(), hashes[3])
	work3 := makeTask("work", gb.RepoUrl(), hashes[3])
	// *4.Commits = {2}
	// *2.Commits = {1}
	toil4 := makeTask("toil", gb.RepoUrl(), hashes[2])
	duty4 := makeTask("duty", gb.RepoUrl(), hashes[2])
	work4 := makeTask("work", gb.RepoUrl(), hashes[2])

	tasks := map[string]map[string][]*types.Task{
		gb.RepoUrl(): {
			"toil": {toil3.Copy(), toil4.Copy()},
			"duty": {duty3.Copy(), duty4.Copy()},
			"work": {work3.Copy(), work4.Copy()},
		},
	}

	retryCountMtx := sync.Mutex{}
	retryCount := map[string]int{}
	causeConcurrentUpdate := func(tasks []*types.Task) {
		retryCountMtx.Lock()
		defer retryCountMtx.Unlock()
		retryCount[tasks[0].Name]++
		if tasks[0].Name == "toil" && retryCount["toil"] < 2 {
			toil2.Started = time.Now().UTC()
			require.NoError(t, d.PutTasks([]*types.Task{toil2}))
			s.tCache.AddTasks([]*types.Task{toil2})
		}
		if tasks[0].Name == "duty" && retryCount["duty"] < 3 {
			duty2.Started = time.Now().UTC()
			require.NoError(t, d.PutTasks([]*types.Task{duty2}))
			s.tCache.AddTasks([]*types.Task{duty2})
		}
		if tasks[0].Name == "work" && retryCount["work"] < 4 {
			work2.Started = time.Now().UTC()
			require.NoError(t, d.PutTasks([]*types.Task{work2}))
			s.tCache.AddTasks([]*types.Task{work2})
		}
	}
	s.db = &spyDB{
		DB:         d,
		onPutTasks: causeConcurrentUpdate,
	}

	require.NoError(t, s.addTasks(ctx, tasks))

	retryCountMtx.Lock()
	defer retryCountMtx.Unlock()
	require.Equal(t, 2, retryCount["toil"])
	require.Equal(t, 3, retryCount["duty"])
	require.Equal(t, 4, retryCount["work"])

	modified := []*types.Task{}
	check := func(t2, t3, t4 *types.Task) {
		t2InDB, err := d.GetTaskById(t2.Id)
		require.NoError(t, err)
		assertBlamelist(t, hashes, t2InDB, []int{1})
		t3Arg := tasks[t3.Repo][t3.Name][0]
		t3InDB, err := d.GetTaskById(t3Arg.Id)
		require.NoError(t, err)
		assertdeep.Equal(t, t3Arg, t3InDB)
		assertBlamelist(t, hashes, t3InDB, []int{3, 4, 5})
		t4Arg := tasks[t4.Repo][t4.Name][1]
		t4InDB, err := d.GetTaskById(t4Arg.Id)
		require.NoError(t, err)
		assertdeep.Equal(t, t4Arg, t4InDB)
		assertBlamelist(t, hashes, t4InDB, []int{2})
		t2.Commits = t2InDB.Commits
		t2.DbModified = t2InDB.DbModified
		t3.Id = t3InDB.Id
		t3.Commits = t3InDB.Commits
		t3.DbModified = t3InDB.DbModified
		t4.Id = t4InDB.Id
		t4.Commits = t4InDB.Commits
		t4.DbModified = t4InDB.DbModified
		modified = append(modified, t2, t3, t4)
	}

	check(toil2, toil3, toil4)
	check(duty2, duty3, duty4)
	check(work2, work3, work4)
	assertModifiedTasks(t, d, mod, modified)
}

func TestTriggerTaskFailed(t *testing.T) {
	// Verify that if one task out of a set fails to trigger, the others are
	// still inserted into the DB and handled properly, eg. wrt. blamelists.
	ctx, gb, _, s, swarmingClient, commits, _, cleanup := testMultipleCandidatesBackfillingEachOtherSetup(t)
	defer cleanup()

	// Trigger three tasks. We should attempt to trigger tasks at
	// commits[0], commits[4], and either commits[2] or commits[6]. Mock
	// failure to trigger the task at commits[4] and ensure that the other
	// two tasks get inserted with the correct blamelists.
	bot1 := makeBot("bot1", map[string]string{"pool": "Skia"})
	bot2 := makeBot("bot2", map[string]string{"pool": "Skia"})
	bot3 := makeBot("bot3", map[string]string{"pool": "Skia"})
	makeTags := func(commit string) []string {
		return []string{
			"luci_project:",
			"milo_host:https://ci.chromium.org/raw/build/%s",
			"sk_attempt:0",
			"sk_dim_pool:Skia",
			"sk_retry_of:",
			fmt.Sprintf("source_revision:%s", commit),
			fmt.Sprintf("source_repo:%s/+/%%s", gb.RepoUrl()),
			fmt.Sprintf("sk_repo:%s", gb.RepoUrl()),
			fmt.Sprintf("sk_revision:%s", commit),
			"sk_forced_job_id:",
			"sk_name:dummytask",
		}
	}
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2, bot3})
	swarmingClient.MockTriggerTaskFailure(makeTags(commits[4]))
	err := s.MainLoop(ctx)
	s.testWaitGroup.Wait()
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), "Mocked trigger failure!"))
	require.NoError(t, s.tCache.Update())
	require.Equal(t, 6, len(s.queue))
	tasks, err := s.tCache.GetTasksForCommits(gb.RepoUrl(), commits)
	require.NoError(t, err)

	var t1, t2, t3 *types.Task
	for _, byName := range tasks {
		for _, task := range byName {
			if task.Revision == commits[0] {
				t1 = task
			} else if task.Revision == commits[4] {
				t2 = task
			} else if task.Revision == commits[2] || task.Revision == commits[6] {
				t3 = task
			} else {
				require.FailNow(t, fmt.Sprintf("Task has unknown revision: %v", task))
			}
		}
	}
	require.NotNil(t, t1)
	require.Nil(t, t2)
	require.NotNil(t, t3)

	// Ensure that we got the blamelists right.
	var expect1, expect3 []string
	if t3.Revision == commits[2] {
		expect1 = util.CopyStringSlice(commits[:2])
		expect3 = util.CopyStringSlice(commits[2:])
	} else {
		expect1 = util.CopyStringSlice(commits[:6])
		expect3 = util.CopyStringSlice(commits[6:])
	}
	sort.Strings(expect1)
	sort.Strings(expect3)
	sort.Strings(t1.Commits)
	sort.Strings(t3.Commits)
	assertdeep.Equal(t, expect1, t1.Commits)
	assertdeep.Equal(t, expect3, t3.Commits)

	// Check diagnostics.
	diag := lastDiagnostics(t, s)
	failedTrigger := 0
	for _, c := range diag.Candidates {
		if c.Revision == commits[4] {
			require.True(t, strings.Contains(c.Diagnostics.Triggering.TriggerError, "Mocked trigger failure!"))
			failedTrigger++
		} else {
			if c.TaskKey == t1.TaskKey {
				require.Equal(t, "", c.Diagnostics.Triggering.TriggerError)
				require.Equal(t, t1.Id, c.Diagnostics.Triggering.TaskId)
			} else if c.TaskKey == t3.TaskKey {
				require.Equal(t, "", c.Diagnostics.Triggering.TriggerError)
				require.Equal(t, t3.Id, c.Diagnostics.Triggering.TaskId)
			} else {
				require.Nil(t, c.Diagnostics.Triggering)
			}
		}
	}
	// Should be one task that failed
	require.Equal(t, 1, failedTrigger)
}

func TestIsolateTaskFailed(t *testing.T) {
	// Verify that if one task out of a set fails to isolate, the others are
	// still triggered, inserted into the DB, etc.
	ctx, gb, _, s, swarmingClient, commits, _, cleanup := testMultipleCandidatesBackfillingEachOtherSetup(t)
	defer cleanup()

	bots := make([]*swarming_api.SwarmingRpcsBotInfo, 0, 25)
	for i := 0; i < 25; i++ {
		bots = append(bots, makeBot(fmt.Sprintf("bot%d", i), map[string]string{"pool": "Skia"}))
	}
	swarmingClient.MockBots(bots)

	// Create a new commit with a bad isolate.
	gb.Add(ctx, path.Join("infra", "bots", "dummy.isolate"), `sadkldsafkldsafkl30909098]]]]];;0`)
	badCommit := gb.Commit(ctx)

	// Create a commit which fixes the bad isolate.
	gb.Add(ctx, path.Join("infra", "bots", "dummy.isolate"), `{
  'variables': {
    'command': [
      'python', 'recipes.py', 'run',
    ],
    'files': [
      '../../somefile.txt',
    ],
  },
}`)
	fix := gb.Commit(ctx)
	commits = append([]string{fix, badCommit}, commits...)

	// Now we have 9 untested commits. Add 8 more to put the bad commit
	// right in the middle.
	for i := 0; i < 8; i++ {
		commits = append([]string{gb.CommitGen(ctx, "dummyfile")}, commits...)
	}
	// Expect no error since we don't block scheduling for permanent errors.
	updateRepos(t, ctx, s)
	err := s.MainLoop(ctx)
	s.testWaitGroup.Wait()
	require.NotNil(t, err)
	const isolateErrStr = "failed to process isolate"
	require.Contains(t, err.Error(), isolateErrStr)
	require.True(t, specs.ErrorIsPermanent(err))
	require.NoError(t, s.tCache.Update())
	// We'll try to trigger all tasks but the one for the bad commit will
	// fail. Ensure that we triggered all of the others.
	require.Equal(t, 1, len(s.queue))
	tasks, err := s.tCache.GetTasksForCommits(gb.RepoUrl(), commits)
	require.NoError(t, err)

	numTasks := 0
	for _, byName := range tasks {
		for range byName {
			numTasks++
		}
	}
	require.Equal(t, 18, numTasks)

	// Check diagnostics.
	diag := lastDiagnostics(t, s)
	failedIsolate := 0
	for _, c := range diag.Candidates {
		if c.Revision == badCommit {
			require.Contains(t, c.Diagnostics.Triggering.IsolateError, isolateErrStr)
			failedIsolate++
		} else if c.Diagnostics.Triggering == nil {
			// testMultipleCandidatesBackfillingEachOtherSetup triggers a task that is
			// still PENDING at this point, causing candidates for that job to be
			// superseded.
			require.NotNil(t, c.Diagnostics.Filtering)
			require.NotEqual(t, "", c.Diagnostics.Filtering.SupersededByTask)
		} else {
			require.Equal(t, "", c.Diagnostics.Triggering.IsolateError)
			require.NotEqual(t, "", c.Diagnostics.Triggering.TaskId)
		}
	}
	// Should be one task that failed
	require.Equal(t, 1, failedIsolate)
}

func TestTriggerTaskDeduped(t *testing.T) {
	// Verify that we properly handle de-duplicated tasks.
	ctx, gb, _, s, swarmingClient, commits, _, cleanup := testMultipleCandidatesBackfillingEachOtherSetup(t)
	defer cleanup()

	// Trigger three tasks. We should attempt to trigger tasks at
	// commits[0], commits[4], and either commits[2] or commits[6]. Mock
	// deduplication of the task at commits[4] and ensure that the other
	// two tasks are not deduped.
	bot1 := makeBot("bot1", map[string]string{"pool": "Skia"})
	bot2 := makeBot("bot2", map[string]string{"pool": "Skia"})
	bot3 := makeBot("bot3", map[string]string{"pool": "Skia"})
	makeTags := func(commit string) []string {
		return []string{
			"luci_project:",
			"milo_host:https://ci.chromium.org/raw/build/%s",
			"sk_attempt:0",
			"sk_dim_pool:Skia",
			"sk_retry_of:",
			fmt.Sprintf("source_revision:%s", commit),
			fmt.Sprintf("source_repo:%s/+/%%s", gb.RepoUrl()),
			fmt.Sprintf("sk_repo:%s", gb.RepoUrl()),
			fmt.Sprintf("sk_revision:%s", commit),
			"sk_forced_job_id:",
			"sk_name:dummytask",
		}
	}
	swarmingClient.MockBots([]*swarming_api.SwarmingRpcsBotInfo{bot1, bot2, bot3})
	swarmingClient.MockTriggerTaskDeduped(makeTags(commits[4]))
	require.NoError(t, s.MainLoop(ctx))
	s.testWaitGroup.Wait()
	require.NoError(t, s.tCache.Update())
	require.Equal(t, 5, len(s.queue))
	tasks, err := s.tCache.GetTasksForCommits(gb.RepoUrl(), commits)
	require.NoError(t, err)

	var t1, t2, t3 *types.Task
	for _, byName := range tasks {
		for _, task := range byName {
			if task.Revision == commits[0] {
				t1 = task
			} else if task.Revision == commits[4] {
				t2 = task
			} else if task.Revision == commits[2] || task.Revision == commits[6] {
				t3 = task
			} else {
				require.FailNow(t, fmt.Sprintf("Task has unknown revision: %v", task))
			}
		}
	}
	require.NotNil(t, t1)
	require.NotNil(t, t2)
	require.NotNil(t, t3)

	// Ensure that t2 was correctly deduped, and the others weren't.
	require.Equal(t, types.TASK_STATUS_PENDING, t1.Status)
	require.Equal(t, types.TASK_STATUS_SUCCESS, t2.Status)
	require.Equal(t, types.TASK_STATUS_PENDING, t3.Status)
}
