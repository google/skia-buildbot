package tryjobs

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	buildbucket_api "go.chromium.org/luci/common/api/buildbucket/buildbucket/v1"
	"go.skia.org/infra/go/buildbucket/mocks"
	"go.skia.org/infra/go/deepequal/assertdeep"
	depot_tools_testutils "go.skia.org/infra/go/depot_tools/testutils"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_scheduler/go/cacher"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/db/memory"
	"go.skia.org/infra/task_scheduler/go/isolate_cache"
	isolate_cache_testutils "go.skia.org/infra/task_scheduler/go/isolate_cache/testutils"
	"go.skia.org/infra/task_scheduler/go/syncer"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	tcc_testutils "go.skia.org/infra/task_scheduler/go/task_cfg_cache/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

const (
	repoBaseName = "skia.git"
	testTasksCfg = `{
  "tasks": {
    "fake-task1": {
      "dependencies": [],
      "dimensions": ["pool:Skia", "os:Ubuntu", "cpu:x86-64-avx2", "gpu:none"],
      "extra_args": [],
      "isolate": "fake1.isolate",
      "priority": 0.8
    },
    "fake-task2": {
      "dependencies": ["fake-task1"],
      "dimensions": ["pool:Skia", "os:Ubuntu", "cpu:x86-64-avx2", "gpu:none"],
      "extra_args": [],
      "isolate": "fake2.isolate",
      "priority": 0.8
    }
  },
  "jobs": {
    "fake-job": {
      "priority": 0.8,
      "tasks": ["fake-task2"]
    }
  }
}`
	gerritIssue    = 2112
	gerritPatchset = 3
	patchProject   = "skia"
	parentProject  = "parent-project"

	fakeGerritUrl = "https://fake-skia-review.googlesource.com"
)

var (
	gerritPatch = types.Patch{
		Server:   fakeGerritUrl,
		Issue:    fmt.Sprintf("%d", gerritIssue),
		Patchset: fmt.Sprintf("%d", gerritPatchset),
	}
)

