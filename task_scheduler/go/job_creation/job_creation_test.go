package job_creation

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

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
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/task_scheduler/go/db/memory"
	"go.skia.org/infra/task_scheduler/go/scheduling"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/syncer"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
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
	btProject, btInstance, btCleanup := tcc_testutils.SetupBigTable(t)
	taskCfgCache, err := task_cfg_cache.NewTaskCfgCache(ctx, repos, btProject, btInstance, nil)
	require.NoError(t, err)
	cas := &mocks.CAS{}
	// Go ahead and mock the single-input merge calls.
	cas.On("Merge", testutils.AnyContext, []string{tcc_testutils.CompileCASDigest}).Return(tcc_testutils.CompileCASDigest, nil)
	cas.On("Merge", testutils.AnyContext, []string{tcc_testutils.TestCASDigest}).Return(tcc_testutils.TestCASDigest, nil)
	cas.On("Merge", testutils.AnyContext, []string{tcc_testutils.PerfCASDigest}).Return(tcc_testutils.PerfCASDigest, nil)

	jc, err := NewJobCreator(ctx, d, time.Duration(math.MaxInt64), 0, tmp, "fake.server", repos, cas, urlMock.Client(), "skia", "fake-bb-target", tryjobs.BUCKET_TESTING, projectRepoMapping, depotTools, g, taskCfgCache, nil, syncer.DefaultNumWorkers)
	require.NoError(t, err)
	return ctx, gb, d, jc, urlMock, cas, func() {
		testutils.AssertCloses(t, jc)
		testutils.RemoveAll(t, tmp)
		gb.Cleanup()
		btCleanup()
		cancel()
	}
}

func updateRepos(t *testing.T, ctx context.Context, jc *JobCreator) {
	acked := false
	ack := func() {
		acked = true
	}
	nack := func() {
		require.FailNow(t, "Should not have called nack()")
	}
	err := jc.repos.UpdateWithCallback(ctx, func(repoUrl string, g *repograph.Graph) error {
		return jc.HandleRepoUpdate(ctx, repoUrl, g, ack, nack)
	})
	require.NoError(t, err)
	require.True(t, acked)
}

func makeDummyCommits(ctx context.Context, gb *git_testutils.GitBuilder, numCommits int) {
	for i := 0; i < numCommits; i++ {
		gb.AddGen(ctx, "dummyfile.txt")
		gb.CommitMsg(ctx, fmt.Sprintf("Dummy #%d/%d", i, numCommits))
	}
}

func TestGatherNewJobs(t *testing.T) {
	ctx, gb, _, jc, _, _, cleanup := setup(t)
	defer cleanup()

	testGatherNewJobs := func(expectedJobs int) {
		updateRepos(t, ctx, jc)
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
	makeDummyCommits(ctx, gb, 1)
	updateRepos(t, ctx, jc)
	testGatherNewJobs(8) // we didn't add to the jobs spec, so 3 jobs/rev.

	// Add several commits on main, ensure that we added all of the Jobs.
	makeDummyCommits(ctx, gb, 10)
	updateRepos(t, ctx, jc)
	testGatherNewJobs(38) // 3 jobs/rev + 8 pre-existing jobs.

	// Add a commit on a branch other than main, run gatherNewJobs, ensure
	// that we added the new Jobs.
	branchName := "otherBranch"
	gb.CreateBranchTrackBranch(ctx, branchName, git.MainBranch)
	msg := "Branch commit"
	fileName := "some_other_file"
	gb.Add(ctx, fileName, msg)
	gb.Commit(ctx)
	updateRepos(t, ctx, jc)
	testGatherNewJobs(41) // 38 previous jobs + 3 new ones.

	// Add several commits in a row on different branches, ensure that we
	// added all of the Jobs for all of the new commits.
	makeDummyCommits(ctx, gb, 5)
	gb.CheckoutBranch(ctx, git.MainBranch)
	makeDummyCommits(ctx, gb, 5)
	updateRepos(t, ctx, jc)
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
	updateRepos(t, ctx, jc)
	testGatherNewJobs(72)
}

func TestPeriodicJobs(t *testing.T) {
	ctx, gb, _, jc, _, _, cleanup := setup(t)
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
	updateRepos(t, ctx, jc)

	// Trigger the periodic jobs. Make sure that we inserted the new Job.
	require.NoError(t, jc.MaybeTriggerPeriodicJobs(ctx, specs.TRIGGER_NIGHTLY))
	require.NoError(t, jc.jCache.Update(ctx))
	start := time.Now().Add(-10 * time.Minute)
	end := time.Now().Add(10 * time.Minute)
	jobs, err := jc.jCache.GetMatchingJobsFromDateRange(names, start, end)
	require.NoError(t, err)
	require.Equal(t, 1, len(jobs[nightlyName]))
	require.Equal(t, nightlyName, jobs[nightlyName][0].Name)
	require.Equal(t, 0, len(jobs[weeklyName]))

	// Ensure that we don't trigger another.
	require.NoError(t, jc.MaybeTriggerPeriodicJobs(ctx, specs.TRIGGER_NIGHTLY))
	require.NoError(t, jc.jCache.Update(ctx))
	jobs, err = jc.jCache.GetMatchingJobsFromDateRange(names, start, end)
	require.NoError(t, err)
	require.Equal(t, 1, len(jobs[nightlyName]))
	require.Equal(t, 0, len(jobs[weeklyName]))

	// Hack the old Job's created time to simulate it scrolling out of the
	// window.
	oldJob := jobs[nightlyName][0]
	oldJob.Created = start.Add(-23 * time.Hour)
	require.NoError(t, jc.db.PutJob(ctx, oldJob))
	jc.jCache.AddJobs([]*types.Job{oldJob})
	require.NoError(t, jc.jCache.Update(ctx))
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
	taskExecs := map[string]types.TaskExecutor{
		types.TaskExecutor_UseDefault: swarmingTaskExec,
		types.TaskExecutor_Swarming:   swarmingTaskExec,
	}
	ts, err := scheduling.NewTaskScheduler(ctx, d, nil, time.Duration(math.MaxInt64), 0, jc.repos, cas, "fake-rbe-instance", taskExecs, urlMock.Client(), 1.0, swarming.POOLS_PUBLIC, "", jc.taskCfgCache, nil, mem_gcsclient.New("fake"), "testing", scheduling.BusyBotsDebugLoggingOff)
	require.NoError(t, err)

	jc.Start(ctx, false)
	ts.Start(ctx)

	// This should cause JobCreator to insert jobs into the DB, and Task
	// Scheduler should trigger tasks for them.
	updateRepos(t, ctx, jc)

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
