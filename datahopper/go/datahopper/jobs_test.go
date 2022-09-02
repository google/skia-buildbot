package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/metrics2/events"
	metrics2_testutils "go.skia.org/infra/go/metrics2/testutils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/db/memory"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	tcc_testutils "go.skia.org/infra/task_scheduler/go/task_cfg_cache/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

// Create a db.JobDB and jobEventDB, a channel which should be read from
// immediately after every Put into the JobDB, and a cleanup function which
// should be deferred.
func setupJobs(t *testing.T, now time.Time) (*jobEventDB, db.JobDB, <-chan struct{}, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	jdb := memory.NewInMemoryJobDB()
	period := TIME_PERIODS[len(TIME_PERIODS)-1]
	if OVERDUE_JOB_METRICS_PERIOD > period {
		period = OVERDUE_JOB_METRICS_PERIOD
	}
	w, err := window.New(ctx, period, 0, nil)
	if err != nil {
		sklog.Fatalf("Failed to create time window: %s", err)
	}
	wait := make(chan struct{})
	jCache, err := cache.NewJobCache(ctx, jdb, w, func() {
		wait <- struct{}{}
	})
	if err != nil {
		sklog.Fatalf("Failed to create job cache: %s", err)
	}
	<-wait
	edb := &jobEventDB{
		cached: []*events.Event{},
		jCache: jCache,
	}
	return edb, jdb, wait, cancel
}

// makeJob returns a fake job with only the fields relevant to this test set.
func makeJob(created time.Time, name string, status types.JobStatus, jobType jobTypeString, duration time.Duration) *types.Job {
	j := &types.Job{
		Created: created,
		Name:    name,
		Status:  status,
	}
	if jobType == FORCED {
		j.IsForce = true
	} else if jobType == TRYJOB {
		j.Issue = "1234"
		j.PatchRepo = "nou.git"
		j.Patchset = "1"
		j.Server = "Jeeves"
	}
	if status != types.JOB_STATUS_IN_PROGRESS {
		j.Finished = created.Add(duration)
	}
	return j
}

// assertJobEvent checks that ev.Data contains j.
func assertJobEvent(t *testing.T, ev *events.Event, j *types.Job) {
	require.Equal(t, JOB_STREAM, ev.Stream)
	var job types.Job
	require.NoError(t, gob.NewDecoder(bytes.NewReader(ev.Data)).Decode(&job))
	assertdeep.Equal(t, j, &job)
	require.True(t, j.Created.Equal(ev.Timestamp))
}

// TestJobUpdate checks that jobEventDB.update creates the correct Events from Jobs in the DB.
func TestJobUpdate(t *testing.T) {
	now := time.Now()
	edb, jdb, wait, cleanup := setupJobs(t, now)
	defer cleanup()
	start := now.Add(-TIME_PERIODS[len(TIME_PERIODS)-1])
	sklog.Errorf("start: %s", start)
	jobs := []*types.Job{
		// 0: Filtered out -- too early.
		makeJob(start.Add(-time.Minute), "A", types.JOB_STATUS_SUCCESS, NORMAL, time.Minute),
		makeJob(start.Add(time.Minute), "A", types.JOB_STATUS_SUCCESS, NORMAL, time.Minute),
		makeJob(start.Add(2*time.Minute), "A", types.JOB_STATUS_FAILURE, NORMAL, time.Minute),
		// 3: Filtered out -- not Done.
		makeJob(start.Add(3*time.Minute), "A", types.JOB_STATUS_IN_PROGRESS, NORMAL, time.Minute),
		makeJob(start.Add(4*time.Minute), "A", types.JOB_STATUS_MISHAP, NORMAL, time.Hour),
		makeJob(start.Add(5*time.Minute), "A", types.JOB_STATUS_CANCELED, NORMAL, time.Hour),
		makeJob(start.Add(6*time.Minute), "B", types.JOB_STATUS_SUCCESS, TRYJOB, time.Minute),
		makeJob(start.Add(7*time.Minute), "A", types.JOB_STATUS_SUCCESS, NORMAL, time.Hour),
	}
	require.NoError(t, jdb.PutJobs(context.Background(), jobs))
	<-wait
	require.NoError(t, edb.update())
	evs, err := edb.Range(JOB_STREAM, start.Add(-time.Hour), start.Add(time.Hour))
	require.NoError(t, err)

	expected := append(jobs[1:3], jobs[4:8]...)
	require.Len(t, evs, len(expected))
	for i, ev := range evs {
		assertJobEvent(t, ev, expected[i])
	}
}

