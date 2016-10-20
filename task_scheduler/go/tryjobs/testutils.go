package tryjobs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"strconv"
	"testing"
	"time"

	buildbucket_api "github.com/luci/luci-go/common/api/buildbucket/buildbucket/v1"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/jsonutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	repoName     = "skia.git"
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
	rietveldUrl  = "https://codereview.chromium.org/"
	gerritUrl    = "https://skia-review.googlesource.com/"
	patchProject = "skia"
)

var (
	rs = db.RepoState{
		Patch: db.Patch{
			Server:   gerritUrl,
			Issue:    "2112",
			Patchset: "3",
		},
		Repo:     repoName,
		Revision: "master",
	}

	projectRepoMapping = map[string]string{
		patchProject: repoName,
	}
)

func MockOutExec() {
	// Mock out exec.Run because "git cl patch" doesn't work with fake
	// issues.
	exec.SetRunForTesting(func(c *exec.Command) error {
		if c.Name == "git" && len(c.Args) >= 2 {
			if c.Args[0] == "cl" && c.Args[1] == "patch" {
				return nil
			} else if c.Args[0] == "reset" && c.Args[1] == "HEAD^" {
				return nil
			}
		}
		return exec.DefaultRun(c)
	})
}

// setup prepares the tests to run. Returns the created temporary dir,
// TryJobIntegrator instance, and URLMock instance.
func setup(t *testing.T) (string, *TryJobIntegrator, *mockhttpclient.URLMock) {
	testutils.SkipIfShort(t)

	// Set up the test Git repo.
	tmpDir, err := ioutil.TempDir("", "try_job_integrator_test_")
	assert.NoError(t, err)
	repoDir := path.Join(tmpDir, repoName)
	assert.NoError(t, os.Mkdir(repoDir, os.ModePerm))
	testutils.Run(t, repoDir, "git", "init")
	testutils.Run(t, repoDir, "git", "remote", "add", "origin", ".")
	assert.NoError(t, os.MkdirAll(path.Join(repoDir, "infra", "bots"), os.ModePerm))
	tasksJson := path.Join(repoDir, "infra", "bots", "tasks.json")
	testutils.WriteFile(t, tasksJson, testTasksCfg)
	testutils.Run(t, repoDir, "git", "add", tasksJson)
	testutils.Run(t, repoDir, "git", "commit", "-m", "Initial Commit")
	testutils.Run(t, repoDir, "git", "push", "origin", "master")
	testutils.Run(t, repoDir, "git", "branch", "-u", "origin/master")
	rm := gitinfo.NewRepoMap(tmpDir)
	_, err = rm.Repo(repoName)
	assert.NoError(t, err)

	// Set up other TryJobIntegrator inputs.
	taskCfgCache := specs.NewTaskCfgCache(path.Join(tmpDir, "cfg_cache"), rm)
	d, err := local_db.NewDB("tasks_db", path.Join(tmpDir, "tasks.db"))
	assert.NoError(t, err)
	cache, err := db.NewJobCache(d, time.Hour, db.DummyGetRevisionTimestamp(time.Now()))
	assert.NoError(t, err)
	mock := mockhttpclient.NewURLMock()
	integrator, err := NewTryJobIntegrator(API_URL_TESTING, BUCKET_TESTING, mock.Client(), d, cache, projectRepoMapping, rm, taskCfgCache)
	assert.NoError(t, err)

	MockOutExec()

	return tmpDir, integrator, mock
}

func Params(t *testing.T, builder, project, revision, server, issue, patchset string) buildbucket.Parameters {
	p := buildbucket.Parameters{
		BuilderName: builder,
		Properties: buildbucket.Properties{
			PatchProject: project,
			Revision:     revision,
		},
	}
	issueInt, err := strconv.Atoi(issue)
	assert.NoError(t, err)
	patchsetInt, err := strconv.Atoi(patchset)
	assert.NoError(t, err)

	if server == rietveldUrl {
		p.Properties.PatchStorage = "rietveld"
		p.Properties.Rietveld = server
		p.Properties.RietveldIssue = jsonutils.Number(issueInt)
		p.Properties.RietveldPatchset = jsonutils.Number(patchsetInt)
	} else if server == gerritUrl {
		p.Properties.PatchStorage = "gerrit"
		p.Properties.Gerrit = gerritUrl
		p.Properties.GerritIssue = jsonutils.Number(issueInt)
		p.Properties.GerritPatchset = patchset
	} else {
		assert.FailNow(t, "Invalid server")
	}
	return p
}

func Build(t *testing.T, now time.Time) *buildbucket_api.ApiBuildMessage {
	return &buildbucket_api.ApiBuildMessage{
		Bucket:            BUCKET_TESTING,
		CreatedBy:         "tests",
		CreatedTs:         now.Unix() * 1000000,
		Id:                rand.Int63(),
		LeaseExpirationTs: now.Add(LEASE_DURATION_INITIAL).Unix() * 1000000,
		LeaseKey:          987654321,
		ParametersJson:    testutils.MarshalJSON(t, Params(t, "fake-job", patchProject, rs.Revision, rs.Server, rs.Issue, rs.Patchset)),
		Status:            "SCHEDULED",
	}
}

