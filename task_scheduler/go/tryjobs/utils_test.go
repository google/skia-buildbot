package tryjobs

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	buildbucket_api "go.chromium.org/luci/common/api/buildbucket/buildbucket/v1"
	"go.skia.org/infra/go/buildbucket/mocks"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/now"
	pubsub_mocks "go.skia.org/infra/go/pubsub/mocks"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
	cacher_mocks "go.skia.org/infra/task_scheduler/go/cacher/mocks"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/db/memory"
	tcc_mocks "go.skia.org/infra/task_scheduler/go/task_cfg_cache/mocks"
	tcc_testutils "go.skia.org/infra/task_scheduler/go/task_cfg_cache/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

const (
	repoUrl = "skia.git"

	gerritIssue    = 2112
	gerritPatchset = 3
	patchProject   = "skia"
	parentProject  = "parent-project"

	fakeGerritUrl     = "https://fake-skia-review.googlesource.com"
	oldBranchName     = "old-branch"
	bbPubSubProject   = "fake-bb-pubsub-project"
	bbPubSubTopic     = "fake-bb-pubsub-topic"
	bbFakeStartToken  = "fake-bb-start-token"
	bbFakeUpdateToken = "fake-bb-update-token"
)

var (
	gerritPatch = types.Patch{
		Server:    fakeGerritUrl,
		Issue:     fmt.Sprintf("%d", gerritIssue),
		PatchRepo: repoUrl,
		Patchset:  fmt.Sprintf("%d", gerritPatchset),
	}

	commit1 = &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    "abc123",
			Author:  "me@google.com",
			Subject: "initial commit",
		},
		Branches: map[string]bool{
			git.MainBranch: true,
		},
	}

	commit2 = &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    "def456",
			Author:  "me@google.com",
			Subject: "second commit",
		},
		Branches: map[string]bool{
			git.MainBranch: true,
		},
		Parents: []string{commit1.Hash},
	}

	repoState1 = types.RepoState{
		Patch:    gerritPatch,
		Repo:     repoUrl,
		Revision: commit1.Hash,
	}
	repoState2 = types.RepoState{
		Patch:    gerritPatch,
		Repo:     repoUrl,
		Revision: commit2.Hash,
	}

	// Arbitrary start time to keep tests consistent.
	ts = time.Unix(1632920378, 0)
)