// TestJobRange checks that jobEventDB.Range returns Events within the given range.
func TestJobRange(t *testing.T) {
	now := time.Now()
	edb, jdb, wait, cleanup := setupJobs(t, now)
	defer cleanup()
	base := now.Add(-time.Hour)
	jobs := []*types.Job{
		makeJob(base.Add(-time.Nanosecond), "A", types.JOB_STATUS_SUCCESS, NORMAL, time.Minute),
		makeJob(base, "A", types.JOB_STATUS_SUCCESS, NORMAL, time.Minute),
		makeJob(base.Add(time.Nanosecond), "A", types.JOB_STATUS_SUCCESS, NORMAL, time.Minute),
		makeJob(base.Add(time.Minute), "A", types.JOB_STATUS_SUCCESS, NORMAL, time.Minute),
	}
	require.NoError(t, jdb.PutJobs(context.Background(), jobs))
	<-wait
	require.NoError(t, edb.update())

	test := func(start, end time.Time, startIdx, count int) {
		evs, err := edb.Range(JOB_STREAM, start, end)
		require.NoError(t, err)
		require.Len(t, evs, count)
		for i, ev := range evs {
			assertJobEvent(t, ev, jobs[startIdx+i])
		}
	}
	before := base.Add(-time.Hour)
	after := base.Add(time.Hour)
	test(before, before, -1, 0)
	test(before, jobs[0].Created, -1, 0)
	test(before, jobs[1].Created, 0, 1)
	test(before, jobs[2].Created, 0, 2)
	test(before, jobs[3].Created, 0, 3)
	test(before, after, 0, 4)
	test(jobs[0].Created, before, -1, 0)
	test(jobs[0].Created, jobs[0].Created, -1, 0)
	test(jobs[0].Created, jobs[1].Created, 0, 1)
	test(jobs[0].Created, jobs[2].Created, 0, 2)
	test(jobs[0].Created, jobs[3].Created, 0, 3)
	test(jobs[0].Created, after, 0, 4)
	test(jobs[1].Created, jobs[0].Created, -1, 0)
	test(jobs[1].Created, jobs[1].Created, -1, 0)
	test(jobs[1].Created, jobs[2].Created, 1, 1)
	test(jobs[1].Created, jobs[3].Created, 1, 2)
	test(jobs[1].Created, after, 1, 3)
	test(jobs[2].Created, jobs[2].Created, -1, 0)
	test(jobs[2].Created, jobs[3].Created, 2, 1)
	test(jobs[2].Created, after, 2, 2)
	test(jobs[3].Created, jobs[3].Created, -1, 0)
	test(jobs[3].Created, after, 3, 1)
	test(after, after, -1, 0)
}

// DynamicAggregateFnTester stores the expected results of a call to a events.DynamicAggregateFn.
type DynamicAggregateFnTester struct {
	t *testing.T
	f events.DynamicAggregateFn
	// expected is map[util.MD5Sum(tags)]value
	expected map[string]float64
}

func newDynamicAggregateFnTester(t *testing.T, f events.DynamicAggregateFn) *DynamicAggregateFnTester {
	return &DynamicAggregateFnTester{
		t:        t,
		f:        f,
		expected: map[string]float64{},
	}
}

// AddAssert causes a later call to Run to check that the DynamicAggregateFn returns the given value
// for the given tags.
func (dt *DynamicAggregateFnTester) AddAssert(tags map[string]string, value float64) {
	hash, err := util.MD5Sum(tags)
	require.NoError(dt.t, err)
	_, exists := dt.expected[hash]
	require.False(dt.t, exists, "Your test broke MD5. %v", tags)
	dt.expected[hash] = value
}

// Run calls the DynamicAggregateFn and checks that the return values are exactly those given by
// AddAssert.
func (dt *DynamicAggregateFnTester) Run(evs []*events.Event) {
	actualTags, actualVals, err := dt.f(evs)
	require.NoError(dt.t, err)
	require.Len(dt.t, actualTags, len(dt.expected))
	require.Len(dt.t, actualVals, len(dt.expected))
	for i, tags := range actualTags {
		actualVal := actualVals[i]
		hash, err := util.MD5Sum(tags)
		require.NoError(dt.t, err)
		expectedVal, ok := dt.expected[hash]
		require.True(dt.t, ok, "Unexpected tags %v", tags)
		require.Equal(dt.t, expectedVal, actualVal, "For tags %v", tags)
	}
}

