package scheduling

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/types"
)

func fullTaskCandidate() *TaskCandidate {
	return &TaskCandidate{
		Attempt:            3,
		BuildbucketBuildId: 8888,
		Commits:            []string{"a", "b"},
		Diagnostics:        &taskCandidateDiagnostics{},
		CasInput:           "lonely-parameter",
		CasDigests:         []string{"browns"},
		Jobs: []*types.Job{{
			Id: "dummy",
		}},
		ParentTaskIds:  []string{"38", "39", "40"},
		RetryOf:        "41",
		Score:          99,
		StealingFromId: "rich",
		TaskKey: types.TaskKey{
			RepoState: types.RepoState{
				Repo:     "nou.git",
				Revision: "1",
			},
			Name: "Build",
		},
		TaskSpec: &specs.TaskSpec{
			CasSpec: "fake",
		},
	}
}

func TestCopyTaskCandidate(t *testing.T) {
	unittest.SmallTest(t)
	v := fullTaskCandidate()
	cp := v.CopyNoDiagnostics()
	require.Nil(t, cp.Diagnostics)
	cp.Diagnostics = &taskCandidateDiagnostics{}
	assertdeep.Copy(t, v, cp)
}

func TestTaskCandidate_EncodedToAndFromJSON_BeforeEqualsAfter(t *testing.T) {
	unittest.SmallTest(t)
	v := fullTaskCandidate()
	jsonB, err := json.Marshal(v)
	require.NoError(t, err)
	var reEncoded TaskCandidate
	err = json.Unmarshal(jsonB, &reEncoded)
	require.NoError(t, err)
	assert.Equal(t, v, &reEncoded)
}

func TestTaskCandidateId(t *testing.T) {
	unittest.SmallTest(t)
	t1 := makeTaskCandidate("task1", []string{"k:v"})
	t1.Repo = "Myrepo"
	t1.Revision = "abc123"
	t1.ForcedJobId = "someID"
	id1 := t1.MakeId()
	k1, err := parseId(id1)
	require.NoError(t, err)
	require.Equal(t, t1.TaskKey, k1)

	// ForcedJobId is allowed to be empty.
	t1.ForcedJobId = ""
	id1 = t1.MakeId()
	k1, err = parseId(id1)
	require.NoError(t, err)
	require.Equal(t, t1.TaskKey, k1)

	// Test a try job.
	t1.Server = "https://my-patch.com"
	t1.Issue = "10101"
	t1.Patchset = "42"
	id1 = t1.MakeId()
	k1, err = parseId(id1)
	require.NoError(t, err)
	require.Equal(t, t1.TaskKey, k1)

	badIds := []string{
		"",
		"taskCandidate|a",
		"taskCandidate|a|b||ab",
		"20160831T000018.497703717Z_000000000000015b",
	}
	for _, id := range badIds {
		_, err := parseId(id)
		require.Error(t, err)
	}
}

func TestReplaceVar(t *testing.T) {
	unittest.SmallTest(t)
	c := makeTaskCandidate("c", []string{"k:v"})
	c.Repo = "my-repo"
	c.Revision = "abc123"
	c.Name = "my-task"
	dummyId := "id123"
	require.Equal(t, "", replaceVars(c, "", dummyId))
	require.Equal(t, "my-repo", replaceVars(c, "<(REPO)", dummyId))
	require.Equal(t, "my-task", replaceVars(c, "<(TASK_NAME)", dummyId))
	require.Equal(t, "abc123", replaceVars(c, "<(REVISION)", dummyId))
	require.Equal(t, "<(REVISION", replaceVars(c, "<(REVISION", dummyId))
	require.Equal(t, "my-repo_my-task_abc123", replaceVars(c, "<(REPO)_<(TASK_NAME)_<(REVISION)", dummyId))
	require.Equal(t, dummyId, replaceVars(c, "<(TASK_ID)", dummyId))
	require.Equal(t, "", replaceVars(c, "<(PATCH_REF)", dummyId))

	c.Issue = "12345"
	c.Patchset = "3"
	c.Server = "https://server"
	require.Equal(t, "refs/changes/45/12345/3", replaceVars(c, "<(PATCH_REF)", dummyId))
}

func TestTaskCandidateJobs(t *testing.T) {
	unittest.SmallTest(t)

	c := TaskCandidate{}
	now := time.Now().UTC()
	j1 := &types.Job{
		Created: now,
		Id:      "j1",
	}
	j2 := &types.Job{
		Created: now,
		Id:      "j2",
	}
	j3 := &types.Job{
		Created: now.Add(time.Second),
		Id:      "j3",
	}
	j4 := &types.Job{
		Created: now.Add(2 * time.Second),
		Id:      "j4",
	}

	for _, j := range []*types.Job{j1, j2, j3, j4} {
		require.False(t, c.HasJob(j))
	}

	c.AddJob(j3)
	require.Len(t, c.Jobs, 1)
	require.False(t, c.HasJob(j1))
	require.False(t, c.HasJob(j2))
	require.True(t, c.HasJob(j3))
	require.False(t, c.HasJob(j4))

	c.AddJob(j1)
	require.Len(t, c.Jobs, 2)
	require.True(t, c.HasJob(j1))
	require.False(t, c.HasJob(j2))
	require.True(t, c.HasJob(j3))
	require.False(t, c.HasJob(j4))
	require.Equal(t, []*types.Job{j1, j3}, c.Jobs)

	c.AddJob(j4)
	require.Len(t, c.Jobs, 3)
	require.True(t, c.HasJob(j1))
	require.False(t, c.HasJob(j2))
	require.True(t, c.HasJob(j3))
	require.True(t, c.HasJob(j4))
	require.Equal(t, []*types.Job{j1, j3, j4}, c.Jobs)

	c.AddJob(j2)
	require.Len(t, c.Jobs, 4)
	require.True(t, c.HasJob(j1))
	require.True(t, c.HasJob(j2))
	require.True(t, c.HasJob(j3))
	require.True(t, c.HasJob(j4))
	// Order is deterministic.
	require.Equal(t, []*types.Job{j1, j2, j3, j4}, c.Jobs)

	c.AddJob(j4)
	require.Len(t, c.Jobs, 4)
	c.AddJob(j2)
	require.Len(t, c.Jobs, 4)
	c.AddJob(j1)
	require.Len(t, c.Jobs, 4)

	for i := 5; i < 100; i++ {
		c.AddJob(&types.Job{
			Created: now.Add(time.Duration(i*23%7) * time.Second),
			Id:      fmt.Sprintf("j%d", i),
		})
	}
	last := time.Time{}
	for _, j := range c.Jobs {
		require.True(t, !last.After(j.Created))
		last = j.Created
	}
}
