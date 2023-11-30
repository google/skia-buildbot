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
	"go.skia.org/infra/go/buildbucket/mocks"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
)

var distantFutureTime = time.Date(3000, time.January, 1, 0, 0, 0, 0, time.UTC)

func assertActiveTryJob(t *testing.T, trybots *TryJobIntegrator, j *types.Job) {
	active, err := trybots.getActiveTryJobs(context.Background())
	require.NoError(t, err)
	expect := []*types.Job{}
	if j != nil {
		expect = append(expect, j)
	}
	assertdeep.Equal(t, expect, active)
}

func assertNoActiveTryJobs(t *testing.T, trybots *TryJobIntegrator) {
	assertActiveTryJob(t, trybots, nil)
}

// Verify that updateJobs sends heartbeats for unfinished try Jobs and
// success/failure for finished Jobs.
func TestUpdateJobs_NoJobs_NoAction(t *testing.T) {
	ctx, trybots, _, mock, _, cleanup := setup(t)
	defer cleanup()

	assertNoActiveTryJobs(t, trybots)
	require.NoError(t, trybots.updateJobs(ctx))
	require.True(t, mock.Empty(), mock.List())
}

func TestUpdateJobs_OneUnfinished_SendsHeartbeat(t *testing.T) {
	ctx, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	j1 := tryjob(ctx, gb.RepoUrl())
	MockHeartbeats(t, mock, ts, []*types.Job{j1}, nil)
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	require.NoError(t, trybots.updateJobs(ctx))
	require.True(t, mock.Empty(), mock.List())
	assertActiveTryJob(t, trybots, j1)
}

func TestUpdateJobs_FinishedJob_SendSuccess(t *testing.T) {
	ctx, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	j1 := tryjob(ctx, gb.RepoUrl())
	j1.Status = types.JOB_STATUS_SUCCESS
	j1.Finished = ts
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	MockJobSuccess(mock, j1, ts, false)
	require.NoError(t, trybots.updateJobs(ctx))
	require.True(t, mock.Empty(), mock.List())
	assertNoActiveTryJobs(t, trybots)
}

func TestUpdateJobs_FailedJob_SendFailure(t *testing.T) {
	ctx, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	j1 := tryjob(ctx, gb.RepoUrl())
	j1.Status = types.JOB_STATUS_FAILURE
	j1.Finished = ts
	j1.BuildbucketLeaseKey = 12345
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	MockJobFailure(mock, j1, ts)
	require.NoError(t, trybots.updateJobs(ctx))
	require.True(t, mock.Empty(), mock.List())
	assertNoActiveTryJobs(t, trybots)
}

func TestUpdateJobs_ManyInProgress_MultipleHeartbeatBatches(t *testing.T) {
	ctx, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	jobs := []*types.Job{}
	for i := 0; i < LEASE_BATCH_SIZE+2; i++ {
		jobs = append(jobs, tryjob(ctx, gb.RepoUrl()))
	}
	sort.Sort(heartbeatJobSlice(jobs))
	MockHeartbeats(t, mock, ts, jobs[:LEASE_BATCH_SIZE], nil)
	MockHeartbeats(t, mock, ts, jobs[LEASE_BATCH_SIZE:], nil)
	require.NoError(t, trybots.db.PutJobs(ctx, jobs))
	trybots.jCache.AddJobs(jobs)
	require.NoError(t, trybots.updateJobs(ctx))
	require.True(t, mock.Empty(), mock.List())
}