func TestComputeAvgJobDuration(t *testing.T) {
	now := time.Now()
	edb, jdb, wait, cleanup := setupJobs(t, now)
	defer cleanup()
	created := now.Add(-time.Hour)

	tester := newDynamicAggregateFnTester(t, computeAvgJobDuration)
	expect := func(jobName string, jobType jobTypeString, jobs []*types.Job) {
		var totalDur float64 = 0
		for _, j := range jobs {
			totalDur += float64(j.Finished.Sub(j.Created))
		}
		tester.AddAssert(map[string]string{
			"job_name": jobName,
			"job_type": string(jobType),
		}, totalDur/float64(len(jobs)))
	}

	// Expect only SUCCESS and FAILURE to contribute to avg duration.
	jobsStatus := []*types.Job{
		makeJob(created, "AllStatus", types.JOB_STATUS_SUCCESS, NORMAL, 10*time.Minute),
		makeJob(created, "AllStatus", types.JOB_STATUS_SUCCESS, NORMAL, 10*time.Minute),
		makeJob(created, "AllStatus", types.JOB_STATUS_FAILURE, NORMAL, 13*time.Minute),
		makeJob(created, "AllStatus", types.JOB_STATUS_CANCELED, NORMAL, time.Minute),
		makeJob(created, "AllStatus", types.JOB_STATUS_MISHAP, NORMAL, time.Minute),
		makeJob(created, "IgnoredStatus", types.JOB_STATUS_CANCELED, NORMAL, time.Minute),
		makeJob(created, "IgnoredStatus", types.JOB_STATUS_MISHAP, NORMAL, time.Minute),
	}
	require.NoError(t, jdb.PutJobs(context.Background(), jobsStatus))
	<-wait

	expect("AllStatus", NORMAL, jobsStatus[0:3])

	jobsType := []*types.Job{
		makeJob(created, "OnlyForced", types.JOB_STATUS_SUCCESS, FORCED, 10*time.Minute),
		makeJob(created, "OnlyForced", types.JOB_STATUS_FAILURE, FORCED, 11*time.Minute),
		makeJob(created, "NormalAndTryjob", types.JOB_STATUS_SUCCESS, NORMAL, 10*time.Minute),
		makeJob(created, "NormalAndTryjob", types.JOB_STATUS_SUCCESS, TRYJOB, 11*time.Minute),
		makeJob(created, "NormalAndTryjob", types.JOB_STATUS_FAILURE, TRYJOB, 12*time.Minute),
		makeJob(created, "NormalAndTryjob", types.JOB_STATUS_FAILURE, NORMAL, 9*time.Minute),
		makeJob(created, "AllTypes", types.JOB_STATUS_SUCCESS, NORMAL, 10*time.Minute),
		makeJob(created, "AllTypes", types.JOB_STATUS_SUCCESS, TRYJOB, 11*time.Minute),
		makeJob(created, "AllTypes", types.JOB_STATUS_SUCCESS, FORCED, 12*time.Minute),
	}
	require.NoError(t, jdb.PutJobs(context.Background(), jobsType))
	<-wait

	expect("OnlyForced", FORCED, jobsType[0:2])
	expect("NormalAndTryjob", NORMAL, []*types.Job{jobsType[2], jobsType[5]})
	expect("NormalAndTryjob", TRYJOB, jobsType[3:5])
	expect("AllTypes", NORMAL, jobsType[6:7])
	expect("AllTypes", TRYJOB, jobsType[7:8])
	expect("AllTypes", FORCED, jobsType[8:9])

	require.NoError(t, edb.update())
	evs, err := edb.Range(JOB_STREAM, created.Add(-time.Hour), created.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, evs, len(jobsStatus)+len(jobsType))

	tester.Run(evs)
}

