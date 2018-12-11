package main

import (
	"bytes"
	"encoding/gob"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/metrics2/events"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/memory"
	"go.skia.org/infra/task_scheduler/go/types"
)

// Create a db.JobDB and jobEventDB.
func setupJobs(t *testing.T, now time.Time) (*jobEventDB, db.JobDB) {
	jdb := memory.NewInMemoryJobDB(nil)
	edb := &jobEventDB{
		cached: []*events.Event{},
		db:     jdb,
	}
	return edb, jdb
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
	assert.Equal(t, JOB_STREAM, ev.Stream)
	var job types.Job
	assert.NoError(t, gob.NewDecoder(bytes.NewReader(ev.Data)).Decode(&job))
	deepequal.AssertDeepEqual(t, j, &job)
	assert.True(t, j.Created.Equal(ev.Timestamp))
}

// TestJobUpdate checks that jobEventDB.update creates the correct Events from Jobs in the DB.
func TestJobUpdate(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now()
	edb, jdb := setupJobs(t, now)
	start := now.Add(-TIME_PERIODS[len(TIME_PERIODS)-1])
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
	assert.NoError(t, jdb.PutJobs(jobs))
	assert.NoError(t, edb.update())
	evs, err := edb.Range(JOB_STREAM, start.Add(-time.Hour), start.Add(time.Hour))
	assert.NoError(t, err)

	expected := append(jobs[1:3], jobs[4:8]...)
	assert.Len(t, evs, len(expected))
	for i, ev := range evs {
		assertJobEvent(t, ev, expected[i])
	}
}

// TestJobRange checks that jobEventDB.Range returns Events within the given range.
func TestJobRange(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now()
	edb, jdb := setupJobs(t, now)
	base := now.Add(-time.Hour)
	jobs := []*types.Job{
		makeJob(base.Add(-time.Nanosecond), "A", types.JOB_STATUS_SUCCESS, NORMAL, time.Minute),
		makeJob(base, "A", types.JOB_STATUS_SUCCESS, NORMAL, time.Minute),
		makeJob(base.Add(time.Nanosecond), "A", types.JOB_STATUS_SUCCESS, NORMAL, time.Minute),
		makeJob(base.Add(time.Minute), "A", types.JOB_STATUS_SUCCESS, NORMAL, time.Minute),
	}
	assert.NoError(t, jdb.PutJobs(jobs))
	assert.NoError(t, edb.update())

	test := func(start, end time.Time, startIdx, count int) {
		evs, err := edb.Range(JOB_STREAM, start, end)
		assert.NoError(t, err)
		assert.Len(t, evs, count)
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
	// expected is map[util.MD5Params(tags)]value
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
	hash, err := util.MD5Params(tags)
	assert.NoError(dt.t, err)
	_, exists := dt.expected[hash]
	assert.False(dt.t, exists, "Your test broke MD5. %v", tags)
	dt.expected[hash] = value
}

// Run calls the DynamicAggregateFn and checks that the return values are exactly those given by
// AddAssert.
func (dt *DynamicAggregateFnTester) Run(evs []*events.Event) {
	actualTags, actualVals, err := dt.f(evs)
	assert.NoError(dt.t, err)
	assert.Len(dt.t, actualTags, len(dt.expected))
	assert.Len(dt.t, actualVals, len(dt.expected))
	for i, tags := range actualTags {
		actualVal := actualVals[i]
		hash, err := util.MD5Params(tags)
		assert.NoError(dt.t, err)
		expectedVal, ok := dt.expected[hash]
		assert.True(dt.t, ok, "Unexpected tags %v", tags)
		assert.Equal(dt.t, expectedVal, actualVal, "For tags %v", tags)
	}
}

func TestComputeAvgJobDuration(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now()
	edb, jdb := setupJobs(t, now)
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
	assert.NoError(t, jdb.PutJobs(jobsStatus))

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
	assert.NoError(t, jdb.PutJobs(jobsType))

	expect("OnlyForced", FORCED, jobsType[0:2])
	expect("NormalAndTryjob", NORMAL, []*types.Job{jobsType[2], jobsType[5]})
	expect("NormalAndTryjob", TRYJOB, jobsType[3:5])
	expect("AllTypes", NORMAL, jobsType[6:7])
	expect("AllTypes", TRYJOB, jobsType[7:8])
	expect("AllTypes", FORCED, jobsType[8:9])

	assert.NoError(t, edb.update())
	evs, err := edb.Range(JOB_STREAM, created.Add(-time.Hour), created.Add(time.Hour))
	assert.NoError(t, err)
	assert.Len(t, evs, len(jobsStatus)+len(jobsType))

	tester.Run(evs)
}

func TestComputeJobFailureMishapRate(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Now()
	edb, jdb := setupJobs(t, now)
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
		assert.NoError(t, jdb.PutJob(makeJob(created, name, status, jobType, time.Minute)))
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

	assert.NoError(t, edb.update())
	evs, err := edb.Range(JOB_STREAM, created.Add(-time.Hour), created.Add(time.Hour))
	assert.NoError(t, err)
	assert.Len(t, evs, jobCount)

	tester.Run(evs)
}