// setup prepares the tests to run. Returns the created temporary dir,
// TryJobIntegrator instance, and URLMock instance.
func setup(t sktest.TestingT) (context.Context, *TryJobIntegrator, *mockhttpclient.URLMock, *mocks.BuildBucketInterface, *pubsub_mocks.Topic) {
	ctx := context.WithValue(context.Background(), now.ContextKey, ts)

	// Set up other TryJobIntegrator inputs.
	taskCfgCache := tcc_mocks.FixedTasksCfg(tcc_testutils.TasksCfg1)
	d := memory.NewInMemoryDB()
	mock := mockhttpclient.NewURLMock()
	projectRepoMapping := map[string]string{
		patchProject: repoUrl,
	}
	g, err := gerrit.NewGerrit(fakeGerritUrl, mock.Client())
	require.NoError(t, err)
	chr := &cacher_mocks.Cacher{}
	chr.On("GetOrCacheRepoState", testutils.AnyContext, repoState1).Return(tcc_testutils.TasksCfg1, nil)
	chr.On("GetOrCacheRepoState", testutils.AnyContext, repoState2).Return(tcc_testutils.TasksCfg1, nil)

	branch1 := &git.Branch{
		Name: git.MainBranch,
		Head: commit2.Hash,
	}
	branch2 := &git.Branch{
		Name: oldBranchName,
		Head: commit1.Hash,
	}
	repoImpl := repograph.NewMemCacheRepoImpl(map[string]*vcsinfo.LongCommit{
		commit1.Hash: commit1,
		commit2.Hash: commit2,
	}, []*git.Branch{branch1, branch2})
	repo, err := repograph.NewWithRepoImpl(ctx, repoImpl)
	require.NoError(t, err)
	rm := map[string]*repograph.Graph{
		repoUrl: repo,
	}
	window, err := window.New(ctx, time.Hour, 100, rm)
	require.NoError(t, err)
	jCache, err := cache.NewJobCache(ctx, d, window, nil)
	require.NoError(t, err)
	pubsubClient := &pubsub_mocks.Client{}
	pubsubTopic := &pubsub_mocks.Topic{}
	pubsubClient.On("Topic", bbPubSubTopic).Return(pubsubTopic, nil)
	integrator, err := NewTryJobIntegrator(ctx, API_URL_TESTING, "fake-bb-target", BUCKET_TESTING, "fake-server", mock.Client(), d, jCache, projectRepoMapping, rm, taskCfgCache, chr, g, pubsubClient)
	require.NoError(t, err)
	return ctx, integrator, mock, MockBuildbucket(integrator), pubsubTopic
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
			Builder: tcc_testutils.BuildTaskName,
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

func tryjobV1(ctx context.Context, repoName string) *types.Job {
	return &types.Job{
		BuildbucketBuildId:  rand.Int63(),
		BuildbucketLeaseKey: rand.Int63(),
		Created:             now.Now(ctx),
		Name:                tcc_testutils.BuildTaskName,
		RepoState:           repoState2,
	}
}

func tryjobV2(ctx context.Context, repoName string) *types.Job {
	job := tryjobV1(ctx, repoName)
	job.BuildbucketLeaseKey = 0
	job.BuildbucketToken = bbFakeStartToken
	job.BuildbucketPubSubTopic = bbPubSubTopic
	return job
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

func MockCancelBuild(mock *mockhttpclient.URLMock, id int64, msg string) {
	req := []byte(fmt.Sprintf("{\"result_details_json\":\"{\\\"message\\\":\\\"%s\\\"}\"}\n", msg))
	resp := []byte("{}")
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/cancel?alt=json&prettyPrint=false", API_URL_TESTING, id), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockCancelBuildFailed(mock *mockhttpclient.URLMock, id int64, msg string, mockErr string) {
	req := []byte(fmt.Sprintf("{\"result_details_json\":\"{\\\"message\\\":\\\"%s\\\"}\"}\n", msg))
	resp := []byte(fmt.Sprintf("{\"error\": {\"message\": \"%s\"}}", mockErr))
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/cancel?alt=json&prettyPrint=false", API_URL_TESTING, id), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockTryLeaseBuild(mock *mockhttpclient.URLMock, id int64) {
	req := mockhttpclient.DONT_CARE_REQUEST
	resp := []byte(fmt.Sprintf("{\"build\": {\"lease_key\": \"%d\"}}", 987654321))
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/lease?alt=json&prettyPrint=false", API_URL_TESTING, id), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockTryLeaseBuildFailed(mock *mockhttpclient.URLMock, id int64, mockErr string) {
	req := mockhttpclient.DONT_CARE_REQUEST
	resp := []byte(fmt.Sprintf("{\"error\": {\"message\": \"%s\"}}", mockErr))
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/lease?alt=json&prettyPrint=false", API_URL_TESTING, id), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockJobStarted(mock *mockhttpclient.URLMock, id int64) {
	// We have to use this because we don't know what the Job ID is going to
	// be until after it's inserted into the DB.
	req := mockhttpclient.DONT_CARE_REQUEST
	resp := []byte(fmt.Sprintf(`{"build": {},"update_build_token":"%s"}`, bbFakeUpdateToken))
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/start?alt=json&prettyPrint=false", API_URL_TESTING, id), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockJobStartedFailed(mock *mockhttpclient.URLMock, id int64, mockErr, mockReason string) {
	// We have to use this because we don't know what the Job ID is going to
	// be until after it's inserted into the DB.
	req := mockhttpclient.DONT_CARE_REQUEST
	resp := []byte(fmt.Sprintf(`{"error":{"message":"%s","reason":"%s"}}`, mockErr, mockReason))
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

func MockJobSuccess(mock *mockhttpclient.URLMock, j *types.Job, now time.Time, dontCareRequest bool) {
	req := mockhttpclient.DONT_CARE_REQUEST
	if !dontCareRequest {
		req = []byte(fmt.Sprintf("{\"lease_key\":\"%d\",\"result_details_json\":\"{\\\"job\\\":%s}\",\"url\":\"fake-server/job/%s\"}\n", j.BuildbucketLeaseKey, serializeJob(j), j.Id))
	}
	resp := []byte("{}")
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/succeed?alt=json&prettyPrint=false", API_URL_TESTING, j.BuildbucketBuildId), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockJobSuccess_Failed(mock *mockhttpclient.URLMock, j *types.Job, now time.Time, dontCareRequest bool, expectErr string) {
	req := mockhttpclient.DONT_CARE_REQUEST
	if !dontCareRequest {
		req = []byte(fmt.Sprintf("{\"lease_key\":\"%d\",\"result_details_json\":\"{\\\"job\\\":%s}\",\"url\":\"fake-server/job/%s\"}\n", j.BuildbucketLeaseKey, serializeJob(j), j.Id))
	}
	resp := []byte(fmt.Sprintf("{\"error\":{\"message\":\"%s\"}}", expectErr))
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/succeed?alt=json&prettyPrint=false", API_URL_TESTING, j.BuildbucketBuildId), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockJobFailure(mock *mockhttpclient.URLMock, j *types.Job, now time.Time) {
	req := []byte(fmt.Sprintf("{\"failure_reason\":\"BUILD_FAILURE\",\"lease_key\":\"%d\",\"result_details_json\":\"{\\\"job\\\":%s}\",\"url\":\"fake-server/job/%s\"}\n", j.BuildbucketLeaseKey, serializeJob(j), j.Id))
	resp := []byte("{}")
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/fail?alt=json&prettyPrint=false", API_URL_TESTING, j.BuildbucketBuildId), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockJobFailure_Failed(mock *mockhttpclient.URLMock, j *types.Job, now time.Time, expectErr string) {
	req := []byte(fmt.Sprintf("{\"failure_reason\":\"BUILD_FAILURE\",\"lease_key\":\"%d\",\"result_details_json\":\"{\\\"job\\\":%s}\",\"url\":\"fake-server/job/%s\"}\n", j.BuildbucketLeaseKey, serializeJob(j), j.Id))
	resp := []byte(fmt.Sprintf("{\"error\":{\"message\":\"%s\"}}", expectErr))
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/fail?alt=json&prettyPrint=false", API_URL_TESTING, j.BuildbucketBuildId), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockJobMishap(mock *mockhttpclient.URLMock, j *types.Job, now time.Time) {
	req := []byte(fmt.Sprintf("{\"failure_reason\":\"INFRA_FAILURE\",\"lease_key\":\"%d\",\"result_details_json\":\"{\\\"job\\\":%s}\",\"url\":\"fake-server/job/%s\"}\n", j.BuildbucketLeaseKey, serializeJob(j), j.Id))
	resp := []byte("{}")
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/fail?alt=json&prettyPrint=false", API_URL_TESTING, j.BuildbucketBuildId), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockJobMishap_Failed(mock *mockhttpclient.URLMock, j *types.Job, now time.Time, expectErr string) {
	req := []byte(fmt.Sprintf("{\"failure_reason\":\"INFRA_FAILURE\",\"lease_key\":\"%d\",\"result_details_json\":\"{\\\"job\\\":%s}\",\"url\":\"fake-server/job/%s\"}\n", j.BuildbucketLeaseKey, serializeJob(j), j.Id))
	resp := []byte(fmt.Sprintf("{\"error\":{\"message\":\"%s\"}}", expectErr))
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/fail?alt=json&prettyPrint=false", API_URL_TESTING, j.BuildbucketBuildId), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockPeek(mock *mockhttpclient.URLMock, builds []*buildbucketpb.Build, now time.Time, cursor, nextcursor string) {
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
	respBytes, err := json.Marshal(&resp)
	if err != nil {
		panic(err)
	}
	mock.MockOnce(fmt.Sprintf("%speek?alt=json&bucket=%s&max_builds=%d&prettyPrint=false&start_cursor=%s", API_URL_TESTING, BUCKET_TESTING, PEEK_MAX_BUILDS, cursor), mockhttpclient.MockGetDialogue(respBytes))
}

func MockPeekFailed(mock *mockhttpclient.URLMock, builds []*buildbucketpb.Build, now time.Time, cursor, nextcursor, mockErr string) {
	legacyBuilds := make([]*buildbucket_api.LegacyApiCommonBuildMessage, 0, len(builds))
	for _, b := range builds {
		legacyBuilds = append(legacyBuilds, &buildbucket_api.LegacyApiCommonBuildMessage{
			Id: b.Id,
		})
	}
	resp := buildbucket_api.LegacyApiSearchResponseMessage{
		Builds:     legacyBuilds,
		NextCursor: nextcursor,
		Error: &buildbucket_api.LegacyApiErrorMessage{
			Message: mockErr,
		},
	}
	respBytes, err := json.Marshal(&resp)
	if err != nil {
		panic(err)
	}
	mock.MockOnce(fmt.Sprintf("%speek?alt=json&bucket=%s&max_builds=%d&prettyPrint=false&start_cursor=%s", API_URL_TESTING, BUCKET_TESTING, PEEK_MAX_BUILDS, cursor), mockhttpclient.MockGetDialogue(respBytes))
}