func TestComputeJobFailureMishapRate(t *testing.T) {
	now := time.Now()
	edb, jdb, wait, cleanup := setupJobs(t, now)
	defer cleanup()
	created := now.Add(-time.Hour)

	tester := newDynamicAggregateFnTester(t, computeJobFailureMishapRate)
	expect := func(jobName string, metric string, numer, denom int) {
		tester.AddAssert(map[string]string{
			"job_name": jobName,
			"job_type": "",
			"metric":   metric,
		}, float64(numer)/float64(denom))
	}

	jobCount := 0
	addJob := func(name string, status types.JobStatus, jobType jobTypeString) {
		jobCount++
		require.NoError(t, jdb.PutJob(context.Background(), makeJob(created, name, status, jobType, time.Minute)))
		<-wait
	}

	{
		name := "AllStatus"
		addJob(name, types.JOB_STATUS_SUCCESS, NORMAL)
		addJob(name, types.JOB_STATUS_SUCCESS, NORMAL)
		addJob(name, types.JOB_STATUS_FAILURE, NORMAL)
		addJob(name, types.JOB_STATUS_CANCELED, NORMAL)
		addJob(name, types.JOB_STATUS_MISHAP, NORMAL)
		expect(name, "failure-rate", 1, 5)
		expect(name, "mishap-rate", 1, 5)
	}
	{
		name := "NoSuccess"
		addJob(name, types.JOB_STATUS_FAILURE, NORMAL)
		addJob(name, types.JOB_STATUS_FAILURE, NORMAL)
		addJob(name, types.JOB_STATUS_CANCELED, NORMAL)
		addJob(name, types.JOB_STATUS_MISHAP, NORMAL)
		expect(name, "failure-rate", 2, 4)
		expect(name, "mishap-rate", 1, 4)
	}
	{
		name := "NoFailure"
		addJob(name, types.JOB_STATUS_SUCCESS, NORMAL)
		addJob(name, types.JOB_STATUS_CANCELED, NORMAL)
		expect(name, "failure-rate", 0, 2)
		expect(name, "mishap-rate", 0, 2)
	}
	{
		name := "IgnoredStatus"
		addJob(name, types.JOB_STATUS_CANCELED, NORMAL)
		expect(name, "failure-rate", 0, 1)
		expect(name, "mishap-rate", 0, 1)
	}
	{
		name := "100PercentSuccess"
		addJob(name, types.JOB_STATUS_SUCCESS, NORMAL)
		expect(name, "failure-rate", 0, 1)
		expect(name, "mishap-rate", 0, 1)
	}
	{
		name := "100PercentFailure"
		addJob(name, types.JOB_STATUS_FAILURE, NORMAL)
		addJob(name, types.JOB_STATUS_FAILURE, NORMAL)
		addJob(name, types.JOB_STATUS_FAILURE, NORMAL)
		expect(name, "failure-rate", 3, 3)
		expect(name, "mishap-rate", 0, 3)
	}
	{
		name := "100PercentMishap"
		addJob(name, types.JOB_STATUS_MISHAP, NORMAL)
		addJob(name, types.JOB_STATUS_MISHAP, NORMAL)
		expect(name, "failure-rate", 0, 2)
		expect(name, "mishap-rate", 2, 2)
	}
	{
		// Job type doesn't matter for these metrics.
		name := "OnlyTryjobs"
		addJob(name, types.JOB_STATUS_SUCCESS, TRYJOB)
		addJob(name, types.JOB_STATUS_FAILURE, TRYJOB)
		addJob(name, types.JOB_STATUS_MISHAP, TRYJOB)
		expect(name, "failure-rate", 1, 3)
		expect(name, "mishap-rate", 1, 3)
	}

	require.NoError(t, edb.update())
	evs, err := edb.Range(JOB_STREAM, created.Add(-time.Hour), created.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, evs, jobCount)

	tester.Run(evs)
}

