package job_creation

import (
	"context"
	"fmt"
	"math"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/cas/mocks"
	depot_tools_testutils "go.skia.org/infra/go/depot_tools/testutils"
	"go.skia.org/infra/go/gcs/mem_gcsclient"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
	cacher_mocks "go.skia.org/infra/task_scheduler/go/cacher/mocks"
	"go.skia.org/infra/task_scheduler/go/db/memory"
	"go.skia.org/infra/task_scheduler/go/scheduling"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/syncer"
	tcc_mocks "go.skia.org/infra/task_scheduler/go/task_cfg_cache/mocks"
	tcc_testutils "go.skia.org/infra/task_scheduler/go/task_cfg_cache/testutils"
	swarming_task_execution "go.skia.org/infra/task_scheduler/go/task_execution/swarmingv2"
	swarming_testutils "go.skia.org/infra/task_scheduler/go/testutils"
	"go.skia.org/infra/task_scheduler/go/tryjobs"
	"go.skia.org/infra/task_scheduler/go/types"
)

const (
	fakeGerritUrl = "https://fake-skia-review.googlesource.com"
)

// Common setup for JobCreator tests.
func setup(t *testing.T) (context.Context, *git_testutils.GitBuilder, *memory.InMemoryDB, *JobCreator, *mockhttpclient.URLMock, *mocks.CAS, func()) {
	ctx, gb, _, _ := tcc_testutils.SetupTestRepo(t)
	ctx, cancel := context.WithCancel(ctx)

	tmp, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	d := memory.NewInMemoryDB()
	urlMock := mockhttpclient.NewURLMock()
	repos, err := repograph.NewLocalMap(ctx, []string{gb.RepoUrl()}, tmp)
	require.NoError(t, err)
	require.NoError(t, repos.Update(ctx))
	projectRepoMapping := map[string]string{
		"skia": gb.RepoUrl(),
	}
	depotTools := depot_tools_testutils.GetDepotTools(t, ctx)
	g, err := gerrit.NewGerrit(fakeGerritUrl, urlMock.Client())
	require.NoError(t, err)

	cas := &mocks.CAS{}
	// Go ahead and mock the single-input merge calls.
	cas.On("Merge", testutils.AnyContext, []string{tcc_testutils.CompileCASDigest}).Return(tcc_testutils.CompileCASDigest, nil)
	cas.On("Merge", testutils.AnyContext, []string{tcc_testutils.TestCASDigest}).Return(tcc_testutils.TestCASDigest, nil)
	cas.On("Merge", testutils.AnyContext, []string{tcc_testutils.PerfCASDigest}).Return(tcc_testutils.PerfCASDigest, nil)

	c1 := repos[gb.RepoUrl()].Get(git.MainBranch)
	c0 := c1.Parents[0]
	rs0 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c0,
	}
	rs1 := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1.Hash,
	}

	taskCfgCache := &tcc_mocks.TaskCfgCache{}
	taskCfgCache.On("Get", testutils.AnyContext, rs1).Return(tcc_testutils.TasksCfg2, nil, nil)
	taskCfgCache.On("Get", testutils.AnyContext, rs0).Return(tcc_testutils.TasksCfg1, nil, nil)
	taskCfgCache.On("Cleanup", testutils.AnyContext, mock.Anything).Return(nil)

	jc, err := newJobCreatorWithoutInit(ctx, d, time.Duration(math.MaxInt64), 0, tmp, "fake.server", repos, cas, urlMock.Client(), "skia", "fake-bb-target", tryjobs.BUCKET_TESTING, projectRepoMapping, depotTools, g, taskCfgCache, nil, syncer.DefaultNumWorkers, true)
	require.NoError(t, err)

	mockCacher := &cacher_mocks.Cacher{}
	jc.cacher = mockCacher
	mockCacher.On("GetOrCacheRepoState", testutils.AnyContext, rs1).Return(tcc_testutils.TasksCfg2, nil)
	mockCacher.On("GetOrCacheRepoState", testutils.AnyContext, rs0).Return(tcc_testutils.TasksCfg1, nil)

	require.NoError(t, jc.initCaches(ctx))

	jc.Start(ctx, false)

	return ctx, gb, d, jc, urlMock, cas, func() {
		taskCfgCache.On("Close").Return(nil)
		testutils.AssertCloses(t, jc)
		mockCacher.AssertExpectations(t)
		testutils.RemoveAll(t, tmp)
		gb.Cleanup()
		cancel()
	}
}

func updateReposAndStartJobs(t *testing.T, ctx context.Context, jc *JobCreator) {
	var wg sync.WaitGroup
	wg.Add(1)
	err := jc.repos.UpdateWithCallback(ctx, func(repoUrl string, g *repograph.Graph) error {
		jc.triggerRepoUpdate(repoUrl, func(err error) {
			require.NoError(t, err)
			wg.Done()
		})
		return nil
	})
	require.NoError(t, err)
	wg.Wait()
}