func TestUpdateJobs_HeartbeatBatchOneFailed_JobIsCanceled(t *testing.T) {
	ctx, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	jobs := []*types.Job{}
	for i := 0; i < LEASE_BATCH_SIZE+2; i++ {
		jobs = append(jobs, tryjob(ctx, gb.RepoUrl()))
	}
	j1, j2 := jobs[0], jobs[1]
	for _, j := range jobs[2:] {
		j.Status = types.JOB_STATUS_SUCCESS
		j.Finished = time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	}
	require.NoError(t, trybots.db.PutJobs(ctx, jobs))
	trybots.jCache.AddJobs(jobs[2:])
	for _, j := range jobs[2:] {
		MockJobSuccess(mock, j, ts, false)
	}
	MockHeartbeats(t, mock, ts, []*types.Job{j1, j2}, map[string]*heartbeatResp{
		j1.Id: {
			BuildId: fmt.Sprintf("%d", j1.BuildbucketBuildId),
			Error: &errMsg{
				Message: "fail",
			},
		},
	})
	require.NoError(t, trybots.updateJobs(ctx))
	require.True(t, mock.Empty(), mock.List())
	active, err := trybots.getActiveTryJobs(ctx)
	require.NoError(t, err)
	assertdeep.Equal(t, []*types.Job{j2}, active)
	canceled, err := trybots.db.GetJobById(ctx, j1.Id)
	require.NoError(t, err)
	require.True(t, canceled.Done())
	require.Equal(t, types.JOB_STATUS_CANCELED, canceled.Status)
}

func TestGetRevision(t *testing.T) {
	ctx, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	// Get the (only) commit from the repo.
	r, err := trybots.getRepo(gb.RepoUrl())
	require.NoError(t, err)
	c := r.Get(git.MainBranch).Hash

	// Fake response from Gerrit.
	ci := &gerrit.ChangeInfo{
		Branch: git.MainBranch,
	}
	serialized := []byte(testutils.MarshalJSON(t, ci))
	// Gerrit API prepends garbage to prevent XSS.
	serialized = append([]byte("abcd\n"), serialized...)
	url := fmt.Sprintf("%s/a/changes/%d/detail?o=ALL_REVISIONS&o=SUBMITTABLE", fakeGerritUrl, gerritIssue)
	mock.Mock(url, mockhttpclient.MockGetDialogue(serialized))

	got, err := trybots.getRevision(ctx, r, strconv.Itoa(gerritIssue))
	require.NoError(t, err)
	require.Equal(t, c, got)
}

func TestCancelBuild_Success(t *testing.T) {
	_, trybots, _, mock, _, cleanup := setup(t)
	defer cleanup()

	const id = int64(12345)
	MockCancelBuild(mock, id, "Canceling!")
	require.NoError(t, trybots.remoteCancelBuild(id, "Canceling!"))
	require.True(t, mock.Empty(), mock.List())
}

func TestCancelBuild_Success_LongMessageTruncated(t *testing.T) {
	_, trybots, _, mock, _, cleanup := setup(t)
	defer cleanup()

	const id = int64(12345)
	MockCancelBuild(mock, id, strings.Repeat("X", maxCancelReasonLen-3)+"...")
	require.NoError(t, trybots.remoteCancelBuild(id, strings.Repeat("X", maxCancelReasonLen+50)))
	require.True(t, mock.Empty(), mock.List())
}

func TestCancelBuild_Failed(t *testing.T) {
	_, trybots, _, mock, _, cleanup := setup(t)
	defer cleanup()

	const id = int64(12345)
	expectErr := "Build does not exist!"
	MockCancelBuildFailed(mock, id, "Canceling!", expectErr)
	require.EqualError(t, trybots.remoteCancelBuild(id, "Canceling!"), expectErr)
	require.True(t, mock.Empty(), mock.List())
}

func TestTryLeaseBuild_Success(t *testing.T) {
	ctx, trybots, _, mock, _, cleanup := setup(t)
	defer cleanup()

	const id = int64(12345)
	MockTryLeaseBuild(mock, id)
	k, bbError, err := trybots.tryLeaseBuild(ctx, id)
	require.NoError(t, err)
	require.Nil(t, bbError)
	require.NotEqual(t, k, 0)
	require.True(t, mock.Empty(), mock.List())
}

