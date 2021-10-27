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
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	buildbucket_api "go.chromium.org/luci/common/api/buildbucket/buildbucket/v1"
	"go.skia.org/infra/go/buildbucket/mocks"
	cas_mocks "go.skia.org/infra/go/cas/mocks"
	depot_tools_testutils "go.skia.org/infra/go/depot_tools/testutils"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_scheduler/go/cacher"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/db/memory"
	"go.skia.org/infra/task_scheduler/go/syncer"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	tcc_testutils "go.skia.org/infra/task_scheduler/go/task_cfg_cache/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

const (
	repoBaseName = "skia.git"
	test         = `a"` + `b` + `c"`

	testTasksCfg = `{
  "casSpecs": {
    "fake": {
      "digest": "` + tcc_testutils.CompileCASDigest + `"
    }
  },
  "tasks": {
    "fake-task1": {
      "casSpec": "fake",
      "dependencies": [],
      "dimensions": ["pool:Skia", "os:Ubuntu", "cpu:x86-64-avx2", "gpu:none"],
      "extra_args": [],
      "priority": 0.8
    },
    "fake-task2": {
      "casSpec": "fake",
      "dependencies": ["fake-task1"],
      "dimensions": ["pool:Skia", "os:Ubuntu", "cpu:x86-64-avx2", "gpu:none"],
      "extra_args": [],
      "priority": 0.8
	},
    "cd-task": {
      "casSpec": "fake",
      "dependencies": [],
      "dimensions": ["pool:Skia", "os:Ubuntu", "cpu:x86-64-avx2", "gpu:none"],
      "extra_args": [],
      "priority": 1.0
    }
  },
  "jobs": {
    "fake-job": {
      "priority": 0.8,
      "tasks": ["fake-task2"]
    },
    "cd-job": {
      "is_cd": true,
      "priority": 1.0,
	  "tasks": ["cd-task"],
	  "trigger": "main"
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

	// Arbitrary start time to keep tests consistent.
	ts = time.Unix(1632920378, 0)
)

// setup prepares the tests to run. Returns the created temporary dir,
// TryJobIntegrator instance, and URLMock instance.
func setup(t sktest.TestingT) (context.Context, *TryJobIntegrator, *git_testutils.GitBuilder, *mockhttpclient.URLMock, *mocks.BuildBucketInterface, func()) {
	unittest.LargeTest(t)

	ctx := now.TimeTravelingContext(ts)

	// Set up the test Git repo.
	gb := git_testutils.GitInit(t, ctx)
	require.NoError(t, os.MkdirAll(path.Join(gb.Dir(), "infra", "bots"), os.ModePerm))
	tasksJson := path.Join("infra", "bots", "tasks.json")
	gb.Add(ctx, tasksJson, testTasksCfg)
	gb.Commit(ctx)

	rs := types.RepoState{
		Patch:    gerritPatch,
		Repo:     gb.RepoUrl(),
		Revision: git.MasterBranch,
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
	window, err := window.New(ctx, time.Hour, 100, rm)
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

	g, err := gerrit.NewGerrit(fakeGerritUrl, mock.Client())
	require.NoError(t, err)

	depotTools := depot_tools_testutils.GetDepotTools(t, ctx)
	s := syncer.New(ctx, rm, depotTools, tmpDir, syncer.DEFAULT_NUM_WORKERS)
	cas := &cas_mocks.CAS{}
	chr := cacher.New(s, taskCfgCache, cas)
	jCache, err := cache.NewJobCache(ctx, d, window, nil)
	require.NoError(t, err)
	integrator, err := NewTryJobIntegrator(API_URL_TESTING, BUCKET_TESTING, "fake-server", mock.Client(), d, jCache, projectRepoMapping, rm, taskCfgCache, chr, g)
	require.NoError(t, err)
	return ctx, integrator, gb, mock, MockBuildbucket(integrator), func() {
		testutils.AssertCloses(t, taskCfgCache)
		testutils.RemoveAll(t, tmpDir)
		gb.Cleanup()
		btCleanup()
		require.NoError(t, s.Close())
	}
}

func MockBuildbucket(tj *TryJobIntegrator) *mocks.BuildBucketInterface {
	bbMock := &mocks.BuildBucketInterface{}
	tj.bb2 = bbMock
	return bbMock
}

func Build(t sktest.TestingT, now time.Time) *buildbucketpb.Build {
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

func tryjob(ctx context.Context, repoName string) *types.Job {
	return &types.Job{
		BuildbucketBuildId:  rand.Int63(),
		BuildbucketLeaseKey: rand.Int63(),
		Created:             now.Now(ctx),
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

func MockHeartbeats(t sktest.TestingT, mock *mockhttpclient.URLMock, now time.Time, jobs []*types.Job, resps map[string]*heartbeatResp) {
	// Create the request data.
	sort.Sort(heartbeatJobSlice(jobs))
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

func MockCancelBuild(mock *mockhttpclient.URLMock, id int64, msg string, err error) {
	req := []byte(fmt.Sprintf("{\"result_details_json\":\"{\\\"message\\\":\\\"%s\\\"}\"}\n", msg))
	respStr := "{}"
	if err != nil {
		respStr = fmt.Sprintf("{\"error\": {\"message\": \"%s\"}}", err)
	}
	resp := []byte(respStr)
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/cancel?alt=json&prettyPrint=false", API_URL_TESTING, id), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockTryLeaseBuild(mock *mockhttpclient.URLMock, id int64, err error) {
	req := mockhttpclient.DONT_CARE_REQUEST
	respStr := fmt.Sprintf("{\"build\": {\"lease_key\": \"%d\"}}", 987654321)
	if err != nil {
		respStr = fmt.Sprintf("{\"error\": {\"message\": \"%s\"}}", err)
	}
	resp := []byte(respStr)
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/lease?alt=json&prettyPrint=false", API_URL_TESTING, id), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockJobStarted(mock *mockhttpclient.URLMock, id int64, err error) {
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

func MockJobSuccess(mock *mockhttpclient.URLMock, j *types.Job, now time.Time, expectErr error, dontCareRequest bool) {
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

func MockJobFailure(mock *mockhttpclient.URLMock, j *types.Job, now time.Time, expectErr error) {
	req := []byte(fmt.Sprintf("{\"failure_reason\":\"BUILD_FAILURE\",\"lease_key\":\"%d\",\"result_details_json\":\"{\\\"job\\\":%s}\",\"url\":\"fake-server/job/%s\"}\n", j.BuildbucketLeaseKey, serializeJob(j), j.Id))
	resp := []byte("{}")
	if expectErr != nil {
		resp = []byte(fmt.Sprintf("{\"error\":{\"message\":\"%s\"}}", expectErr.Error()))
	}
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/fail?alt=json&prettyPrint=false", API_URL_TESTING, j.BuildbucketBuildId), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockJobMishap(mock *mockhttpclient.URLMock, j *types.Job, now time.Time, expectErr error) {
	req := []byte(fmt.Sprintf("{\"failure_reason\":\"INFRA_FAILURE\",\"lease_key\":\"%d\",\"result_details_json\":\"{\\\"job\\\":%s}\",\"url\":\"fake-server/job/%s\"}\n", j.BuildbucketLeaseKey, serializeJob(j), j.Id))
	resp := []byte("{}")
	if expectErr != nil {
		resp = []byte(fmt.Sprintf("{\"error\":{\"message\":\"%s\"}}", expectErr.Error()))
	}
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/fail?alt=json&prettyPrint=false", API_URL_TESTING, j.BuildbucketBuildId), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockPeek(mock *mockhttpclient.URLMock, builds []*buildbucketpb.Build, now time.Time, cursor, nextcursor string, err error) {
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
