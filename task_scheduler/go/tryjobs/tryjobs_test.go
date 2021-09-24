package tryjobs

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
)

// Verify that updateJobs sends heartbeats for unfinished try Jobs and
// success/failure for finished Jobs.
func TestUpdateJobs(t *testing.T) {
	_, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	now := time.Now()

	assertActiveTryJob := func(j *types.Job) {
		active, err := trybots.getActiveTryJobs()
		require.NoError(t, err)
		expect := []*types.Job{}
		if j != nil {
			expect = append(expect, j)
		}
		assertdeep.Equal(t, expect, active)
	}
	assertNoActiveTryJobs := func() {
		assertActiveTryJob(nil)
	}

	// No jobs.
	assertNoActiveTryJobs()
	require.NoError(t, trybots.updateJobs(now))
	require.True(t, mock.Empty())

	// One unfinished try job.
	j1 := tryjob(gb.RepoUrl())
	MockHeartbeats(t, mock, now, []*types.Job{j1}, nil)
	require.NoError(t, trybots.db.PutJobs([]*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	require.NoError(t, trybots.updateJobs(now))
	require.True(t, mock.Empty())
	assertActiveTryJob(j1)

	// Send success/failure for finished jobs, not heartbeats.
	j1.Status = types.JOB_STATUS_SUCCESS
	j1.Finished = now
	require.NoError(t, trybots.db.PutJobs([]*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	MockJobSuccess(mock, j1, now, nil, false)
	require.NoError(t, trybots.updateJobs(now))
	require.True(t, mock.Empty())
	assertNoActiveTryJobs()

	// Failure.
	j1, err := trybots.db.GetJobById(j1.Id)
	require.NoError(t, err)
	j1.BuildbucketLeaseKey = 12345
	j1.Status = types.JOB_STATUS_FAILURE
	require.NoError(t, trybots.db.PutJobs([]*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	MockJobFailure(mock, j1, now, nil)
	require.NoError(t, trybots.updateJobs(now))
	require.True(t, mock.Empty())
	assertNoActiveTryJobs()

	// More than one batch of heartbeats.
	jobs := []*types.Job{}
	for i := 0; i < LEASE_BATCH_SIZE+2; i++ {
		jobs = append(jobs, tryjob(gb.RepoUrl()))
	}
	sort.Sort(types.JobSlice(jobs))
	MockHeartbeats(t, mock, now, jobs[:LEASE_BATCH_SIZE], nil)
	MockHeartbeats(t, mock, now, jobs[LEASE_BATCH_SIZE:], nil)
	require.NoError(t, trybots.db.PutJobs(jobs))
	trybots.jCache.AddJobs(jobs)
	require.NoError(t, trybots.updateJobs(now))
	require.True(t, mock.Empty())

	// Test heartbeat failure for one job, ensure that it gets canceled.
	j1, j2 := jobs[0], jobs[1]
	for _, j := range jobs[2:] {
		j.Status = types.JOB_STATUS_SUCCESS
		j.Finished = time.Now()
	}
	require.NoError(t, trybots.db.PutJobs(jobs[2:]))
	trybots.jCache.AddJobs(jobs[2:])
	for _, j := range jobs[2:] {
		MockJobSuccess(mock, j, now, nil, false)
	}
	MockHeartbeats(t, mock, now, []*types.Job{j1, j2}, map[string]*heartbeatResp{
		j1.Id: {
			BuildId: fmt.Sprintf("%d", j1.BuildbucketBuildId),
			Error: &errMsg{
				Message: "fail",
			},
		},
	})
	require.NoError(t, trybots.updateJobs(now))
	require.True(t, mock.Empty())
	active, err := trybots.getActiveTryJobs()
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Job{j2}, active)
	canceled, err := trybots.db.GetJobById(j1.Id)
	require.NoError(t, err)
	require.True(t, canceled.Done())
	require.Equal(t, types.JOB_STATUS_CANCELED, canceled.Status)
}

func TestGetRepo(t *testing.T) {
	_, trybots, _, _, _, cleanup := setup(t)
	defer cleanup()

	// Test basic.
	url, r, err := trybots.getRepo(patchProject)
	require.NoError(t, err)
	repo := trybots.projectRepoMapping[patchProject]
	require.Equal(t, repo, url)
	require.NotNil(t, r)

	// Bogus repo.
	_, _, err = trybots.getRepo("bogus")
	require.EqualError(t, err, "Unknown patch project \"bogus\"")

	// Cross-repo try job.
	// TODO(borenet): Cross-repo try jobs are disabled until we fire out a
	// workaround.
	//parentUrl := trybots.projectRepoMapping[parentProject]
	//props.PatchProject = patchProject
	//url, r, patchRepo, err = trybots.getRepo(props)
	//require.NoError(t, err)
	//require.Equal(t, parentUrl, url)
	//require.Equal(t, repo, patchRepo)
}

func TestGetRevision(t *testing.T) {
	_, trybots, _, mock, _, cleanup := setup(t)
	defer cleanup()

	// Get the (only) commit from the repo.
	_, r, err := trybots.getRepo(patchProject)
	require.NoError(t, err)
	c := r.Get(git.MasterBranch).Hash

	// Fake response from Gerrit.
	ci := &gerrit.ChangeInfo{
		Branch: git.MasterBranch,
	}
	serialized := []byte(testutils.MarshalJSON(t, ci))
	// Gerrit API prepends garbage to prevent XSS.
	serialized = append([]byte("abcd\n"), serialized...)
	url := fmt.Sprintf("%s/a/changes/%d/detail?o=ALL_REVISIONS&o=SUBMITTABLE", fakeGerritUrl, gerritIssue)
	mock.Mock(url, mockhttpclient.MockGetDialogue(serialized))

	got, err := trybots.getRevision(context.TODO(), r, gerritIssue)
	require.NoError(t, err)
	require.Equal(t, c, got)
}

func TestCancelBuild(t *testing.T) {
	_, trybots, _, mock, _, cleanup := setup(t)
	defer cleanup()

	id := int64(12345)
	MockCancelBuild(mock, id, "Canceling!", nil)
	require.NoError(t, trybots.remoteCancelBuild(id, "Canceling!"))
	require.True(t, mock.Empty())

	// Check that reason is truncated if it's long.
	MockCancelBuild(mock, id, strings.Repeat("X", maxCancelReasonLen-3)+"...", nil)
	require.NoError(t, trybots.remoteCancelBuild(id, strings.Repeat("X", maxCancelReasonLen+50)))
	require.True(t, mock.Empty())

	err := fmt.Errorf("Build does not exist!")
	MockCancelBuild(mock, id, "Canceling!", err)
	require.EqualError(t, trybots.remoteCancelBuild(id, "Canceling!"), err.Error())
	require.True(t, mock.Empty())
}

func TestTryLeaseBuild(t *testing.T) {
	_, trybots, _, mock, _, cleanup := setup(t)
	defer cleanup()

	id := int64(12345)
	MockTryLeaseBuild(mock, id, nil)
	k, err := trybots.tryLeaseBuild(id)
	require.NoError(t, err)
	require.NotEqual(t, k, 0)
	require.True(t, mock.Empty())

	expect := fmt.Errorf("Can't lease this!")
	MockTryLeaseBuild(mock, id, expect)
	_, err = trybots.tryLeaseBuild(id)
	require.Contains(t, err.Error(), expect.Error())
	require.True(t, mock.Empty())
}

func TestJobStarted(t *testing.T) {
	_, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	j := tryjob(gb.RepoUrl())

	// Success
	MockJobStarted(mock, j.BuildbucketBuildId, nil)
	require.NoError(t, trybots.jobStarted(j))
	require.True(t, mock.Empty())

	// Failure
	err := fmt.Errorf("fail")
	MockJobStarted(mock, j.BuildbucketBuildId, err)
	require.EqualError(t, trybots.jobStarted(j), err.Error())
	require.True(t, mock.Empty())
}

func TestJobFinished(t *testing.T) {
	_, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	j := tryjob(gb.RepoUrl())
	now := time.Now()

	// Job not actually finished.
	require.EqualError(t, trybots.jobFinished(j), "JobFinished called for unfinished Job!")

	// Successful job.
	j.Status = types.JOB_STATUS_SUCCESS
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs([]*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	MockJobSuccess(mock, j, now, nil, false)
	require.NoError(t, trybots.jobFinished(j))
	require.True(t, mock.Empty())

	// Successful job, failed to update.
	err := fmt.Errorf("fail")
	MockJobSuccess(mock, j, now, err, false)
	require.EqualError(t, trybots.jobFinished(j), err.Error())
	require.True(t, mock.Empty())

	// Failed job.
	j.Status = types.JOB_STATUS_FAILURE
	require.NoError(t, trybots.db.PutJobs([]*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	MockJobFailure(mock, j, now, nil)
	require.NoError(t, trybots.jobFinished(j))
	require.True(t, mock.Empty())

	// Failed job, failed to update.
	MockJobFailure(mock, j, now, err)
	require.EqualError(t, trybots.jobFinished(j), err.Error())
	require.True(t, mock.Empty())

	// Mishap.
	j.Status = types.JOB_STATUS_MISHAP
	require.NoError(t, trybots.db.PutJobs([]*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	MockJobMishap(mock, j, now, nil)
	require.NoError(t, trybots.jobFinished(j))
	require.True(t, mock.Empty())

	// Mishap, failed to update.
	MockJobMishap(mock, j, now, err)
	require.EqualError(t, trybots.jobFinished(j), err.Error())
	require.True(t, mock.Empty())
}

type addedJobs map[string]*types.Job

func (aj addedJobs) getAddedJob(t *testing.T, d db.JobReader) *types.Job {
	allJobs, err := d.GetJobsFromDateRange(time.Time{}, time.Now(), "")
	require.NoError(t, err)
	for _, job := range allJobs {
		if _, ok := aj[job.Id]; !ok {
			aj[job.Id] = job
			return job
		}
	}
	return nil
}

func TestInsertNewJob(t *testing.T) {
	ctx, trybots, gb, mock, mockBB, cleanup := setup(t)
	defer cleanup()

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MasterBranch)

	now := time.Now()

	aj := addedJobs(map[string]*types.Job{})

	// Normal job, Gerrit patch.
	b1 := Build(t, now)
	MockTryLeaseBuild(mock, b1.Id, nil)
	MockJobStarted(mock, b1.Id, nil)
	mockBB.On("GetBuild", ctx, b1.Id).Return(b1, nil)
	err := trybots.insertNewJob(ctx, b1.Id)
	require.NoError(t, err)
	require.True(t, mock.Empty())
	result := aj.getAddedJob(t, trybots.db)
	require.Equal(t, result.BuildbucketBuildId, b1.Id)
	require.NotEqual(t, "", result.BuildbucketLeaseKey)
	require.True(t, result.Valid())

	// Failed to lease build.
	expectErr := fmt.Errorf("Can't lease this!")
	MockTryLeaseBuild(mock, b1.Id, expectErr)
	mockBB.On("GetBuild", ctx, b1.Id).Return(b1, nil)
	err = trybots.insertNewJob(ctx, b1.Id)
	require.Contains(t, err.Error(), expectErr.Error())
	result = aj.getAddedJob(t, trybots.db)
	require.Nil(t, result)
	require.True(t, mock.Empty())

	// No GerritChanges.
	b2 := Build(t, now)
	b2.Input.GerritChanges = nil
	MockCancelBuild(mock, b2.Id, fmt.Sprintf("Invalid Build %d: input should have exactly one GerritChanges: ", b2.Id), nil)
	mockBB.On("GetBuild", ctx, b2.Id).Return(b2, nil)
	err = trybots.insertNewJob(ctx, b2.Id)
	require.NoError(t, err) // We don't report errors for bad data from buildbucket.
	result = aj.getAddedJob(t, trybots.db)
	require.Nil(t, result)
	require.True(t, mock.Empty())

	// Invalid repo.
	b3 := Build(t, now)
	b3.Input.GerritChanges[0].Project = "bogus-repo"
	MockCancelBuild(mock, b3.Id, "Unable to find repo: Unknown patch project \\\\\\\"bogus-repo\\\\\\\"", nil)
	mockBB.On("GetBuild", ctx, b3.Id).Return(b3, nil)
	err = trybots.insertNewJob(ctx, b3.Id)
	require.NoError(t, err) // We don't report errors for bad data from buildbucket.
	result = aj.getAddedJob(t, trybots.db)
	require.Nil(t, result)
	require.True(t, mock.Empty())

	// Invalid JobSpec.
	rs := types.RepoState{
		Patch:    gerritPatch,
		Repo:     gb.RepoUrl(),
		Revision: trybots.rm[gb.RepoUrl()].Get(git.MasterBranch).Hash,
	}
	rs.Patch.PatchRepo = rs.Repo
	b8 := Build(t, now)
	b8.Builder.Builder = "bogus-job"
	MockCancelBuild(mock, b8.Id, fmt.Sprintf("Failed to create Job from JobSpec: bogus-job @ %+v: No such job: bogus-job", rs), nil)
	mockBB.On("GetBuild", ctx, b8.Id).Return(b8, nil)
	err = trybots.insertNewJob(ctx, b8.Id)
	require.NoError(t, err) // We don't report errors for bad data from buildbucket.
	result = aj.getAddedJob(t, trybots.db)
	require.Nil(t, result)
	require.True(t, mock.Empty())

	// Failure to cancel the build.
	b9 := Build(t, now)
	b9.Builder.Builder = "bogus-job"
	expect := fmt.Errorf("no cancel!")
	MockCancelBuild(mock, b9.Id, fmt.Sprintf("Failed to create Job from JobSpec: bogus-job @ %+v: No such job: bogus-job", rs), expect)
	mockBB.On("GetBuild", ctx, b9.Id).Return(b9, nil)
	err = trybots.insertNewJob(ctx, b9.Id)
	require.EqualError(t, err, expect.Error())
	result = aj.getAddedJob(t, trybots.db)
	require.Nil(t, result)
	require.True(t, mock.Empty())
}

func mockGetChangeInfo(t *testing.T, mock *mockhttpclient.URLMock, id int, project, branch string) {
	ci := &gerrit.ChangeInfo{
		Id:      strconv.FormatInt(gerritIssue, 10),
		Project: project,
		Branch:  branch,
	}
	issueBytes, err := json.Marshal(ci)
	require.NoError(t, err)
	issueBytes = append([]byte("XSS\n"), issueBytes...)
	mock.Mock(fmt.Sprintf("%s/a%s", fakeGerritUrl, fmt.Sprintf(gerrit.URLTmplChange, ci.Id)), mockhttpclient.MockGetDialogue(issueBytes))
}

func TestRetry(t *testing.T) {
	ctx, trybots, _, mock, mockBB, cleanup := setup(t)
	defer cleanup()

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MasterBranch)

	now := time.Now()

	// Insert one try job.
	aj := addedJobs(map[string]*types.Job{})
	b1 := Build(t, now)
	MockTryLeaseBuild(mock, b1.Id, nil)
	MockJobStarted(mock, b1.Id, nil)
	mockBB.On("GetBuild", ctx, b1.Id).Return(b1, nil)
	err := trybots.insertNewJob(ctx, b1.Id)
	require.NoError(t, err)
	j1 := aj.getAddedJob(t, trybots.db)
	require.True(t, mock.Empty())
	require.Equal(t, j1.BuildbucketBuildId, b1.Id)
	require.NotEqual(t, "", j1.BuildbucketLeaseKey)
	require.True(t, j1.Valid())
	require.False(t, j1.IsForce)
	require.NoError(t, trybots.db.PutJobs([]*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	require.NoError(t, trybots.jCache.Update(ctx))

	// Obtain a second try job, ensure that it gets IsForce = true.
	b2 := Build(t, now)
	MockTryLeaseBuild(mock, b2.Id, nil)
	MockJobStarted(mock, b2.Id, nil)
	mockBB.On("GetBuild", ctx, b2.Id).Return(b2, nil)
	err = trybots.insertNewJob(ctx, b2.Id)
	require.NoError(t, err)
	require.True(t, mock.Empty())
	j2 := aj.getAddedJob(t, trybots.db)
	require.Equal(t, j2.BuildbucketBuildId, b2.Id)
	require.NotEqual(t, "", j2.BuildbucketLeaseKey)
	require.True(t, j2.Valid())
	require.True(t, j2.IsForce)
}

func TestPoll(t *testing.T) {
	ctx, trybots, _, mock, mockBB, cleanup := setup(t)
	defer cleanup()

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MasterBranch)

	now := time.Now()

	assertAdded := func(builds []*buildbucketpb.Build) {
		jobs, err := trybots.getActiveTryJobs()
		require.NoError(t, err)
		byId := make(map[int64]*types.Job, len(jobs))
		for _, j := range jobs {
			// Check that the job creation time is reasonable.
			require.True(t, j.Created.Year() > 1969 && j.Created.Year() < 3000)
			byId[j.BuildbucketBuildId] = j
			j.Status = types.JOB_STATUS_SUCCESS
			j.Finished = now
		}
		for _, b := range builds {
			_, ok := byId[b.Id]
			require.True(t, ok)
		}
		require.NoError(t, trybots.db.PutJobs(jobs))
		trybots.jCache.AddJobs(jobs)
	}

	makeBuilds := func(n int) []*buildbucketpb.Build {
		builds := make([]*buildbucketpb.Build, 0, n)
		for i := 0; i < n; i++ {
			builds = append(builds, Build(t, now))
		}
		return builds
	}

	mockBuilds := func(builds []*buildbucketpb.Build) []*buildbucketpb.Build {
		MockPeek(mock, builds, now, "", "", nil)
		for _, b := range builds {
			MockTryLeaseBuild(mock, b.Id, nil)
			MockJobStarted(mock, b.Id, nil)
			mockBB.On("GetBuild", ctx, b.Id).Return(b, nil)
		}
		return builds
	}

	check := func(builds []*buildbucketpb.Build) {
		require.Nil(t, trybots.Poll(ctx))
		require.True(t, mock.Empty())
		assertAdded(builds)
	}

	// Single new build, success.
	check(mockBuilds(makeBuilds(1)))

	// Multiple new builds, success.
	check(mockBuilds(makeBuilds(5)))

	// More than one page of new builds.
	builds := makeBuilds(PEEK_MAX_BUILDS + 5)
	MockPeek(mock, builds[:PEEK_MAX_BUILDS], now, "", "cursor1", nil)
	MockPeek(mock, builds[PEEK_MAX_BUILDS:], now, "cursor1", "", nil)
	for _, b := range builds {
		MockTryLeaseBuild(mock, b.Id, nil)
		MockJobStarted(mock, b.Id, nil)
		mockBB.On("GetBuild", ctx, b.Id).Return(b, nil)
	}
	check(builds)

	// Multiple new builds, fail insertNewJob, ensure successful builds
	// are inserted.
	builds = makeBuilds(5)
	failIdx := 2
	failBuild := builds[failIdx]
	failBuild.Input.GerritChanges[0].Project = "bogus"
	MockPeek(mock, builds, now, "", "", nil)
	builds = append(builds[:failIdx], builds[failIdx+1:]...)
	for _, b := range builds {
		MockTryLeaseBuild(mock, b.Id, nil)
		MockJobStarted(mock, b.Id, nil)
		mockBB.On("GetBuild", ctx, b.Id).Return(b, nil)
	}
	mockBB.On("GetBuild", ctx, failBuild.Id).Return(failBuild, nil)
	MockCancelBuild(mock, failBuild.Id, "Unable to find repo: Unknown patch project \\\\\\\"bogus\\\\\\\"", nil)
	check(builds)

	// Multiple new builds, fail jobStarted, ensure that the others are
	// properly added.
	builds = makeBuilds(5)
	failBuild = builds[failIdx]
	MockPeek(mock, builds, now, "", "", nil)
	builds = append(builds[:failIdx], builds[failIdx+1:]...)
	for _, b := range builds {
		MockTryLeaseBuild(mock, b.Id, nil)
		MockJobStarted(mock, b.Id, nil)
		mockBB.On("GetBuild", ctx, b.Id).Return(b, nil)
	}
	mockBB.On("GetBuild", ctx, failBuild.Id).Return(failBuild, nil)
	MockTryLeaseBuild(mock, failBuild.Id, nil)
	MockJobStarted(mock, failBuild.Id, fmt.Errorf("Failed to start build."))
	require.EqualError(t, trybots.Poll(ctx), "Got errors loading builds from Buildbucket: [Failed to send job-started notification with: Failed to start build.]")
	require.True(t, mock.Empty())
	assertAdded(builds)

	// More than one page of new builds, fail peeking a page, ensure that
	// other jobs get added.
	builds = makeBuilds(PEEK_MAX_BUILDS + 5)
	err := fmt.Errorf("Failed peek")
	MockPeek(mock, builds[:PEEK_MAX_BUILDS], now, "", "cursor1", nil)
	MockPeek(mock, builds[PEEK_MAX_BUILDS:], now, "cursor1", "", err)
	builds = builds[:PEEK_MAX_BUILDS]
	for _, b := range builds {
		MockTryLeaseBuild(mock, b.Id, nil)
		MockJobStarted(mock, b.Id, nil)
		mockBB.On("GetBuild", ctx, b.Id).Return(b, nil)
	}
	require.EqualError(t, trybots.Poll(ctx), "Got errors loading builds from Buildbucket: [Failed peek]")
	require.True(t, mock.Empty())
	assertAdded(builds)
}