func TestTryLeaseBuild_Failure(t *testing.T) {
	ctx, trybots, _, mock, _, cleanup := setup(t)
	defer cleanup()

	const id = int64(12345)
	expect := "Can't lease this!"
	MockTryLeaseBuildFailed(mock, id, expect)
	_, bbError, err := trybots.tryLeaseBuild(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, bbError)
	require.Contains(t, bbError.Message, expect)
	require.True(t, mock.Empty(), mock.List())
}

func TestJobStarted_Success(t *testing.T) {
	ctx, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	j := tryjob(ctx, gb.RepoUrl())

	MockJobStarted(mock, j.BuildbucketBuildId)
	bbError, err := trybots.jobStarted(j)
	require.NoError(t, err)
	require.Nil(t, bbError)
	require.True(t, mock.Empty(), mock.List())
}

func TestJobStarted_Failure(t *testing.T) {
	ctx, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	j := tryjob(ctx, gb.RepoUrl())

	expectErr := "fail"
	MockJobStartedFailed(mock, j.BuildbucketBuildId, expectErr)
	bbError, err := trybots.jobStarted(j)
	require.NoError(t, err)
	require.NotNil(t, bbError)
	require.Contains(t, bbError.Message, expectErr)
	require.True(t, mock.Empty(), mock.List())
}

func TestJobFinished_NotActuallyFinished(t *testing.T) {
	ctx, trybots, gb, _, _, cleanup := setup(t)
	defer cleanup()

	j := tryjob(ctx, gb.RepoUrl())

	require.ErrorContains(t, trybots.jobFinished(j), "JobFinished called for unfinished Job!")
}

func TestJobFinished_JobSucceeded_UpdateSucceeds(t *testing.T) {
	ctx, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	j := tryjob(ctx, gb.RepoUrl())
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_SUCCESS
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	MockJobSuccess(mock, j, now, false)
	require.NoError(t, trybots.jobFinished(j))
	require.True(t, mock.Empty(), mock.List())
}

func TestJobFinished_JobSucceeded_UpdateFails(t *testing.T) {
	ctx, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	j := tryjob(ctx, gb.RepoUrl())
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_SUCCESS
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	expectErr := "fail"
	MockJobSuccess_Failed(mock, j, now, false, expectErr)
	require.EqualError(t, trybots.jobFinished(j), expectErr)
	require.True(t, mock.Empty(), mock.List())
}

func TestJobFinished_JobFailed_UpdateSucceeds(t *testing.T) {
	ctx, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	j := tryjob(ctx, gb.RepoUrl())
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_FAILURE
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	MockJobFailure(mock, j, now)
	require.NoError(t, trybots.jobFinished(j))
	require.True(t, mock.Empty(), mock.List())
}

func TestJobFinished_JobFailed_UpdateFails(t *testing.T) {
	ctx, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	j := tryjob(ctx, gb.RepoUrl())
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_FAILURE
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	expectErr := "fail"
	MockJobFailure_Failed(mock, j, now, expectErr)
	require.EqualError(t, trybots.jobFinished(j), expectErr)
	require.True(t, mock.Empty(), mock.List())
}

func TestJobFinished_JobMishap_UpdateSucceeds(t *testing.T) {
	ctx, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	j := tryjob(ctx, gb.RepoUrl())
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_MISHAP
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	MockJobMishap(mock, j, now)
	require.NoError(t, trybots.jobFinished(j))
	require.True(t, mock.Empty(), mock.List())
}

func TestJobFinished_JobMishap_UpdateFails(t *testing.T) {
	ctx, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	j := tryjob(ctx, gb.RepoUrl())
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	j.Status = types.JOB_STATUS_MISHAP
	j.Finished = now
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	expectErr := "fail"
	MockJobMishap_Failed(mock, j, now, expectErr)
	require.EqualError(t, trybots.jobFinished(j), expectErr)
	require.True(t, mock.Empty(), mock.List())
}

type addedJobs map[string]*types.Job

func (aj addedJobs) getAddedJob(ctx context.Context, t *testing.T, d db.JobReader) *types.Job {
	allJobs, err := d.GetJobsFromDateRange(ctx, time.Time{}, distantFutureTime, "")
	require.NoError(t, err)
	for _, job := range allJobs {
		if _, ok := aj[job.Id]; !ok {
			aj[job.Id] = job
			return job
		}
	}
	return nil
}