// setup prepares the tests to run. Returns the created temporary dir,
// TryJobIntegrator instance, and URLMock instance.
func setup(t sktest.TestingT) (context.Context, *TryJobIntegrator, *git_testutils.GitBuilder, *mockhttpclient.URLMock, *mocks.BuildBucketInterface, func()) {
	unittest.LargeTest(t)

	ctx := context.Background()

	// Set up the test Git repo.
	gb := git_testutils.GitInit(t, ctx)
	require.NoError(t, os.MkdirAll(path.Join(gb.Dir(), "infra", "bots"), os.ModePerm))
	tasksJson := path.Join("infra", "bots", "tasks.json")
	gb.Add(ctx, tasksJson, testTasksCfg)
	gb.Add(ctx, path.Join("infra", "bots", "fake1.isolate"), "{}")
	gb.Add(ctx, path.Join("infra", "bots", "fake2.isolate"), "{}")
	gb.Commit(ctx)

	rs := types.RepoState{
		Patch:    gerritPatch,
		Repo:     gb.RepoUrl(),
		Revision: "master",
	}

	// Create a ref for a fake patch.
	gb.CreateFakeGerritCLGen(ctx, rs.Issue, rs.Patchset)

	// Create a second repo, for cross-repo tryjob testing.
	gb2 := git_testutils.GitInit(t, ctx)
	gb2.CommitGen(ctx, "somefile")

	// Create repo map.
	tmpDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	rm, err := repograph.NewLocalMap(ctx, []string{gb.RepoUrl(), gb2.RepoUrl()}, tmpDir)
	require.NoError(t, err)
	require.NoError(t, rm.Update(ctx))

	// Set up other TryJobIntegrator inputs.
	window, err := window.New(time.Hour, 100, rm)
	require.NoError(t, err)
	btProject, btInstance, btCleanup := tcc_testutils.SetupBigTable(t)
	taskCfgCache, err := task_cfg_cache.NewTaskCfgCache(ctx, rm, btProject, btInstance, nil)
	require.NoError(t, err)
	d := memory.NewInMemoryDB()
	mock := mockhttpclient.NewURLMock()
	projectRepoMapping := map[string]string{
		patchProject:  gb.RepoUrl(),
		parentProject: gb2.RepoUrl(),
	}

	gitcookies := path.Join(tmpDir, "gitcookies_fake")
	require.NoError(t, ioutil.WriteFile(gitcookies, []byte(".googlesource.com\tTRUE\t/\tTRUE\t123\to\tgit-user.google.com=abc123"), os.ModePerm))
	g, err := gerrit.NewGerrit(fakeGerritUrl, gitcookies, mock.Client())
	require.NoError(t, err)

	depotTools := depot_tools_testutils.GetDepotTools(t, ctx)
	s := syncer.New(ctx, rm, depotTools, tmpDir, syncer.DEFAULT_NUM_WORKERS)
	isolateClient, err := isolate.NewClient(tmpDir, isolate.ISOLATE_SERVER_URL_FAKE)
	require.NoError(t, err)
	btCleanupIsolate := isolate_cache_testutils.SetupSharedBigTable(t, btProject, btInstance)
	isolateCache, err := isolate_cache.New(ctx, btProject, btInstance, nil)
	require.NoError(t, err)
	chr := cacher.New(s, taskCfgCache, isolateClient, isolateCache)
	jCache, err := cache.NewJobCache(ctx, d, window, nil)
	require.NoError(t, err)
	integrator, err := NewTryJobIntegrator(API_URL_TESTING, BUCKET_TESTING, "fake-server", mock.Client(), d, jCache, projectRepoMapping, rm, taskCfgCache, chr, g)
	require.NoError(t, err)
	return ctx, integrator, gb, mock, mockBuildbucket(integrator), func() {
		testutils.AssertCloses(t, taskCfgCache)
		testutils.RemoveAll(t, tmpDir)
		gb.Cleanup()
		btCleanupIsolate()
		btCleanup()
		require.NoError(t, s.Close())
	}
}

func mockBuildbucket(tj *TryJobIntegrator) *mocks.BuildBucketInterface {
	bbMock := &mocks.BuildBucketInterface{}
	tj.bb2 = bbMock
	return bbMock
}

func build(t sktest.TestingT, now time.Time) *buildbucketpb.Build {
	issue, err := strconv.Atoi(gerritPatch.Issue)
	require.NoError(t, err)
	patchset, err := strconv.Atoi(gerritPatch.Patchset)
	require.NoError(t, err)
	ts, err := ptypes.TimestampProto(now)
	require.NoError(t, err)
	return &buildbucketpb.Build{
		Builder: &buildbucketpb.BuilderID{
			Project: "TESTING",
			Bucket:  BUCKET_TESTING,
			Builder: "fake-job",
		},
		CreatedBy:  "tests",
		CreateTime: ts,
		Id:         rand.Int63(),
		Input: &buildbucketpb.Build_Input{
			GerritChanges: []*buildbucketpb.GerritChange{
				{
					Host:     strings.TrimPrefix(fakeGerritUrl, "https://"),
					Project:  patchProject,
					Change:   int64(issue),
					Patchset: int64(patchset),
				},
			},
		},
		Status: buildbucketpb.Status_SCHEDULED,
	}
}

func tryjob(repoName string) *types.Job {
	return &types.Job{
		BuildbucketBuildId:  rand.Int63(),
		BuildbucketLeaseKey: rand.Int63(),
		Created:             time.Now(),
		Name:                "fake-name",
		RepoState: types.RepoState{
			Patch: types.Patch{
				Server:   "fake-server",
				Issue:    "fake-issue",
				Patchset: "fake-patchset",
			},
			Repo:     repoName,
			Revision: "fake-revision",
		},
	}
}

type errMsg struct {
	Message string `json:"message"`
}