func TestOverdueJobSpecMetrics(t *testing.T) {

	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, wd)

	ctx, gb, _, _ := tcc_testutils.SetupTestRepo(t)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	repos, err := repograph.NewLocalMap(ctx, []string{gb.RepoUrl()}, wd)
	require.NoError(t, err)
	require.NoError(t, repos.Update(ctx))
	repo := repos[gb.RepoUrl()]

	d := memory.NewInMemoryDB()
	period := TIME_PERIODS[len(TIME_PERIODS)-1]
	if OVERDUE_JOB_METRICS_PERIOD > period {
		period = OVERDUE_JOB_METRICS_PERIOD
	}
	w, err := window.New(ctx, period, OVERDUE_JOB_METRICS_NUM_COMMITS, repos)
	if err != nil {
		sklog.Fatalf("Failed to create time window: %s", err)
	}
	wait := make(chan struct{})
	jCache, err := cache.NewJobCache(ctx, d, w, func() {
		wait <- struct{}{}
	})
	if err != nil {
		sklog.Fatalf("Failed to create job cache: %s", err)
	}

	btProject, btInstance, btCleanup := tcc_testutils.SetupBigTable(t)
	defer btCleanup()
	tcc, err := task_cfg_cache.NewTaskCfgCache(ctx, repos, btProject, btInstance, nil)
	require.NoError(t, err)

	c1, err := git.GitDir(gb.Dir()).RevParse(ctx, "HEAD^")
	require.NoError(t, err)
	c1time := repo.Get(c1).Timestamp
	c2, err := git.GitDir(gb.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	// c2 is 5 seconds after c1
	c2time := repo.Get(c2).Timestamp

	// Load the TasksCfg for each commit into the cache.
	insertTasksCfg := func(commit string) {
		out, err := git.GitDir(gb.Dir()).Git(ctx, "show", fmt.Sprintf("%s:infra/bots/tasks.json", commit))
		require.NoError(t, err)
		cfg, err := specs.ParseTasksCfg(out)
		require.NoError(t, err)
		sklog.Errorf("Inserting TasksCfg for %s", commit)
		require.NoError(t, tcc.Set(ctx, types.RepoState{
			Repo:     gb.RepoUrl(),
			Revision: commit,
		}, cfg, nil))
	}
	insertTasksCfg(c1)
	insertTasksCfg(c2)

	// At 'now', c1 is 60 seconds old, c2 is 55 seconds old, and c3 (below) is 50 seconds old.
	now := c1time.Add(time.Minute)
	c1age := "60"
	c2age := "55"
	c3age := "50"

	om, err := newOverdueJobMetrics(jCache, repos, tcc, w)
	require.NoError(t, err)

	check := func(buildAge, testAge, perfAge string) {
		d.Wait()
		<-wait
		require.NoError(t, om.updateOverdueJobSpecMetrics(ctx, now))
		tags := map[string]string{
			"repo":        gb.RepoUrl(),
			"job_name":    tcc_testutils.BuildTaskName,
			"job_trigger": "",
		}
		require.Equal(t, buildAge, metrics2_testutils.GetRecordedMetric(t, MEASUREMENT_OVERDUE_JOB_SPECS, tags))

		tags["job_name"] = tcc_testutils.TestTaskName
		require.Equal(t, testAge, metrics2_testutils.GetRecordedMetric(t, MEASUREMENT_OVERDUE_JOB_SPECS, tags))

		tags["job_name"] = tcc_testutils.PerfTaskName
		require.Equal(t, perfAge, metrics2_testutils.GetRecordedMetric(t, MEASUREMENT_OVERDUE_JOB_SPECS, tags))
	}

	// No jobs have finished yet.
	check(c1age, c1age, c2age)

	// Insert jobs.
	j1 := &types.Job{
		Name: tcc_testutils.BuildTaskName,
		RepoState: types.RepoState{
			Repo:     gb.RepoUrl(),
			Revision: c1,
		},
		Created: c1time,
	}
	j2 := &types.Job{
		Name: tcc_testutils.BuildTaskName,
		RepoState: types.RepoState{
			Repo:     gb.RepoUrl(),
			Revision: c2,
		},
		Created: c2time,
	}
	j3 := &types.Job{
		Name: tcc_testutils.TestTaskName,
		RepoState: types.RepoState{
			Repo:     gb.RepoUrl(),
			Revision: c1,
		},
		Created: c1time,
	}
	j4 := &types.Job{
		Name: tcc_testutils.TestTaskName,
		RepoState: types.RepoState{
			Repo:     gb.RepoUrl(),
			Revision: c2,
		},
		Created: c2time,
	}
	j5 := &types.Job{
		Name: tcc_testutils.PerfTaskName,
		RepoState: types.RepoState{
			Repo:     gb.RepoUrl(),
			Revision: c2,
		},
		Created: c2time,
	}
	require.NoError(t, d.PutJobs(ctx, []*types.Job{j1, j2, j3, j4, j5}))

	// Jobs have not completed, so same as above.
	check(c1age, c1age, c2age)

	// One job is complete.
	j2.Status = types.JOB_STATUS_SUCCESS
	j2.Finished = time.Now()
	require.NoError(t, d.PutJob(ctx, j2))

	// Expect Build to be up-to-date.
	check("0", c1age, c2age)

	// Revert back to c1 (no Perf task) and check that Perf job disappears.
	content, err := git.GitDir(gb.Dir()).GetFile(ctx, "infra/bots/tasks.json", c1)
	require.NoError(t, err)
	gb.Add(ctx, "infra/bots/tasks.json", content)
	c3 := gb.CommitMsgAt(ctx, "c3", c1time.Add(10*time.Second)) // 5 seconds after c2
	require.NoError(t, repos.Update(ctx))
	c3time := repo.Get(c3).Timestamp
	insertTasksCfg(c3)

	// Update to c3. Build job age is now at c3. Perf job should be missing.
	j6 := &types.Job{
		Name: tcc_testutils.BuildTaskName,
		RepoState: types.RepoState{
			Repo:     gb.RepoUrl(),
			Revision: c3,
		},
		Created: c3time,
	}
	require.NoError(t, d.PutJob(ctx, j6))

	check(c3age, c1age, fmt.Sprintf("Could not find anything for overdue_job_specs_s{job_name=\"%s\",job_trigger=\"\",repo=\"%s\"}", tcc_testutils.PerfTaskName, gb.RepoUrl()))
}