func TestInsertNewJob_LeaseSucceeds_StatusIsRequested(t *testing.T) {
	ctx, trybots, _, mock, mockBB, cleanup := setup(t)
	defer cleanup()

	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	aj := addedJobs(map[string]*types.Job{})

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)

	// Normal job, Gerrit patch.
	b1 := Build(t, now)
	mockBB.On("GetBuild", ctx, b1.Id).Return(b1, nil)
	MockTryLeaseBuild(mock, b1.Id)
	err := trybots.insertNewJob(ctx, b1.Id)
	require.NoError(t, err)
	require.True(t, mock.Empty(), mock.List())
	result := aj.getAddedJob(ctx, t, trybots.db)
	require.Equal(t, result.BuildbucketBuildId, b1.Id)
	require.NotEqual(t, "", result.BuildbucketLeaseKey)
	require.Equal(t, types.JOB_STATUS_REQUESTED, result.Status)
	require.True(t, result.RepoState.Patch.Full())
	require.True(t, result.RepoState.Patch.Valid())
	// Revision is expected to be unset; set it and check that everything else
	// is valid.
	require.Empty(t, result.RepoState.Revision)
	result.RepoState.Revision = "fake"
	require.True(t, result.RepoState.Valid())
}

func TestInsertNewJob_NoGerritChanges_BuildIsCanceled(t *testing.T) {
	ctx, trybots, _, mock, mockBB, cleanup := setup(t)
	defer cleanup()

	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	aj := addedJobs(map[string]*types.Job{})

	b2 := Build(t, now)
	b2.Input.GerritChanges = nil
	MockCancelBuild(mock, b2.Id, fmt.Sprintf("Invalid Build %d: input should have exactly one GerritChanges: ", b2.Id))
	mockBB.On("GetBuild", ctx, b2.Id).Return(b2, nil)
	err := trybots.insertNewJob(ctx, b2.Id)
	require.NoError(t, err) // We don't report errors for bad data from buildbucket.
	result := aj.getAddedJob(ctx, t, trybots.db)
	require.Nil(t, result)
	require.True(t, mock.Empty(), mock.List())
}

func TestInsertNewJob_InvalidRepo_BuildIsCanceled(t *testing.T) {
	ctx, trybots, _, mock, mockBB, cleanup := setup(t)
	defer cleanup()

	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	aj := addedJobs(map[string]*types.Job{})

	b3 := Build(t, now)
	b3.Input.GerritChanges[0].Project = "bogus-repo"
	MockCancelBuild(mock, b3.Id, `Unknown patch project \\\"bogus-repo\\\"`)
	mockBB.On("GetBuild", ctx, b3.Id).Return(b3, nil)
	err := trybots.insertNewJob(ctx, b3.Id)
	require.NoError(t, err) // We don't report errors for bad data from buildbucket.
	result := aj.getAddedJob(ctx, t, trybots.db)
	require.Nil(t, result)
	require.True(t, mock.Empty(), mock.List())
}

func TestInsertNewJob_LeaseFailed_BuildIsCanceled(t *testing.T) {
	ctx, trybots, _, mock, mockBB, cleanup := setup(t)
	defer cleanup()

	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	aj := addedJobs(map[string]*types.Job{})

	b4 := Build(t, now)
	mockBB.On("GetBuild", ctx, b4.Id).Return(b4, nil)
	expectErr := "Can't lease this!"
	MockTryLeaseBuildFailed(mock, b4.Id, expectErr)
	MockCancelBuild(mock, b4.Id, `Buildbucket refused lease with \\\"Can't lease this!\\\"`)
	err := trybots.insertNewJob(ctx, b4.Id)
	require.NoError(t, err) // We don't report errors for bad data from buildbucket.
	result := aj.getAddedJob(ctx, t, trybots.db)
	require.Nil(t, result)
	require.True(t, mock.Empty(), mock.List())
}