type heartbeat struct {
	BuildId           string `json:"build_id"`
	LeaseExpirationTs string `json:"lease_expiration_ts"`
	LeaseKey          string `json:"lease_key"`
}

type heartbeatResp struct {
	BuildId string  `json:"build_id,omitempty"`
	Error   *errMsg `json:"error,omitempty"`
}

func mockHeartbeats(t sktest.TestingT, mock *mockhttpclient.URLMock, now time.Time, jobs []*types.Job, resps map[string]*heartbeatResp) {
	// Create the request data.
	expiry := fmt.Sprintf("%d", now.Add(LEASE_DURATION).Unix()*secondsToMicros)
	heartbeats := make([]*heartbeat, 0, len(jobs))
	for _, j := range jobs {
		heartbeats = append(heartbeats, &heartbeat{
			BuildId:           fmt.Sprintf("%d", j.BuildbucketBuildId),
			LeaseExpirationTs: expiry,
			LeaseKey:          fmt.Sprintf("%d", j.BuildbucketLeaseKey),
		})
	}
	req, err := json.Marshal(&struct {
		Heartbeats []*heartbeat `json:"heartbeats"`
	}{
		Heartbeats: heartbeats,
	})
	require.NoError(t, err)
	req = append(req, []byte("\n")...)

	// Create the response data.
	if resps == nil {
		resps = map[string]*heartbeatResp{}
	}
	results := make([]*heartbeatResp, 0, len(jobs))
	for _, j := range jobs {
		r, ok := resps[j.Id]
		if !ok {
			r = &heartbeatResp{
				BuildId: fmt.Sprintf("%d", j.BuildbucketBuildId),
			}
		}
		results = append(results, r)
	}
	resp, err := json.Marshal(&struct {
		Results []*heartbeatResp `json:"results"`
	}{
		Results: results,
	})
	require.NoError(t, err)
	resp = append(resp, []byte("\n")...)

	mock.MockOnce(fmt.Sprintf("%sheartbeat?alt=json&prettyPrint=false", API_URL_TESTING), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func mockCancelBuild(mock *mockhttpclient.URLMock, id int64, msg string, err error) {
	req := []byte(fmt.Sprintf("{\"result_details_json\":\"{\\\"message\\\":\\\"%s\\\"}\"}\n", msg))
	respStr := "{}"
	if err != nil {
		respStr = fmt.Sprintf("{\"error\": {\"message\": \"%s\"}}", err)
	}
	resp := []byte(respStr)
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/cancel?alt=json&prettyPrint=false", API_URL_TESTING, id), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func mockTryLeaseBuild(mock *mockhttpclient.URLMock, id int64, err error) {
	req := mockhttpclient.DONT_CARE_REQUEST
	respStr := fmt.Sprintf("{\"build\": {\"lease_key\": \"%d\"}}", 987654321)
	if err != nil {
		respStr = fmt.Sprintf("{\"error\": {\"message\": \"%s\"}}", err)
	}
	resp := []byte(respStr)
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/lease?alt=json&prettyPrint=false", API_URL_TESTING, id), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func mockJobStarted(mock *mockhttpclient.URLMock, id int64, err error) {
	// We have to use this because we don't know what the Job ID is going to
	// be until after it's inserted into the DB.
	req := mockhttpclient.DONT_CARE_REQUEST
	resp := []byte("{}")
	if err != nil {
		resp = []byte(fmt.Sprintf("{\"error\":{\"message\":\"%s\"}}", err.Error()))
	}
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/start?alt=json&prettyPrint=false", API_URL_TESTING, id), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func serializeJob(j *types.Job) string {
	jobBytes, err := json.Marshal(j)
	if err != nil {
		sklog.Fatal(err)
	}
	escape, err := json.Marshal(string(jobBytes))
	if err != nil {
		sklog.Fatal(err)
	}
	return string(escape[1 : len(escape)-1])
}

func mockJobSuccess(mock *mockhttpclient.URLMock, j *types.Job, now time.Time, expectErr error, dontCareRequest bool) {
	req := mockhttpclient.DONT_CARE_REQUEST
	if !dontCareRequest {
		req = []byte(fmt.Sprintf("{\"lease_key\":\"%d\",\"result_details_json\":\"{\\\"job\\\":%s}\",\"url\":\"fake-server/job/%s\"}\n", j.BuildbucketLeaseKey, serializeJob(j), j.Id))
	}
	resp := []byte("{}")
	if expectErr != nil {
		resp = []byte(fmt.Sprintf("{\"error\":{\"message\":\"%s\"}}", expectErr.Error()))
	}
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/succeed?alt=json&prettyPrint=false", API_URL_TESTING, j.BuildbucketBuildId), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func mockJobFailure(mock *mockhttpclient.URLMock, j *types.Job, now time.Time, expectErr error) {
	req := []byte(fmt.Sprintf("{\"failure_reason\":\"BUILD_FAILURE\",\"lease_key\":\"%d\",\"result_details_json\":\"{\\\"job\\\":%s}\",\"url\":\"fake-server/job/%s\"}\n", j.BuildbucketLeaseKey, serializeJob(j), j.Id))
	resp := []byte("{}")
	if expectErr != nil {
		resp = []byte(fmt.Sprintf("{\"error\":{\"message\":\"%s\"}}", expectErr.Error()))
	}
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/fail?alt=json&prettyPrint=false", API_URL_TESTING, j.BuildbucketBuildId), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func mockJobMishap(mock *mockhttpclient.URLMock, j *types.Job, now time.Time, expectErr error) {
	req := []byte(fmt.Sprintf("{\"failure_reason\":\"INFRA_FAILURE\",\"lease_key\":\"%d\",\"result_details_json\":\"{\\\"job\\\":%s}\",\"url\":\"fake-server/job/%s\"}\n", j.BuildbucketLeaseKey, serializeJob(j), j.Id))
	resp := []byte("{}")
	if expectErr != nil {
		resp = []byte(fmt.Sprintf("{\"error\":{\"message\":\"%s\"}}", expectErr.Error()))
	}
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/fail?alt=json&prettyPrint=false", API_URL_TESTING, j.BuildbucketBuildId), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func mockPeek(mock *mockhttpclient.URLMock, builds []*buildbucketpb.Build, now time.Time, cursor, nextcursor string, err error) {
	legacyBuilds := make([]*buildbucket_api.LegacyApiCommonBuildMessage, 0, len(builds))
	for _, b := range builds {
		legacyBuilds = append(legacyBuilds, &buildbucket_api.LegacyApiCommonBuildMessage{
			Id: b.Id,
		})
	}
	resp := buildbucket_api.LegacyApiSearchResponseMessage{
		Builds:     legacyBuilds,
		NextCursor: nextcursor,
	}
	if err != nil {
		resp.Error = &buildbucket_api.LegacyApiErrorMessage{
			Message: err.Error(),
		}
	}
	respBytes, err := json.Marshal(&resp)
	if err != nil {
		panic(err)
	}
	mock.MockOnce(fmt.Sprintf("%speek?alt=json&bucket=%s&max_builds=%d&prettyPrint=false&start_cursor=%s", API_URL_TESTING, BUCKET_TESTING, PEEK_MAX_BUILDS, cursor), mockhttpclient.MockGetDialogue(respBytes))
}

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
	mockHeartbeats(t, mock, now, []*types.Job{j1}, nil)
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
	mockJobSuccess(mock, j1, now, nil, false)
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
	mockJobFailure(mock, j1, now, nil)
	require.NoError(t, trybots.updateJobs(now))
	require.True(t, mock.Empty())
	assertNoActiveTryJobs()

	// More than one batch of heartbeats.
	jobs := []*types.Job{}
	for i := 0; i < LEASE_BATCH_SIZE+2; i++ {
		jobs = append(jobs, tryjob(gb.RepoUrl()))
	}
	sort.Sort(types.JobSlice(jobs))
	mockHeartbeats(t, mock, now, jobs[:LEASE_BATCH_SIZE], nil)
	mockHeartbeats(t, mock, now, jobs[LEASE_BATCH_SIZE:], nil)
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
		mockJobSuccess(mock, j, now, nil, false)
	}
	mockHeartbeats(t, mock, now, []*types.Job{j1, j2}, map[string]*heartbeatResp{
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
	c := r.Get("master").Hash

	// Fake response from Gerrit.
	ci := &gerrit.ChangeInfo{
		Branch: "master",
	}
	serialized := []byte(testutils.MarshalJSON(t, ci))
	// Gerrit API prepends garbage to prevent XSS.
	serialized = append([]byte("abcd\n"), serialized...)
	url := fmt.Sprintf("%s/a/changes/%d/detail?o=ALL_REVISIONS", fakeGerritUrl, gerritIssue)
	mock.Mock(url, mockhttpclient.MockGetDialogue(serialized))

	got, err := trybots.getRevision(context.TODO(), r, gerritIssue)
	require.NoError(t, err)
	require.Equal(t, c, got)
}

func TestCancelBuild(t *testing.T) {
	_, trybots, _, mock, _, cleanup := setup(t)
	defer cleanup()

	id := int64(12345)
	mockCancelBuild(mock, id, "Canceling!", nil)
	require.NoError(t, trybots.remoteCancelBuild(id, "Canceling!"))
	require.True(t, mock.Empty())

	// Check that reason is truncated if it's long.
	mockCancelBuild(mock, id, strings.Repeat("X", maxCancelReasonLen-3)+"...", nil)
	require.NoError(t, trybots.remoteCancelBuild(id, strings.Repeat("X", maxCancelReasonLen+50)))
	require.True(t, mock.Empty())

	err := fmt.Errorf("Build does not exist!")
	mockCancelBuild(mock, id, "Canceling!", err)
	require.EqualError(t, trybots.remoteCancelBuild(id, "Canceling!"), err.Error())
	require.True(t, mock.Empty())
}

func TestTryLeaseBuild(t *testing.T) {
	_, trybots, _, mock, _, cleanup := setup(t)
	defer cleanup()

	id := int64(12345)
	mockTryLeaseBuild(mock, id, nil)
	k, err := trybots.tryLeaseBuild(id)
	require.NoError(t, err)
	require.NotEqual(t, k, 0)
	require.True(t, mock.Empty())

	expect := fmt.Errorf("Can't lease this!")
	mockTryLeaseBuild(mock, id, expect)
	_, err = trybots.tryLeaseBuild(id)
	require.Contains(t, err.Error(), expect.Error())
	require.True(t, mock.Empty())
}

func TestJobStarted(t *testing.T) {
	_, trybots, gb, mock, _, cleanup := setup(t)
	defer cleanup()

	j := tryjob(gb.RepoUrl())

	// Success
	mockJobStarted(mock, j.BuildbucketBuildId, nil)
	require.NoError(t, trybots.jobStarted(j))
	require.True(t, mock.Empty())

	// Failure
	err := fmt.Errorf("fail")
	mockJobStarted(mock, j.BuildbucketBuildId, err)
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
	mockJobSuccess(mock, j, now, nil, false)
	require.NoError(t, trybots.jobFinished(j))
	require.True(t, mock.Empty())

	// Successful job, failed to update.
	err := fmt.Errorf("fail")
	mockJobSuccess(mock, j, now, err, false)
	require.EqualError(t, trybots.jobFinished(j), err.Error())
	require.True(t, mock.Empty())

	// Failed job.
	j.Status = types.JOB_STATUS_FAILURE
	require.NoError(t, trybots.db.PutJobs([]*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	mockJobFailure(mock, j, now, nil)
	require.NoError(t, trybots.jobFinished(j))
	require.True(t, mock.Empty())

	// Failed job, failed to update.
	mockJobFailure(mock, j, now, err)
	require.EqualError(t, trybots.jobFinished(j), err.Error())
	require.True(t, mock.Empty())

	// Mishap.
	j.Status = types.JOB_STATUS_MISHAP
	require.NoError(t, trybots.db.PutJobs([]*types.Job{j}))
	trybots.jCache.AddJobs([]*types.Job{j})
	mockJobMishap(mock, j, now, nil)
	require.NoError(t, trybots.jobFinished(j))
	require.True(t, mock.Empty())

	// Mishap, failed to update.
	mockJobMishap(mock, j, now, err)
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

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, "master")

	now := time.Now()

	aj := addedJobs(map[string]*types.Job{})

	// Normal job, Gerrit patch.
	b1 := build(t, now)
	mockTryLeaseBuild(mock, b1.Id, nil)
	mockJobStarted(mock, b1.Id, nil)
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
	mockTryLeaseBuild(mock, b1.Id, expectErr)
	mockBB.On("GetBuild", ctx, b1.Id).Return(b1, nil)
	err = trybots.insertNewJob(ctx, b1.Id)
	require.Contains(t, err.Error(), expectErr.Error())
	result = aj.getAddedJob(t, trybots.db)
	require.Nil(t, result)
	require.True(t, mock.Empty())

	// No GerritChanges.
	b2 := build(t, now)
	b2.Input.GerritChanges = nil
	mockCancelBuild(mock, b2.Id, fmt.Sprintf("Invalid Build %d: input should have exactly one GerritChanges: ", b2.Id), nil)
	mockBB.On("GetBuild", ctx, b2.Id).Return(b2, nil)
	err = trybots.insertNewJob(ctx, b2.Id)
	require.NoError(t, err) // We don't report errors for bad data from buildbucket.
	result = aj.getAddedJob(t, trybots.db)
	require.Nil(t, result)
	require.True(t, mock.Empty())

	// Invalid repo.
	b3 := build(t, now)
	b3.Input.GerritChanges[0].Project = "bogus-repo"
	mockCancelBuild(mock, b3.Id, "Unable to find repo: Unknown patch project \\\\\\\"bogus-repo\\\\\\\"", nil)
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
		Revision: trybots.rm[gb.RepoUrl()].Get("master").Hash,
	}
	rs.Patch.PatchRepo = rs.Repo
	b8 := build(t, now)
	b8.Builder.Builder = "bogus-job"
	mockCancelBuild(mock, b8.Id, fmt.Sprintf("Failed to create Job from JobSpec: bogus-job @ %+v: No such job: bogus-job", rs), nil)
	mockBB.On("GetBuild", ctx, b8.Id).Return(b8, nil)
	err = trybots.insertNewJob(ctx, b8.Id)
	require.NoError(t, err) // We don't report errors for bad data from buildbucket.
	result = aj.getAddedJob(t, trybots.db)
	require.Nil(t, result)
	require.True(t, mock.Empty())

	// Failure to cancel the build.
	b9 := build(t, now)
	b9.Builder.Builder = "bogus-job"
	expect := fmt.Errorf("no cancel!")
	mockCancelBuild(mock, b9.Id, fmt.Sprintf("Failed to create Job from JobSpec: bogus-job @ %+v: No such job: bogus-job", rs), expect)
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
	mock.Mock(fmt.Sprintf("%s/a%s", fakeGerritUrl, fmt.Sprintf(gerrit.URL_TMPL_CHANGE, ci.Id)), mockhttpclient.MockGetDialogue(issueBytes))
}

func TestRetry(t *testing.T) {
	ctx, trybots, _, mock, mockBB, cleanup := setup(t)
	defer cleanup()

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, "master")

	now := time.Now()

	// Insert one try job.
	aj := addedJobs(map[string]*types.Job{})
	b1 := build(t, now)
	mockTryLeaseBuild(mock, b1.Id, nil)
	mockJobStarted(mock, b1.Id, nil)
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
	require.NoError(t, trybots.jCache.Update())

	// Obtain a second try job, ensure that it gets IsForce = true.
	b2 := build(t, now)
	mockTryLeaseBuild(mock, b2.Id, nil)
	mockJobStarted(mock, b2.Id, nil)
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

	mockGetChangeInfo(t, mock, gerritIssue, patchProject, "master")

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
			builds = append(builds, build(t, now))
		}
		return builds
	}

	mockBuilds := func(builds []*buildbucketpb.Build) []*buildbucketpb.Build {
		mockPeek(mock, builds, now, "", "", nil)
		for _, b := range builds {
			mockTryLeaseBuild(mock, b.Id, nil)
			mockJobStarted(mock, b.Id, nil)
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
	mockPeek(mock, builds[:PEEK_MAX_BUILDS], now, "", "cursor1", nil)
	mockPeek(mock, builds[PEEK_MAX_BUILDS:], now, "cursor1", "", nil)
	for _, b := range builds {
		mockTryLeaseBuild(mock, b.Id, nil)
		mockJobStarted(mock, b.Id, nil)
		mockBB.On("GetBuild", ctx, b.Id).Return(b, nil)
	}
	check(builds)

	// Multiple new builds, fail insertNewJob, ensure successful builds
	// are inserted.
	builds = makeBuilds(5)
	failIdx := 2
	failBuild := builds[failIdx]
	failBuild.Input.GerritChanges[0].Project = "bogus"
	mockPeek(mock, builds, now, "", "", nil)
	builds = append(builds[:failIdx], builds[failIdx+1:]...)
	for _, b := range builds {
		mockTryLeaseBuild(mock, b.Id, nil)
		mockJobStarted(mock, b.Id, nil)
		mockBB.On("GetBuild", ctx, b.Id).Return(b, nil)
	}
	mockBB.On("GetBuild", ctx, failBuild.Id).Return(failBuild, nil)
	mockCancelBuild(mock, failBuild.Id, "Unable to find repo: Unknown patch project \\\\\\\"bogus\\\\\\\"", nil)
	check(builds)

	// Multiple new builds, fail jobStarted, ensure that the others are
	// properly added.
	builds = makeBuilds(5)
	failBuild = builds[failIdx]
	mockPeek(mock, builds, now, "", "", nil)
	builds = append(builds[:failIdx], builds[failIdx+1:]...)
	for _, b := range builds {
		mockTryLeaseBuild(mock, b.Id, nil)
		mockJobStarted(mock, b.Id, nil)
		mockBB.On("GetBuild", ctx, b.Id).Return(b, nil)
	}
	mockBB.On("GetBuild", ctx, failBuild.Id).Return(failBuild, nil)
	mockTryLeaseBuild(mock, failBuild.Id, nil)
	mockJobStarted(mock, failBuild.Id, fmt.Errorf("Failed to start build."))
	require.EqualError(t, trybots.Poll(ctx), "Got errors loading builds from Buildbucket: [Failed to send job-started notification with: Failed to start build.]")
	require.True(t, mock.Empty())
	assertAdded(builds)

	// More than one page of new builds, fail peeking a page, ensure that
	// other jobs get added.
	builds = makeBuilds(PEEK_MAX_BUILDS + 5)
	err := fmt.Errorf("Failed peek")
	mockPeek(mock, builds[:PEEK_MAX_BUILDS], now, "", "cursor1", nil)
	mockPeek(mock, builds[PEEK_MAX_BUILDS:], now, "cursor1", "", err)
	builds = builds[:PEEK_MAX_BUILDS]
	for _, b := range builds {
		mockTryLeaseBuild(mock, b.Id, nil)
		mockJobStarted(mock, b.Id, nil)
		mockBB.On("GetBuild", ctx, b.Id).Return(b, nil)
	}
	require.EqualError(t, trybots.Poll(ctx), "Got errors loading builds from Buildbucket: [Failed peek]")
	require.True(t, mock.Empty())
	assertAdded(builds)
}