func mockLastCommit(t *testing.T, ctx context.Context, gb *git_testutils.GitBuilder, taskCfgCache *tcc_mocks.TaskCfgCache, cacher *cacher_mocks.Cacher) {
	tasksCfg, err := specs.ReadTasksCfg(gb.Dir())
	require.NoError(t, err)

	hash, err := git.CheckoutDir(gb.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	rs := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: hash,
	}
	taskCfgCache.On("Get", testutils.AnyContext, rs).Return(tasksCfg, nil, nil)
	cacher.On("GetOrCacheRepoState", testutils.AnyContext, rs).Return(tasksCfg, nil, nil)
}

func makeFakeCommits(t *testing.T, ctx context.Context, gb *git_testutils.GitBuilder, taskCfgCache *tcc_mocks.TaskCfgCache, cacher *cacher_mocks.Cacher, numCommits int) {
	for i := 0; i < numCommits; i++ {
		gb.AddGen(ctx, "fakefile.txt")
		gb.CommitMsg(ctx, fmt.Sprintf("Fake #%d/%d", i, numCommits))
		mockLastCommit(t, ctx, gb, taskCfgCache, cacher)
	}
}

func TestGatherNewJobs(t *testing.T) {
	ctx, gb, _, jc, _, _, cleanup := setup(t)
	defer cleanup()

	tcc := jc.taskCfgCache.(*tcc_mocks.TaskCfgCache)
	cacher := jc.cacher.(*cacher_mocks.Cacher)

	testGatherNewJobs := func(expectedJobs int) {
		updateReposAndStartJobs(t, ctx, jc)
		jobs, err := jc.jCache.InProgressJobs()
		require.NoError(t, err)
		require.Equal(t, expectedJobs, len(jobs))
	}

	// Ensure that the JobDB is empty.
	jobs, err := jc.jCache.InProgressJobs()
	require.NoError(t, err)
	require.Equal(t, 0, len(jobs))

	// Run gatherNewJobs, ensure that we added jobs for all commits in the
	// repo.
	testGatherNewJobs(5) // c1 has 2 jobs, c2 has 3 jobs.

	// Run gatherNewJobs again, ensure that we didn't add the same Jobs
	// again.
	testGatherNewJobs(5) // no new jobs == 5 total jobs.

	// Add a commit on main, run gatherNewJobs, ensure that we added the
	// new Jobs.
	makeFakeCommits(t, ctx, gb, tcc, cacher, 1)
	updateReposAndStartJobs(t, ctx, jc)
	testGatherNewJobs(8) // we didn't add to the jobs spec, so 3 jobs/rev.

	// Add several commits on main, ensure that we added all of the Jobs.
	makeFakeCommits(t, ctx, gb, tcc, cacher, 10)
	updateReposAndStartJobs(t, ctx, jc)
	testGatherNewJobs(38) // 3 jobs/rev + 8 pre-existing jobs.

	// Add a commit on a branch other than main, run gatherNewJobs, ensure
	// that we added the new Jobs.
	branchName := "otherBranch"
	gb.CreateBranchTrackBranch(ctx, branchName, git.MainBranch)
	msg := "Branch commit"
	fileName := "some_other_file"
	gb.Add(ctx, fileName, msg)
	gb.Commit(ctx)
	mockLastCommit(t, ctx, gb, tcc, cacher)
	updateReposAndStartJobs(t, ctx, jc)
	testGatherNewJobs(41) // 38 previous jobs + 3 new ones.

	// Add several commits in a row on different branches, ensure that we
	// added all of the Jobs for all of the new commits.
	makeFakeCommits(t, ctx, gb, tcc, cacher, 5)
	gb.CheckoutBranch(ctx, git.MainBranch)
	makeFakeCommits(t, ctx, gb, tcc, cacher, 5)
	updateReposAndStartJobs(t, ctx, jc)
	testGatherNewJobs(71) // 10 commits x 3 jobs/commit = 30, plus 41

	// Add one more commit on the non-main branch which marks all but one
	// job to only run on main. Ensure that we don't pick them up.
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
	mockLastCommit(t, ctx, gb, tcc, cacher)
	updateReposAndStartJobs(t, ctx, jc)
	testGatherNewJobs(72)
}