func TestStartJob_NormalJob_Succeeds(t *testing.T) {
	ctx, trybots, gb, mock, mockBB, cleanup := setup(t)
	defer cleanup()

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	aj := addedJobs(map[string]*types.Job{})
	b1 := Build(t, now)
	mockBB.On("GetBuild", ctx, b1.Id).Return(b1, nil)
	MockTryLeaseBuild(mock, b1.Id)
	require.NoError(t, trybots.insertNewJob(ctx, b1.Id))
	j1 := aj.getAddedJob(ctx, t, trybots.db)
	require.Empty(t, j1.Revision) // Revision isn't set until startJob runs.

	MockJobStarted(mock, b1.Id)
	err := trybots.startJob(ctx, j1)
	require.NoError(t, err)
	require.True(t, mock.Empty(), mock.List())
	require.Equal(t, j1.BuildbucketBuildId, b1.Id)
	require.NotEqual(t, "", j1.BuildbucketLeaseKey)
	expectRev := strings.TrimSpace(gb.Git(ctx, "rev-parse", git.MainBranch))
	require.Equal(t, expectRev, j1.Revision)
}

func TestStartJob_RevisionAlreadySet_Succeeds(t *testing.T) {
	ctx, trybots, gb, mock, mockBB, cleanup := setup(t)
	defer cleanup()

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	aj := addedJobs(map[string]*types.Job{})
	b1 := Build(t, now)
	mockBB.On("GetBuild", ctx, b1.Id).Return(b1, nil)
	MockTryLeaseBuild(mock, b1.Id)
	require.NoError(t, trybots.insertNewJob(ctx, b1.Id))
	j1 := aj.getAddedJob(ctx, t, trybots.db)
	j1.Revision = "main"

	MockJobStarted(mock, b1.Id)
	err := trybots.startJob(ctx, j1)
	require.NoError(t, err)
	require.True(t, mock.Empty(), mock.List())
	require.Equal(t, j1.BuildbucketBuildId, b1.Id)
	require.NotEqual(t, "", j1.BuildbucketLeaseKey)
	// TODO(borenet): This is identical behavior to when job.Revision is empty,
	// at least from the perspective of the test.  Ideally we'd add a different
	// branch with a different commit hash and verify that we used that instead.
	expectRev := strings.TrimSpace(gb.Git(ctx, "rev-parse", git.MainBranch))
	require.Equal(t, expectRev, j1.Revision)
}

func TestStartJob_NormalJob_Failed(t *testing.T) {
	ctx, trybots, _, mock, mockBB, cleanup := setup(t)
	defer cleanup()

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	aj := addedJobs(map[string]*types.Job{})
	b1 := Build(t, now)
	mockBB.On("GetBuild", ctx, b1.Id).Return(b1, nil)
	MockTryLeaseBuild(mock, b1.Id)
	require.NoError(t, trybots.insertNewJob(ctx, b1.Id))
	j1 := aj.getAddedJob(ctx, t, trybots.db)

	expectErr := "Can't start this build!"
	MockJobStartedFailed(mock, b1.Id, expectErr)
	err := trybots.startJob(ctx, j1)
	require.Contains(t, err.Error(), expectErr)
	require.True(t, mock.Empty(), mock.List())
	updatedJ1, err := trybots.jCache.GetJob(j1.Id)
	require.NoError(t, err)
	require.Equal(t, types.JOB_STATUS_CANCELED, updatedJ1.Status)
	// TODO(borenet): Add a field to Job to give more details and check it here.
}