func tryjob() *db.Job {
	return &db.Job{
		BuildbucketBuildId:  rand.Int63(),
		BuildbucketLeaseKey: rand.Int63(),
		Created:             time.Now(),
		Name:                "fake-name",
		RepoState: db.RepoState{
			Patch: db.Patch{
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

func MockHeartbeats(t *testing.T, mock *mockhttpclient.URLMock, now time.Time, jobs []*db.Job, resps map[string]*heartbeatResp) {
	// Create the request data.
	expiry := fmt.Sprintf("%d", now.Add(LEASE_DURATION).Unix()*1000000)
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
	assert.NoError(t, err)
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
	assert.NoError(t, err)
	resp = append(resp, []byte("\n")...)

	mock.MockOnce(fmt.Sprintf("%sheartbeat?alt=json", API_URL_TESTING), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockCancelBuild(mock *mockhttpclient.URLMock, id int64, err error) {
	respStr := "{}"
	if err != nil {
		respStr = fmt.Sprintf("{\"error\": {\"message\": \"%s\"}}", err)
	}
	resp := []byte(respStr)
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/cancel?alt=json", API_URL_TESTING, id), mockhttpclient.MockPostDialogue("", nil, resp))
}

func MockTryLeaseBuild(mock *mockhttpclient.URLMock, id int64, now time.Time, err error) {
	req := []byte(fmt.Sprintf("{\"lease_expiration_ts\":\"%d\"}\n", now.Add(LEASE_DURATION_INITIAL).Unix()*1000000))
	respStr := fmt.Sprintf("{\"build\": {\"lease_key\": \"%d\"}}", 987654321)
	if err != nil {
		respStr = fmt.Sprintf("{\"error\": {\"message\": \"%s\"}}", err)
	}
	resp := []byte(respStr)
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/lease?alt=json", API_URL_TESTING, id), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockJobStarted(mock *mockhttpclient.URLMock, id int64, now time.Time, err error) {
	// We have to use this because we don't know what the Job ID is going to
	// be until after it's inserted into the DB.
	req := mockhttpclient.DONT_CARE_REQUEST
	resp := []byte("{}")
	if err != nil {
		resp = []byte(fmt.Sprintf("{\"error\":{\"message\":\"%s\"}}", err.Error()))
	}
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/start?alt=json", API_URL_TESTING, id), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockJobSuccess(mock *mockhttpclient.URLMock, j *db.Job, now time.Time, err error) {
	req := []byte(fmt.Sprintf("{\"lease_key\":\"%d\",\"result_details_json\":\"{\\\"result\\\": \\\"TODO(borenet)\\\"}\",\"url\":\"https://task-scheduler.skia.org/job/%s\"}\n", j.BuildbucketLeaseKey, j.Id))
	resp := []byte("{}")
	if err != nil {
		resp = []byte(fmt.Sprintf("{\"error\":{\"message\":\"%s\"}}", err.Error()))
	}
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/succeed?alt=json", API_URL_TESTING, j.BuildbucketBuildId), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockJobFailure(mock *mockhttpclient.URLMock, j *db.Job, now time.Time, err error) {
	req := []byte(fmt.Sprintf("{\"failure_reason\":\"BUILD_FAILURE\",\"lease_key\":\"%d\",\"result_details_json\":\"{\\\"result\\\": \\\"TODO(borenet)\\\"}\",\"url\":\"https://task-scheduler.skia.org/job/%s\"}\n", j.BuildbucketLeaseKey, j.Id))
	resp := []byte("{}")
	if err != nil {
		resp = []byte(fmt.Sprintf("{\"error\":{\"message\":\"%s\"}}", err.Error()))
	}
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/fail?alt=json", API_URL_TESTING, j.BuildbucketBuildId), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockJobMishap(mock *mockhttpclient.URLMock, j *db.Job, now time.Time, err error) {
	req := []byte(fmt.Sprintf("{\"failure_reason\":\"INFRA_FAILURE\",\"lease_key\":\"%d\",\"result_details_json\":\"{\\\"result\\\": \\\"TODO(borenet)\\\"}\",\"url\":\"https://task-scheduler.skia.org/job/%s\"}\n", j.BuildbucketLeaseKey, j.Id))
	resp := []byte("{}")
	if err != nil {
		resp = []byte(fmt.Sprintf("{\"error\":{\"message\":\"%s\"}}", err.Error()))
	}
	mock.MockOnce(fmt.Sprintf("%sbuilds/%d/fail?alt=json", API_URL_TESTING, j.BuildbucketBuildId), mockhttpclient.MockPostDialogue("application/json", req, resp))
}

func MockPeek(mock *mockhttpclient.URLMock, builds []*buildbucket_api.ApiBuildMessage, now time.Time, cursor, nextcursor string, err error) {
	resp := buildbucket_api.ApiSearchResponseMessage{
		Builds:     builds,
		NextCursor: nextcursor,
	}
	if err != nil {
		resp.Error = &buildbucket_api.ApiErrorMessage{
			Message: err.Error(),
		}
	}
	respBytes, err := json.Marshal(&resp)
	if err != nil {
		panic(err)
	}
	mock.MockOnce(fmt.Sprintf("%speek?alt=json&bucket=%s&max_builds=50&start_cursor=%s", API_URL_TESTING, BUCKET_TESTING, cursor), mockhttpclient.MockGetDialogue(respBytes))
}