func TestPeriodicJobs(t *testing.T) {
	ctx, gb, _, jc, _, _, cleanup := setup(t)
	defer cleanup()

	startTime := time.Unix(1754927651, 0) // Arbitrary start time.
	nowContext := now.TimeTravelingContext(ctx, startTime)
	advanceTime := func(duration time.Duration) {
		nowContext.SetTime(now.Now(ctx).Add(duration))
	}
	ctx = nowContext

	tcc := jc.taskCfgCache.(*tcc_mocks.TaskCfgCache)
	cacher := jc.cacher.(*cacher_mocks.Cacher)

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
				CasSpec:          "compile",
				Priority:         1.0,
			},
		},
		CasSpecs: map[string]*specs.CasSpec{
			"compile": {
				Digest: "abc123/45",
			},
		},
	}
	gb.Add(ctx, specs.TASKS_CFG_FILE, testutils.MarshalJSON(t, &cfg))
	gb.Commit(ctx)
	mockLastCommit(t, ctx, gb, tcc, cacher)
	updateReposAndStartJobs(t, ctx, jc)

	// Trigger the periodic jobs. Make sure that we inserted the new Job.
	require.NoError(t, jc.MaybeTriggerPeriodicJobs(ctx, specs.TRIGGER_NIGHTLY))
	require.NoError(t, jc.jCache.Update(ctx))
	start := startTime.Add(-10 * time.Minute)
	end := startTime.Add(10 * time.Minute)
	jobs, err := jc.jCache.GetMatchingJobsFromDateRange(names, start, end)
	require.NoError(t, err)
	require.Equal(t, 1, len(jobs[nightlyName]))
	require.Equal(t, nightlyName, jobs[nightlyName][0].Name)
	require.Equal(t, 0, len(jobs[weeklyName]))

	// Ensure that we don't trigger another.
	advanceTime(time.Second) // This ensures that we see the job we just added.
	require.NoError(t, jc.MaybeTriggerPeriodicJobs(ctx, specs.TRIGGER_NIGHTLY))
	require.NoError(t, jc.jCache.Update(ctx))
	jobs, err = jc.jCache.GetMatchingJobsFromDateRange(names, start, end)
	require.NoError(t, err)
	require.Equal(t, 1, len(jobs[nightlyName]))
	require.Equal(t, 0, len(jobs[weeklyName]))

	// Fast-forward time to scroll the old job out of the window.
	advanceTime(24 * time.Hour)
	require.NoError(t, jc.window.Update(ctx))
	require.NoError(t, jc.jCache.Update(ctx))
	start = now.Now(ctx).Add(-10 * time.Minute)
	end = now.Now(ctx).Add(10 * time.Minute)
	jobs, err = jc.jCache.GetMatchingJobsFromDateRange(names, start, end)
	require.NoError(t, err)
	require.Equal(t, 0, len(jobs[nightlyName]))
	require.Equal(t, 0, len(jobs[weeklyName]))
	require.NoError(t, jc.MaybeTriggerPeriodicJobs(ctx, specs.TRIGGER_NIGHTLY))
	require.NoError(t, jc.jCache.Update(ctx))
	jobs, err = jc.jCache.GetMatchingJobsFromDateRange(names, start, end)
	require.NoError(t, err)
	require.Equal(t, 1, len(jobs[nightlyName]))
	require.Equal(t, nightlyName, jobs[nightlyName][0].Name)
	require.Equal(t, 0, len(jobs[weeklyName]))

	// Make sure we don't confuse different triggers.
	require.NoError(t, jc.MaybeTriggerPeriodicJobs(ctx, specs.TRIGGER_WEEKLY))
	require.NoError(t, jc.jCache.Update(ctx))
	jobs, err = jc.jCache.GetMatchingJobsFromDateRange(names, start, end)
	require.NoError(t, err)
	require.Equal(t, 1, len(jobs[nightlyName]))
	require.Equal(t, nightlyName, jobs[nightlyName][0].Name)
	require.Equal(t, 1, len(jobs[weeklyName]))
	require.Equal(t, weeklyName, jobs[weeklyName][0].Name)
}

func TestTaskSchedulerIntegration(t *testing.T) {
	ctx, _, d, jc, _, cas, cleanup := setup(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	tmp, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)
	swarmingClient := swarming_testutils.NewTestClient()
	urlMock := mockhttpclient.NewURLMock()
	cas.On("Close").Return(nil)
	swarmingTaskExec := swarming_task_execution.NewSwarmingV2TaskExecutor(swarmingClient, "fake-cas-instance", "")
	taskExecs := types.NewTaskExecutors("fake-swarming")
	taskExecs.Set("fake-swarming", swarmingTaskExec, swarming.POOLS_PUBLIC)
	ts, err := scheduling.NewTaskScheduler(ctx, d, nil, time.Duration(math.MaxInt64), 0, jc.repos, cas, "fake-rbe-instance", taskExecs, urlMock.Client(), 1.0, "", jc.taskCfgCache, nil, mem_gcsclient.New("fake"), "testing", scheduling.BusyBotsDebugLoggingOff)
	require.NoError(t, err)

	jc.Start(ctx, false)
	ts.Start(ctx)

	// This should cause JobCreator to insert jobs into the DB, and Task
	// Scheduler should trigger tasks for them.
	updateReposAndStartJobs(t, ctx, jc)

	bot1 := &apipb.BotInfo{
		BotId: "bot1",
		Dimensions: []*apipb.StringListPair{
			{
				Key:   "pool",
				Value: []string{"Skia"},
			},
			{
				Key:   "os",
				Value: []string{"Ubuntu"},
			},
		},
	}
	swarmingClient.MockBots([]*apipb.BotInfo{bot1})

	require.Eventually(t, func() bool {
		tasks, err := d.GetTasksFromDateRange(ctx, vcsinfo.MinTime, vcsinfo.MaxTime, "")
		require.NoError(t, err)
		if len(tasks) > 0 {
			sklog.Errorf("Triggered tasks!")
			return true
		}
		return false
	}, 2*time.Minute, 100*time.Millisecond)
}