func TestStartJob_InvalidJobSpec_Failed(t *testing.T) {
	ctx, trybots, gb, mock, mockBB, cleanup := setup(t)
	defer cleanup()

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)
	aj := addedJobs(map[string]*types.Job{})

	rs := types.RepoState{
		Patch:    gerritPatch,
		Repo:     gb.RepoUrl(),
		Revision: trybots.rm[gb.RepoUrl()].Get(git.MainBranch).Hash,
	}
	rs.Patch.PatchRepo = rs.Repo
	b2 := Build(t, now)
	b2.Builder.Builder = "bogus-job"
	mockBB.On("GetBuild", ctx, b2.Id).Return(b2, nil)
	MockTryLeaseBuild(mock, b2.Id)
	require.NoError(t, trybots.insertNewJob(ctx, b2.Id))
	j2 := aj.getAddedJob(ctx, t, trybots.db)
	mockBB.On("GetBuild", ctx, b2.Id).Return(b2, nil)
	err := trybots.startJob(ctx, j2)
	require.NoError(t, err) // We don't report errors for bad data from buildbucket.
	require.True(t, mock.Empty(), mock.List())
	j2, err = trybots.jCache.GetJob(j2.Id)
	require.NoError(t, err)
	require.Equal(t, types.JOB_STATUS_MISHAP, j2.Status)
	// TODO(borenet): Add a field to Job to give more details and check it here.
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

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)

	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)

	// Insert one try job.
	aj := addedJobs(map[string]*types.Job{})
	b1 := Build(t, now)
	MockTryLeaseBuild(mock, b1.Id)
	MockJobStarted(mock, b1.Id)
	mockBB.On("GetBuild", ctx, b1.Id).Return(b1, nil)
	require.NoError(t, trybots.insertNewJob(ctx, b1.Id))
	j1 := aj.getAddedJob(ctx, t, trybots.db)
	require.NoError(t, trybots.startJob(ctx, j1))
	require.True(t, mock.Empty(), mock.List())
	require.Equal(t, j1.BuildbucketBuildId, b1.Id)
	require.NotEqual(t, "", j1.BuildbucketLeaseKey)
	require.True(t, j1.Valid())
	require.False(t, j1.IsForce)
	require.NoError(t, trybots.db.PutJobs(ctx, []*types.Job{j1}))
	trybots.jCache.AddJobs([]*types.Job{j1})
	require.NoError(t, trybots.jCache.Update(ctx))

	// Obtain a second try job, ensure that it gets IsForce = true.
	b2 := Build(t, now)
	MockTryLeaseBuild(mock, b2.Id)
	MockJobStarted(mock, b2.Id)
	mockBB.On("GetBuild", ctx, b2.Id).Return(b2, nil)
	require.NoError(t, trybots.insertNewJob(ctx, b2.Id))
	j2 := aj.getAddedJob(ctx, t, trybots.db)
	require.NoError(t, trybots.startJob(ctx, j2))
	require.True(t, mock.Empty(), mock.List())
	require.Equal(t, j2.BuildbucketBuildId, b2.Id)
	require.NotEqual(t, "", j2.BuildbucketLeaseKey)
	require.True(t, j2.Valid())
	require.True(t, j2.IsForce)
}

func testPollAssertAdded(t *testing.T, now time.Time, trybots *TryJobIntegrator, builds []*buildbucketpb.Build) {
	jobs, err := trybots.jCache.NotYetStartedJobs()
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
	require.NoError(t, trybots.db.PutJobs(context.Background(), jobs))
	trybots.jCache.AddJobs(jobs)
}

func testPollMakeBuilds(t *testing.T, now time.Time, n int) []*buildbucketpb.Build {
	builds := make([]*buildbucketpb.Build, 0, n)
	for i := 0; i < n; i++ {
		builds = append(builds, Build(t, now))
	}
	return builds
}

func testPollMockBuilds(t *testing.T, now time.Time, trybots *TryJobIntegrator, mock *mockhttpclient.URLMock, mockBB *mocks.BuildBucketInterface, builds []*buildbucketpb.Build) []*buildbucketpb.Build {
	MockPeek(mock, builds, now, "", "")
	for _, b := range builds {
		MockTryLeaseBuild(mock, b.Id)
		mockBB.On("GetBuild", context.Background(), b.Id).Return(b, nil)
	}
	return builds
}

func testPollCheck(t *testing.T, now time.Time, trybots *TryJobIntegrator, mock *mockhttpclient.URLMock, builds []*buildbucketpb.Build) {
	require.NoError(t, trybots.Poll(context.Background()))
	require.True(t, mock.Empty(), mock.List())
	testPollAssertAdded(t, now, trybots, builds)
}

func TestPoll_OneNewBuild_Success(t *testing.T) {
	_, trybots, _, mock, mockBB, cleanup := setup(t)
	defer cleanup()
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)

	testPollCheck(t, now, trybots, mock, testPollMockBuilds(t, now, trybots, mock, mockBB, testPollMakeBuilds(t, now, 1)))
}

func TestPoll_MultipleNewBuilds_Success(t *testing.T) {
	_, trybots, _, mock, mockBB, cleanup := setup(t)
	defer cleanup()
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)

	testPollCheck(t, now, trybots, mock, testPollMockBuilds(t, now, trybots, mock, mockBB, testPollMakeBuilds(t, now, 5)))
}

func TestPoll_MultiplePagesOfNewBuilds_Success(t *testing.T) {
	_, trybots, _, mock, mockBB, cleanup := setup(t)
	defer cleanup()
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)

	builds := testPollMakeBuilds(t, now, PEEK_MAX_BUILDS+5)
	MockPeek(mock, builds[:PEEK_MAX_BUILDS], now, "", "cursor1")
	MockPeek(mock, builds[PEEK_MAX_BUILDS:], now, "cursor1", "")
	for _, b := range builds {
		MockTryLeaseBuild(mock, b.Id)
		mockBB.On("GetBuild", context.Background(), b.Id).Return(b, nil)
	}
	testPollCheck(t, now, trybots, mock, builds)
}

func TestPoll_MultipleNewBuilds_OneFailsInsert_OthersInsertSuccessfully(t *testing.T) {
	_, trybots, _, mock, mockBB, cleanup := setup(t)
	defer cleanup()
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)

	builds := testPollMakeBuilds(t, now, 5)
	failIdx := 2
	failBuild := builds[failIdx]
	failBuild.Input.GerritChanges[0].Project = "bogus"
	MockPeek(mock, builds, now, "", "")
	builds = append(builds[:failIdx], builds[failIdx+1:]...)
	for _, b := range builds {
		MockTryLeaseBuild(mock, b.Id)
		mockBB.On("GetBuild", context.Background(), b.Id).Return(b, nil)
	}
	mockBB.On("GetBuild", context.Background(), failBuild.Id).Return(failBuild, nil)
	MockCancelBuild(mock, failBuild.Id, `Unknown patch project \\\"bogus\\\"`)
	testPollCheck(t, now, trybots, mock, builds)
}

func TestPoll_MultiplePagesOfNewBuilds_OnePeekFails_OthersInsertSuccessfully(t *testing.T) {
	_, trybots, _, mock, mockBB, cleanup := setup(t)
	defer cleanup()
	mockGetChangeInfo(t, mock, gerritIssue, patchProject, git.MainBranch)
	now := time.Date(2021, time.April, 27, 0, 0, 0, 0, time.UTC)

	builds := testPollMakeBuilds(t, now, PEEK_MAX_BUILDS+5)
	mockErr := "Failed peek"
	MockPeek(mock, builds[:PEEK_MAX_BUILDS], now, "", "cursor1")
	MockPeekFailed(mock, builds[PEEK_MAX_BUILDS:], now, "cursor1", "", mockErr)
	builds = builds[:PEEK_MAX_BUILDS]
	for _, b := range builds {
		MockTryLeaseBuild(mock, b.Id)
		mockBB.On("GetBuild", context.Background(), b.Id).Return(b, nil)
	}
	require.ErrorContains(t, trybots.Poll(context.Background()), "got errors loading builds from Buildbucket: [Failed peek]")
	require.True(t, mock.Empty(), mock.List())
	testPollAssertAdded(t, now, trybots, builds)
}
